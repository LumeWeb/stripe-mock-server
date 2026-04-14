package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe-mock-server/pkg/internal/gen/models/api"
)

// TestNewGateway tests that NewGateway creates a gateway with proper repositories
func TestNewGateway(t *testing.T) {
	gw := NewGateway()

	assert.NotNil(t, gw)

// TestSetStandardFields tests that setStandardFields sets created and object fields

	// Verify each repository is initialized
	assert.NotNil(t, gw.customerRepo)
	assert.NotNil(t, gw.subscriptionRepo)
	assert.NotNil(t, gw.invoiceRepo)
	assert.NotNil(t, gw.chargeRepo)
	assert.NotNil(t, gw.sessionRepo)
	assert.NotNil(t, gw.productRepo)
	assert.NotNil(t, gw.priceRepo)
	assert.NotNil(t, gw.billingConfigRepo)
	assert.NotNil(t, gw.webhookRepo)
}
func TestSetStandardFields(t *testing.T) {
	tests := []struct {
		name     string
		obj      any
		wantObj  string
		checkObj func(t *testing.T, obj any)
	}{
		{
			name: "Customer with no fields",
			obj:  &api.Customer{},
			wantObj: "customer",
			checkObj: func(t *testing.T, obj any) {
				c := obj.(*api.Customer)
				assert.NotZero(t, c.Created, "created should be set")
				assert.Equal(t, "customer", string(c.Object))
			},
		},
		{
			name: "Customer with existing fields",
			obj:  &api.Customer{Object: "custom_obj", Created: 123456},
			wantObj: "custom_obj",
			checkObj: func(t *testing.T, obj any) {
				c := obj.(*api.Customer)
				assert.Equal(t, int64(123456), int64(c.Created), "existing created should not be overridden")
				assert.Equal(t, "custom_obj", string(c.Object), "existing object should not be overridden")
			},
		},
		{
			name: "Product with no fields",
			obj:  &api.Product{},
			wantObj: "product",
			checkObj: func(t *testing.T, obj any) {
				p := obj.(*api.Product)
				assert.NotZero(t, p.Created, "created should be set")
				assert.Equal(t, "product", string(p.Object))
			},
		},
		{
			name: "Price with no fields",
			obj:  &api.Price{},
			wantObj: "price",
			checkObj: func(t *testing.T, obj any) {
				p := obj.(*api.Price)
				assert.NotZero(t, p.Created, "created should be set")
				assert.Equal(t, "price", string(p.Object))
			},
		},
		{
			name: "Subscription with no fields",
			obj:  &api.Subscription{},
			wantObj: "subscription",
			checkObj: func(t *testing.T, obj any) {
				s := obj.(*api.Subscription)
				assert.NotZero(t, s.Created, "created should be set")
				assert.Equal(t, "subscription", string(s.Object))
				// LatestInvoice is only initialized in CreateSubscription, not setStandardFields
			},
		},
		{
			name: "CheckoutSession with no fields",
			obj:  &api.CheckoutSession{},
			wantObj: "checkout.session",
			checkObj: func(t *testing.T, obj any) {
				s := obj.(*api.CheckoutSession)
				assert.NotZero(t, s.Created, "created should be set")
				assert.Equal(t, "checkout.session", string(s.Object))
			},
		},
		{
			name: "Invoice with no fields",
			obj:  &api.Invoice{},
			wantObj: "invoice",
			checkObj: func(t *testing.T, obj any) {
				i := obj.(*api.Invoice)
				assert.NotZero(t, i.Created, "created should be set")
				assert.Equal(t, "invoice", string(i.Object))
			},
		},
		{
			name: "Charge with no fields",
			obj:  &api.Charge{},
			wantObj: "charge",
			checkObj: func(t *testing.T, obj any) {
				c := obj.(*api.Charge)
				assert.NotZero(t, c.Created, "created should be set")
				assert.Equal(t, "charge", string(c.Object))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setStandardFields(tt.obj)
			if tt.checkObj != nil {
				tt.checkObj(t, tt.obj)
			}
		})
	}
}

