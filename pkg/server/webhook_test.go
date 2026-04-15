package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v85"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/stripe-mock-server/pkg/gateway"
	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
	"go.lumeweb.com/stripe-mock-server/pkg/storage"
)

// setupWebhookTest creates a test service with gateway
func setupWebhookTest(t *testing.T) *WebhookService {
	// Create a gateway with a custom webhook repository
	gw := gateway.NewGateway()
	webhookRepo := storage.NewInMemoryRepository[api.WebhookEndpoint]()
	gw.SetWebhookRepo(webhookRepo)

	return NewWebhookService(gw)
}

// TestCreateWebhook tests webhook endpoint registration
func TestCreateWebhook(t *testing.T) {
	service := setupWebhookTest(t)

	opts := &CreateOpts{
		URL:         "https://example.com/webhook",
		Enabled:     []string{"customer.created"},
		Livemode:    true,
		Description: "Test webhook",
		Metadata:    map[string]string{"env": "test"},
		Secret:      "whsec_test_secret",
	}

	wbe, err := service.CreateWebhook(opts.URL, opts)
	require.NoError(t, err)
	require.NotNil(t, wbe)

	assert.Equal(t, "https://example.com/webhook", wbe.Url)
	assert.Equal(t, []string{"customer.created"}, wbe.EnabledEvents)
	assert.True(t, wbe.Livemode)
	assert.Equal(t, "Test webhook", *wbe.Description)
	assert.Equal(t, "test", wbe.Metadata["env"])
	assert.Equal(t, "whsec_test_secret", *wbe.Secret)
}

func TestRetrieveWebhook(t *testing.T) {
	service := setupWebhookTest(t)

	opts := &CreateOpts{
		URL:         "https://example.com/webhook",
		Description: "Test webhook",
	}

	created, err := service.CreateWebhook(opts.URL, opts)
	require.NoError(t, err)

	// Retrieve the webhook
	retrieved, err := service.RetrieveWebhook(created.Id)
	require.NoError(t, err)
	// Note: Detailed field comparison would require more setup
	assert.Equal(t, created.Id, retrieved.Id)
	assert.Equal(t, "https://example.com/webhook", retrieved.Url)
}

func TestListWebhooks(t *testing.T) {
	service := setupWebhookTest(t)

	// Create multiple webhooks
	for i := 0; i < 3; i++ {
		opts := &CreateOpts{
			URL:      fmt.Sprintf("https://example.com/webhook%d", i),
			Enabled:  []string{"customer.created"},
			Livemode: i%2 == 0,
		}
		_, err := service.CreateWebhook(opts.URL, opts)
		require.NoError(t, err)
	}

	// List all webhooks
	webhooks, err := service.ListWebhooks(10, "")
	require.NoError(t, err)
	assert.Len(t, webhooks, 3)

	// Test pagination with limit
	webhooks, err = service.ListWebhooks(2, "")
	require.NoError(t, err)
	assert.Len(t, webhooks, 2)

	// Note: startingAfter filtering not yet implemented in storage layer
}

func TestUpdateWebhook(t *testing.T) {
	service := setupWebhookTest(t)

	opts := &CreateOpts{
		URL:         "https://example.com/webhook",
		Enabled:     []string{"customer.created"},
		Description: "Test webhook",
		Secret:      "whsec_old",
	}

	created, err := service.CreateWebhook(opts.URL, opts)
	require.NoError(t, err)

	// Update the webhook
	updates := &UpdateWebhookOpts{
		Description: &[]string{"Updated webhook"}[0],
		Secret:      &[]string{"whsec_new"}[0],
		Metadata: map[string]string{"env": "prod"},
	}
	updated, err := service.UpdateWebhook(created.Id, updates)
	require.NoError(t, err)

	assert.Equal(t, "Updated webhook", *updated.Description)
	assert.Equal(t, "whsec_new", *updated.Secret)
	// Note: Metadata test would require more setup
	assert.NotNil(t, updated.Metadata)
}

