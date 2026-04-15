package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v85"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/stripe-mock-server/pkg/gateway"
	"go.lumeweb.com/stripe-mock-server/pkg/generator"
	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
)

// Test UpdateCustomerMetadata (dry unit test for helper function)
func TestUpdateCustomerMetadata(t *testing.T) {
	t.Run("update nil metadata with new data", func(t *testing.T) {
		customer := &api.Customer{
			Name: strPtr("Test Customer"),
		}

		newMetadata := map[string]any{
			"key1": "value1",
			"key2": "value2",
		}

		updateCustomerMetadata(customer, newMetadata)

		require.NotNil(t, customer.Metadata)
		assert.Equal(t, "value1", (*customer.Metadata)["key1"])
		assert.Equal(t, "value2", (*customer.Metadata)["key2"])
	})

	t.Run("update existing metadata with new data", func(t *testing.T) {
		existing := map[string]string{
			"existing": "value",
		}
		customer := &api.Customer{
			Name:     strPtr("Test Customer"),
			Metadata: &existing,
		}

		newMetadata := map[string]any{
			"new_key": "new_value",
		}

		updateCustomerMetadata(customer, newMetadata)

		assert.Equal(t, "value", (*customer.Metadata)["existing"])
		assert.Equal(t, "new_value", (*customer.Metadata)["new_key"])
	})

	t.Run("merge with mixed types in new metadata", func(t *testing.T) {
		customer := &api.Customer{
			Name: strPtr("Test Customer"),
		}

		newMetadata := map[string]any{
			"string":  "value",
			"number":  123,
			"boolean": true,
		}

		updateCustomerMetadata(customer, newMetadata)

		require.NotNil(t, customer.Metadata)
		assert.Equal(t, "value", (*customer.Metadata)["string"])
		assert.Equal(t, "123", (*customer.Metadata)["number"])
		assert.Equal(t, "true", (*customer.Metadata)["boolean"])
	})

	t.Run("nil new metadata does nothing", func(t *testing.T) {
		customer := &api.Customer{
			Name: strPtr("Test Customer"),
		}

		updateCustomerMetadata(customer, nil)

		assert.Nil(t, customer.Metadata)
	})

	t.Run("empty new metadata does nothing", func(t *testing.T) {
		existing := map[string]string{
			"existing": "value",
		}
		customer := &api.Customer{
			Name:     strPtr("Test Customer"),
			Metadata: &existing,
		}

		updateCustomerMetadata(customer, map[string]any{})

		assert.Equal(t, "value", (*customer.Metadata)["existing"])
	})

	t.Run("new metadata overwrites existing keys", func(t *testing.T) {
		existing := map[string]string{
			"key": "old_value",
		}
		customer := &api.Customer{
			Name:     strPtr("Test Customer"),
			Metadata: &existing,
		}

		newMetadata := map[string]any{
			"key": "new_value",
		}

		updateCustomerMetadata(customer, newMetadata)

		assert.Equal(t, "new_value", (*customer.Metadata)["key"])
	})
}

