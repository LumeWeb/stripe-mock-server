package expander

import (
	"time"

	"go.lumeweb.com/stripe-mock-server/pkg/gateway"
	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
)

// SubscriptionExpander expands all related objects on a Subscription
type SubscriptionExpander struct {
	sub *api.Subscription
}

// NewSubscriptionExpander creates an expander for a Subscription
func NewSubscriptionExpander(sub *api.Subscription) *SubscriptionExpander {
	return &SubscriptionExpander{sub: sub}
}

// ExpandAll expands all related objects on the Subscription
func (e *SubscriptionExpander) ExpandAll(gw *gateway.Gateway) error {
	if e.sub == nil {
		return nil
	}

	e.expandCustomer(gw)
	e.expandLatestInvoice(gw)
	e.expandItemsPriceProduct(gw)

	return nil
}

func (e *SubscriptionExpander) expandCustomer(gw *gateway.Gateway) {
	// Try to extract ID from the union field
	id, err := e.sub.Customer.AsSubscriptionCustomer0()
	if err != nil || id == "" {
		return
	}

	// Fetch the full customer
	customer, err := gw.GetCustomer(id)
	if err != nil {
		customer = createMinimalCustomer(id)
	}

	// Set expanded customer
	e.sub.Customer.FromCustomer(*customer)
}

func (e *SubscriptionExpander) expandLatestInvoice(gw *gateway.Gateway) {
	if e.sub.LatestInvoice == nil {
		return
	}

	// Try to extract ID from the union field
	id, err := e.sub.LatestInvoice.AsSubscriptionLatestInvoice0()
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
	e.sub.LatestInvoice.FromInvoice(*invoice)
}

func (e *SubscriptionExpander) expandItemsPriceProduct(gw *gateway.Gateway) {
	for i := range e.sub.Items.Data {
		e.expandPriceProduct(&e.sub.Items.Data[i].Price, gw)
	}
}

func (e *SubscriptionExpander) expandPriceProduct(price *api.Price, gw *gateway.Gateway) {
	if price == nil {
		return
	}

	// Try to extract ID from the union field
	id, err := price.Product.AsPriceProduct0()
	if err != nil || id == "" {
		return
	}

	// Fetch the full product
	product, err := gw.GetProduct(id)
	if err != nil {
		// Create a minimal product if not found
		product = &api.Product{
			Id:       id,
			Object:   api.ProductObjectEnumProduct,
			Created:  int(time.Now().Unix()),
			Livemode: true,
			Metadata: map[string]string{},
		}
	}

	// Set expanded product
	price.Product.FromProduct(*product)
}