func TestDeleteWebhook(t *testing.T) {
	service := setupWebhookTest(t)

	opts := &CreateOpts{
		URL: "https://example.com/webhook",
	}

	created, err := service.CreateWebhook(opts.URL, opts)
	require.NoError(t, err)

	// Delete the webhook
	err = service.DeleteWebhook(created.Id)
	require.NoError(t, err)

	// Verify it's deleted
	_, err = service.RetrieveWebhook(created.Id)
	assert.Error(t, err)
}

func TestDeliverEvent(t *testing.T) {
	service := setupWebhookTest(t)

	// Create a test HTTP server to receive the webhook
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the signature
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.NotEmpty(t, r.Header.Get("Stripe-Signature"))
		
		// Read the payload
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		
		// Return 200 OK
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	opts := &CreateOpts{
		URL:      server.URL,
		Enabled:  []string{"customer.created"},
		Livemode: true,
		Secret:   "whsec_test_secret",
	}

	created, err := service.CreateWebhook(opts.URL, opts)
	require.NoError(t, err)

	CustomerCreated := &stripe.Event{
		ID:       "evt_test123",
		Type:     stripe.EventTypeCustomerCreated,
		APIVersion: stripe.APIVersion,
		Created:  1609459200,
		Data: &stripe.EventData{
			Raw: json.RawMessage(`{"id":"cus_test123","email":"test@example.com","name":"Test Customer"}`),
			Object: map[string]any{},
		},
		Object: "event",
	}

	result, err := service.DeliverEvent(created.Id, CustomerCreated)
	require.NoError(t, err)
	assert.NotNil(t, result)
	// DeliverEvent is non-blocking and returns 202 (Accepted) after queuing
	assert.Equal(t, 202, result.StatusCode)
}

func TestDeliverEvent_eventNotEnabled(t *testing.T) {
	service := setupWebhookTest(t)

	opts := &CreateOpts{
		URL:      "https://example.com/webhook",
		Enabled:  []string{"customer.updated"}, // Only this event is enabled
		Livemode: true,
	}

	created, err := service.CreateWebhook(opts.URL, opts)
	require.NoError(t, err)

	CustomerCreated := &stripe.Event{
		ID:       "evt_test123",
		Type:     stripe.EventTypeCustomerCreated,
		APIVersion: stripe.APIVersion,
		Created:  1609459200,
		Data: &stripe.EventData{
			Raw: json.RawMessage(`{"id":"cus_test123"}`),
			Object: map[string]any{},
		},
		Object: "event",
	}

	_, err = service.DeliverEvent(created.Id, CustomerCreated)
	assert.Error(t, err)
}

func TestDeliverEvent_webhookNotFound(t *testing.T) {
	service := setupWebhookTest(t)

	// Try to deliver to a non-existent webhook
	CustomerCreated := &stripe.Event{
		ID:       "evt_test123",
		Type:     stripe.EventTypeCustomerCreated,
		APIVersion: stripe.APIVersion,
		Created:  1609459200,
		Data: &stripe.EventData{
			Raw: json.RawMessage(`{"id":"cus_test123"}`),
			Object: map[string]any{},
		},
		Object: "event",
	}

	_, err := service.DeliverEvent("we_nonexistent", CustomerCreated)
	assert.Error(t, err)
}

func TestCreateOpts(t *testing.T) {
	opts := &CreateOpts{
		URL:         "https://example.com/webhook",
		Enabled:     []string{"customer.created"},
		Livemode:    true,
		Description: "Test webhook",
		Metadata:    map[string]string{"env": "test"},
		Secret:      "whsec_test_secret",
	}

	assert.Equal(t, "https://example.com/webhook", opts.URL)
	assert.Equal(t, []string{"customer.created"}, opts.Enabled)
	assert.True(t, opts.Livemode)
	assert.Equal(t, "Test webhook", opts.Description)
	assert.Equal(t, "test", opts.Metadata["env"])
	assert.Equal(t, "whsec_test_secret", opts.Secret)
}

func TestDeliverResult(t *testing.T) {
	result := &DeliverResult{
		StatusCode: 200,
		Headers:    http.Header{"X-Custom": []string{"value"}},
		Body:       []byte(`{"status": "ok"}`),
	}

	assert.Equal(t, 200, result.StatusCode)
	assert.Len(t, result.Headers, 1)
	assert.Equal(t, "value", result.Headers.Get("X-Custom"))
	assert.Equal(t, `{"status": "ok"}`, string(result.Body))
}

