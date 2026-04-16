package expander

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/stripe-mock-server/pkg/gateway"
	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
)

func TestExpandNilGateway(t *testing.T) {
	// Should not panic with nil gateway
	session := &api.CheckoutSession{
		Id: "cs_test",
	}
	err := Expand(session, nil)
	assert.NoError(t, err)
}

func TestExpandNilResource(t *testing.T) {
	gw := gateway.NewGateway()
	err := Expand(nil, gw)
	assert.NoError(t, err)
}

func TestExpandCheckoutSession(t *testing.T) {
	gw := gateway.NewGateway()

	// Create a customer
	customer := &api.Customer{
		Id:     "cus_test123",
		Object: api.CustomerObjectEnumCustomer,
		Email:  new("test@example.com"),
	}
	_, err := gw.CreateCustomer(customer)
	require.NoError(t, err)

	// Create a subscription
	sub := &api.Subscription{
		Id:     "sub_test123",
		Object: api.SubscriptionObjectEnumSubscription,
		Status: api.SubscriptionStatusActive,
	}
	_, err = gw.CreateSubscription(sub)
	require.NoError(t, err)

	// Create a checkout session with ID references
	var custUnion api.CheckoutSession_Customer
	_ = custUnion.FromCheckoutSessionCustomer0("cus_test123")

	var subUnion api.CheckoutSession_Subscription
	_ = subUnion.FromCheckoutSessionSubscription0("sub_test123")

	session := &api.CheckoutSession{
		Id:           "cs_test",
		Object:       api.CheckoutSessionObjectEnumCheckoutSession,
		Customer:     &custUnion,
		Subscription: &subUnion,
	}

	// Expand the session
	err = Expand(session, gw)
	require.NoError(t, err)

	// Verify customer is expanded
	expandedCustomer, err := session.Customer.AsCustomer()
	require.NoError(t, err)
	assert.Equal(t, "cus_test123", expandedCustomer.Id)

	// Verify subscription is expanded
	expandedSub, err := session.Subscription.AsSubscription()
	require.NoError(t, err)
	assert.Equal(t, "sub_test123", expandedSub.Id)
}

func TestExpandSubscription(t *testing.T) {
	gw := gateway.NewGateway()

	// Create a customer
	customer := &api.Customer{
		Id:     "cus_test456",
		Object: api.CustomerObjectEnumCustomer,
		Email:  new("test456@example.com"),
	}
	_, err := gw.CreateCustomer(customer)
	require.NoError(t, err)

	// Create a subscription with customer ID reference
	var custUnion api.Subscription_Customer
	_ = custUnion.FromSubscriptionCustomer0("cus_test456")

	sub := &api.Subscription{
		Id:       "sub_test456",
		Object:   api.SubscriptionObjectEnumSubscription,
		Status:   api.SubscriptionStatusActive,
		Customer: custUnion,
	}

	// Expand the subscription
	err = Expand(sub, gw)
	require.NoError(t, err)

	// Verify customer is expanded
	expandedCustomer, err := sub.Customer.AsCustomer()
	require.NoError(t, err)
	assert.Equal(t, "cus_test456", expandedCustomer.Id)
}

func TestExpandInvoice(t *testing.T) {
	gw := gateway.NewGateway()

	// Create a customer
	customer := &api.Customer{
		Id:     "cus_test789",
		Object: api.CustomerObjectEnumCustomer,
		Email:  new("test789@example.com"),
	}
	_, err := gw.CreateCustomer(customer)
	require.NoError(t, err)

	// Create an invoice with customer ID reference
	var custUnion api.Invoice_Customer
	_ = custUnion.FromInvoiceCustomer0("cus_test789")

	invoice := &api.Invoice{
		Id:       "in_test789",
		Object:   api.InvoiceObjectEnumInvoice,
		Customer: custUnion,
	}

	// Expand the invoice
	err = Expand(invoice, gw)
	require.NoError(t, err)

	// Verify customer is expanded
	expandedCustomer, err := invoice.Customer.AsCustomer()
	require.NoError(t, err)
	assert.Equal(t, "cus_test789", expandedCustomer.Id)
}

func TestExpandCharge(t *testing.T) {
	gw := gateway.NewGateway()

	// Create a customer
	customer := &api.Customer{
		Id:     "cus_test_charge",
		Object: api.CustomerObjectEnumCustomer,
		Email:  new("charge@example.com"),
	}
	_, err := gw.CreateCustomer(customer)
	require.NoError(t, err)

	// Create a charge with customer ID reference
	var custUnion api.Charge_Customer
	_ = custUnion.FromChargeCustomer0("cus_test_charge")

	charge := &api.Charge{
		Id:       "ch_test",
		Object:   api.ChargeObjectEnumCharge,
		Customer: &custUnion,
	}

	// Expand the charge
	err = Expand(charge, gw)
	require.NoError(t, err)

	// Verify customer is expanded
	expandedCustomer, err := charge.Customer.AsCustomer()
	require.NoError(t, err)
	assert.Equal(t, "cus_test_charge", expandedCustomer.Id)
}