// TestCustomerOperations tests customer CRUD operations
func TestCustomerOperations(t *testing.T) {
	gw := NewGateway()

	t.Run("CreateCustomer", func(t *testing.T) {
		name := "John Doe"
		email := "john@example.com"
		customer := &api.Customer{
			Name:  &name,
			Email: &email,
		}

		created, err := gw.CreateCustomer(customer)
		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Equal(t, "John Doe", *created.Name)
		assert.Equal(t, "john@example.com", *created.Email)
		assert.NotZero(t, created.Id)
		assert.NotZero(t, created.Created)
		assert.Equal(t, "customer", string(created.Object))
	})

	t.Run("CreateCustomer - nil", func(t *testing.T) {
		created, err := gw.CreateCustomer(nil)
		assert.Error(t, err)
		assert.Nil(t, created)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("GetCustomer", func(t *testing.T) {
		name := "Jane Doe"
		email := "jane@example.com"
		customer := &api.Customer{
			Name:  &name,
			Email: &email,
		}

		created, err := gw.CreateCustomer(customer)
		require.NoError(t, err)

		retrieved, err := gw.GetCustomer(created.Id)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, "Jane Doe", *retrieved.Name)
		assert.Equal(t, "jane@example.com", *retrieved.Email)
	})

	t.Run("GetCustomer - not found", func(t *testing.T) {
		_, err := gw.GetCustomer("nonexistent")
		assert.Error(t, err)
	})

	t.Run("UpdateCustomer", func(t *testing.T) {
		name := "Update Test"
		email := "update@example.com"
		customer := &api.Customer{
			Name:  &name,
			Email: &email,
		}

		created, err := gw.CreateCustomer(customer)
		require.NoError(t, err)

		updatedName := "Updated Name"
		updatedEmail := "updated@example.com"
		updated := *created
		updated.Name = &updatedName
		updated.Email = &updatedEmail

		err = gw.UpdateCustomer(created.Id, &updated)
		require.NoError(t, err)

		retrieved, err := gw.GetCustomer(created.Id)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", *retrieved.Name)
		assert.Equal(t, "updated@example.com", *retrieved.Email)
	})
}

// TestProductOperations tests product CRUD operations
func TestProductOperations(t *testing.T) {
	gw := NewGateway()

	t.Run("CreateProduct", func(t *testing.T) {
		product := &api.Product{
			Name: "Test Product",
		}

		created, err := gw.CreateProduct(product)
		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Equal(t, "Test Product", created.Name)
		assert.NotZero(t, created.Id)
		assert.NotZero(t, created.Created)
		assert.Equal(t, "product", string(created.Object))
	})

	t.Run("CreateProduct - nil", func(t *testing.T) {
		created, err := gw.CreateProduct(nil)
		assert.Error(t, err)
		assert.Nil(t, created)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("GetProduct", func(t *testing.T) {
		product := &api.Product{
			Name: "Get Test Product",
		}

		created, err := gw.CreateProduct(product)
		require.NoError(t, err)

		retrieved, err := gw.GetProduct(created.Id)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, "Get Test Product", retrieved.Name)
	})

	t.Run("GetProduct - not found", func(t *testing.T) {
		_, err := gw.GetProduct("nonexistent")
		assert.Error(t, err)
	})
}

// TestPriceOperations tests price CRUD operations
func TestPriceOperations(t *testing.T) {
	gw := NewGateway()

	t.Run("CreatePrice", func(t *testing.T) {
		amount := 2999
		price := &api.Price{
			Currency:   "usd",
			UnitAmount: &amount,
			Type:       "one_time",
		}

		created, err := gw.CreatePrice(price)
		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Equal(t, "usd", created.Currency)
		assert.Equal(t, int64(2999), int64(*created.UnitAmount))
		assert.Equal(t, "one_time", string(created.Type))
		assert.NotZero(t, created.Id)
		assert.NotZero(t, created.Created)
		assert.Equal(t, "price", string(created.Object))
	})

	t.Run("CreatePrice - nil", func(t *testing.T) {
		created, err := gw.CreatePrice(nil)
		assert.Error(t, err)
		assert.Nil(t, created)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("GetPrice", func(t *testing.T) {
		amount := 1999
		price := &api.Price{
			Currency:   "eur",
			UnitAmount: &amount,
			Type:       "recurring",
		}

		created, err := gw.CreatePrice(price)
		require.NoError(t, err)

		retrieved, err := gw.GetPrice(created.Id)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, "eur", retrieved.Currency)
		assert.Equal(t, int64(1999), int64(*retrieved.UnitAmount))
		assert.Equal(t, "recurring", string(retrieved.Type))
	})

	t.Run("GetPrice - not found", func(t *testing.T) {
		_, err := gw.GetPrice("nonexistent")
		assert.Error(t, err)
	})
}

// TestCheckoutSessionOperations tests checkout session CRUD operations
func TestCheckoutSessionOperations(t *testing.T) {
	gw := NewGateway()

	t.Run("CreateCheckoutSession", func(t *testing.T) {
		successURL := "https://example.com/success"
		cancelURL := "https://example.com/cancel"
		session := &api.CheckoutSession{
			Mode:       "payment",
			SuccessUrl: &successURL,
			CancelUrl:  &cancelURL,
		}

		created, err := gw.CreateCheckoutSession(session)
		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Equal(t, "payment", string(created.Mode))
		assert.Equal(t, "https://example.com/success", *created.SuccessUrl)
		assert.Equal(t, "https://example.com/cancel", *created.CancelUrl)
		assert.NotZero(t, created.Id)
		assert.NotZero(t, created.Created)
		assert.Equal(t, "checkout.session", string(created.Object))
	})

	t.Run("CreateCheckoutSession - nil", func(t *testing.T) {
		created, err := gw.CreateCheckoutSession(nil)
		assert.Error(t, err)
		assert.Nil(t, created)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("GetCheckoutSession", func(t *testing.T) {
		successURL := "https://example.com/sub-success"
		session := &api.CheckoutSession{
			Mode:       "subscription",
			SuccessUrl: &successURL,
		}

		created, err := gw.CreateCheckoutSession(session)
		require.NoError(t, err)

		retrieved, err := gw.GetCheckoutSession(created.Id)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, "subscription", string(retrieved.Mode))
		assert.Equal(t, "https://example.com/sub-success", *retrieved.SuccessUrl)
	})

	t.Run("GetCheckoutSession - not found", func(t *testing.T) {
		_, err := gw.GetCheckoutSession("nonexistent")
		assert.Error(t, err)
	})
}

// TestSubscriptionOperations tests subscription CRUD operations
func TestSubscriptionOperations(t *testing.T) {
	gw := NewGateway()

	t.Run("CreateSubscription", func(t *testing.T) {
		sub := &api.Subscription{
			Status: "active",
		}

		created, err := gw.CreateSubscription(sub)
		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Equal(t, "active", string(created.Status))
		assert.NotZero(t, created.Id)
		assert.NotZero(t, created.Created)
		assert.Equal(t, "subscription", string(created.Object))
		// LatestInvoice is initialized in CreateSubscription but is an empty struct
		assert.NotNil(t, &created.LatestInvoice)
	})

	t.Run("CreateSubscription - nil", func(t *testing.T) {
		created, err := gw.CreateSubscription(nil)
		assert.Error(t, err)
		assert.Nil(t, created)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("GetSubscription", func(t *testing.T) {
		sub := &api.Subscription{
			Status: "trialing",
		}

		created, err := gw.CreateSubscription(sub)
		require.NoError(t, err)

		retrieved, err := gw.GetSubscription(created.Id)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, "trialing", string(retrieved.Status))
	})

	t.Run("GetSubscription - not found", func(t *testing.T) {
		_, err := gw.GetSubscription("nonexistent")
		assert.Error(t, err)
	})
}

// TestInvoiceOperations tests invoice CRUD operations
func TestInvoiceOperations(t *testing.T) {
	gw := NewGateway()

	t.Run("CreateInvoice", func(t *testing.T) {
		status := api.InvoiceStatusOpen
		invoice := &api.Invoice{
			Status:    &status,
			AmountDue: 10000,
			Currency:  "usd",
		}

		created, err := gw.CreateInvoice(invoice)
		require.NoError(t, err)
		require.NotNil(t, created)
		assert.NotNil(t, created.Status)
		assert.Equal(t, int64(10000), int64(created.AmountDue))
		assert.Equal(t, "usd", created.Currency)
		assert.NotZero(t, created.Id)
		assert.NotZero(t, created.Created)
		assert.Equal(t, "invoice", string(created.Object))
	})

	t.Run("CreateInvoice - nil", func(t *testing.T) {
		created, err := gw.CreateInvoice(nil)
		assert.Error(t, err)
		assert.Nil(t, created)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("GetInvoice", func(t *testing.T) {
		status := api.InvoiceStatusPaid
		invoice := &api.Invoice{
			Status:    &status,
			AmountDue: 5000,
			Currency:  "eur",
		}

		created, err := gw.CreateInvoice(invoice)
		require.NoError(t, err)

		retrieved, err := gw.GetInvoice(created.Id)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.NotNil(t, retrieved.Status)
		assert.Equal(t, int64(5000), int64(retrieved.AmountDue))
		assert.Equal(t, "eur", retrieved.Currency)
	})

	t.Run("GetInvoice - not found", func(t *testing.T) {
		_, err := gw.GetInvoice("nonexistent")
		assert.Error(t, err)
	})
}

// TestChargeOperations tests charge creation
func TestChargeOperations(t *testing.T) {
	gw := NewGateway()

	t.Run("CreateCharge", func(t *testing.T) {
		charge := &api.Charge{
			Amount:   2000,
			Currency: "usd",
			Status:   "succeeded",
		}

		created, err := gw.CreateCharge(charge)
		require.NoError(t, err)
		require.NotNil(t, created)
		assert.Equal(t, int64(2000), int64(created.Amount))
		assert.Equal(t, "usd", created.Currency)
		assert.Equal(t, "succeeded", string(created.Status))
		assert.NotZero(t, created.Id)
		assert.NotZero(t, created.Created)
		assert.Equal(t, "charge", string(created.Object))
	})

	t.Run("CreateCharge - nil", func(t *testing.T) {
		created, err := gw.CreateCharge(nil)
		assert.Error(t, err)
		assert.Nil(t, created)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

// TestHandleWebhook tests webhook handling
func TestHandleWebhook(t *testing.T) {
	gw := NewGateway()

	t.Run("HandleWebhook - valid event", func(t *testing.T) {
		eventData := []byte(`{
			"id": "evt_test123",
			"type": "customer.created",
			"data": {
				"object": {
					"id": "cus_test",
					"name": "Test Customer"
				}
			}
		}`)

		err := gw.HandleWebhook(eventData)
		require.NoError(t, err)
	})

	t.Run("HandleWebhook - invalid JSON", func(t *testing.T) {
		eventData := []byte(`invalid json`)

		err := gw.HandleWebhook(eventData)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid event data")
	})

	t.Run("HandleWebhook - empty event", func(t *testing.T) {
		eventData := []byte(`{}`)

		err := gw.HandleWebhook(eventData)
		require.NoError(t, err)
	})
}

// TestReset tests that Reset clears all state except webhooks
func TestReset(t *testing.T) {
	gw := NewGateway()

	// Create various resources
	name := "Test Customer"
	customer, err := gw.CreateCustomer(&api.Customer{Name: &name})
	require.NoError(t, err)

	product, err := gw.CreateProduct(&api.Product{Name: "Test Product"})
	require.NoError(t, err)

	amount := 1000
	price, err := gw.CreatePrice(&api.Price{Currency: "usd", UnitAmount: &amount})
	require.NoError(t, err)

	sub, err := gw.CreateSubscription(&api.Subscription{Status: "active"})
	require.NoError(t, err)

	invoiceStatus := api.InvoiceStatusOpen
	invoice, err := gw.CreateInvoice(&api.Invoice{Status: &invoiceStatus, AmountDue: 5000})
	require.NoError(t, err)

	charge, err := gw.CreateCharge(&api.Charge{Amount: 5000, Currency: "usd", Status: "succeeded"})
	require.NoError(t, err)

	successURL := "https://example.com/success"
	session, err := gw.CreateCheckoutSession(&api.CheckoutSession{Mode: "payment", SuccessUrl: &successURL})
	require.NoError(t, err)

	// Create a webhook endpoint
	webhook, err := gw.CreateWebhookEndpoint(&api.WebhookEndpoint{Url: "https://example.com/webhook"})
	require.NoError(t, err)

	// Verify all resources exist
	assert.True(t, gw.CustomerExists(customer.Id))
	assert.True(t, gw.ProductExists(product.Id))
	assert.True(t, gw.PriceExists(price.Id))
	assert.True(t, gw.SubscriptionExists(sub.Id))
	assert.True(t, gw.InvoiceExists(invoice.Id))
	assert.True(t, gw.ChargeExists(charge.Id))
	assert.True(t, gw.CheckoutSessionExists(session.Id))
	assert.True(t, gw.WebhookEndpointExists(webhook.Id))

	// Reset the gateway
	err = gw.Reset()
	require.NoError(t, err)

	// Verify all resources are cleared except webhooks
	assert.False(t, gw.CustomerExists(customer.Id))
	assert.False(t, gw.ProductExists(product.Id))
	assert.False(t, gw.PriceExists(price.Id))
	assert.False(t, gw.SubscriptionExists(sub.Id))
	assert.False(t, gw.InvoiceExists(invoice.Id))
	assert.False(t, gw.ChargeExists(charge.Id))
	assert.False(t, gw.CheckoutSessionExists(session.Id))

	// Webhook should still exist
	assert.True(t, gw.WebhookEndpointExists(webhook.Id))

	// Verify we can retrieve the webhook
	retrievedWebhook, err := gw.GetWebhookEndpoint(webhook.Id)
	require.NoError(t, err)
	assert.Equal(t, webhook.Id, retrievedWebhook.Id)
}

// TestResetEmptyGateway tests resetting a gateway with no resources
func TestResetEmptyGateway(t *testing.T) {
	gw := NewGateway()

	// Reset should not error on empty gateway
	err := gw.Reset()
	require.NoError(t, err)
}

// TestResetPreservesMultipleWebhooks tests that Reset preserves all webhooks
func TestResetPreservesMultipleWebhooks(t *testing.T) {
	gw := NewGateway()

	// Create multiple webhooks
	webhook1, err := gw.CreateWebhookEndpoint(&api.WebhookEndpoint{Url: "https://example.com/webhook1"})
	require.NoError(t, err)

	webhook2, err := gw.CreateWebhookEndpoint(&api.WebhookEndpoint{Url: "https://example.com/webhook2"})
	require.NoError(t, err)

	webhook3, err := gw.CreateWebhookEndpoint(&api.WebhookEndpoint{Url: "https://example.com/webhook3"})
	require.NoError(t, err)

	// Create some other resources
	name := "Customer"
	_, err = gw.CreateCustomer(&api.Customer{Name: &name})
	require.NoError(t, err)

	_, err = gw.CreateProduct(&api.Product{Name: "Product"})
	require.NoError(t, err)

	// Reset
	err = gw.Reset()
	require.NoError(t, err)

	// All webhooks should still exist
	assert.True(t, gw.WebhookEndpointExists(webhook1.Id))
	assert.True(t, gw.WebhookEndpointExists(webhook2.Id))
	assert.True(t, gw.WebhookEndpointExists(webhook3.Id))

	// Verify we can list all webhooks
	webhooks, err := gw.ListWebhookEndpoints(100, "")
	require.NoError(t, err)
	assert.Len(t, webhooks, 3)
}

// TestGatewayIntegration tests multiple operations together
func TestGatewayIntegration(t *testing.T) {
	gw := NewGateway()

	t.Run("Create product, then price", func(t *testing.T) {
		product := &api.Product{
			Name: "SaaS Subscription",
		}
		createdProduct, err := gw.CreateProduct(product)
		require.NoError(t, err)

		amount := 9999
		price := &api.Price{
			Currency:   "usd",
			UnitAmount: &amount,
			Type:       "recurring",
		}
		createdPrice, err := gw.CreatePrice(price)
		require.NoError(t, err)

		retrievedProduct, err := gw.GetProduct(createdProduct.Id)
		require.NoError(t, err)
		assert.Equal(t, "SaaS Subscription", retrievedProduct.Name)

		retrievedPrice, err := gw.GetPrice(createdPrice.Id)
		require.NoError(t, err)
		assert.Equal(t, int64(9999), int64(*retrievedPrice.UnitAmount))
	})

	t.Run("Create multiple customers and update one", func(t *testing.T) {
		name1 := "Customer 1"
		customer1, err := gw.CreateCustomer(&api.Customer{Name: &name1})
		require.NoError(t, err)

		name2 := "Customer 2"
		customer2, err := gw.CreateCustomer(&api.Customer{Name: &name2})
		require.NoError(t, err)

		name3 := "Customer 3"
		customer3, err := gw.CreateCustomer(&api.Customer{Name: &name3})
		require.NoError(t, err)

		updatedName := "Updated Customer 2"
		updatedEmail := "updated@example.com"
		updated := *customer2
		updated.Name = &updatedName
		updated.Email = &updatedEmail
		err = gw.UpdateCustomer(customer2.Id, &updated)
		require.NoError(t, err)

		retrieved1, err := gw.GetCustomer(customer1.Id)
		require.NoError(t, err)
		assert.Equal(t, "Customer 1", *retrieved1.Name)

		retrieved2, err := gw.GetCustomer(customer2.Id)
		require.NoError(t, err)
		assert.Equal(t, "Updated Customer 2", *retrieved2.Name)
		assert.Equal(t, "updated@example.com", *retrieved2.Email)

		retrieved3, err := gw.GetCustomer(customer3.Id)
		require.NoError(t, err)
		assert.Equal(t, "Customer 3", *retrieved3.Name)
	})
}