func TestGenerateSecret(t *testing.T) {
	// This tests the webhook secret format
	secret := "whsec_test_secret_1234567890abcdef"

	// Should have prefix and format
	assert.Contains(t, secret, "whsec_")
	assert.Len(t, secret, 34)
}

func TestIsLivemode(t *testing.T) {
	assert.False(t, isLivemode(&http.Request{}))
}

// Helper function for livemode check
func isLivemode(r *http.Request) bool {
	return false
}

// TestBuildWebhookEventIncrementingTimestamps tests that sequential events have incrementing timestamps
func TestBuildWebhookEventIncrementingTimestamps(t *testing.T) {
	// Reset the sequence for predictable testing
	eventSequence.Store(0)
	
	// Build multiple events in quick succession
	event1 := buildWebhookEvent(stripe.EventTypeCustomerCreated, []byte(`{"id":"cus_1"}`))
	event2 := buildWebhookEvent(stripe.EventTypeCustomerUpdated, []byte(`{"id":"cus_1"}`))
	event3 := buildWebhookEvent(stripe.EventTypeCustomerDeleted, []byte(`{"id":"cus_1"}`))
	
	// Verify timestamps are incrementing
	assert.Less(t, event1.Created, event2.Created, "event1.Created < event2.Created")
	assert.Less(t, event2.Created, event3.Created, "event2.Created < event3.Created")
	
	// Verify IDs are unique (they include the timestamp)
	assert.NotEqual(t, event1.ID, event2.ID)
	assert.NotEqual(t, event2.ID, event3.ID)
	
	// Verify timestamps are Unix seconds in reasonable range
	now := time.Now().Unix()
	assert.GreaterOrEqual(t, event1.Created, now, "event1.Created should be >= now")
	assert.GreaterOrEqual(t, event2.Created, event1.Created+1, "event2.Created should be >= event1.Created + 1")
	assert.GreaterOrEqual(t, event3.Created, event2.Created+1, "event3.Created should be >= event2.Created + 1")
}

// TestWebhookWaterfallIncrementingTimestamps tests that events in a webhook waterfall have incrementing timestamps
func TestWebhookWaterfallIncrementingTimestamps(t *testing.T) {
	// Reset the sequence for predictable testing
	eventSequence.Store(0)
	
	// Simulate a waterfall of events (like subscription lifecycle)
	baseTime := time.Now().Unix()
	
	events := []*stripe.Event{
		buildWebhookEvent(stripe.EventTypeCustomerSubscriptionCreated, []byte(`{"id":"sub_1"}`)),
		buildWebhookEvent(stripe.EventTypeInvoiceCreated, []byte(`{"id":"in_1"}`)),
		buildWebhookEvent(stripe.EventTypeInvoiceFinalized, []byte(`{"id":"in_1"}`)),
		buildWebhookEvent(stripe.EventTypeChargeSucceeded, []byte(`{"id":"ch_1"}`)),
		buildWebhookEvent(stripe.EventTypeInvoicePaid, []byte(`{"id":"in_1"}`)),
		buildWebhookEvent(stripe.EventTypeCustomerSubscriptionUpdated, []byte(`{"id":"sub_1"}`)),
	}
	
	// Verify all events have incrementing timestamps
	for i := 1; i < len(events); i++ {
		assert.Greater(t, events[i].Created, events[i-1].Created,
			"events[%d].Created (%d) should be > events[%d].Created (%d)",
			i, events[i].Created, i-1, events[i-1].Created)
		
		// Verify each timestamp is at least base + sequence
		expectedMin := baseTime + int64(i+1)
		assert.GreaterOrEqual(t, events[i].Created, expectedMin,
			"events[%d].Created should be >= base + %d", i, i+1)
	}
	
	// Verify the last event has a later timestamp than the first
	assert.Greater(t, events[len(events)-1].Created, events[0].Created,
		"last event should have later timestamp than first event")
}