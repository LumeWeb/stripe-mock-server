package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
	"go.lumeweb.com/stripe-mock-server/pkg/storage"
)

// Gateway provides business logic for Stripe-like operations
type Gateway struct {
	customerRepo     storage.Repository[api.Customer]
	subscriptionRepo storage.Repository[api.Subscription]
	invoiceRepo      storage.Repository[api.Invoice]
	chargeRepo       storage.Repository[api.Charge]
	sessionRepo      storage.Repository[api.CheckoutSession]
	productRepo      storage.Repository[api.Product]
	priceRepo        storage.Repository[api.Price]
	billingConfigRepo storage.Repository[api.BillingPortalConfiguration]
	webhookRepo      storage.Repository[api.WebhookEndpoint]
}

// NewGateway creates a new gateway with typed repositories
func NewGateway() *Gateway {
	return &Gateway{
		customerRepo:     storage.NewInMemoryRepository[api.Customer](),
		subscriptionRepo: storage.NewInMemoryRepository[api.Subscription](),
		invoiceRepo:      storage.NewInMemoryRepository[api.Invoice](),
		chargeRepo:       storage.NewInMemoryRepository[api.Charge](),
		sessionRepo:      storage.NewInMemoryRepository[api.CheckoutSession](),
		productRepo:      storage.NewInMemoryRepository[api.Product](),
		priceRepo:        storage.NewInMemoryRepository[api.Price](),
		billingConfigRepo: storage.NewInMemoryRepository[api.BillingPortalConfiguration](),
		webhookRepo:      storage.NewInMemoryRepository[api.WebhookEndpoint](),
	}
}

// SetWebhookRepo sets the webhook repository (for testing)
func (g *Gateway) SetWebhookRepo(repo storage.Repository[api.WebhookEndpoint]) {
	g.webhookRepo = repo
}

// Helper to set standard fields
func setStandardFields(obj any) {
	now := int(time.Now().Unix())
	switch v := obj.(type) {
	case *api.Customer:
		if v.Created == 0 {
			v.Created = now
		}
		if v.Object == "" {
			v.Object = "customer"
		}
	case *api.Product:
		if v.Created == 0 {
			v.Created = now
		}
		if v.Object == "" {
			v.Object = "product"
		}
	case *api.Price:
		if v.Created == 0 {
			v.Created = now
		}
		if v.Object == "" {
			v.Object = "price"
		}
	case *api.Subscription:
		if v.Created == 0 {
			v.Created = now
		}
		if v.Object == "" {
			v.Object = "subscription"
		}
	case *api.CheckoutSession:
		if v.Created == 0 {
			v.Created = now
		}
		if v.Object == "" {
			v.Object = "checkout.session"
		}
	case *api.Invoice:
		if v.Created == 0 {
			v.Created = now
		}
		if v.Object == "" {
			v.Object = "invoice"
		}
	case *api.Charge:
		if v.Created == 0 {
			v.Created = now
		}
		if v.Object == "" {
			v.Object = "charge"
		}
	case *api.BillingPortalConfiguration:
		if v.Created == 0 {
			v.Created = now
		}
		if v.Object == "" {
			v.Object = "billing_portal_configuration"
		}
	}
}

// Customer operations

func (g *Gateway) CreateCustomer(customer *api.Customer) (*api.Customer, error) {
	if customer == nil {
		return nil, fmt.Errorf("customer cannot be nil")
	}
	setStandardFields(customer)
	id, err := g.customerRepo.Create(*customer)
	if err != nil {
		return nil, err
	}
	return g.customerRepo.Get(id)
}

func (g *Gateway) GetCustomer(id string) (*api.Customer, error) {
	return g.customerRepo.Get(id)
}

func (g *Gateway) UpdateCustomer(id string, customer *api.Customer) error {
	return g.customerRepo.Update(id, *customer)
}

