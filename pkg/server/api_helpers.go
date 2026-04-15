package server

import (
	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
)

// webhookCustomer wraps *api.Customer for use as APIObject
type webhookCustomer struct {
	customer *api.Customer
}
func (w *webhookCustomer) GetID() string { return w.customer.Id }
func (w *webhookCustomer) GetObject() string { return string(w.customer.Object) }

// webhookProduct wraps *api.Product for use as APIObject
type webhookProduct struct {
	product *api.Product
}
func (w *webhookProduct) GetID() string { return w.product.Id }
func (w *webhookProduct) GetObject() string { return string(w.product.Object) }

// webhookPrice wraps *api.Price for use as APIObject
type webhookPrice struct {
	price *api.Price
}
func (w *webhookPrice) GetID() string { return w.price.Id }
func (w *webhookPrice) GetObject() string { return string(w.price.Object) }

// newWebhookCustomer creates a webhookCustomer wrapper
func newWebhookCustomer(c *api.Customer) *webhookCustomer {
	return &webhookCustomer{customer: c}
}

// newWebhookProduct creates a webhookProduct wrapper
func newWebhookProduct(p *api.Product) *webhookProduct {
	return &webhookProduct{product: p}
}

// webhookCheckoutSession wraps *api.CheckoutSession for use as APIObject
type webhookCheckoutSession struct {
	session *api.CheckoutSession
}
func (w *webhookCheckoutSession) GetID() string { return w.session.Id }
func (w *webhookCheckoutSession) GetObject() string { return "checkout.session" }

// newWebhookPrice creates a webhookPrice wrapper
func newWebhookPrice(p *api.Price) *webhookPrice {
	return &webhookPrice{price: p}
}

// newWebhookCheckoutSession creates a webhookCheckoutSession wrapper
func newWebhookCheckoutSession(s *api.CheckoutSession) *webhookCheckoutSession {
	return &webhookCheckoutSession{session: s}
}

// webhookSubscription wraps *api.Subscription for use as APIObject
type webhookSubscription struct {
	subscription *api.Subscription
}
func (w *webhookSubscription) GetID() string { return w.subscription.Id }
func (w *webhookSubscription) GetObject() string { return "subscription" }

// newWebhookSubscription creates a webhookSubscription wrapper
func newWebhookSubscription(s *api.Subscription) *webhookSubscription {
	return &webhookSubscription{subscription: s}
}

// webhookInvoice wraps *api.Invoice for use as APIObject
type webhookInvoice struct {
	invoice *api.Invoice
}
func (w *webhookInvoice) GetID() string { return w.invoice.Id }
func (w *webhookInvoice) GetObject() string { return "invoice" }

// newWebhookInvoice creates a webhookInvoice wrapper
func newWebhookInvoice(i *api.Invoice) *webhookInvoice {
	return &webhookInvoice{invoice: i}
}

// webhookCharge wraps *api.Charge for use as APIObject
type webhookCharge struct {
	charge *api.Charge
}
func (w *webhookCharge) GetID() string { return w.charge.Id }
func (w *webhookCharge) GetObject() string { return "charge" }

// newWebhookCharge creates a webhookCharge wrapper
func newWebhookCharge(c *api.Charge) *webhookCharge {
	return &webhookCharge{charge: c}
}