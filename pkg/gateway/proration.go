package gateway

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
)

// Cadence represents a billing frequency.
// Matches Stripe's interval values: day, week, month, year.
type Cadence string

const (
	CadenceDaily   Cadence = "day"
	CadenceWeekly  Cadence = "week"
	CadenceMonthly Cadence = "month"
	CadenceYearly  Cadence = "year"
)

// ProrationBehavior represents how proration is handled for subscription changes.
// Matches Stripe's proration_behavior parameter for subscription updates.
type ProrationBehavior string

const (
	// ProrationBehaviorNone disables proration. No credits are created for unused time,
	// and the customer is charged full amounts immediately. For cross-cadence changes,
	// this means no credit for unused old plan time but full charge for new plan.
	ProrationBehaviorNone ProrationBehavior = "none"

	// ProrationBehaviorCreateProrations creates proration items when applicable.
	// Proration items are added to the next invoice (or immediately for cross-cadence changes).
	// This is the default Stripe behavior.
	ProrationBehaviorCreateProrations ProrationBehavior = "create_prorations"

	// ProrationBehaviorAlwaysInvoice creates prorations and immediately invoices the customer,
	// attempting payment collection regardless of billing cycle requirements.
	ProrationBehaviorAlwaysInvoice ProrationBehavior = "always_invoice"
)

// ProrationResult represents the calculation of credits and charges when changing plans mid-cycle.
// Matches portal-plugin-billing/pkg/subscription ProrationResult.
type ProrationResult struct {
	// UnusedCredit is the amount of unused credit from the previous plan.
	UnusedCredit decimal.Decimal
	// NewCharge is the total charge for the new plan prorated to the remaining cycle time.
	NewCharge decimal.Decimal
	// CreditDue is the net amount owed after accounting unused credit against new charges.
	// Positive = customer owes money, Negative = customer has credit balance.
	CreditDue decimal.Decimal
	// EffectiveDate is when the proration takes effect.
	EffectiveDate time.Time
}

// ProrationAmountByTime calculates a prorated amount based on time precision.
//
// This function uses the exact time remaining in a billing cycle to calculate the prorated
// amount, matching Stripe's behavior which calculates prorations to the second. This is
// more accurate than day-based calculations because it accounts for partial days.
//
// The formula used is: amount * (remainingDuration / totalDuration)
func ProrationAmountByTime(amount decimal.Decimal, totalDuration, remainingDuration time.Duration) decimal.Decimal {
	if totalDuration == 0 || remainingDuration == 0 {
		return decimal.Zero
	}
	ratio := decimal.NewFromInt(int64(remainingDuration)).Div(decimal.NewFromInt(int64(totalDuration)))
	return amount.Mul(ratio)
}

// ProrationTimeRatio calculates the precise ratio of elapsed time to total cycle duration.
//
// This function uses exact time differences (seconds) rather than truncated whole days,
// matching Stripe's proration behavior which calculates to the second precision.
func ProrationTimeRatio(elapsed, totalDuration time.Duration) float64 {
	if totalDuration == 0 {
		return 0
	}
	return float64(elapsed) / float64(totalDuration)
}

// CalculateProration calculates proration for a plan change mid-cycle.
//
// This implementation follows Stripe's calendar-accurate proration model:
//
// Same Cadence (e.g., monthly → monthly):
//   - Credits: Time-prorated old unused time using exact time precision
//   - Charges: Time-prorated new plan rate for remaining time
//   - Preserves original billing cycle anchor
//
// Different Cadence (e.g., monthly → yearly):
//   - Credits: Time-prorated old unused time using exact time precision
//   - Charges: FULL new plan amount immediately (not time-prorated by remaining time)
//   - New billing cycle starts at upgrade time
//
// Invoice Line Items (Not Customer Balance):
//   - Credits are invoice line items (negative amounts)
//   - Charges are invoice line items (positive amounts)
//   - Credits offset charges directly on invoice total
//   - Only unused credits after offset go to customer.balance
//
// Proration Behavior:
//   - None: No credits, full charges (if applicable)
//   - CreateProrations: Credits + charges as appropriate
//   - AlwaysInvoice: Credits + charges + immediate invoice
func CalculateProration(
	now time.Time,
	oldPriceUnitAmount int64,
	newPriceUnitAmount int64,
	oldCurrentPeriodStart int64,
	oldCurrentPeriodEnd int64,
	oldCadence Cadence,
	newCadence Cadence,
	prorationBehavior ProrationBehavior,
) (*ProrationResult, error) {
	// Convert API timestamps to time.Time
	oldStart := time.Unix(oldCurrentPeriodStart, 0)
	oldEnd := time.Unix(oldCurrentPeriodEnd, 0)

	// Validate timestamps
	if oldStart.IsZero() || oldEnd.IsZero() {
		return nil, fmt.Errorf("billing cycle must have valid start and end dates")
	}
	if oldStart.After(oldEnd) {
		return nil, fmt.Errorf("billing cycle start date cannot be after end date")
	}

	// Check if now is within cycle
	if now.Before(oldStart) || now.After(oldEnd) {
		return nil, fmt.Errorf("proration date must be within billing cycle")
	}

	// Calculate durations using time.Duration for precision
	cycleDuration := oldEnd.Sub(oldStart)
	remainingDuration := oldEnd.Sub(now)

	if cycleDuration == 0 {
		return nil, fmt.Errorf("billing cycle duration cannot be zero")
	}

	// Create price amounts
	oldAmount := decimal.NewFromInt(oldPriceUnitAmount)
	newAmount := decimal.NewFromInt(newPriceUnitAmount)

	// Determine if same cadence
	sameCadence := oldCadence == newCadence

	// Dispatch to appropriate proration function
	if sameCadence {
		result := sameCadenceProration(oldAmount, newAmount, cycleDuration, remainingDuration, now, prorationBehavior)
		return &result, nil
	}
	result := crossCadenceProration(oldAmount, newAmount, cycleDuration, remainingDuration, now, prorationBehavior)
	return &result, nil
}