func (g *Gateway) ListCustomers() ([]api.Customer, error) {
	return g.customerRepo.List()
}

// Product operations

func (g *Gateway) CreateProduct(product *api.Product) (*api.Product, error) {
	if product == nil {
		return nil, fmt.Errorf("product cannot be nil")
	}
	setStandardFields(product)
	id, err := g.productRepo.Create(*product)
	if err != nil {
		return nil, err
	}
	return g.productRepo.Get(id)
}

// UpdateProduct updates an existing product
func (g *Gateway) UpdateProduct(id string, product *api.Product) error {
	return g.productRepo.Update(id, *product)
}

func (g *Gateway) GetProduct(id string) (*api.Product, error) {
	return g.productRepo.Get(id)
}

func (g *Gateway) ListProducts() ([]api.Product, error) {
	return g.productRepo.List()
}

// Price operations

func (g *Gateway) CreatePrice(price *api.Price) (*api.Price, error) {
	if price == nil {
		return nil, fmt.Errorf("price cannot be nil")
	}
	setStandardFields(price)
	id, err := g.priceRepo.Create(*price)
	if err != nil {
		return nil, err
	}
	return g.priceRepo.Get(id)
}

func (g *Gateway) GetPrice(id string) (*api.Price, error) {
	return g.priceRepo.Get(id)
}

func (g *Gateway) ListPrices() ([]api.Price, error) {
	return g.priceRepo.List()
}

// Checkout Session operations

func (g *Gateway) CreateCheckoutSession(session *api.CheckoutSession) (*api.CheckoutSession, error) {
	if session == nil {
		return nil, fmt.Errorf("session cannot be nil")
	}
	setStandardFields(session)
	id, err := g.sessionRepo.Create(*session)
	if err != nil {
		return nil, err
	}
	return g.sessionRepo.Get(id)
}

func (g *Gateway) GetCheckoutSession(id string) (*api.CheckoutSession, error) {
	return g.sessionRepo.Get(id)
}

func (g *Gateway) ListCheckoutSessions() ([]api.CheckoutSession, error) {
	return g.sessionRepo.List()
}

