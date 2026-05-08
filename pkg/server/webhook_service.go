package server

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/avast/retry-go/v5"
	"github.com/gammazero/workerpool"
	"github.com/stripe/stripe-go/v85"
	"github.com/stripe/stripe-go/v85/webhook"
	"go.uber.org/zap"
	"go.lumeweb.com/stripe-mock-server/pkg/gateway"
	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
)

const (
	webhookIdPrefix       = "we_"
	eventIdPrefix         = "evt_"
	maxAttempts           = 5
	WebhookStatusEnabled  = "enabled"
	WebhookStatusDisabled = "disabled"
)

const (
	defaultApiVersion = stripe.APIVersion
)

// eventSequence is a global counter for generating incrementing event timestamps
// This ensures events in a waterfall have incrementing Created timestamps for sorting
var eventSequence atomic.Int64

// WebhookService manages webhook endpoints and delivers events to them
type WebhookService struct {
	gateway    *gateway.Gateway
	workerPool *workerpool.WorkerPool
	apiVersion string
}

// Gateway returns the gateway used by the webhook service (for testing)
func (s *WebhookService) Gateway() *gateway.Gateway {
	return s.gateway
}

// WebhookDeliveryTask represents a webhook delivery job
type WebhookDeliveryTask struct {
	webhookId string
	Event     *stripe.Event
}

// NewWebhookService creates a new webhook service with a default API version
func NewWebhookService(gw *gateway.Gateway, apiVersion string) *WebhookService {
	// Create worker pool with 1 worker to ensure sequential processing
	wp := workerpool.New(1)

	return &WebhookService{
		gateway:    gw,
		workerPool: wp,
		apiVersion: apiVersion,
	}
}

// CreateWebhook registers a new webhook endpoint
func (s *WebhookService) CreateWebhook(url string, opts *CreateOpts) (*api.WebhookEndpoint, error) {
	wbe := &api.WebhookEndpoint{
		Id:            generateWebhookId(),
		Url:           url,
		EnabledEvents: opts.Enabled,
		Status:        WebhookStatusEnabled,
		Livemode:      opts.Livemode,
		Created:       int(time.Now().Unix()),
	}

	if opts.Description != "" {
		wbe.Description = &opts.Description
	}
	if opts.Metadata != nil {
		wbe.Metadata = opts.Metadata
	}
	if opts.Secret != "" {
		wbe.Secret = &opts.Secret
	}

	if opts.Enabled == nil {
		wbe.EnabledEvents = []string{"*"}
	}

	return s.gateway.CreateWebhookEndpoint(wbe)
}

// RetrieveWebhook gets a webhook endpoint
func (s *WebhookService) RetrieveWebhook(id string) (*api.WebhookEndpoint, error) {
	return s.gateway.GetWebhookEndpoint(id)
}

// ListWebhooks lists all webhook endpoints
func (s *WebhookService) ListWebhooks(limit int, startingAfter string) ([]*api.WebhookEndpoint, error) {
	return s.gateway.ListWebhookEndpoints(limit, startingAfter)
}

// UpdateWebhook updates a webhook endpoint with typed options
// Only fields that are set in opts will be updated; nil or empty values are ignored.
func (s *WebhookService) UpdateWebhook(id string, opts *UpdateWebhookOpts) (*api.WebhookEndpoint, error) {
	// Get current webhook
	current, err := s.gateway.GetWebhookEndpoint(id)
	if err != nil {
		return nil, err
	}

	// Apply updates for each field that is set
	if opts.URL != nil {
		current.Url = *opts.URL
	}
	if opts.Description != nil {
		current.Description = opts.Description
	}
	if opts.Livemode != nil {
		current.Livemode = *opts.Livemode
	}
	if opts.Secret != nil {
		current.Secret = opts.Secret
	}
	if opts.Status != nil {
		current.Status = *opts.Status
	}
	if opts.Metadata != nil {
		current.Metadata = opts.Metadata
	}
	if opts.EnabledEvents != nil {
		current.EnabledEvents = opts.EnabledEvents
	}

	return s.gateway.UpdateWebhookEndpoint(id, current)
}