// sameCadenceProration handles plan changes where old and new cadences match.
//
// Calculates proration using time precision (seconds) to match Stripe's calendar-accurate proration.
// Preserves original billing cycle anchor.
//
// With ProrationBehaviorNone:
//   - No credit for unused time
//   - No immediate charge (charges at next cycle)
//
// With CreateProrations/AlwaysInvoice:
//   - Credit: Time-prorated old unused time using exact time remaining
//   - Charge: Time-prorated new plan rate for remaining time
func sameCadenceProration(oldAmount, newAmount decimal.Decimal, cycleDuration, remainingDuration time.Duration, now time.Time, behavior ProrationBehavior) ProrationResult {
	if behavior == ProrationBehaviorNone {
		// No proration - customer pays full new rate at next cycle
		// No immediate charge, no credit issued
		return ProrationResult{
			UnusedCredit:  decimal.Zero,
			NewCharge:     decimal.Zero,
			CreditDue:     decimal.Zero,
			EffectiveDate: now,
		}
	}

	// Ensure exact zero when remaining time is zero (avoid decimal precision issues)
	if remainingDuration <= 0 {
		return ProrationResultFromComponents(decimal.Zero, decimal.Zero, now)
	}

	unusedCredit := ProrationAmountByTime(oldAmount, cycleDuration, remainingDuration)
	newCharge := ProrationAmountByTime(newAmount, cycleDuration, remainingDuration)

	return ProrationResultFromComponents(unusedCredit, newCharge, now)
}

// crossCadenceProration handles plan changes where old and new cadences differ.
//
// This function implements Stripe's behavior for interval changes:
//   - Credit: Time-prorated old unused time at the old rate using exact time remaining
//   - Charge: FULL new plan amount immediately (not time-prorated by remaining time)
//   - Billing date resets to NOW
//
// With ProrationBehaviorNone:
//   - No credit for unused time
//   - Full new charge immediately
//
// With CreateProrations/AlwaysInvoice:
//   - Credit calculated and applied to same invoice
//   - Full new charge on same invoice
func crossCadenceProration(oldAmount, newAmount decimal.Decimal, cycleDuration, remainingDuration time.Duration, now time.Time, behavior ProrationBehavior) ProrationResult {
	// Handle ProrationBehaviorNone - no credit for unused time
	if behavior == ProrationBehaviorNone {
		return ProrationResult{
			UnusedCredit:  decimal.Zero,
			NewCharge:     newAmount,
			CreditDue:     newAmount,
			EffectiveDate: now,
		}
	}

	// Ensure exact zero when remaining time is zero
	if remainingDuration <= 0 {
		return ProrationResultFromComponents(decimal.Zero, newAmount, now)
	}

	// Calculate credit using exact time precision
	unusedCredit := ProrationAmountByTime(oldAmount, cycleDuration, remainingDuration)

	// Charge full new amount immediately (not prorated by time)
	newCharge := newAmount

	return ProrationResultFromComponents(unusedCredit, newCharge, now)
}

// ProrationResultFromComponents constructs a proration result from its component parts.
//
// Note: Stripe creates invoice line items directly (negative amounts for credits,
// positive for charges) on the same invoice. Credits offset charges directly on the
// invoice total, and only unused credits after offset go to customer balance.
func ProrationResultFromComponents(unusedCredit, newCharge decimal.Decimal, effectiveDate time.Time) ProrationResult {
	return ProrationResult{
		UnusedCredit:  unusedCredit,
		NewCharge:     newCharge,
		CreditDue:     newCharge.Sub(unusedCredit),
		EffectiveDate: effectiveDate,
	}
}

// ShouldCharge determines whether a charge should be applied based on the proration result.
func ShouldCharge(result ProrationResult) bool {
	return result.NewCharge.GreaterThan(decimal.Zero)
}

// ShouldIssueCredit determines whether a credit should be issued based on the proration result.
func ShouldIssueCredit(result ProrationResult) bool {
	return result.UnusedCredit.GreaterThan(decimal.Zero)
}

// ExtractCadenceFromPrice extracts the billing cadence from a Price object.
// Returns the interval (day, week, month, year) from the Price's Recurring field.
func ExtractCadenceFromPrice(price *api.Price) Cadence {
	if price == nil || price.Recurring == nil {
		return CadenceMonthly // Default to monthly
	}

	// Use the AsRecurring method to extract the recurring data
	recurring, err := price.Recurring.AsRecurring()
	if err != nil {
		return CadenceMonthly
	}

	switch string(recurring.Interval) {
	case "day":
		return CadenceDaily
	case "week":
		return CadenceWeekly
	case "month":
		return CadenceMonthly
	case "year":
		return CadenceYearly
	}

	return CadenceMonthly
}
