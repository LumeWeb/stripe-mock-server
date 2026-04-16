package expander

import (
	"time"

	"go.lumeweb.com/stripe-mock-server/pkg/gateway"
	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
)

// createMinimalCustomer creates a fallback customer object when gateway lookup fails
func createMinimalCustomer(id string) *api.Customer {
	m := map[string]string{}
	return &api.Customer{
		Id:       id,
		Object:   api.CustomerObjectEnumCustomer,
		Created:  int(time.Now().Unix()),
		Livemode: false,
		Metadata: &m,
	}
}

// ResourceExpander is the interface for resources that can expand their related objects
type ResourceExpander interface {
	ExpandAll(gw *gateway.Gateway) error
}

// Expand expands all related objects for a webhook payload resource
func Expand(resource any, gw *gateway.Gateway) error {
	if gw == nil {
		return nil // No gateway, skip expansion
	}
	switch r := resource.(type) {
	case *api.CheckoutSession:
		return NewCheckoutSessionExpander(r).ExpandAll(gw)
	case *api.Subscription:
		return NewSubscriptionExpander(r).ExpandAll(gw)
	case *api.Invoice:
		return NewInvoiceExpander(r).ExpandAll(gw)
	case *api.Charge:
		return NewChargeExpander(r).ExpandAll(gw)
	case ResourceExpander:
		return r.ExpandAll(gw)
	}
	return nil
}

// MustExpand calls Expand and ignores errors (for use in MarshalJSON where errors can't be returned)
func MustExpand(resource any, gw *gateway.Gateway) {
	_ = Expand(resource, gw)
}
