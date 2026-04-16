package expander

import (
	"time"

	"go.lumeweb.com/stripe-mock-server/pkg/gateway"
	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
)

// InvoiceExpander expands all related objects on an Invoice
type InvoiceExpander struct {
	invoice *api.Invoice
}

// NewInvoiceExpander creates an expander for an Invoice
func NewInvoiceExpander(invoice *api.Invoice) *InvoiceExpander {
	return &InvoiceExpander{invoice: invoice}
}

// ExpandAll expands all related objects on the Invoice
func (e *InvoiceExpander) ExpandAll(gw *gateway.Gateway) error {
	if e.invoice == nil {
		return nil
	}

	e.expandCustomer(gw)

	return nil
}

func (e *InvoiceExpander) expandCustomer(gw *gateway.Gateway) {
	// Try to extract ID from the union field
	id, err := e.invoice.Customer.AsInvoiceCustomer0()
	if err != nil || id == "" {
		return
	}

	// Fetch the full customer
	customer, err := gw.GetCustomer(id)
	if err != nil {
		// Create a minimal customer if not found
		m := map[string]string{}
		customer = &api.Customer{
			Id:       id,
			Object:   api.CustomerObjectEnumCustomer,
			Created:  int(time.Now().Unix()),
			Livemode: false,
			Metadata: &m,
		}
	}

	// Set expanded customer
	e.invoice.Customer.FromCustomer(*customer)
}