func TestExpandAlreadyExpanded(t *testing.T) {
	gw := gateway.NewGateway()

	// Create a checkout session with already expanded customer
	customer := &api.Customer{
		Id:     "cus_already_expanded",
		Object: api.CustomerObjectEnumCustomer,
		Email:  new("already@example.com"),
	}

	var custUnion api.CheckoutSession_Customer
	custUnion.FromCustomer(*customer)

	session := &api.CheckoutSession{
		Id:       "cs_already",
		Object:   api.CheckoutSessionObjectEnumCheckoutSession,
		Customer: &custUnion,
	}

	// Expand should not fail even if already expanded
	err := Expand(session, gw)
	require.NoError(t, err)

	// Customer should still be expanded
	expandedCustomer, err := session.Customer.AsCustomer()
	require.NoError(t, err)
	assert.Equal(t, "cus_already_expanded", expandedCustomer.Id)
}

func TestMustExpand(t *testing.T) {
	// Should not panic with nil gateway
	session := &api.CheckoutSession{Id: "cs_test"}
	MustExpand(session, nil)

	// Should not panic with nil resource
	MustExpand(nil, gateway.NewGateway())

	// Should work with valid resource and gateway
	gw := gateway.NewGateway()
	customer := &api.Customer{Id: "cus_must", Object: api.CustomerObjectEnumCustomer}
	gw.CreateCustomer(customer)

	var custUnion api.CheckoutSession_Customer
	custUnion.FromCheckoutSessionCustomer0("cus_must")

	session = &api.CheckoutSession{
		Id:       "cs_must",
		Customer: &custUnion,
	}
	MustExpand(session, gw)
}

func TestExpandCheckoutSessionWithInvoice(t *testing.T) {
	gw := gateway.NewGateway()

	// Create an invoice
	invoice := &api.Invoice{
		Id:     "in_test_invoice",
		Object: api.InvoiceObjectEnumInvoice,
	}
	gw.CreateInvoice(invoice)

	// Create a checkout session with invoice reference
	var invUnion api.CheckoutSession_Invoice
	invUnion.FromCheckoutSessionInvoice0("in_test_invoice")

	session := &api.CheckoutSession{
		Id:      "cs_inv_test",
		Object:  api.CheckoutSessionObjectEnumCheckoutSession,
		Invoice: &invUnion,
	}

	err := Expand(session, gw)
	require.NoError(t, err)

	// Verify invoice is expanded
	expandedInvoice, err := session.Invoice.AsInvoice()
	require.NoError(t, err)
	assert.Equal(t, "in_test_invoice", expandedInvoice.Id)
}

func TestExpandSubscriptionWithLatestInvoice(t *testing.T) {
	gw := gateway.NewGateway()

	// Create an invoice
	invoice := &api.Invoice{
		Id:     "in_latest",
		Object: api.InvoiceObjectEnumInvoice,
	}
	gw.CreateInvoice(invoice)

	// Create a subscription with latest_invoice reference
	var invUnion api.Subscription_LatestInvoice
	invUnion.FromSubscriptionLatestInvoice0("in_latest")

	sub := &api.Subscription{
		Id:            "sub_inv",
		Object:        api.SubscriptionObjectEnumSubscription,
		LatestInvoice: &invUnion,
	}

	err := Expand(sub, gw)
	require.NoError(t, err)

	// Verify invoice is expanded
	expandedInvoice, err := sub.LatestInvoice.AsInvoice()
	require.NoError(t, err)
	assert.Equal(t, "in_latest", expandedInvoice.Id)
}

func TestExpandSubscriptionWithItems(t *testing.T) {
	gw := gateway.NewGateway()

	// Create a product
	product := &api.Product{
		Id:     "prod_items",
		Object: api.ProductObjectEnumProduct,
		Name:   "Test Product",
	}
	gw.CreateProduct(product)

	// Create a price with product ID reference
	var productUnion api.Price_Product
	productUnion.FromPriceProduct0("prod_items")

	price := &api.Price{
		Id:       "price_items",
		Object:   api.PriceObjectEnumPrice,
		Product: productUnion,
	}

	// Create a subscription with items
	var custUnion api.Subscription_Customer
	custUnion.FromSubscriptionCustomer0("cus_items")

	sub := &api.Subscription{
		Id:       "sub_items",
		Object:   api.SubscriptionObjectEnumSubscription,
		Customer: custUnion,
	}
	sub.Items.Data = []api.SubscriptionItem{
		{
			Id:    "si_1",
			Price: *price,
		},
	}

	err := Expand(sub, gw)
	require.NoError(t, err)

	// Verify product is expanded in the price
	expandedProduct, err := sub.Items.Data[0].Price.Product.AsProduct()
	require.NoError(t, err)
	assert.Equal(t, "prod_items", expandedProduct.Id)
}

func TestExpandNonExistentResource(t *testing.T) {
	gw := gateway.NewGateway()

	// Create a checkout session with non-existent customer ID
	var custUnion api.CheckoutSession_Customer
	_ = custUnion.FromCheckoutSessionCustomer0("cus_nonexistent")

	session := &api.CheckoutSession{
		Id:       "cs_nonexistent",
		Object:   api.CheckoutSessionObjectEnumCheckoutSession,
		Customer: &custUnion,
	}

	// Expand should create a minimal customer
	err := Expand(session, gw)
	require.NoError(t, err)

	// Customer should still be expanded with minimal data
	expandedCustomer, err := session.Customer.AsCustomer()
	require.NoError(t, err)
	assert.Equal(t, "cus_nonexistent", expandedCustomer.Id)
}