// Test Customer Handlers
func TestCustomerHandlers(t *testing.T) {
	s := setupTestServer(t, false)

	t.Run("CreateCustomer", func(t *testing.T) {
		data := map[string]any{
			"name":  "John Doe",
			"email": "john@example.com",
		}

		status, result, err := s.handleCreateCustomer(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		customer, ok := result.(*api.Customer)
		require.True(t, ok)
		assert.Equal(t, "John Doe", *customer.Name)
		assert.Equal(t, "john@example.com", *customer.Email)
		assert.NotZero(t, customer.Id)
		assert.Equal(t, "customer", string(customer.Object))
	})

	t.Run("CreateCustomer with metadata", func(t *testing.T) {
		data := map[string]any{
			"name":     "Jane Doe",
			"email":    "jane@example.com",
			"metadata": map[string]any{"plan": "premium", "tier": "1"},
		}

		status, result, err := s.handleCreateCustomer(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		customer, ok := result.(*api.Customer)
		require.True(t, ok)
		assert.Equal(t, "Jane Doe", *customer.Name)
		require.NotNil(t, customer.Metadata)
		assert.Equal(t, "premium", (*customer.Metadata)["plan"])
		assert.Equal(t, "1", (*customer.Metadata)["tier"])
	})

	t.Run("CreateCustomer with balance", func(t *testing.T) {
		balance := 1000
		data := map[string]any{
			"name":    "Customer with Balance",
			"balance": balance,
		}

		status, result, err := s.handleCreateCustomer(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		customer, ok := result.(*api.Customer)
		require.True(t, ok)
		assert.NotNil(t, customer.Balance)
		assert.Equal(t, 1000, *customer.Balance)
	})

	t.Run("UpdateCustomer", func(t *testing.T) {
		// First create a customer
		createData := map[string]any{
			"name":  "Original Name",
			"email": "original@example.com",
		}
		status, result, err := s.handleCreateCustomer(nil, nil, createData)
		require.NoError(t, err)
		customer := result.(*api.Customer)

		// Update the customer
		updateData := map[string]any{
			"email":    "updated@example.com",
			"metadata": map[string]any{"status": "updated"},
		}

		status, result, err = s.handleUpdateCustomer(nil, map[string]string{"id": customer.Id}, updateData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		updated, ok := result.(*api.Customer)
		require.True(t, ok)
		assert.Equal(t, "Original Name", *updated.Name)
		assert.Equal(t, "updated@example.com", *updated.Email)
		require.NotNil(t, updated.Metadata)
		assert.Equal(t, "updated", (*updated.Metadata)["status"])
	})

	t.Run("UpdateCustomer - not found", func(t *testing.T) {
		updateData := map[string]any{
			"email": "updated@example.com",
		}

		status, _, err := s.handleUpdateCustomer(nil, map[string]string{"id": "nonexistent"}, updateData)
		assert.Equal(t, http.StatusNotFound, status)
		assert.Error(t, err)
	})

	t.Run("RetrieveCustomer", func(t *testing.T) {
		// First create a customer
		createData := map[string]any{
			"name":  "Retrieve Test",
			"email": "retrieve@example.com",
		}
		status, result, err := s.handleCreateCustomer(nil, nil, createData)
		require.NoError(t, err)
		customer := result.(*api.Customer)

		// Retrieve the customer
		status, result, err = s.handleRetrieveCustomer(nil, map[string]string{"id": customer.Id}, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		retrieved, ok := result.(*api.Customer)
		require.True(t, ok)
		assert.Equal(t, "Retrieve Test", *retrieved.Name)
		assert.Equal(t, "retrieve@example.com", *retrieved.Email)
		assert.Equal(t, customer.Id, retrieved.Id)
	})

	t.Run("RetrieveCustomer - not found", func(t *testing.T) {
		status, _, err := s.handleRetrieveCustomer(nil, map[string]string{"id": "nonexistent"}, nil)
		assert.Equal(t, http.StatusNotFound, status)
		assert.Error(t, err)
	})
}

// Test Product Handlers
func TestProductHandlers(t *testing.T) {
	s := setupTestServer(t, false)

	t.Run("CreateProduct", func(t *testing.T) {
		data := map[string]any{
			"name": "Test Product",
		}

		status, result, err := s.handleCreateProduct(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		product, ok := result.(*api.Product)
		require.True(t, ok)
		assert.Equal(t, "Test Product", product.Name)
		assert.NotZero(t, product.Id)
		assert.Equal(t, "product", string(product.Object))
	})

	t.Run("CreateProduct with metadata", func(t *testing.T) {
		data := map[string]any{
			"name":     "Product with Metadata",
			"metadata": map[string]any{"category": "electronics", "sku": "12345"},
		}

		status, result, err := s.handleCreateProduct(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		product, ok := result.(*api.Product)
		require.True(t, ok)
		assert.Equal(t, "Product with Metadata", product.Name)
		assert.Equal(t, "electronics", product.Metadata["category"])
		assert.Equal(t, "12345", product.Metadata["sku"])
	})

	t.Run("CreateProduct with active flag", func(t *testing.T) {
		data := map[string]any{
			"name":   "Inactive Product",
			"active": "false",
		}

		status, result, err := s.handleCreateProduct(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		product, ok := result.(*api.Product)
		require.True(t, ok)
		assert.False(t, product.Active)
	})

	t.Run("CreateProduct with description", func(t *testing.T) {
		description := "A test product description"
		data := map[string]any{
			"name":        "Product with Description",
			"description": description,
		}

		status, result, err := s.handleCreateProduct(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		product, ok := result.(*api.Product)
		require.True(t, ok)
		assert.Equal(t, description, *product.Description)
	})

	t.Run("UpdateProduct with default_price", func(t *testing.T) {
		// Create a product first
		createData := map[string]any{
			"name": "Test Product for Update",
		}
		status, result, err := s.handleCreateProduct(nil, nil, createData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)
		product := result.(*api.Product)

		// Create a price
		priceData := map[string]any{
			"currency":  "usd",
			"unit_amount": 1999,
			"type":       "recurring",
			"product":    product.Id,
		}
		priceStatus, priceResult, priceErr := s.handleCreatePrice(nil, nil, priceData)
		require.NoError(t, priceErr)
		assert.Equal(t, http.StatusOK, priceStatus)
		price := priceResult.(*api.Price)

		// Update the product to set default_price
		updateData := map[string]any{
			"default_price": price.Id,
		}
		status, result, err = s.handleUpdateProduct(nil, map[string]string{"id": product.Id}, updateData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		updatedProduct := result.(*api.Product)
		assert.Equal(t, product.Id, updatedProduct.Id)
		require.NotNil(t, updatedProduct.DefaultPrice)
		updatedPriceID, _ := updatedProduct.DefaultPrice.AsProductDefaultPrice0()
		assert.Equal(t, price.Id, updatedPriceID)
	})

	t.Run("UpdateProduct - not found", func(t *testing.T) {
		updateData := map[string]any{
			"default_price": "price_test123",
		}
		status, _, err := s.handleUpdateProduct(nil, map[string]string{"id": "nonexistent_product"}, updateData)
		assert.Equal(t, http.StatusNotFound, status)
		assert.Error(t, err)
	})

	t.Run("UpdateProduct - partial update name only", func(t *testing.T) {
		// Create a product
		createData := map[string]any{
			"name": "Original Name",
		}
		status, result, err := s.handleCreateProduct(nil, nil, createData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)
		product := result.(*api.Product)

		// Update only the name
		updateData := map[string]any{
			"name": "New Name",
		}
		status, result, err = s.handleUpdateProduct(nil, map[string]string{"id": product.Id}, updateData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		updatedProduct := result.(*api.Product)
		assert.Equal(t, "New Name", updatedProduct.Name)
		// Should preserve other fields
		assert.NotZero(t, updatedProduct.Id, "Product ID should be preserved")
		assert.NotZero(t, updatedProduct.Created, "Created timestamp should be preserved")
	})
}

// Test Price Handlers
func TestPriceHandlers(t *testing.T) {
	s := setupTestServer(t, false)

	t.Run("CreatePrice", func(t *testing.T) {
		data := map[string]any{
			"currency":    "usd",
			"unit_amount": 2999,
			"product":     "prod_test",
		}

		status, result, err := s.handleCreatePrice(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		price, ok := result.(*api.Price)
		require.True(t, ok)
		assert.Equal(t, "usd", price.Currency)
		assert.Equal(t, int64(2999), int64(*price.UnitAmount))
		assert.Equal(t, "one_time", string(price.Type))
	})

	t.Run("CreatePrice with recurring", func(t *testing.T) {
		data := map[string]any{
			"currency":    "usd",
			"unit_amount": 9999,
			"recurring": map[string]any{
				"interval": "month",
			},
		}

		status, result, err := s.handleCreatePrice(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		price, ok := result.(*api.Price)
		require.True(t, ok)
		assert.Equal(t, "usd", price.Currency)
		assert.Equal(t, int64(9999), int64(*price.UnitAmount))
		assert.Equal(t, "recurring", string(price.Type))
		assert.NotNil(t, price.Recurring)
	})

	t.Run("CreatePrice with metadata", func(t *testing.T) {
		data := map[string]any{
			"currency":    "eur",
			"unit_amount": 1999,
			"metadata":    map[string]any{"plan": "standard"},
		}

		status, result, err := s.handleCreatePrice(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		price, ok := result.(*api.Price)
		require.True(t, ok)
		assert.Equal(t, "eur", price.Currency)
		assert.Equal(t, "standard", price.Metadata["plan"])
	})
}

// Test Checkout Session Handlers
func TestCheckoutSessionHandlers(t *testing.T) {
	s := setupTestServer(t, false)

	t.Run("CreateCheckoutSession", func(t *testing.T) {
		data := map[string]any{
			"mode":         "payment",
			"success_url":  "https://example.com/success",
			"cancel_url":   "https://example.com/cancel",
			"line_items": []map[string]any{
				{
					"price":    "price_test",
					"quantity": 1,
				},
			},
		}

		status, result, err := s.handleCreateCheckoutSession(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		session, ok := result.(*api.CheckoutSession)
		require.True(t, ok)
		assert.Equal(t, "payment", string(session.Mode))
		assert.NotZero(t, session.Id)
		assert.Equal(t, "checkout.session", string(session.Object))
	})

	t.Run("CreateCheckoutSession with subscription mode", func(t *testing.T) {
		data := map[string]any{
			"mode":         "subscription",
			"success_url":  "https://example.com/success",
			"cancel_url":   "https://example.com/cancel",
			"line_items": []map[string]any{
				{
					"price":    "price_sub",
					"quantity": 1,
				},
			},
		}

		status, result, err := s.handleCreateCheckoutSession(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		session, ok := result.(*api.CheckoutSession)
		require.True(t, ok)
		assert.Equal(t, "subscription", string(session.Mode))
	})

	t.Run("CreateCheckoutSession with customer", func(t *testing.T) {
		// First create a customer
		customerData := map[string]any{
			"name":  "Session Customer",
			"email": "session@example.com",
		}
		_, customerResult, err := s.handleCreateCustomer(nil, nil, customerData)
		require.NoError(t, err)
		customer := customerResult.(*api.Customer)

		// Create a checkout session with the customer
		sessionData := map[string]any{
			"mode":         "payment",
			"success_url":  "https://example.com/success",
			"cancel_url":   "https://example.com/cancel",
			"customer":     customer.Id,
			"line_items": []map[string]any{
				{
					"price":    "price_test",
					"quantity": 1,
				},
			},
		}

		status, result, err := s.handleCreateCheckoutSession(nil, nil, sessionData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		session, ok := result.(*api.CheckoutSession)
		require.True(t, ok)
		assert.NotNil(t, session.Customer)
	})

	t.Run("RetrieveCheckoutSession", func(t *testing.T) {
		// First create a session
		createData := map[string]any{
			"mode":        "payment",
			"success_url": "https://example.com/success",
		}
		status, result, err := s.handleCreateCheckoutSession(nil, nil, createData)
		require.NoError(t, err)
		session := result.(*api.CheckoutSession)

		// Retrieve the session
		status, result, err = s.handleRetrieveCheckoutSession(nil, map[string]string{"id": session.Id}, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		retrieved, ok := result.(*api.CheckoutSession)
		require.True(t, ok)
		assert.Equal(t, "payment", string(retrieved.Mode))
		assert.Equal(t, session.Id, retrieved.Id)
	})

	t.Run("RetrieveCheckoutSession - not found", func(t *testing.T) {
		status, _, err := s.handleRetrieveCheckoutSession(nil, map[string]string{"id": "nonexistent"}, nil)
		assert.Equal(t, http.StatusNotFound, status)
		assert.Error(t, err)
	})

	t.Run("CompleteCheckoutSession - success", func(t *testing.T) {
		// First create a session
		createData := map[string]any{
			"mode":        "payment",
			"success_url": "https://example.com/success",
		}
		status, result, err := s.handleCreateCheckoutSession(nil, nil, createData)
		require.NoError(t, err)
		session := result.(*api.CheckoutSession)
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, api.CheckoutSessionStatusOpen, *session.Status)

		// Complete the session
		status, result, err = s.handleCompleteCheckoutSession(nil, map[string]string{"id": session.Id}, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		completed, ok := result.(*api.CheckoutSession)
		require.True(t, ok)
		assert.Equal(t, api.CheckoutSessionStatusComplete, *completed.Status)
		assert.Equal(t, api.CheckoutSessionPaymentStatusPaid, completed.PaymentStatus)
	})

	t.Run("CompleteCheckoutSession - not found", func(t *testing.T) {
		status, _, err := s.handleCompleteCheckoutSession(nil, map[string]string{"id": "nonexistent"}, nil)
		assert.Equal(t, http.StatusBadRequest, status)
		assert.Error(t, err)
	})

	t.Run("CompleteCheckoutSession - already complete", func(t *testing.T) {
		// First create a session
		createData := map[string]any{
			"mode":        "payment",
			"success_url": "https://example.com/success",
		}
		status, result, err := s.handleCreateCheckoutSession(nil, nil, createData)
		require.NoError(t, err)
		session := result.(*api.CheckoutSession)
		assert.Equal(t, http.StatusOK, status)
		assert.Equal(t, api.CheckoutSessionStatusOpen, *session.Status)

		// Complete the session
		status, result, err = s.handleCompleteCheckoutSession(nil, map[string]string{"id": session.Id}, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)
		completed := result.(*api.CheckoutSession)
		assert.Equal(t, api.CheckoutSessionStatusComplete, *completed.Status)

		// Try to complete again - should fail
		status, _, err = s.handleCompleteCheckoutSession(nil, map[string]string{"id": session.Id}, nil)
		assert.Equal(t, http.StatusBadRequest, status)
		assert.Error(t, err)
	})
}

// Test Billing Portal Handlers
func TestBillingPortalHandlers(t *testing.T) {
	s := setupTestServer(t, false)

	t.Run("CreateBillingPortalSession", func(t *testing.T) {
		data := map[string]any{
			"customer":   "cus_test123",
			"return_url": "https://example.com/return",
		}

		status, result, err := s.handleCreateBillingPortalSession(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		session, ok := result.(*api.BillingPortalSession)
		require.True(t, ok)
		assert.Equal(t, "cus_test123", session.Customer)
		assert.Equal(t, "https://example.com/return", *session.ReturnUrl)
		assert.NotZero(t, session.Id)
		assert.Equal(t, "billing_portal.session", string(session.Object))
	})

	t.Run("CreateBillingPortalSession with optional fields", func(t *testing.T) {
		data := map[string]any{
			"customer":         "cus_test456",
			"return_url":       "https://example.com/return",
			"configuration":    "bpc_config123",
			"customer_account": "acct_test",
			"on_behalf_of":     "acct_on_behalf",
			"locale":           "en",
		}

		status, result, err := s.handleCreateBillingPortalSession(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		session, ok := result.(*api.BillingPortalSession)
		require.True(t, ok)
		assert.Equal(t, "cus_test456", session.Customer)
		assert.Equal(t, "acct_test", *session.CustomerAccount)
		assert.Equal(t, "acct_on_behalf", *session.OnBehalfOf)
		assert.Equal(t, "en", string(*session.Locale))
	})

	t.Run("CreateBillingPortalConfiguration", func(t *testing.T) {
		data := map[string]any{
			"business_profile": map[string]any{
				"headline":            "Test Business",
				"privacy_policy_url":  "https://example.com/privacy",
				"terms_of_service_url": "https://example.com/terms",
			},
			"features": map[string]any{
				"invoice_history": map[string]any{
					"enabled": true,
				},
				"customer_update": map[string]any{
					"enabled":         true,
					"allowed_updates": []any{"address", "email"},
				},
			},
			"default_return_url": "https://example.com",
		}

		status, result, err := s.handleCreateBillingPortalConfiguration(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		config, ok := result.(*api.BillingPortalConfiguration)
		require.True(t, ok)
		assert.NotNil(t, config.BusinessProfile)
		assert.Equal(t, "Test Business", *config.BusinessProfile.Headline)
		assert.NotNil(t, config.Features)
		assert.True(t, config.Features.InvoiceHistory.Enabled)
		assert.True(t, config.Features.CustomerUpdate.Enabled)
	})

	t.Run("CreateBillingPortalConfiguration with metadata", func(t *testing.T) {
		data := map[string]any{
			"name": "Test Config",
			"features": map[string]any{
				"invoice_history": map[string]any{
					"enabled": true,
				},
			},
			"metadata": map[string]any{
				"environment": "test",
				"version":     "1.0",
			},
		}

		status, result, err := s.handleCreateBillingPortalConfiguration(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		config, ok := result.(*api.BillingPortalConfiguration)
		require.True(t, ok)
		require.NotNil(t, config.Name)
		assert.Equal(t, "Test Config", *config.Name)
		require.NotNil(t, config.Metadata)
		assert.Equal(t, "test", (*config.Metadata)["environment"])
		assert.Equal(t, "1.0", (*config.Metadata)["version"])
	})

	t.Run("CreateBillingPortalConfiguration with payment method update", func(t *testing.T) {
		data := map[string]any{
			"features": map[string]any{
				"payment_method_update": map[string]any{
					"enabled":    true,
					"cancel_url": "https://example.com/cancel",
				},
			},
		}

		status, result, err := s.handleCreateBillingPortalConfiguration(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		config, ok := result.(*api.BillingPortalConfiguration)
		require.True(t, ok)
		assert.NotNil(t, config.Features.PaymentMethodUpdate)
		assert.True(t, config.Features.PaymentMethodUpdate.Enabled)
		// PaymentMethodUpdate doesn't have a cancel_url field in the generated types
	})
}

// Test Subscription Handlers
func TestSubscriptionHandlers(t *testing.T) {
	s := setupTestServer(t, false)

	t.Run("RetrieveSubscription - not found returns mock", func(t *testing.T) {
		status, result, err := s.handleRetrieveSubscription(nil, map[string]string{"id": "nonexistent"}, nil)
		assert.Equal(t, http.StatusOK, status)
		require.NoError(t, err)

		subscription, ok := result.(*api.Subscription)
		require.True(t, ok)
		assert.Equal(t, "nonexistent", subscription.Id)
		assert.Equal(t, "active", string(subscription.Status))
	})

	t.Run("RetrieveSubscription - existing subscription", func(t *testing.T) {
		// First create a subscription through the gateway
		sub := &api.Subscription{
			Status: "trialing",
		}
		created, err := s.gateway.CreateSubscription(sub)
		require.NoError(t, err)

		// Retrieve the subscription
		status, result, err := s.handleRetrieveSubscription(nil, map[string]string{"id": created.Id}, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		retrieved, ok := result.(*api.Subscription)
		require.True(t, ok)
		assert.Equal(t, created.Id, retrieved.Id)
		assert.Equal(t, "trialing", string(retrieved.Status))
	})

	t.Run("CreateSubscription - success", func(t *testing.T) {
		// First create a customer
		customerData := map[string]any{
			"name":  "Test Customer",
			"email": "test@example.com",
		}
		status, result, err := s.handleCreateCustomer(nil, nil, customerData)
		require.NoError(t, err)
		customer := result.(*api.Customer)
		
		// Create subscription for the customer
		subData := map[string]any{
			"customer": customer.Id,
		}
		status, result, err = s.handleCreateSubscription(nil, nil, subData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)
		
		sub, ok := result.(*api.Subscription)
		require.True(t, ok)
		assert.NotEmpty(t, sub.Id)
		assert.Equal(t, api.SubscriptionStatusIncomplete, sub.Status)
	})

	t.Run("triggerSubscriptionLifecycle - webhook waterfall", func(t *testing.T) {
		// Setup fresh test server for this test to avoid webhook conflicts
		s := setupTestServer(t, false)
		
		// Setup test server with webhook endpoint to capture events
		var receivedEvents []string
		var eventsMutex sync.Mutex
		
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var event map[string]any
			if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			
			eventsMutex.Lock()
			if eventType, ok := event["type"]; ok {
				receivedEvents = append(receivedEvents, eventType.(string))
			}
			eventsMutex.Unlock()
			
			w.WriteHeader(http.StatusOK)
		}))
		defer testServer.Close()
		
		// Create webhook endpoint that listens to all subscription-related events
		opts := &CreateOpts{
			URL:     testServer.URL,
			Enabled: []string{"*"},
			Livemode: false,
			Secret:  "whsec_test_secret",
		}
		
		webhook, err := s.webhook.CreateWebhook(testServer.URL, opts)
		require.NoError(t, err)
		require.NotNil(t, webhook)
		
		// Create a subscription (this triggers the lifecycle)
		subData := map[string]any{
			"customer": "cus_test",
		}
		status, result, err := s.handleCreateSubscription(nil, nil, subData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)
		
		sub := result.(*api.Subscription)
		assert.NotEmpty(t, sub.Id)
		
		// Wait for webhook deliveries to complete
		time.Sleep(2 * time.Second)
		
		// Verify all 6 events were received
		expectedEvents := []string{
			string(stripe.EventTypeCustomerSubscriptionCreated),
			string(stripe.EventTypeInvoiceCreated),
			string(stripe.EventTypeInvoiceFinalized),
			string(stripe.EventTypeChargeSucceeded),
			string(stripe.EventTypeInvoicePaid),
			string(stripe.EventTypeCustomerSubscriptionUpdated),
		}
		
		eventsMutex.Lock()
		defer eventsMutex.Unlock()
		
		// Check that we received all expected events (order may vary due to async processing)
		assert.Equal(t, len(expectedEvents), len(receivedEvents), "Expected %d events, got %d: %v", len(expectedEvents), len(receivedEvents), receivedEvents)
		
		// Create a map for quick lookup
		receivedMap := make(map[string]bool)
		for _, event := range receivedEvents {
			receivedMap[event] = true
		}
		
		// Verify each expected event was received
		for _, expected := range expectedEvents {
			assert.True(t, receivedMap[expected], "Expected event %s to be received", expected)
		}
		
		// Verify first event is subscription.created (this should always be first)
		if len(receivedEvents) > 0 {
			assert.Equal(t, string(stripe.EventTypeCustomerSubscriptionCreated), receivedEvents[0], "First event should be subscription.created")
		}
	})
	
	t.Run("RenewSubscription - success", func(t *testing.T) {
		// Setup: Create an active subscription first
		s := setupTestServer(t, false)
		
		// Create subscription
		subData := map[string]any{
			"customer": "cus_test",
			"items": []map[string]any{
				{
					"price": "price_test_monthly",
				},
			},
		}
		status, result, err := s.handleCreateSubscription(nil, nil, subData)
		require.NoError(t, err, "Create subscription should succeed")
		require.Equal(t, http.StatusOK, status, "Should return 200 OK")
		
		sub := result.(*api.Subscription)
		originalPeriodEnd := int64(sub.Items.Data[0].CurrentPeriodEnd)
		require.Greater(t, originalPeriodEnd, int64(0), "Period end should be set")
		
		// Wait for creation lifecycle to complete
		time.Sleep(500 * time.Millisecond)
		
		// Trigger renewal
		pathParams := map[string]string{"id": sub.Id}
		status, result, err = s.handleRenewSubscription(nil, pathParams, nil)
		require.NoError(t, err, "Renew subscription should succeed")
		require.Equal(t, http.StatusOK, status, "Should return 200 OK")
		
		updatedSub := result.(*api.Subscription)
		require.Equal(t, sub.Id, updatedSub.Id, "Should return same subscription ID")
		// Period end should be advanced by 30 days (2592000 seconds)
		expectedAdvance := int64(30 * 24 * 60 * 60)
		require.Equal(t, originalPeriodEnd+expectedAdvance, int64(updatedSub.Items.Data[0].CurrentPeriodEnd), 
			"Period end should be advanced by 30 days")
		assert.NotEqual(t, originalPeriodEnd, int64(updatedSub.Items.Data[0].CurrentPeriodEnd), 
			"Period end should be different after renewal")
		
		// Status should still be active
		assert.Equal(t, api.SubscriptionStatusActive, updatedSub.Status, 
			"Status should remain active")
	})
	
	t.Run("RenewSubscription - not found", func(t *testing.T) {
		s := setupTestServer(t, false)
		
		pathParams := map[string]string{"id": "sub_nonexistent"}
		status, _, err := s.handleRenewSubscription(nil, pathParams, nil)
		require.Equal(t, http.StatusNotFound, status, "Should return 404 for non-existent subscription")
		require.Error(t, err, "Should return error for non-existent subscription")
	})
	
	t.Run("RenewSubscription - canceled subscription fails", func(t *testing.T) {
		s := setupTestServer(t, false)
		
		// Create subscription and then manually set to canceled
		subData := map[string]any{"customer": "cus_test"}
		status, result, err := s.handleCreateSubscription(nil, nil, subData)
		require.NoError(t, err)
		sub := result.(*api.Subscription)
		sub.Status = api.SubscriptionStatusCanceled
		err = s.gateway.UpdateSubscription(sub.Id, sub)
		require.NoError(t, err)
		
		// Try to renew - should fail
		pathParams := map[string]string{"id": sub.Id}
		status, _, err = s.handleRenewSubscription(nil, pathParams, nil)
		require.Equal(t, http.StatusBadRequest, status, "Should return 400 for canceled subscription")
		require.Error(t, err, "Should return error for canceled subscription")
		assert.Contains(t, err.Error(), "cannot renew subscription with status canceled", 
			"Error message should mention canceled status")
	})

	t.Run("PlanChange - update subscription items", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create subscription with items
		subData := map[string]any{
			"customer": "cus_test",
			"items": []map[string]any{
				{
					"price": "price_old",
				},
			},
		}
		status, result, err := s.handleCreateSubscription(nil, nil, subData)
		require.NoError(t, err, "Create subscription should succeed")
		require.Equal(t, http.StatusOK, status)

		sub := result.(*api.Subscription)
		require.NotEmpty(t, sub.Items.Data, "Subscription should have items")
		oldItemId := sub.Items.Data[0].Id

		// Set subscription to active status (simulates successful payment)
		sub.Status = api.SubscriptionStatusActive
		err = s.gateway.UpdateSubscription(sub.Id, sub)
		require.NoError(t, err, "Failed to set subscription status to active")

		// Update subscription to new price
		updateData := map[string]any{
			"items": []map[string]any{
				{
					"id":   oldItemId,
					"price": "price_new",
				},
			},
			"proration_behavior": "create_prorations",
		}

		pathParams := map[string]string{"id": sub.Id}
		status, result, err = s.handleUpdateSubscription(nil, pathParams, updateData)
		require.NoError(t, err, "Update subscription should succeed")
		require.Equal(t, http.StatusOK, status)

		updatedSub := result.(*api.Subscription)
		assert.Equal(t, "price_new", updatedSub.Items.Data[0].Price.Id, "Price ID should be updated")
	})

	t.Run("PlanChange - subscription not found", func(t *testing.T) {
		s := setupTestServer(t, false)

		pathParams := map[string]string{"id": "sub_nonexistent"}
		updateData := map[string]any{
			"items": []map[string]any{
				{"id": "si_xxx", "price": "price_new"},
			},
		}

		status, _, err := s.handleUpdateSubscription(nil, pathParams, updateData)
		require.Equal(t, http.StatusNotFound, status, "Should return 404 for non-existent subscription")
		require.Error(t, err)
	})
}

// Test Integration Scenarios
func TestHandlerIntegration(t *testing.T) {
	s := setupTestServer(t, false)

	t.Run("Create product, then price, then checkout session", func(t *testing.T) {
		// Create a product
		productData := map[string]any{
			"name": "Integration Test Product",
		}
		status, result, err := s.handleCreateProduct(nil, nil, productData)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)
		product := result.(*api.Product)

		// Create a price for the product
		priceData := map[string]any{
			"currency":    "usd",
			"unit_amount": 1999,
			"product":     product.Id,
		}
		_, result, err = s.handleCreatePrice(nil, nil, priceData)
		require.NoError(t, err)
		price := result.(*api.Price)

		// Create a checkout session
		sessionData := map[string]any{
			"mode":        "payment",
			"success_url": "https://example.com/success",
			"line_items": []map[string]any{
				{
					"price":    price.Id,
					"quantity": 1,
				},
			},
		}
		status, result, err = s.handleCreateCheckoutSession(nil, nil, sessionData)
		require.NoError(t, err)

		session, ok := result.(*api.CheckoutSession)
		require.True(t, ok)
		assert.Equal(t, "payment", string(session.Mode))
		assert.NotZero(t, session.Id)
	})

	t.Run("Create customer, update metadata, retrieve customer", func(t *testing.T) {
		// Create a customer
		customerData := map[string]any{
			"name":  "Integration Customer",
			"email": "integration@example.com",
		}
		status, result, err := s.handleCreateCustomer(nil, nil, customerData)
		require.NoError(t, err)
		customer := result.(*api.Customer)

		// Update customer with metadata
		updateData := map[string]any{
			"metadata": map[string]any{
				"tier":     "premium",
				"source":   "integration",
				"verified": "true",
			},
		}

		status, result, err = s.handleUpdateCustomer(nil, map[string]string{"id": customer.Id}, updateData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		// Retrieve and verify
		status, result, err = s.handleRetrieveCustomer(nil, map[string]string{"id": customer.Id}, nil)
		require.NoError(t, err)

		retrieved, ok := result.(*api.Customer)
		require.True(t, ok)
		assert.Equal(t, "premium", (*retrieved.Metadata)["tier"])
		assert.Equal(t, "integration", (*retrieved.Metadata)["source"])
		assert.Equal(t, "true", (*retrieved.Metadata)["verified"])
	})

	t.Run("Create customer, create billing portal session", func(t *testing.T) {
		// Create a customer
		customerData := map[string]any{
			"name":  "Portal Customer",
			"email": "portal@example.com",
		}
		status, result, err := s.handleCreateCustomer(nil, nil, customerData)
		require.NoError(t, err)
		customer := result.(*api.Customer)

		// Create billing portal session
		sessionData := map[string]any{
			"customer":   customer.Id,
			"return_url": "https://example.com/return",
		}

		status, result, err = s.handleCreateBillingPortalSession(nil, nil, sessionData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		session, ok := result.(*api.BillingPortalSession)
		require.True(t, ok)
		assert.Equal(t, customer.Id, session.Customer)
		assert.Equal(t, "https://example.com/return", *session.ReturnUrl)
	})

	t.Run("Create billing portal configuration with all features", func(t *testing.T) {
		configData := map[string]any{
			"business_profile": map[string]any{
				"headline":            "Full Features",
				"privacy_policy_url":  "https://example.com/privacy",
				"terms_of_service_url": "https://example.com/terms",
			},
			"features": map[string]any{
				"invoice_history": map[string]any{
					"enabled":       true,
					"default_limit": 50,
				},
				"customer_update": map[string]any{
					"enabled":         true,
					"allowed_updates": []any{"address", "email", "name", "phone"},
				},
				"payment_method_update": map[string]any{
					"enabled":    true,
					"cancel_url": "https://example.com/cancel",
				},
			},
			"default_return_url": "https://example.com",
		}

		status, result, err := s.handleCreateBillingPortalConfiguration(nil, nil, configData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		config, ok := result.(*api.BillingPortalConfiguration)
		require.True(t, ok)
		assert.NotNil(t, config.BusinessProfile)
		assert.NotNil(t, config.Features)
		assert.True(t, config.Features.InvoiceHistory.Enabled)
		assert.True(t, config.Features.CustomerUpdate.Enabled)
		assert.True(t, config.Features.PaymentMethodUpdate.Enabled)
		assert.Len(t, config.Features.CustomerUpdate.AllowedUpdates, 4)
	})
}

// Test Handler Edge Cases
func TestHandlerEdgeCases(t *testing.T) {
	s := setupTestServer(t, false)

	t.Run("CreateCustomer with empty metadata", func(t *testing.T) {
		data := map[string]any{
			"name":     "Customer",
			"metadata": map[string]any{},
		}

		status, result, err := s.handleCreateCustomer(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		customer, ok := result.(*api.Customer)
		require.True(t, ok)
		// Empty metadata map doesn't create a field
		assert.Nil(t, customer.Metadata)
	})

	t.Run("UpdateCustomer with no changes", func(t *testing.T) {
		createData := map[string]any{
			"name":  "Test",
			"email": "test@example.com",
		}
		status, result, err := s.handleCreateCustomer(nil, nil, createData)
		require.NoError(t, err)
		customer := result.(*api.Customer)

		updateData := map[string]any{}
		status, result, err = s.handleUpdateCustomer(nil, map[string]string{"id": customer.Id}, updateData)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		updated, ok := result.(*api.Customer)
		require.True(t, ok)
		assert.Equal(t, *customer.Name, *updated.Name)
		assert.Equal(t, *customer.Email, *updated.Email)
	})

	t.Run("CreatePrice without recurring is one_time", func(t *testing.T) {
		data := map[string]any{
			"currency":    "usd",
			"unit_amount": 1000,
		}

		status, result, err := s.handleCreatePrice(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		price, ok := result.(*api.Price)
		require.True(t, ok)
		assert.Equal(t, "one_time", string(price.Type))
		assert.Nil(t, price.Recurring)
	})

	t.Run("CreateBillingPortalConfiguration with minimal data", func(t *testing.T) {
		data := map[string]any{
			"features": map[string]any{
				"invoice_history": map[string]any{
					"enabled": true,
				},
			},
		}

		status, result, err := s.handleCreateBillingPortalConfiguration(nil, nil, data)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		config, ok := result.(*api.BillingPortalConfiguration)
		require.True(t, ok)
		assert.NotNil(t, config.Features)
		assert.True(t, config.Features.InvoiceHistory.Enabled)
		assert.Empty(t, config.Name)
	})
}

// Helper function
func strPtr(s string) *string {
	return &s
}

// TestListHandlers tests all list endpoint handlers
func TestListHandlers(t *testing.T) {
	t.Run("ListProducts - empty list", func(t *testing.T) {
		s := setupTestServer(t, false)

		status, result, err := s.handleListProducts(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		assert.Equal(t, false, list["has_more"])
		data, ok := list["data"].([]api.Product)
		require.True(t, ok)
		assert.Empty(t, data)
	})

	t.Run("ListProducts - with data", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create some products
		for i := 0; i < 3; i++ {
			data := map[string]any{"name": fmt.Sprintf("Product %d", i)}
			_, _, err := s.handleCreateProduct(nil, nil, data)
			require.NoError(t, err)
		}

		status, result, err := s.handleListProducts(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		data, ok := list["data"].([]api.Product)
		require.True(t, ok)
		assert.Len(t, data, 3)
	})

	t.Run("ListPrices - empty list", func(t *testing.T) {
		s := setupTestServer(t, false)

		status, result, err := s.handleListPrices(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		assert.Equal(t, false, list["has_more"])
		data, ok := list["data"].([]api.Price)
		require.True(t, ok)
		assert.Empty(t, data)
	})

	t.Run("ListPrices - with data", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create some prices
		for i := 0; i < 3; i++ {
			data := map[string]any{
				"currency":    "usd",
				"unit_amount": 1000 * (i + 1),
			}
			_, _, err := s.handleCreatePrice(nil, nil, data)
			require.NoError(t, err)
		}

		status, result, err := s.handleListPrices(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		data, ok := list["data"].([]api.Price)
		require.True(t, ok)
		assert.Len(t, data, 3)
	})

	t.Run("ListCustomers - empty list", func(t *testing.T) {
		s := setupTestServer(t, false)

		status, result, err := s.handleListCustomers(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		assert.Equal(t, false, list["has_more"])
		data, ok := list["data"].([]api.Customer)
		require.True(t, ok)
		assert.Empty(t, data)
	})

	t.Run("ListCustomers - with data", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create some customers
		for i := 0; i < 3; i++ {
			data := map[string]any{
				"name":  fmt.Sprintf("Customer %d", i),
				"email": fmt.Sprintf("customer%d@example.com", i),
			}
			_, _, err := s.handleCreateCustomer(nil, nil, data)
			require.NoError(t, err)
		}

		status, result, err := s.handleListCustomers(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		data, ok := list["data"].([]api.Customer)
		require.True(t, ok)
		assert.Len(t, data, 3)
	})

	t.Run("ListSubscriptions - empty list", func(t *testing.T) {
		s := setupTestServer(t, false)

		status, result, err := s.handleListSubscriptions(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		assert.Equal(t, false, list["has_more"])
		data, ok := list["data"].([]api.Subscription)
		require.True(t, ok)
		assert.Empty(t, data)
	})

	t.Run("ListSubscriptions - with data", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create subscriptions directly via gateway
		for i := 0; i < 3; i++ {
			sub := &api.Subscription{
				Status: api.SubscriptionStatusActive,
			}
			_, err := s.gateway.CreateSubscription(sub)
			require.NoError(t, err)
		}

		status, result, err := s.handleListSubscriptions(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		data, ok := list["data"].([]api.Subscription)
		require.True(t, ok)
		assert.Len(t, data, 3)
	})

	t.Run("ListInvoices - empty list", func(t *testing.T) {
		s := setupTestServer(t, false)

		status, result, err := s.handleListInvoices(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		assert.Equal(t, false, list["has_more"])
		data, ok := list["data"].([]api.Invoice)
		require.True(t, ok)
		assert.Empty(t, data)
	})

	t.Run("ListInvoices - with data", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create invoices directly via gateway
		for i := 0; i < 3; i++ {
			status := api.InvoiceStatusDraft
			invoice := &api.Invoice{
				Status: &status,
			}
			_, err := s.gateway.CreateInvoice(invoice)
			require.NoError(t, err)
		}

		status, result, err := s.handleListInvoices(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		data, ok := list["data"].([]api.Invoice)
		require.True(t, ok)
		assert.Len(t, data, 3)
	})

	t.Run("ListCharges - empty list", func(t *testing.T) {
		s := setupTestServer(t, false)

		status, result, err := s.handleListCharges(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		assert.Equal(t, false, list["has_more"])
		data, ok := list["data"].([]api.Charge)
		require.True(t, ok)
		assert.Empty(t, data)
	})

	t.Run("ListCharges - with data", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create charges directly via gateway
		for i := 0; i < 3; i++ {
			charge := &api.Charge{
				Amount:   1000 * (i + 1),
				Currency: "usd",
			}
			_, err := s.gateway.CreateCharge(charge)
			require.NoError(t, err)
		}

		status, result, err := s.handleListCharges(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		data, ok := list["data"].([]api.Charge)
		require.True(t, ok)
		assert.Len(t, data, 3)
	})

	t.Run("ListCheckoutSessions - empty list", func(t *testing.T) {
		s := setupTestServer(t, false)

		status, result, err := s.handleListCheckoutSessions(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		assert.Equal(t, false, list["has_more"])
		data, ok := list["data"].([]api.CheckoutSession)
		require.True(t, ok)
		assert.Empty(t, data)
	})

	t.Run("ListCheckoutSessions - with data", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create checkout sessions
		for i := 0; i < 3; i++ {
			data := map[string]any{
				"mode":        "payment",
				"success_url": fmt.Sprintf("https://example.com/success/%d", i),
			}
			_, _, err := s.handleCreateCheckoutSession(nil, nil, data)
			require.NoError(t, err)
		}

		status, result, err := s.handleListCheckoutSessions(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		data, ok := list["data"].([]api.CheckoutSession)
		require.True(t, ok)
		assert.Len(t, data, 3)
	})

	t.Run("ListBillingPortalConfigurations - empty list", func(t *testing.T) {
		s := setupTestServer(t, false)

		status, result, err := s.handleListBillingPortalConfigurations(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		assert.Equal(t, false, list["has_more"])
		data, ok := list["data"].([]api.BillingPortalConfiguration)
		require.True(t, ok)
		assert.Empty(t, data)
	})

	t.Run("ListBillingPortalConfigurations - with data", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create billing portal configurations
		for i := 0; i < 3; i++ {
			data := map[string]any{
				"features": map[string]any{
					"invoice_history": map[string]any{
						"enabled": true,
					},
				},
			}
			_, _, err := s.handleCreateBillingPortalConfiguration(nil, nil, data)
			require.NoError(t, err)
		}

		status, result, err := s.handleListBillingPortalConfigurations(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		data, ok := list["data"].([]api.BillingPortalConfiguration)
		require.True(t, ok)
		assert.Len(t, data, 3)
	})

	t.Run("ListWebhookEndpoints - empty list", func(t *testing.T) {
		s := setupTestServer(t, false)

		status, result, err := s.handleListWebhookEndpoints(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		assert.Equal(t, false, list["has_more"])
		data, ok := list["data"].([]*api.WebhookEndpoint)
		require.True(t, ok)
		assert.Empty(t, data)
	})

	t.Run("ListWebhookEndpoints - with data", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create webhook endpoints
		for i := 0; i < 3; i++ {
			data := map[string]any{
				"url":     fmt.Sprintf("https://example.com/webhook/%d", i),
				"enabled_events": []string{"*"},
			}
			_, _, err := s.handleCreateWebhookEndpoint(nil, nil, data)
			require.NoError(t, err)
		}

		status, result, err := s.handleListWebhookEndpoints(nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, status)

		list, ok := result.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "list", list["object"])
		data, ok := list["data"].([]*api.WebhookEndpoint)
		require.True(t, ok)
		assert.Len(t, data, 3)
	})
}

// TestSubscriptionCancellation tests subscription cancellation scenarios
func TestSubscriptionCancellation(t *testing.T) {
	t.Run("Immediate cancellation - DELETE subscription", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create an active subscription directly
		sub := createTestSubscription(s.gateway, "cus_test", "price_test")

		// Cancel the subscription
		pathParams := map[string]string{"id": sub.Id}
		status, result, err := s.handleCancelSubscription(nil, pathParams, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		canceled := result.(*api.Subscription)
		assert.Equal(t, api.SubscriptionStatusCanceled, canceled.Status)
		assert.True(t, canceled.CancelAtPeriodEnd)
		assert.NotNil(t, canceled.CanceledAt)
	})

	t.Run("Cancel at period end - PUT with cancel_at_period_end=true", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create an active subscription directly
		sub := createTestSubscription(s.gateway, "cus_test", "price_test")

		// Schedule cancellation at period end
		pathParams := map[string]string{"id": sub.Id}
		updateData := map[string]any{
			"cancel_at_period_end": true,
		}
		status, result, err := s.handleUpdateSubscription(nil, pathParams, updateData)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		updated := result.(*api.Subscription)
		assert.Equal(t, api.SubscriptionStatusActive, updated.Status) // Still active
		assert.True(t, updated.CancelAtPeriodEnd)
		assert.NotNil(t, updated.CanceledAt)
	})

	t.Run("Expire scheduled cancellation", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create an active subscription directly
		sub := createTestSubscription(s.gateway, "cus_test", "price_test")

		// Schedule cancellation at period end
		pathParams := map[string]string{"id": sub.Id}
		updateData := map[string]any{
			"cancel_at_period_end": true,
		}
		status, _, err := s.handleUpdateSubscription(nil, pathParams, updateData)
		require.NoError(t, err)

		// Expire the subscription (simulate period end)
		status, result, err := s.handleExpireSubscription(nil, pathParams, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		expired := result.(*api.Subscription)
		assert.Equal(t, api.SubscriptionStatusCanceled, expired.Status)
		assert.False(t, expired.CancelAtPeriodEnd) // Cleared after expiration
	})

	t.Run("Cancel non-existent subscription", func(t *testing.T) {
		s := setupTestServer(t, false)

		pathParams := map[string]string{"id": "sub_nonexistent"}
		status, _, err := s.handleCancelSubscription(nil, pathParams, nil)
		require.Equal(t, http.StatusNotFound, status)
		require.Error(t, err)
	})

	t.Run("Expire subscription not scheduled for cancellation", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create an active subscription (not scheduled for cancellation)
		sub := createTestSubscription(s.gateway, "cus_test", "price_test")

		// Try to expire without scheduling
		pathParams := map[string]string{"id": sub.Id}
		status, _, err := s.handleExpireSubscription(nil, pathParams, nil)
		require.Equal(t, http.StatusBadRequest, status)
		require.Error(t, err)
	})
}

// createTestSubscription creates an active subscription for testing
func createTestSubscription(gw *gateway.Gateway, customerID, priceID string) *api.Subscription {
	now := int(time.Now().Unix())
	id := "sub_test_" + generator.RandomString(8)

	var cust api.Subscription_Customer
	_ = cust.FromSubscriptionCustomer0(customerID)

	sub := &api.Subscription{
		Id:                id,
		Object:            api.SubscriptionObjectEnumSubscription,
		Status:            api.SubscriptionStatusActive,
		Created:           now,
		Livemode:          false,
		Customer:          cust,
		CancelAtPeriodEnd: false,
	}

	// Add subscription item with period
	sub.Items.Data = []api.SubscriptionItem{
		{
			Id:                 "si_test_" + generator.RandomString(8),
			Object:             api.SubscriptionItemObjectEnumSubscriptionItem,
			Created:            now,
			CurrentPeriodStart: now,
			CurrentPeriodEnd:   now + 2592000, // 30 days
			Price: api.Price{
				Id:     priceID,
				Object: api.PriceObjectEnumPrice,
			},
		},
	}

  created, _ := gw.CreateSubscription(sub)
	// Ensure status is active
	created.Status = api.SubscriptionStatusActive
	gw.UpdateSubscription(created.Id, created)
	return created
}

// TestSubscriptionPartialUpdates tests partial update support
func TestSubscriptionPartialUpdates(t *testing.T) {
	t.Run("Update only cancel_at_period_end", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create an active subscription
		sub := createTestSubscription(s.gateway, "cus_test", "price_test")
		originalItemID := sub.Items.Data[0].Id

		// Update only cancel_at_period_end (no items)
		pathParams := map[string]string{"id": sub.Id}
		updateData := map[string]any{
			"cancel_at_period_end": true,
		}
		status, result, err := s.handleUpdateSubscription(nil, pathParams, updateData)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		updated := result.(*api.Subscription)
		assert.True(t, updated.CancelAtPeriodEnd)
		assert.Equal(t, originalItemID, updated.Items.Data[0].Id) // Items unchanged
	})

	t.Run("Update only items", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create an active subscription
		sub := createTestSubscription(s.gateway, "cus_test", "price_test")

		// Update only items (no cancel_at_period_end)
		pathParams := map[string]string{"id": sub.Id}
		updateData := map[string]any{
			"items": []map[string]any{
				{"id": sub.Items.Data[0].Id, "price": "price_new"},
			},
		}
		status, result, err := s.handleUpdateSubscription(nil, pathParams, updateData)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		updated := result.(*api.Subscription)
		assert.False(t, updated.CancelAtPeriodEnd) // Unchanged
		assert.Equal(t, "price_new", updated.Items.Data[0].Price.Id)
	})

	t.Run("Update both cancel_at_period_end and items", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create an active subscription
		sub := createTestSubscription(s.gateway, "cus_test", "price_test")

		// Update both in one request
		pathParams := map[string]string{"id": sub.Id}
		updateData := map[string]any{
			"cancel_at_period_end": true,
			"items": []map[string]any{
				{"id": sub.Items.Data[0].Id, "price": "price_new"},
			},
		}
		status, result, err := s.handleUpdateSubscription(nil, pathParams, updateData)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		updated := result.(*api.Subscription)
		assert.True(t, updated.CancelAtPeriodEnd)
		assert.Equal(t, "price_new", updated.Items.Data[0].Price.Id)
	})

	t.Run("Cancel scheduled cancellation with false", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create subscription with cancel_at_period_end=true
		sub := createTestSubscription(s.gateway, "cus_test", "price_test")
		pathParams := map[string]string{"id": sub.Id}
		updateData := map[string]any{"cancel_at_period_end": true}
		s.handleUpdateSubscription(nil, pathParams, updateData)

		// Cancel the scheduled cancellation
		updateData = map[string]any{"cancel_at_period_end": false}
		status, result, err := s.handleUpdateSubscription(nil, pathParams, updateData)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		updated := result.(*api.Subscription)
		assert.False(t, updated.CancelAtPeriodEnd)
	})
}

// TestExpandSubscription tests expand parameter support
func TestExpandSubscription(t *testing.T) {
	t.Run("Expand items.data.price.product", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create a product
		product := &api.Product{
			Object:   api.ProductObjectEnumProduct,
			Livemode: false,
			Name:     "Test Product",
			Metadata: map[string]string{"plan_id": "plan_123"},
		}
		createdProduct, err := s.gateway.CreateProduct(product)
		require.NoError(t, err)
		productID := createdProduct.Id

		// Create a price with this product
		price := &api.Price{
			Object:   api.PriceObjectEnumPrice,
			Livemode: false,
			Currency: "usd",
		}
		price.Product.FromProduct(*createdProduct)
		createdPrice, err := s.gateway.CreatePrice(price)
		require.NoError(t, err)
		priceID := createdPrice.Id

		// Create a subscription with this price
		sub := createTestSubscription(s.gateway, "cus_test", priceID)
		// Update the subscription's price to have the full product reference
		sub.Items.Data[0].Price = *createdPrice
		s.gateway.UpdateSubscription(sub.Id, sub)

		// Create request with expand parameter
		req := httptest.NewRequest("GET", "/v1/subscriptions/"+sub.Id+"?expand[]=items.data.price.product", nil)
		pathParams := map[string]string{"id": sub.Id}

		status, result, err := s.handleRetrieveSubscription(req, pathParams, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		retrieved := result.(*api.Subscription)
		assert.Len(t, retrieved.Items.Data, 1)

		// Verify product is expanded (not just an ID string)
		expandedProduct, err := retrieved.Items.Data[0].Price.Product.AsProduct()
		require.NoError(t, err)
		assert.Equal(t, productID, expandedProduct.Id)
		assert.Equal(t, "Test Product", expandedProduct.Name)
		assert.Equal(t, "plan_123", expandedProduct.Metadata["plan_id"])
	})

	t.Run("Expand customer", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create a customer with email
		m := map[string]string{}
		customer := &api.Customer{
			Object:   api.CustomerObjectEnumCustomer,
			Livemode: false,
			Email:    strPtr("test@example.com"),
			Metadata: &m,
		}
		createdCustomer, err := s.gateway.CreateCustomer(customer)
		require.NoError(t, err)
		customerID := createdCustomer.Id

		// Create a subscription for this customer
		sub := createTestSubscription(s.gateway, customerID, "price_test")

		// Create request with expand parameter
		req := httptest.NewRequest("GET", "/v1/subscriptions/"+sub.Id+"?expand[]=customer", nil)
		pathParams := map[string]string{"id": sub.Id}

		status, result, err := s.handleRetrieveSubscription(req, pathParams, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		retrieved := result.(*api.Subscription)

		// Verify customer is expanded (not just an ID string)
		expandedCustomer, err := retrieved.Customer.AsCustomer()
		require.NoError(t, err)
		assert.Equal(t, customerID, expandedCustomer.Id)
		assert.NotNil(t, expandedCustomer.Email)
		assert.Equal(t, "test@example.com", *expandedCustomer.Email)
	})

	t.Run("No expand returns ID-only references", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create a subscription
		sub := createTestSubscription(s.gateway, "cus_test", "price_test")

		// Request without expand parameter
		req := httptest.NewRequest("GET", "/v1/subscriptions/"+sub.Id, nil)
		pathParams := map[string]string{"id": sub.Id}

		status, result, err := s.handleRetrieveSubscription(req, pathParams, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		retrieved := result.(*api.Subscription)

		// Verify customer is ID-only (string)
		customerID, err := retrieved.Customer.AsSubscriptionCustomer0()
		require.NoError(t, err)
		assert.Equal(t, "cus_test", customerID)
	})
}

// TestPauseResumeSubscription tests pause and resume subscription handlers
func TestPauseResumeSubscription(t *testing.T) {
	t.Run("Pause active subscription", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create an active subscription
		sub := createTestSubscription(s.gateway, "cus_test", "price_test")
		assert.Equal(t, api.SubscriptionStatusActive, sub.Status)

		// Pause the subscription
		pathParams := map[string]string{"id": sub.Id}
		status, result, err := s.handlePauseSubscription(nil, pathParams, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		paused := result.(*api.Subscription)
		assert.Equal(t, api.SubscriptionStatusPaused, paused.Status)
		assert.Equal(t, sub.Id, paused.Id)
	})

	t.Run("Resume paused subscription", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create and pause a subscription
		sub := createTestSubscription(s.gateway, "cus_test", "price_test")
		pausedSub, err := s.gateway.PauseSubscription(sub.Id)
		require.NoError(t, err)
		assert.Equal(t, api.SubscriptionStatusPaused, pausedSub.Status)

		// Resume the subscription
		pathParams := map[string]string{"id": sub.Id}
		status, result, err := s.handleResumeSubscription(nil, pathParams, nil)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, status)

		resumed := result.(*api.Subscription)
		assert.Equal(t, api.SubscriptionStatusActive, resumed.Status)
		assert.Equal(t, sub.Id, resumed.Id)
	})

	t.Run("Pause non-existent subscription", func(t *testing.T) {
		s := setupTestServer(t, false)

		pathParams := map[string]string{"id": "sub_nonexistent"}
		status, _, err := s.handlePauseSubscription(nil, pathParams, nil)
		assert.Equal(t, http.StatusNotFound, status)
		assert.Error(t, err)
	})

	t.Run("Resume non-existent subscription", func(t *testing.T) {
		s := setupTestServer(t, false)

		pathParams := map[string]string{"id": "sub_nonexistent"}
		status, _, err := s.handleResumeSubscription(nil, pathParams, nil)
		assert.Equal(t, http.StatusNotFound, status)
		assert.Error(t, err)
	})

	t.Run("Pause already paused subscription fails", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create and pause a subscription
		sub := createTestSubscription(s.gateway, "cus_test", "price_test")
		_, err := s.gateway.PauseSubscription(sub.Id)
		require.NoError(t, err)

		// Try to pause again
		pathParams := map[string]string{"id": sub.Id}
		status, _, err := s.handlePauseSubscription(nil, pathParams, nil)
		assert.Equal(t, http.StatusBadRequest, status)
		assert.Error(t, err)
	})

	t.Run("Resume active subscription fails", func(t *testing.T) {
		s := setupTestServer(t, false)

		// Create an active subscription
		sub := createTestSubscription(s.gateway, "cus_test", "price_test")

		// Try to resume an active subscription
		pathParams := map[string]string{"id": sub.Id}
		status, _, err := s.handleResumeSubscription(nil, pathParams, nil)
		assert.Equal(t, http.StatusBadRequest, status)
		assert.Error(t, err)
	})
}
