package expander

import (
	"time"

	"go.lumeweb.com/stripe-mock-server/pkg/gateway"
	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
)

// CheckoutSessionExpander expands all related objects on a CheckoutSession
type CheckoutSessionExpander struct {
	session *api.CheckoutSession
}

// NewCheckoutSessionExpander creates an expander for a CheckoutSession
func NewCheckoutSessionExpander(session *api.CheckoutSession) *CheckoutSessionExpander {
	return &CheckoutSessionExpander{session: session}
}

// ExpandAll expands all related objects on the CheckoutSession
func (e *CheckoutSessionExpander) ExpandAll(gw *gateway.Gateway) error {
	if e.session == nil {
		return nil
	}

	e.expandCustomer(gw)
	e.expandSubscription(gw)
	e.expandInvoice(gw)

	return nil
}

func (e *CheckoutSessionExpander) expandCustomer(gw *gateway.Gateway) {
	if e.session.Customer == nil {
		return
	}

	// Try to extract ID from the union field
	id, err := e.session.Customer.AsCheckoutSessionCustomer0()
	if err != nil || id == "" {
		return
	}

	// Fetch the full customer
	customer, err := gw.GetCustomer(id)
	if err != nil {
		customer = createMinimalCustomer(id)
	}

	// Set expanded customer
	e.session.Customer.FromCustomer(*customer)
}

func (e *CheckoutSessionExpander) expandSubscription(gw *gateway.Gateway) {
	if e.session.Subscription == nil {
		return
	}

	// Try to extract ID from the union field
	id, err := e.session.Subscription.AsCheckoutSessionSubscription0()
	if err != nil || id == "" {
		return
	}

	// Fetch the full subscription
	subscription, err := gw.GetSubscription(id)
	if err != nil {
		// Create a minimal subscription if not found
		subscription = &api.Subscription{
			Id:       id,
			Object:   api.SubscriptionObjectEnumSubscription,
			Created:  int(time.Now().Unix()),
			Livemode: true,
		}
	}

	// Set expanded subscription
	e.session.Subscription.FromSubscription(*subscription)
}

func (e *CheckoutSessionExpander) expandInvoice(gw *gateway.Gateway) {
	if e.session.Invoice == nil {
		return
	}

	// Try to extract ID from the union field
	id, err := e.session.Invoice.AsCheckoutSessionInvoice0()
	if err != nil || id == "" {
		return
	}

	// Fetch the full invoice
	invoice, err := gw.GetInvoice(id)
	if err != nil {
		// Create a minimal invoice if not found
		invoice = &api.Invoice{
			Id:       id,
			Object:   api.InvoiceObjectEnumInvoice,
			Created:  int(time.Now().Unix()),
			Livemode: true,
		}
	}

	// Set expanded invoice
	e.session.Invoice.FromInvoice(*invoice)
}
