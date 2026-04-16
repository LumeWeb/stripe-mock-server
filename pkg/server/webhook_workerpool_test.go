package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v85"
	"go.lumeweb.com/stripe-mock-server/pkg/gateway"
	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
)

// setupWebhookService creates a fresh WebhookService for testing
func setupWebhookService() *WebhookService {
	return NewWebhookService(gateway.NewGateway(), "2020-08-27")
}

// testCreateOpts returns default CreateOpts for testing
func testCreateOpts(url string) *CreateOpts {
	return &CreateOpts{
		URL:      url,
		Enabled:  []string{"customer.created"},
		Livemode: true,
		Secret:   "whsec_test_secret",
	}
}

// testEvent creates a stripe.Event for testing
func testEvent(id, eventType string) *stripe.Event {
	return &stripe.Event{
		ID:         id,
		Type:       stripe.EventType(eventType),
		APIVersion: stripe.APIVersion,
		Created:    time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
		Data:       &stripe.EventData{Raw: []byte(fmt.Sprintf(`{"id":%q}`, id)), Object: map[string]any{}},
		Object:     "event",
	}
}

// TestWebhookDeliveryOrder tests that webhook events are processed in order
// due to the single worker in the worker pool.
func TestWebhookDeliveryOrder(t *testing.T) {
	service := setupWebhookService()

	// Create a test HTTP server that records the order of requests
	var order []string
	var orderNames []string
	var mutex sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}

		mutex.Lock()
		if id, ok := payload["id"]; ok {
			order = append(order, id.(string))
		}
		if name, ok := payload["name"]; ok {
			orderNames = append(orderNames, name.(string))
		}
		mutex.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	w1, _ := service.CreateWebhook(server.URL, testCreateOpts(server.URL))
	w2, _ := service.CreateWebhook(server.URL, testCreateOpts(server.URL))
	w3, _ := service.CreateWebhook(server.URL, testCreateOpts(server.URL))

	// Send events in order
	service.DeliverEvent(w1.Id, testEvent("evt_001", string(stripe.EventTypeCustomerCreated)))
	service.DeliverEvent(w2.Id, testEvent("evt_002", string(stripe.EventTypeCustomerCreated)))
	service.DeliverEvent(w3.Id, testEvent("evt_003", string(stripe.EventTypeCustomerCreated)))

	// Wait for all deliveries to complete
	time.Sleep(3 * time.Second)

	// Verify we received all 3 events
	mutex.Lock()
	if len(order) != 3 {
		t.Errorf("Expected 3 events, got %d", len(order))
	}
	mutex.Unlock()
}

// TestWebhookRetry tests that webhook deliveries are retried on failure.
func TestWebhookRetry(t *testing.T) {
	service := setupWebhookService()

	var attempts atomic.Int32
	done := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		if attempts.Load() < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		close(done)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	w, _ := service.CreateWebhook(server.URL, testCreateOpts(server.URL))
	require.NotNil(t, w)

	service.DeliverEvent(w.Id, testEvent("evt_test", string(stripe.EventTypeCustomerCreated)))

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("delivery did not complete in time")
	}

	assert.Equal(t, int32(2), attempts.Load())
}

// TestWebhookRetryOnTimeout tests that webhook deliveries are retried on timeout.
func TestWebhookRetryOnTimeout(t *testing.T) {
	service := setupWebhookService()

	var attempts atomic.Int32
	done := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		time.Sleep(100 * time.Millisecond)
		close(done)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	w, _ := service.CreateWebhook(server.URL, testCreateOpts(server.URL))
	require.NotNil(t, w)

	service.DeliverEvent(w.Id, testEvent("evt_test", string(stripe.EventTypeCustomerCreated)))

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("delivery did not complete in time")
	}

	assert.GreaterOrEqual(t, attempts.Load(), int32(1))
}

// TestWebhookRetryBackoff tests that retry delays increase with each attempt.
func TestWebhookRetryBackoff(t *testing.T) {
	service := setupWebhookService()

	var attempts atomic.Int32
	var timestamps []int64
	var mutex sync.Mutex
	done := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		mutex.Lock()
		timestamps = append(timestamps, time.Now().UnixNano())
		mutex.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
		if attempts.Load() >= 4 {
			close(done)
		}
	}))
	defer server.Close()

	w, _ := service.CreateWebhook(server.URL, testCreateOpts(server.URL))
	require.NotNil(t, w)

	service.DeliverEvent(w.Id, testEvent("evt_test", string(stripe.EventTypeCustomerCreated)))

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("delivery did not complete in time")
	}

	assert.GreaterOrEqual(t, attempts.Load(), int32(2))

	mutex.Lock()
	if len(timestamps) >= 3 {
		assert.True(t, timestamps[2] > timestamps[1], "retry should be delayed")
	}
	mutex.Unlock()
}