func (g *Gateway) CompleteCheckoutSession(id string) (*api.CheckoutSession, error) {
	// Get existing session
	session, err := g.GetCheckoutSession(id)
	if err != nil {
		return nil, err
	}

	// Validate session is in 'open' status
	if *session.Status != "open" {
		return nil, fmt.Errorf("session must be in 'open' status to complete")
	}

	// Update session status to 'complete'
	*session.Status = api.CheckoutSessionStatusComplete
	session.PaymentStatus = "paid"

	// If mode is subscription, create a subscription and link it
	if session.Mode == api.CheckoutSessionModeEnumSubscription {
		// Extract customer ID if present
		var customerID string
		if session.Customer != nil {
			customerID, _ = session.Customer.AsCheckoutSessionCustomer0()
		}

		// Create subscription
		sub := &api.Subscription{
			Object:   api.SubscriptionObjectEnumSubscription,
			Status:   api.SubscriptionStatusActive,
			Livemode: session.Livemode,
		}
		if customerID != "" {
			var custUnion api.Subscription_Customer
			custUnion.FromSubscriptionCustomer0(customerID)
			sub.Customer = custUnion
		}

		createdSub, err := g.CreateSubscription(sub)
		if err == nil {
			// Link subscription to session
			var subUnion api.CheckoutSession_Subscription
			subUnion.FromCheckoutSessionSubscription0(createdSub.Id)
			session.Subscription = &subUnion
		}
	}

	// Save updated session
	err = g.sessionRepo.Update(id, *session)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// Subscription operations

func (g *Gateway) CreateSubscription(sub *api.Subscription) (*api.Subscription, error) {
	if sub == nil {
		return nil, fmt.Errorf("subscription cannot be nil")
	}
	setStandardFields(sub)
	if sub.LatestInvoice == nil {
		sub.LatestInvoice = &api.Subscription_LatestInvoice{}
	}
	id, err := g.subscriptionRepo.Create(*sub)
	if err != nil {
		return nil, err
	}
	return g.subscriptionRepo.Get(id)
}

func (g *Gateway) GetSubscription(id string) (*api.Subscription, error) {
	return g.subscriptionRepo.Get(id)
}

func (g *Gateway) ListSubscriptions() ([]api.Subscription, error) {
	return g.subscriptionRepo.List()
}

func (g *Gateway) UpdateSubscription(id string, subscription *api.Subscription) error {
	return g.subscriptionRepo.Update(id, *subscription)
}

// CreateSubscriptionWithInvoice creates a subscription (invoice creation handled separately for now)
func (g *Gateway) CreateSubscriptionWithInvoice(sub *api.Subscription, amount int64, currency string) (*api.Subscription, *api.Invoice, error) {
	if sub == nil {
		return nil, nil, fmt.Errorf("subscription cannot be nil")
	}
	
	setStandardFields(sub)
	// For testing purposes: use a simple string "status" field directly
	sub.Status = api.SubscriptionStatusIncomplete
	if sub.LatestInvoice == nil {
		sub.LatestInvoice = &api.Subscription_LatestInvoice{}
	}
	
	// Create subscription
	id, err := g.subscriptionRepo.Create(*sub)
	if err != nil {
		return nil, nil, err
	}
	
	// For now, return subscription without creating invoice (invoice creation complex)
	existingSub, err := g.GetSubscription(id)
	if err != nil {
		return nil, nil, err
	}
	return existingSub, nil, nil
}

// UpdateSubscription updates subscription status for webhook sequence
func (g *Gateway) UpdateSubscriptionStatus(id string, status string) (*api.Subscription, error) {
	// Get existing subscription with both return values as they're used
	sub, err := g.GetSubscription(id)
	if err != nil {
		return nil, err
	}
	
	// Update status field
	sub.Status = api.SubscriptionStatus(status)
	
	// Save updated subscription
	err = g.subscriptionRepo.Update(id, *sub)
	if err != nil {
		return nil, err
	}
	
	return sub, nil
}

// CancelSubscription immediately cancels a subscription.
// Sets status to canceled and marks it as deleted.
// Returns the canceled subscription.
func (g *Gateway) CancelSubscription(id string) (*api.Subscription, error) {
	sub, err := g.GetSubscription(id)
	if err != nil {
		return nil, err
	}

	// Set status to canceled
	sub.Status = api.SubscriptionStatusCanceled
	
	// Set canceled_at timestamp
	canceledAt := int(time.Now().Unix())
	sub.CanceledAt = &canceledAt
	
	// Mark as canceled at period end (immediate, so true)
	sub.CancelAtPeriodEnd = true

	// Save updated subscription
	if err := g.subscriptionRepo.Update(id, *sub); err != nil {
		return nil, err
	}

	return g.GetSubscription(id)
}

// SetCancelAtPeriodEnd schedules a subscription for cancellation at the end of the current period.
// This creates a "grace period" where the subscription remains active but will cancel.
// Returns the updated subscription.
func (g *Gateway) SetCancelAtPeriodEnd(id string) (*api.Subscription, error) {
	sub, err := g.GetSubscription(id)
	if err != nil {
		return nil, err
	}

	// Validate subscription is active
	if sub.Status != api.SubscriptionStatusActive {
		return nil, fmt.Errorf("subscription must be active to schedule cancellation")
	}

	// Set cancel_at_period_end flag
	sub.CancelAtPeriodEnd = true
	
	// Set canceled_at timestamp (when cancellation was requested)
	canceledAt := int(time.Now().Unix())
	sub.CanceledAt = &canceledAt

	// Save updated subscription
	if err := g.subscriptionRepo.Update(id, *sub); err != nil {
		return nil, err
	}

	return g.GetSubscription(id)
}

// ExpireCanceledSubscription finalizes a subscription that was scheduled for cancellation.
// Called when current_period_end is reached for a subscription with cancel_at_period_end=true.
// Returns the deleted subscription.
func (g *Gateway) ExpireCanceledSubscription(id string) (*api.Subscription, error) {
	sub, err := g.GetSubscription(id)
	if err != nil {
		return nil, err
	}

	// Validate subscription has cancel_at_period_end set
	if !sub.CancelAtPeriodEnd {
		return nil, fmt.Errorf("subscription not scheduled for cancellation")
	}

	// Set status to canceled
	sub.Status = api.SubscriptionStatusCanceled
	
	// Clear the flag
	sub.CancelAtPeriodEnd = false

	// Save updated subscription
	if err := g.subscriptionRepo.Update(id, *sub); err != nil {
		return nil, err
	}

	return g.GetSubscription(id)
}

// PauseSubscription pauses an active subscription
func (g *Gateway) PauseSubscription(id string) (*api.Subscription, error) {
	sub, err := g.GetSubscription(id)
	if err != nil {
		return nil, err
	}

	// Validate subscription is active
	if sub.Status != api.SubscriptionStatusActive {
		return nil, fmt.Errorf("cannot pause subscription with status %s", sub.Status)
	}

	// Set status to paused
	sub.Status = api.SubscriptionStatusPaused

	// Save updated subscription
	if err := g.subscriptionRepo.Update(id, *sub); err != nil {
		return nil, err
	}

	return g.GetSubscription(id)
}

// ResumeSubscription resumes a paused subscription
func (g *Gateway) ResumeSubscription(id string) (*api.Subscription, error) {
	sub, err := g.GetSubscription(id)
	if err != nil {
		return nil, err
	}

	// Validate subscription is paused
	if sub.Status != api.SubscriptionStatusPaused {
		return nil, fmt.Errorf("cannot resume subscription with status %s", sub.Status)
	}

	// Set status back to active
	sub.Status = api.SubscriptionStatusActive

	// Save updated subscription
	if err := g.subscriptionRepo.Update(id, *sub); err != nil {
		return nil, err
	}

	return g.GetSubscription(id)
}

// RenewSubscription performs a renewal cycle for an active subscription
// Steps:
// 1. Validate subscription exists and is active
// 2. Calculate next period end based on interval
// 3. Update current_period_start and current_period_end
// 4. Return updated subscription
func (g *Gateway) RenewSubscription(id string) (*api.Subscription, error) {
	// Get existing subscription
	sub, err := g.GetSubscription(id)
	if err != nil {
		return nil, fmt.Errorf("subscription not found: %w", err)
	}
	
	// Validate status - only active subscriptions can renew
	if sub.Status != api.SubscriptionStatusActive {
		return nil, fmt.Errorf("cannot renew subscription with status %s", sub.Status)
	}
	
	// If subscription has items, update their period fields
	if len(sub.Items.Data) > 0 {
		// Get interval from first item (simplified - assumes monthly)
		// In production, would parse subscription.items[0].plan.interval_unit
		periodDuration := int64(30 * 24 * 60 * 60) // 30 days in seconds
		
		// Start of new period becomes old end of previous period
		newPeriodStart := sub.Items.Data[0].CurrentPeriodEnd
		sub.Items.Data[0].CurrentPeriodStart = newPeriodStart
		// Advance the period by the duration (convert int64 to int)
		sub.Items.Data[0].CurrentPeriodEnd = int(newPeriodStart) + int(periodDuration)
	}
	
	// Update subscription in storage
	err = g.subscriptionRepo.Update(id, *sub)
	if err != nil {
		return nil, err
	}
	
	return sub, nil
}

// Invoice operations

func (g *Gateway) CreateInvoice(invoice *api.Invoice) (*api.Invoice, error) {
	if invoice == nil {
		return nil, fmt.Errorf("invoice cannot be nil")
	}
	setStandardFields(invoice)
	id, err := g.invoiceRepo.Create(*invoice)
	if err != nil {
		return nil, err
	}
	return g.invoiceRepo.Get(id)
}

func (g *Gateway) UpdateInvoice(id string, invoice *api.Invoice) error {
	return g.invoiceRepo.Update(id, *invoice)
}

func (g *Gateway) GetInvoice(id string) (*api.Invoice, error) {
	return g.invoiceRepo.Get(id)
}

func (g *Gateway) ListInvoices() ([]api.Invoice, error) {
	return g.invoiceRepo.List()
}

// Charge operations

func (g *Gateway) CreateCharge(charge *api.Charge) (*api.Charge, error) {
	if charge == nil {
		return nil, fmt.Errorf("charge cannot be nil")
	}
	setStandardFields(charge)
	id, err := g.chargeRepo.Create(*charge)
	if err != nil {
		return nil, err
	}
	return g.chargeRepo.Get(id)
}

func (g *Gateway) UpdateCharge(id string, charge *api.Charge) error {
	return g.chargeRepo.Update(id, *charge)
}

func (g *Gateway) ListCharges() ([]api.Charge, error) {
	return g.chargeRepo.List()
}

// Billing Portal Configuration operations (using generated types)

func (g *Gateway) CreateBillingPortalConfiguration(config *api.BillingPortalConfiguration) (*api.BillingPortalConfiguration, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	setStandardFields(config)
	id, err := g.billingConfigRepo.Create(*config)
	if err != nil {
		return nil, err
	}
	return g.billingConfigRepo.Get(id)
}

func (g *Gateway) GetBillingPortalConfiguration(id string) (*api.BillingPortalConfiguration, error) {
	return g.billingConfigRepo.Get(id)
}

func (g *Gateway) ListBillingPortalConfigurations() ([]api.BillingPortalConfiguration, error) {
	return g.billingConfigRepo.List()
}

// Webhook operations

func (g *Gateway) CreateWebhookEndpoint(wbe *api.WebhookEndpoint) (*api.WebhookEndpoint, error) {
	if wbe == nil {
		return nil, fmt.Errorf("webhook endpoint cannot be nil")
	}
	// Webhook endpoints don't need created object fields (already set)
	id, err := g.webhookRepo.Create(*wbe)
	if err != nil {
		return nil, err
	}
	return g.webhookRepo.Get(id)
}

func (g *Gateway) GetWebhookEndpoint(id string) (*api.WebhookEndpoint, error) {
	return g.webhookRepo.Get(id)
}

func (g *Gateway) ListWebhookEndpoints(limit int, startingAfter string) ([]*api.WebhookEndpoint, error) {
	items, err := g.webhookRepo.List()
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	ptrs := make([]*api.WebhookEndpoint, 0, len(items))
	for _, item := range items {
		ptrs = append(ptrs, &item)
	}

	// Apply cursor-based pagination
	if startingAfter != "" {
		startIndex := 0
		for i, ep := range ptrs {
			if ep.Id == startingAfter {
				startIndex = i + 1
				break
			}
		}
		if startIndex < len(ptrs) {
			ptrs = ptrs[startIndex:]
		} else {
			ptrs = nil
		}
	}

	// Apply limit
	if limit > 0 && limit < len(ptrs) {
		ptrs = ptrs[:limit]
	}

	return ptrs, nil
}

func (g *Gateway) UpdateWebhookEndpoint(id string, wbe *api.WebhookEndpoint) (*api.WebhookEndpoint, error) {
	if wbe == nil {
		return nil, fmt.Errorf("webhook endpoint cannot be nil")
	}
	if err := g.webhookRepo.Update(id, *wbe); err != nil {
		return nil, err
	}
	return g.webhookRepo.Get(id)
}

func (g *Gateway) DeleteWebhookEndpoint(id string) error {
	return g.webhookRepo.Delete(id)
}

func (g *Gateway) DeliverWebhookEvent(webhookId string, eventType string, payload map[string]any) (interface{}, error) {
	// Placeholder - integrate with webhook service
	return nil, fmt.Errorf("not implemented")
}

// Reset clears all state except registered webhooks
func (g *Gateway) Reset() error {
	// Clear all repositories except webhooks
	if err := g.customerRepo.Clear(); err != nil {
		return fmt.Errorf("failed to clear customers: %w", err)
	}
	if err := g.subscriptionRepo.Clear(); err != nil {
		return fmt.Errorf("failed to clear subscriptions: %w", err)
	}
	if err := g.invoiceRepo.Clear(); err != nil {
		return fmt.Errorf("failed to clear invoices: %w", err)
	}
	if err := g.chargeRepo.Clear(); err != nil {
		return fmt.Errorf("failed to clear charges: %w", err)
	}
	if err := g.sessionRepo.Clear(); err != nil {
		return fmt.Errorf("failed to clear checkout sessions: %w", err)
	}
	if err := g.productRepo.Clear(); err != nil {
		return fmt.Errorf("failed to clear products: %w", err)
	}
	if err := g.priceRepo.Clear(); err != nil {
		return fmt.Errorf("failed to clear prices: %w", err)
	}
	if err := g.billingConfigRepo.Clear(); err != nil {
		return fmt.Errorf("failed to clear billing configurations: %w", err)
	}
	// Note: webhookRepo is intentionally NOT cleared
	return nil
}

// CustomerExists checks if a customer exists
func (g *Gateway) CustomerExists(id string) bool {
	return g.customerRepo.Exists(id)
}

// ProductExists checks if a product exists
func (g *Gateway) ProductExists(id string) bool {
	return g.productRepo.Exists(id)
}

// PriceExists checks if a price exists
func (g *Gateway) PriceExists(id string) bool {
	return g.priceRepo.Exists(id)
}

// SubscriptionExists checks if a subscription exists
func (g *Gateway) SubscriptionExists(id string) bool {
	return g.subscriptionRepo.Exists(id)
}

// InvoiceExists checks if an invoice exists
func (g *Gateway) InvoiceExists(id string) bool {
	return g.invoiceRepo.Exists(id)
}

// ChargeExists checks if a charge exists
func (g *Gateway) ChargeExists(id string) bool {
	return g.chargeRepo.Exists(id)
}

// CheckoutSessionExists checks if a checkout session exists
func (g *Gateway) CheckoutSessionExists(id string) bool {
	return g.sessionRepo.Exists(id)
}

// BillingPortalConfigurationExists checks if a billing portal configuration exists
func (g *Gateway) BillingPortalConfigurationExists(id string) bool {
	return g.billingConfigRepo.Exists(id)
}

// WebhookEndpointExists checks if a webhook endpoint exists
func (g *Gateway) WebhookEndpointExists(id string) bool {
	return g.webhookRepo.Exists(id)
}

// HandleWebhook processes a webhook event
func (g *Gateway) HandleWebhook(eventData json.RawMessage) error {
	var event map[string]any
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("invalid event data: %w", err)
	}
	// Just log the event for now
	fmt.Printf("Webhook received: %+v\n", event)
	return nil
}
// SubscriptionItemUpdate represents an item update for a subscription
type SubscriptionItemUpdate struct {
	ID        string // existing subscription item ID
	PriceID   string // new price ID (required for update)
	Quantity  int    // new quantity (optional)
	Deleted   bool   // mark for deletion
}