// DeleteWebhook removes a webhook endpoint
func (s *WebhookService) DeleteWebhook(id string) error {
	return s.gateway.DeleteWebhookEndpoint(id)
}

// DeliverEvent queues a webhook event for delivery
func (s *WebhookService) DeliverEvent(webhookId string, event *stripe.Event) (*DeliverResult, error) {
	zap.L().Debug("DeliverEvent",
		zap.String("webhook_id", webhookId),
		zap.String("event_type", string(event.Type)),
		zap.String("event_id", event.ID),
	)
	// Validate webhook exists and is enabled
	w, err := s.gateway.GetWebhookEndpoint(webhookId)
	if err != nil {
		zap.L().Debug("DeliverEvent: webhook not found", zap.String("webhook_id", webhookId), zap.Error(err))
		return nil, fmt.Errorf("webhook not found: %w", err)
	}

	if w.Status != WebhookStatusEnabled {
		zap.L().Debug("DeliverEvent: webhook not enabled", zap.String("webhook_id", webhookId), zap.String("status", w.Status))
		return nil, fmt.Errorf("webhook is not enabled")
	}

	// Check if event type is enabled for this webhook
	if !s.isEventEnabled(w, event.Type) {
		return nil, fmt.Errorf("event type %s not enabled for webhook", event.Type)
	}

	// Queue delivery task with the Event
	task := WebhookDeliveryTask{
		webhookId: webhookId,
		Event:     event,
	}

	// Submit to worker pool with retry logic
	s.workerPool.Submit(func() {
		s.processDeliveryWithRetry(task)
	})

	// Return immediately - delivery happens asynchronously
	return &DeliverResult{
		StatusCode: 202,
		Headers:    http.Header{},
		Body:       []byte(""),
	}, nil
}

// isEventEnabled checks if an event type is enabled for a webhook
func (s *WebhookService) isEventEnabled(w *api.WebhookEndpoint, eventType stripe.EventType) bool {
	if len(w.EnabledEvents) == 0 || w.EnabledEvents[0] == "*" {
		zap.L().Debug("isEventEnabled: wildcard or no enabled_events",
			zap.String("webhook_id", w.Id),
			zap.String("event_type", string(eventType)),
			zap.Bool("enabled", true),
		)
		return true
	}
	event := string(eventType)
	for _, enabled := range w.EnabledEvents {
		if strings.Contains(event, enabled) {
			zap.L().Debug("isEventEnabled: matched",
				zap.String("webhook_id", w.Id),
				zap.String("event_type", string(eventType)),
				zap.String("enabled_pattern", enabled),
			)
			return true
		}
	}
	zap.L().Debug("isEventEnabled: not matched",
		zap.String("webhook_id", w.Id),
		zap.String("event_type", string(eventType)),
		zap.Strings("enabled_events", w.EnabledEvents),
	)
	return false
}

// IsEventEnabled checks if an event type is enabled for a webhook (public API)
func (s *WebhookService) IsEventEnabled(w *api.WebhookEndpoint, eventType stripe.EventType) bool {
	return s.isEventEnabled(w, eventType)
}

// processDeliveryWithRetry processes a single webhook delivery with retry logic
func (s *WebhookService) processDeliveryWithRetry(task WebhookDeliveryTask) {
	err := retry.New(
		retry.Attempts(maxAttempts),
		retry.Delay(1*time.Second),
		retry.MaxDelay(10*time.Second),
		retry.DelayType(retry.BackOffDelay),
		retry.RetryIf(func(err error) bool {
			return err != nil
		}),
	).Do(func() error {
		return s.deliverToWebhook(task)
	})
	_ = err
}

