package expander

import (
	"go.lumeweb.com/stripe-mock-server/pkg/gateway"
	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
)

// ChargeExpander expands all related objects on a Charge
type ChargeExpander struct {
	charge *api.Charge
}

// NewChargeExpander creates an expander for a Charge
func NewChargeExpander(charge *api.Charge) *ChargeExpander {
	return &ChargeExpander{charge: charge}
}

// ExpandAll expands all related objects on the Charge
func (e *ChargeExpander) ExpandAll(gw *gateway.Gateway) error {
	if e.charge == nil {
		return nil
	}

	e.expandCustomer(gw)

	return nil
}

func (e *ChargeExpander) expandCustomer(gw *gateway.Gateway) {
	if e.charge.Customer == nil {
		return
	}

	// Try to extract ID from the union field
	id, err := e.charge.Customer.AsChargeCustomer0()
	if err != nil || id == "" {
		return
	}

	// Fetch the full customer
	customer, err := gw.GetCustomer(id)
	if err != nil {
		customer = createMinimalCustomer(id)
	}

	// Set expanded customer
	e.charge.Customer.FromCustomer(*customer)
}
