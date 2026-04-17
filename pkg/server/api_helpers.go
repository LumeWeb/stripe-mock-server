package server

import (
	"encoding/json"

	"go.lumeweb.com/stripe-mock-server/pkg/expander"
	"go.lumeweb.com/stripe-mock-server/pkg/gateway"
	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
)

// webhookCustomer wraps *api.Customer for use as APIObject
type webhookCustomer struct {
	customer *api.Customer
	gw       *gateway.Gateway
}

func (w *webhookCustomer) GetID() string     { return w.customer.Id }
func (w *webhookCustomer) GetObject() string { return string(w.customer.Object) }
func (w *webhookCustomer) MarshalJSON() ([]byte, error) {
	customerCopy := *w.customer
	expander.MustExpand(&customerCopy, w.gw)
	return json.Marshal(&customerCopy)
}

// webhookProduct wraps *api.Product for use as APIObject
type webhookProduct struct {
	product *api.Product
	gw      *gateway.Gateway
}

func (w *webhookProduct) GetID() string     { return w.product.Id }
func (w *webhookProduct) GetObject() string { return string(w.product.Object) }
func (w *webhookProduct) MarshalJSON() ([]byte, error) {
	productCopy := *w.product
	expander.MustExpand(&productCopy, w.gw)
	return json.Marshal(&productCopy)
}

// webhookPrice wraps *api.Price for use as APIObject
type webhookPrice struct {
	price *api.Price
	gw    *gateway.Gateway
}

func (w *webhookPrice) GetID() string     { return w.price.Id }
func (w *webhookPrice) GetObject() string { return string(w.price.Object) }
func (w *webhookPrice) MarshalJSON() ([]byte, error) {
	priceCopy := *w.price
	expander.MustExpand(&priceCopy, w.gw)
	return json.Marshal(&priceCopy)
}

// webhookCheckoutSession wraps *api.CheckoutSession for use as APIObject
type webhookCheckoutSession struct {
	session *api.CheckoutSession
	gw      *gateway.Gateway
}

func (w *webhookCheckoutSession) GetID() string     { return w.session.Id }
func (w *webhookCheckoutSession) GetObject() string { return "checkout.session" }
func (w *webhookCheckoutSession) MarshalJSON() ([]byte, error) {
	// Make a copy to avoid racing with the original resource
	sessionCopy := *w.session
	expander.MustExpand(&sessionCopy, w.gw)
	return json.Marshal(&sessionCopy)
}

// webhookSubscription wraps *api.Subscription for use as APIObject
type webhookSubscription struct {
	subscription *api.Subscription
	gw           *gateway.Gateway
}

func (w *webhookSubscription) GetID() string     { return w.subscription.Id }
func (w *webhookSubscription) GetObject() string { return "subscription" }
func (w *webhookSubscription) MarshalJSON() ([]byte, error) {
	subCopy := *w.subscription
	expander.MustExpand(&subCopy, w.gw)
	return json.Marshal(&subCopy)
}

// webhookInvoice wraps *api.Invoice for use as APIObject
type webhookInvoice struct {
	invoice *api.Invoice
	gw      *gateway.Gateway
}

func (w *webhookInvoice) GetID() string     { return w.invoice.Id }
func (w *webhookInvoice) GetObject() string { return "invoice" }
func (w *webhookInvoice) MarshalJSON() ([]byte, error) {
	invoiceCopy := *w.invoice
	expander.MustExpand(&invoiceCopy, w.gw)
	return json.Marshal(&invoiceCopy)
}

// webhookCharge wraps *api.Charge for use as APIObject
type webhookCharge struct {
	charge *api.Charge
	gw     *gateway.Gateway
}

func (w *webhookCharge) GetID() string     { return w.charge.Id }
func (w *webhookCharge) GetObject() string { return "charge" }
func (w *webhookCharge) MarshalJSON() ([]byte, error) {
	chargeCopy := *w.charge
	expander.MustExpand(&chargeCopy, w.gw)
	return json.Marshal(&chargeCopy)
}

// Constructor functions

func newWebhookCustomer(c *api.Customer, gw *gateway.Gateway) *webhookCustomer {
	return &webhookCustomer{customer: c, gw: gw}
}

func newWebhookProduct(p *api.Product, gw *gateway.Gateway) *webhookProduct {
	return &webhookProduct{product: p, gw: gw}
}

func newWebhookPrice(p *api.Price, gw *gateway.Gateway) *webhookPrice {
	return &webhookPrice{price: p, gw: gw}
}

func newWebhookCheckoutSession(s *api.CheckoutSession, gw *gateway.Gateway) *webhookCheckoutSession {
	return &webhookCheckoutSession{session: s, gw: gw}
}

func newWebhookSubscription(s *api.Subscription, gw *gateway.Gateway) *webhookSubscription {
	return &webhookSubscription{subscription: s, gw: gw}
}

func newWebhookInvoice(i *api.Invoice, gw *gateway.Gateway) *webhookInvoice {
	return &webhookInvoice{invoice: i, gw: gw}
}

func newWebhookCharge(c *api.Charge, gw *gateway.Gateway) *webhookCharge {
	return &webhookCharge{charge: c, gw: gw}
}