// deliverToWebhook sends an event to a webhook endpoint
func (s *WebhookService) deliverToWebhook(task WebhookDeliveryTask) error {
	// Get webhook
	w, err := s.gateway.GetWebhookEndpoint(task.webhookId)
	if err != nil {
		return fmt.Errorf("webhook not found: %w", err)
	}

	if w.Status != WebhookStatusEnabled {
		return fmt.Errorf("webhook is not enabled")
	}

	// Use webhook's API version if set, otherwise use service default
	apiVersion := s.apiVersion
	if w.ApiVersion != nil {
		apiVersion = *w.ApiVersion
	}

	// Clone and update event with appropriate API version before marshaling
	event := *task.Event
	event.APIVersion = apiVersion

	// Marshal event to JSON (event already has Data.Raw populated)
	payloadBytes, err := json.Marshal(&event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Generate signature using stripe SDK
	timestamp := time.Now().Unix()
	if w.Secret == nil {
		return fmt.Errorf("webhook has no secret configured")
	}
	signature := webhook.ComputeSignature(time.Unix(timestamp, 0), payloadBytes, *w.Secret)

	// Create signature header
	signedHeader := fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(signature))

	// Send webhook
	result, err := s.sendWebhook(w.Url, payloadBytes, signedHeader)
	if err != nil {
		return err
	}

	if result.StatusCode >= 200 && result.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("webhook delivery failed with status %d: %s", result.StatusCode, string(result.Body))
}

// buildWebhookEvent builds a stripe.Event struct from event type and resource JSON
// Uses an incrementing sequence number to ensure events have unique, sortable timestamps
func buildWebhookEvent(eventType stripe.EventType, resourceJSON []byte) *stripe.Event {
	// Get base timestamp and add sequence for incrementing timestamps
	// This ensures events in a waterfall can be sorted by creation time
	seq := eventSequence.Add(1)
	created := time.Now().Unix() + seq

	// Parse the resource JSON into a map for Data.Object (fat events)
	var obj map[string]any
	if err := json.Unmarshal(resourceJSON, &obj); err != nil {
		obj = map[string]any{}
	}

	return &stripe.Event{
		ID:         fmt.Sprintf("%s%d", eventIdPrefix, created),
		Type:       eventType,
		Object:     "event",
		APIVersion: stripe.APIVersion,
		Created:    created,
		Data: &stripe.EventData{
			Raw:    resourceJSON,
			Object: obj,
		},
	}
}

// sendWebhook sends a webhook event via HTTP
func (s *WebhookService) sendWebhook(url string, eventJSON []byte, signedHeader string) (*DeliverResult, error) {
	req, err := http.NewRequest("POST", url, bytes.NewReader(eventJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Stripe-Signature", signedHeader)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &DeliverResult{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header.Clone(),
		Body:       body,
	}, nil
}

// Close gracefully stops the worker pool
func (s *WebhookService) Close() {
	s.workerPool.Stop()
}

// Drain waits for all pending webhook deliveries to complete
// This is useful in tests to ensure asynchronous webhook logging
// doesn't race with test completion
func (s *WebhookService) Drain() {
	s.workerPool.StopWait()
}

// CreateOpts for creating a webhook
type CreateOpts struct {
	URL         string
	Enabled     []string
	Livemode    bool
	Description string
	Metadata    map[string]string
	Secret      string
}

// UpdateWebhookOpts defines options for updating a webhook endpoint.
// Only fields that are set will be updated; nil or empty values are ignored.
type UpdateWebhookOpts struct {
	// URL of the webhook endpoint.
	URL *string

	// Optional description of what the webhook is used for.
	Description *string

	// The list of events to enable for this endpoint. `["*"]` indicates that all events are enabled.
	EnabledEvents []string

	// If the object exists in live mode, the value is `true`.
	Livemode *bool

	// Set of key-value pairs that you can attach to an object.
	Metadata map[string]string

	// The status of the webhook. It can be `enabled` or `disabled`.
	Status *string

	// The endpoint's secret, used to generate webhook signatures.
	Secret *string
}

// DeliverResult contains the result of delivering a webhook event
type DeliverResult struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// Helper functions
func generateEventId() string {
	return fmt.Sprintf("%s%d", eventIdPrefix, time.Now().Unix())
}

func generateWebhookId() string {
	return fmt.Sprintf("%s%d", webhookIdPrefix, time.Now().Unix())
}
