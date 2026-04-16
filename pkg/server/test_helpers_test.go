package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// WebhookCapture captures webhook events for testing
type WebhookCapture struct {
	Events    *[]map[string]any
	Mutex     *sync.Mutex
	Server    *httptest.Server
	webhookID string
}

// SetupWebhookCapture creates a test server that captures webhook events
func SetupWebhookCapture(t *testing.T, s *Server) *WebhookCapture {
	var receivedEvents []map[string]any
	var eventsMutex sync.Mutex

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event map[string]any
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		eventsMutex.Lock()
		receivedEvents = append(receivedEvents, event)
		eventsMutex.Unlock()
		w.WriteHeader(http.StatusOK)
	}))

	opts := &CreateOpts{
		URL:      testServer.URL,
		Enabled:  []string{"*"},
		Livemode: true,
		Secret:   "whsec_test_secret",
	}

	webhook, err := s.webhook.CreateWebhook(testServer.URL, opts)
	require.NoError(t, err)
	require.NotNil(t, webhook)

	return &WebhookCapture{
		Events:    &receivedEvents,
		Mutex:     &eventsMutex,
		Server:    testServer,
		webhookID: webhook.Id,
	}
}

// Close closes the test server
func (wc *WebhookCapture) Close() {
	wc.Server.Close()
}

// GetEventTypes returns all captured event types
func (wc *WebhookCapture) GetEventTypes() []string {
	wc.Mutex.Lock()
	defer wc.Mutex.Unlock()

	types := make([]string, len(*wc.Events))
	for i, e := range *wc.Events {
		types[i] = e["type"].(string)
	}
	return types
}

// GetEvents returns all captured events
func (wc *WebhookCapture) GetEvents() []map[string]any {
	wc.Mutex.Lock()
	defer wc.Mutex.Unlock()

	events := make([]map[string]any, len(*wc.Events))
	copy(events, *wc.Events)
	return events
}

// GetEventByType returns the first event matching the given type
func (wc *WebhookCapture) GetEventByType(eventType string) (map[string]any, bool) {
	wc.Mutex.Lock()
	defer wc.Mutex.Unlock()

	for _, e := range *wc.Events {
		if e["type"] == eventType {
			return e, true
		}
	}
	return nil, false
}

// CountEventType counts occurrences of a specific event type
func (wc *WebhookCapture) CountEventType(eventType string) int {
	wc.Mutex.Lock()
	defer wc.Mutex.Unlock()

	count := 0
	for _, e := range *wc.Events {
		if e["type"] == eventType {
			count++
		}
	}
	return count
}

// ContainsEventType checks if an event type was captured
func (wc *WebhookCapture) ContainsEventType(eventType string) bool {
	return wc.CountEventType(eventType) > 0
}