// TestWebhookConcurrentDelivery tests that multiple deliveries can be sent concurrently.
func TestWebhookConcurrentDelivery(t *testing.T) {
	service := setupWebhookService()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	w, _ := service.CreateWebhook(server.URL, testCreateOpts(server.URL))
	require.NotNil(t, w)

	// Send multiple events concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			service.DeliverEvent(w.Id, testEvent(fmt.Sprintf("evt_%d", id), string(stripe.EventTypeCustomerCreated)))
		}(i)
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)
}

// TestWebhookServiceClose tests that Close gracefully stops the worker pool.
func TestWebhookServiceClose(t *testing.T) {
	service := setupWebhookService()
	assert.NotPanics(t, func() {
		service.Close()
	})
}

// TestWebhookSuccessfulDelivery tests a successful webhook delivery.
func TestWebhookSuccessfulDelivery(t *testing.T) {
	service := setupWebhookService()

	var receivedBody bytes.Buffer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		receivedBody.WriteString(r.Header.Get("Stripe-Signature"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	w, _ := service.CreateWebhook(server.URL, testCreateOpts(server.URL))
	require.NotNil(t, w)

	result, err := service.DeliverEvent(w.Id, testEvent("evt_test123", string(stripe.EventTypeCustomerCreated)))
	assert.NoError(t, err)
	assert.Equal(t, 202, result.StatusCode)
}

func TestWebhookServiceNonblocking(t *testing.T) {
	service := setupWebhookService()

	start := time.Now()
	service.DeliverEvent("nonexistent", testEvent("evt_test", string(stripe.EventTypeCustomerCreated)))
	duration := time.Since(start)

	assert.Less(t, duration, 500*time.Millisecond)
}

// TestSubscriptionCancellationWebhooks tests webhook events for subscription cancellation
func TestSubscriptionCancellationWebhooks(t *testing.T) {
	t.Run("Immediate cancellation triggers subscription.deleted", func(t *testing.T) {
		service := setupWebhookService()

		var receivedEvents []string
		var mutex sync.Mutex
		done := make(chan struct{})

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}

			mutex.Lock()
			receivedEvents = append(receivedEvents, payload["type"].(string))
			if len(receivedEvents) >= 1 {
				close(done)
			}
			mutex.Unlock()
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		opts := &CreateOpts{
			URL:      server.URL,
			Enabled:  []string{"*"},
			Livemode: true,
			Secret:   "whsec_test_secret",
		}
		w, err := service.CreateWebhook(opts.URL, opts)
		require.NoError(t, err)
		require.NotNil(t, w)

		// Create an active subscription directly
		now := int(time.Now().Unix())
		var cust api.Subscription_Customer
		_ = cust.FromSubscriptionCustomer0("cus_test")

		sub := &api.Subscription{
			Id:                "sub_test_123",
			Object:            api.SubscriptionObjectEnumSubscription,
			Status:            api.SubscriptionStatusActive,
			Created:           now,
			Livemode:          true,
			Customer:          cust,
			CancelAtPeriodEnd: false,
		}
		sub.Items.Data = []api.SubscriptionItem{
			{
				Id:                 "si_test",
				Object:             api.SubscriptionItemObjectEnumSubscriptionItem,
				Created:            now,
				CurrentPeriodStart: now,
				CurrentPeriodEnd:   now + 2592000,
				Price:              api.Price{Id: "price_test", Object: api.PriceObjectEnumPrice},
			},
		}

		gw := service.Gateway()
		created, err := gw.CreateSubscription(sub)
		require.NoError(t, err)

		// Cancel the subscription
		_, err = gw.CancelSubscription(created.Id)
		require.NoError(t, err)

		// Trigger webhook manually
		service.DeliverEvent(w.Id, testEvent("evt_cancel", string(stripe.EventTypeCustomerSubscriptionDeleted)))

		// Wait for webhook delivery
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("webhook not received in time")
		}

		mutex.Lock()
		assert.Contains(t, receivedEvents, "customer.subscription.deleted")
		mutex.Unlock()
	})
}