// UpdateSubscriptionItems updates subscription items and calculates proration
func (g *Gateway) UpdateSubscriptionItems(
	subscriptionID string,
	updates []SubscriptionItemUpdate,
	prorationBehavior string,
) (*api.Subscription, *ProrationResult, error) {
	// Get current subscription
	sub, err := g.GetSubscription(subscriptionID)
	if err != nil {
		return nil, nil, err
	}

	// Validate subscription exists and is active
	if sub.Status != api.SubscriptionStatusActive {
		return nil, nil, fmt.Errorf("subscription status must be active")
	}

	// Validate items exist in subscription
	if len(sub.Items.Data) == 0 {
		return nil, nil, fmt.Errorf("subscription has no items to update")
	}

	// Track old and new prices for proration calculation
	var oldPriceID, newPriceID string
	var prorationResult *ProrationResult

	// Build new items slice, filtering out deleted items
	newItems := make([]api.SubscriptionItem, 0, len(sub.Items.Data))
	for _, existingItem := range sub.Items.Data {
		update := findUpdateById(updates, existingItem.Id)
		if update != nil {
			// Item marked for deletion - skip it
			if update.Deleted {
				continue
			}

			// Find new price
			if update.PriceID == "" {
				return nil, nil, fmt.Errorf("price ID required for update")
			}

			// Track price change for proration
			oldPriceID = existingItem.Price.Id
			newPriceID = update.PriceID

			updatedItem := existingItem
			updatedItem.Price.Id = update.PriceID
			updatedItem.Price.Type = api.PriceTypeEnumRecurring
			newItems = append(newItems, updatedItem)
		} else {
			// Item not in update list, keep as is
			newItems = append(newItems, existingItem)
		}
	}

	// Update subscription items
	sub.Items.Data = newItems

	// Calculate proration if we have a price change
	if oldPriceID != "" && newPriceID != "" && prorationBehavior != "none" {
		// Look up the old and new prices
		oldPrice, err := g.GetPrice(oldPriceID)
		if err != nil {
			// If price not found, use default amount
			oldPrice = &api.Price{Id: oldPriceID, UnitAmount: intPtr(1000)}
		}

		newPrice, err := g.GetPrice(newPriceID)
		if err != nil {
			// If price not found, use default amount
			newPrice = &api.Price{Id: newPriceID, UnitAmount: intPtr(2000)}
		}

		// Get amounts (default to 1000 and 2000 cents if not set)
		oldAmount := int64(1000)
		if oldPrice.UnitAmount != nil {
			oldAmount = int64(*oldPrice.UnitAmount)
		}

		newAmount := int64(2000)
		if newPrice.UnitAmount != nil {
			newAmount = int64(*newPrice.UnitAmount)
		}

		// Extract cadences
		oldCadence := ExtractCadenceFromPrice(oldPrice)
		newCadence := ExtractCadenceFromPrice(newPrice)

		// Calculate proration using the first item's period
		if len(sub.Items.Data) > 0 {
			si := sub.Items.Data[0]
			if si.CurrentPeriodStart != 0 && si.CurrentPeriodEnd != 0 {
				prorationResult, err = CalculateProration(
					time.Now(),
					oldAmount,
					newAmount,
					int64(si.CurrentPeriodStart),
					int64(si.CurrentPeriodEnd),
					oldCadence,
					newCadence,
					ProrationBehavior(prorationBehavior),
				)
				if err != nil {
					log.Printf("Proration calculation failed: %v", err)
				}
			}
		}
	}

	// Save subscription
	if err := g.subscriptionRepo.Update(subscriptionID, *sub); err != nil {
		return nil, nil, err
	}

	// Return updated subscription (re-fetch to get latest)
	updated, err := g.subscriptionRepo.Get(subscriptionID)
	if err != nil {
		return nil, nil, err
	}

	return updated, prorationResult, nil
}

func intPtr(i int) *int {
	return &i
}

func findUpdateById(updates []SubscriptionItemUpdate, id string) *SubscriptionItemUpdate {
	for i := range updates {
		if updates[i].ID == id {
			return &updates[i]
		}
	}
	return nil
}
