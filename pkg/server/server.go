package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v85"
	"github.com/stripe/stripe-mock/param"
	"github.com/stripe-mock-server/pkg/gateway"
	"github.com/stripe-mock-server/pkg/generator"
	"github.com/stripe-mock-server/pkg/internal/gen/models/api"
	"github.com/stripe-mock-server/pkg/spec"
	"github.com/stripe-mock-server/pkg/storage"
)

// Version set in Stripe-Mock-Version response header
const Version = "mock-server-v1"

// DoubleSlashFixHandler deduplicates doubled slashes in incoming paths
type DoubleSlashFixHandler struct {
	Mux http.Handler
}

func (h *DoubleSlashFixHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.URL.Path = strings.Replace(r.URL.Path, "//", "/", -1)
	h.Mux.ServeHTTP(w, r)
}

// CustomHandlerFunc defines the signature for custom request handlers
type CustomHandlerFunc func(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error)

// Server handles incoming HTTP requests
// APIObject defines a common interface for all Stripe API resources.
// Resources support marshaling to JSON and provide basic identification.
// For compile-time type safety, pass types from pkg/internal/gen/models/api.
type APIObject interface {
	// GetID returns the resource ID
	GetID() string

	// GetObject returns the object type (e.g., "customer", "product")
	GetObject() string
}

type Server struct {
	mux        *http.ServeMux
	gateway    *gateway.Gateway
	spec       *spec.Spec
	verbose    bool
	webhook    *WebhookService
}

// NewServer creates a new Server
func NewServer(spec *spec.Spec, verbose bool) (*Server, error) {
	s := &Server{
		mux:     http.NewServeMux(),
		spec:    spec,
		verbose: verbose,
		gateway: gateway.NewGateway(),
	}

	// Initialize webhook service
	s.webhook = NewWebhookService(s.gateway)

	// Register all OpenAPI routes with Go 1.22+ path variables
	for path, verbs := range spec.Paths {
		for verb, operation := range verbs {
			pattern := string(verb) + " " + string(path)
			op := operation // Capture in closure
			s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
				s.handleOpenAPIRoute(w, r, op)
			})
		}
	}

	// Register webhook routes

	// Register reset endpoint
	s.mux.HandleFunc("POST /v1/reset", s.handleReset)

	return s, nil
}

// RegisterCustomHandler registers a custom handler that overrides default behavior
func (s *Server) RegisterCustomHandler(verb, path string, handler CustomHandlerFunc) error {
	if !isValidMethod(verb) {
		return fmt.Errorf("invalid HTTP method: %s", verb)
	}
	if !strings.HasPrefix(path, "/v1") {
		return fmt.Errorf("path must start with /v1: %s", path)
	}

	s.mux.HandleFunc(verb+" "+path, func(w http.ResponseWriter, r *http.Request) {
		if !validateAuth(r.Header.Get("Authorization")) {
			writeError(w, http.StatusUnauthorized, invalidAuthorization)
			return
		}

		pathParams := extractPathParams(r)
		params, err := parseRequestParams(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		statusCode, data, err := handler(r, pathParams, params)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, storage.ErrNotFound) {
				status = http.StatusNotFound
			}
			writeError(w, status, err.Error())
			return
		}
		writeResponse(w, time.Now(), statusCode, data)
	})

	return nil
}

// triggerWebhookEvent triggers webhook events for the given resource type.
// This function looks up all webhooks that are subscribed to the event type
// and delivers the event asynchronously using the worker pool.
func (s *Server) triggerWebhookEvent(eventType stripe.EventType, obj APIObject) {
	// Marshal resource to JSON bytes
	resourceJSON, err := json.Marshal(obj)
	if err != nil {
		log.Printf("Failed to marshal resource for webhook event: %v", err)
		return
	}

	// Get all webhooks that are subscribed to this event type
	webhooks, err := s.webhook.ListWebhooks(100, "")
	if err != nil {
		log.Printf("Failed to list webhooks: %v", err)
		return
	}

	// Build event using helper
	event := buildWebhookEvent(eventType, resourceJSON)

	// Deliver to each subscribed webhook
	deliveredCount := 0
	for _, w := range webhooks {
		// Check if event type is enabled for this webhook
		if s.webhook.IsEventEnabled(w, eventType) {
			_, err := s.webhook.DeliverEvent(w.Id, event)
			if err != nil {
				log.Printf("Failed to deliver event %s to webhook %s: %v",
					eventType, w.Id, err)
			} else {
				deliveredCount++
			}
		}
	}

	if s.verbose {
		log.Printf("Webhook trigger: event=%s, events_delivered=%d, webhooks_checked=%d",
			eventType, deliveredCount, len(webhooks))
	}
}

// triggerSubscriptionLifecycle triggers the webhook waterfall for subscription lifecycle
// This simulates: subscription.created → invoice.created → invoice.finalized → charge.succeeded → invoice.paid → subscription.updated
func (s *Server) triggerSubscriptionLifecycle(subscriptionID string) {
	// Fetch fresh copy from storage to avoid race conditions
	subscription, err := s.gateway.GetSubscription(subscriptionID)
	if err != nil {
		log.Printf("Failed to fetch subscription for lifecycle: %v", err)
		return
	}

	// Event 1: subscription.created (status: incomplete)
	s.triggerWebhookEvent(stripe.EventTypeCustomerSubscriptionCreated, newWebhookSubscription(subscription))
	
	// Create a draft invoice for the subscription
	invoiceId := "in_" + generator.RandomString(14)
	draftStatus := api.InvoiceStatusDraft
	invoice := &api.Invoice{
		Id:        invoiceId,
		Object:    api.InvoiceObjectEnumInvoice,
		Status:    &draftStatus,
		AmountDue: 1000, // Simplified default amount
		Currency:  "usd",
		Created:   int(time.Now().Unix()),
		Livemode:  false,
	}
	
	// Event 2: invoice.created (status: draft)
	createdInvoice, err := s.gateway.CreateInvoice(invoice)
	if err != nil {
		log.Printf("Failed to create invoice for subscription lifecycle: %v", err)
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoiceCreated, newWebhookInvoice(createdInvoice))
	
	// Event 3: invoice.finalized (status: open)
	openStatus := api.InvoiceStatusOpen
	createdInvoice.Status = &openStatus
	if err := s.gateway.UpdateInvoice(createdInvoice.Id, createdInvoice); err != nil {
		log.Printf("Failed to update invoice status to open: %v", err)
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoiceFinalized, newWebhookInvoice(createdInvoice))
	
	// Event 4: charge.succeeded
	chargeId := "ch_" + generator.RandomString(14)
	charge := &api.Charge{
		Id:       chargeId,
		Object:   "charge",
		Amount:   createdInvoice.AmountDue,
		Currency: createdInvoice.Currency,
		Status:   "succeeded",
		Created:  int(time.Now().Unix()),
		Livemode: false,
	}
	createdCharge, err := s.gateway.CreateCharge(charge)
	if err != nil {
		log.Printf("Failed to create charge for subscription lifecycle: %v", err)
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeChargeSucceeded, newWebhookCharge(createdCharge))
	
	// Event 5: invoice.paid (status: paid)
	paidStatus := api.InvoiceStatusPaid
	createdInvoice.Status = &paidStatus
	if err := s.gateway.UpdateInvoice(createdInvoice.Id, createdInvoice); err != nil {
		log.Printf("Failed to update invoice status to paid: %v", err)
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoicePaid, newWebhookInvoice(createdInvoice))
	
	// Event 6: subscription.updated (status: active)
	subscription.Status = api.SubscriptionStatusActive
	if err := s.gateway.UpdateSubscription(subscription.Id, subscription); err != nil {
		log.Printf("Failed to update subscription status to active: %v", err)
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeCustomerSubscriptionUpdated, newWebhookSubscription(subscription))
}

// triggerSubscriptionRenewal triggers the 5-event renewal waterfall
// This simulates Stripe's automatic billing cycle completion when current_period_end is reached
func (s *Server) triggerSubscriptionRenewal(subscriptionID string) {
	// Fetch fresh copy from storage to avoid race conditions
	subscription, err := s.gateway.GetSubscription(subscriptionID)
	if err != nil {
		log.Printf("Failed to fetch subscription for renewal: %v", err)
		return
	}

	// Get the price amount from subscription items (simplified)
	// In production, would sum all items with quantities
	amount := 1000 // Simplified default
	currency := "usd"
	
	// Event 1: invoice.created (status: draft)
	invoiceId := "in_" + generator.RandomString(14)
	draftStatus := api.InvoiceStatusDraft
	renewReason := api.InvoiceBillingReasonEnumSubscriptionCycle
	invoice := &api.Invoice{
		Id:        invoiceId,
		Object:    api.InvoiceObjectEnumInvoice,
		Status:    &draftStatus,
		AmountDue: amount,
		Currency:  currency,
		Created:   int(time.Now().Unix()),
		Livemode:  false,
		// Mark as renewal invoice
		BillingReason: &renewReason,
	}
	
	createdInvoice, err := s.gateway.CreateInvoice(invoice)
	if err != nil {
		log.Printf("Failed to create invoice for renewal: %v", err)
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoiceCreated, newWebhookInvoice(createdInvoice))
	
	// Event 2: invoice.finalized (status: open)
	openStatus := api.InvoiceStatusOpen
	createdInvoice.Status = &openStatus
	// Add hosted_invoice_url (would be real URL in production)
	// createdInvoice.HostedInvoiceUrl = "https://invoice.stripe.com/..."
	
	if err := s.gateway.UpdateInvoice(createdInvoice.Id, createdInvoice); err != nil {
		log.Printf("Failed to update invoice status to open: %v", err)
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoiceFinalized, newWebhookInvoice(createdInvoice))
	
	// Event 3: charge.succeeded
	chargeId := "ch_" + generator.RandomString(14)
	charge := &api.Charge{
		Id:       chargeId,
		Object:   "charge",
		Amount:   createdInvoice.AmountDue,
		Currency: createdInvoice.Currency,
		Status:   "succeeded",
		Created:  int(time.Now().Unix()),
		Livemode: false,
		// Invoice: createdInvoice.Id, // Link to invoice (if field exists)
	}
	
	createdCharge, err := s.gateway.CreateCharge(charge)
	if err != nil {
		log.Printf("Failed to create charge for renewal: %v", err)
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeChargeSucceeded, newWebhookCharge(createdCharge))
	
	// Event 4: invoice.paid (status: paid)
	paidStatus := api.InvoiceStatusPaid
	createdInvoice.Status = &paidStatus
	if err := s.gateway.UpdateInvoice(createdInvoice.Id, createdInvoice); err != nil {
		log.Printf("Failed to update invoice status to paid: %v", err)
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoicePaid, newWebhookInvoice(createdInvoice))
	
	// Event 5: customer.subscription.updated (current_period_end advanced)
	// Subscription already updated in RenewSubscription gateway method
	s.triggerWebhookEvent(stripe.EventTypeCustomerSubscriptionUpdated, newWebhookSubscription(subscription))
}

// registerWebhookRoutes registers webhook endpoint routes

// handleReset handles the reset endpoint
func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	if !validateAuth(r.Header.Get("Authorization")) {
		writeError(w, http.StatusUnauthorized, invalidAuthorization)
		return
	}

	if err := s.gateway.Reset(); err != nil {
		log.Printf("Failed to reset state: %v", err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to reset state: %v", err))
		return
	}

	writeResponse(w, time.Now(), http.StatusOK, map[string]any{
		"object":  "reset",
		"deleted": true,
	})
}

// HandleHTTP returns an http.Handler for the server
func (s *Server) HandleHTTP() http.Handler {
	return &DoubleSlashFixHandler{Mux: s.mux}
}

// HandleRequest handles a single HTTP request (for testing)
func (s *Server) HandleRequest(w http.ResponseWriter, r *http.Request) {
	s.HandleHTTP().ServeHTTP(w, r)
}

// handleOpenAPIRoute handles OpenAPI-generated routes
func (s *Server) handleOpenAPIRoute(w http.ResponseWriter, r *http.Request, operation *spec.Operation) {
	start := time.Now()
	log.Printf("Request: %v %v", r.Method, r.URL.Path)

	if !validateAuth(r.Header.Get("Authorization")) {
		writeError(w, http.StatusUnauthorized, invalidAuthorization)
		return
	}

	params, err := param.ParseParams(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Couldn't parse query/body: %v", err))
		return
	}

	response, ok := operation.Responses["200"]
	if !ok {
		writeError(w, http.StatusInternalServerError, "Couldn't find 200 response in spec")
		return
	}

	responseContent, ok := response.Content["application/json"]
	if !ok || responseContent.Schema == nil {
		writeError(w, http.StatusInternalServerError, "Couldn't find application/json in response")
		return
	}
	w.Header().Set("Content-Type", "application/json")

	if _, err := validateAndCoerceRequest(r, operation, params); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	gen := generator.Generator{
		Components: generator.ComponentsForValidation{
			Schemas: spec.EmbeddedComponents().Schemas,
		},
		Verbose: s.verbose,
	}

	responseData, err := gen.Generate(&generator.GenerateParams{Schema: responseContent.Schema})
	if err != nil {
		log.Printf("Couldn't generate response: %v", err)
		writeError(w, http.StatusInternalServerError, "Couldn't generate response")
		return
	}

	writeResponse(w, start, http.StatusOK, responseData)
}

// Helper functions

func isValidMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodHead:
		return true
	}
	return false
}

func extractPathParams(r *http.Request) map[string]string {
	params := make(map[string]string)
	if id := r.PathValue("id"); id != "" {
		params["id"] = id
	}
	return params
}

func parseRequestParams(r *http.Request) (map[string]any, error) {
	contentType := r.Header.Get("Content-Type")

	if r.Method != http.MethodGet && r.Method != http.MethodDelete &&
		(contentType == "application/json" || strings.HasPrefix(contentType, "application/json")) {
		var jsonBody map[string]any
		if err := json.NewDecoder(r.Body).Decode(&jsonBody); err != nil {
			return nil, fmt.Errorf("invalid JSON body: %v", err)
		}

		// Merge query params
		queryParams, err := param.ParseParams(r)
		if err == nil {
			for k, v := range queryParams {
				jsonBody[k] = v
			}
		}
		return jsonBody, nil
	}

	return param.ParseParams(r)
}

func validateAuth(auth string) bool {
	if auth == "" {
		return false
	}
	parts := strings.Split(auth, " ")
	if len(parts) != 2 || parts[1] == "" {
		return false
	}

	var key string
	switch parts[0] {
	case "Basic":
		keyBytes, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return false
		}
		key = string(keyBytes)
	case "Bearer":
		key = parts[1]
	default:
		return false
	}

	keyParts := strings.Split(key, "_")
	if len(keyParts) != 3 {
		return false
	}
	return (keyParts[0] == "rk" || keyParts[0] == "sk") &&
		keyParts[1] == "test" &&
		len(keyParts[2]) > 0
}

func validateAndCoerceRequest(r *http.Request, operation *spec.Operation, requestData map[string]interface{}) (map[string]interface{}, error) {
	if r.Method != http.MethodDelete && r.Method != http.MethodGet {
		contentType := r.Header.Get("Content-Type")
		if contentType == "" && operation.RequestBody != nil {
			for mediaType := range operation.RequestBody.Content {
				return nil, fmt.Errorf("missing Content-Type header, expected: %s", mediaType)
			}
		}
	}

	var requestSchema *spec.Schema
	if r.Method == http.MethodGet {
		requestSchema = spec.BuildQuerySchema(operation)
	} else if operation.RequestBody != nil {
		for _, mt := range operation.RequestBody.Content {
			requestSchema = mt.Schema
			break
		}
	}

	if requestSchema != nil {
		// Coercion removed - custom handlers handle their own type conversion
	}
	return requestData, nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Request-Id", "req_"+generator.RandomString(10))
	w.Header().Set("Stripe-Mock-Version", Version)

	response := map[string]any{
		"error": map[string]string{
			"message": message,
			"type":    "invalid_request_error",
		},
	}

	jsonBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error serializing error response: %v", err)
		jsonBytes = []byte(`{"error":{"message":"Internal server error","type":"internal_server_error"}}`)
	}

	w.WriteHeader(status)
	_, _ = w.Write(jsonBytes)
}

func writeResponse(w http.ResponseWriter, start time.Time, status int, data any) {
	if data == nil {
		data = http.StatusText(status)
	}

	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}

	var encodedData []byte
	var err error
	if dataString, ok := data.(string); ok {
		encodedData = []byte(dataString)
	} else {
		encodedData, err = json.Marshal(data)
		if err != nil {
			log.Printf("Error serializing response: %v", err)
			writeError(w, http.StatusInternalServerError, "Internal server error")
			return
		}
	}

	w.Header().Set("Stripe-Mock-Version", Version)
	w.Header().Set("Request-Id", "req_"+generator.RandomString(10))
	w.WriteHeader(status)

	if _, err := w.Write(encodedData); err != nil {
		log.Printf("Error writing to client: %v", err)
	}

	if !start.IsZero() {
		log.Printf("Response: elapsed=%v status=%v", time.Since(start), status)
	}
}

const invalidAuthorization = "Invalid authorization header"

// Generic retrieve helper using the gateway
func retrieve[T any](getFunc func(id string) (*T, error), id string) (int, *T, error) {
	resource, err := getFunc(id)
	if err != nil {
		return http.StatusNotFound, nil, err
	}
	return http.StatusOK, resource, nil
}

// createInvoiceChargeSequence creates an invoice and charge sequence for webhook delivery
// Returns the created invoice and charge, or error
func (s *Server) createInvoiceChargeSequence(amount int64, currency string, reason *api.InvoiceBillingReasonEnum) (*api.Invoice, *api.Charge, error) {
	now := int(time.Now().Unix())
	invoiceId := "in_" + generator.RandomString(14)

	// Create draft invoice
	draftStatus := api.InvoiceStatusDraft
	draftAmount := int(amount)
	invoice := &api.Invoice{
		Id:        invoiceId,
		Object:    api.InvoiceObjectEnumInvoice,
		Status:    &draftStatus,
		AmountDue: draftAmount,
		Currency:  currency,
		Created:   now,
		Livemode:  false,
	}
	if reason != nil {
		invoice.BillingReason = reason
	}

	createdInvoice, err := s.gateway.CreateInvoice(invoice)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create invoice: %w", err)
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoiceCreated, newWebhookInvoice(createdInvoice))

	// Finalize invoice
	openStatus := api.InvoiceStatusOpen
	createdInvoice.Status = &openStatus
	if err := s.gateway.UpdateInvoice(createdInvoice.Id, createdInvoice); err != nil {
		return nil, nil, fmt.Errorf("failed to finalize invoice: %w", err)
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoiceFinalized, newWebhookInvoice(createdInvoice))

	// Create charge
	chargeId := "ch_" + generator.RandomString(14)
	amountInt := int(amount)
	charge := &api.Charge{
		Id:       chargeId,
		Object:   api.ChargeObjectEnumCharge,
		Amount:   amountInt,
		Currency: currency,
		Status:   api.ChargeStatusSucceeded,
		Created:  now,
		Livemode: false,
	}
	if _, err := s.gateway.CreateCharge(charge); err != nil {
		return nil, nil, fmt.Errorf("failed to create charge: %w", err)
	}
	s.triggerWebhookEvent(stripe.EventTypeChargeSucceeded, newWebhookCharge(charge))

	// Mark invoice paid
	paidStatus := api.InvoiceStatusPaid
	createdInvoice.Status = &paidStatus
	if err := s.gateway.UpdateInvoice(createdInvoice.Id, createdInvoice); err != nil {
		return nil, nil, fmt.Errorf("failed to mark invoice paid: %w", err)
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoicePaid, newWebhookInvoice(createdInvoice))

	return createdInvoice, charge, nil
}

// triggerSubscriptionUpdate triggers subscription.updated webhook event
func (s *Server) triggerSubscriptionUpdate(subscription *api.Subscription) {
	s.triggerWebhookEvent(stripe.EventTypeCustomerSubscriptionUpdated, newWebhookSubscription(subscription))
}

// triggerSubscriptionDeleted triggers subscription.deleted webhook event
func (s *Server) triggerSubscriptionDeleted(subscription *api.Subscription) {
	s.triggerWebhookEvent(stripe.EventTypeCustomerSubscriptionDeleted, newWebhookSubscription(subscription))
}

// triggerSubscriptionPaused triggers subscription.paused webhook event
func (s *Server) triggerSubscriptionPaused(subscription *api.Subscription) {
	s.triggerWebhookEvent(stripe.EventTypeCustomerSubscriptionPaused, newWebhookSubscription(subscription))
}

// triggerSubscriptionResumed triggers subscription.resumed webhook event
func (s *Server) triggerSubscriptionResumed(subscription *api.Subscription) {
	s.triggerWebhookEvent(stripe.EventTypeCustomerSubscriptionResumed, newWebhookSubscription(subscription))
}

// triggerPlanChangeWithInvoice triggers the webhook cascade for plan change with immediate invoicing
// Events: subscription.updated → invoice.created → invoice.finalized → charge.succeeded → invoice.paid
func (s *Server) triggerPlanChangeWithInvoice(subscription *api.Subscription, proration *gateway.ProrationResult) {
	// 1. subscription.updated
	s.triggerWebhookEvent(stripe.EventTypeCustomerSubscriptionUpdated, newWebhookSubscription(subscription))

	// Skip if no proration amount
	if proration == nil || proration.CreditDue.IsZero() {
		return
	}

	now := int(time.Now().Unix())
	invoiceId := "in_" + generator.RandomString(14)

	// 2. invoice.created (status: draft)
	draftStatus := api.InvoiceStatusDraft
	invoice := &api.Invoice{
		Id:        invoiceId,
		Object:    api.InvoiceObjectEnumInvoice,
		Status:    &draftStatus,
		AmountDue: int(proration.CreditDue.IntPart()),
		Currency:  "usd",
		Created:   now,
		Livemode:  false,
	}

	createdInvoice, err := s.gateway.CreateInvoice(invoice)
	if err != nil {
		log.Printf("Failed to create invoice for plan change: %v", err)
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoiceCreated, newWebhookInvoice(createdInvoice))

	// 3. invoice.finalized (status: open)
	openStatus := api.InvoiceStatusOpen
	createdInvoice.Status = &openStatus
	if err := s.gateway.UpdateInvoice(createdInvoice.Id, createdInvoice); err != nil {
		log.Printf("Failed to finalize invoice: %v", err)
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoiceFinalized, newWebhookInvoice(createdInvoice))

	// 4. charge.succeeded
	chargeId := "ch_" + generator.RandomString(14)
	charge := &api.Charge{
		Id:        chargeId,
		Object:    api.ChargeObjectEnumCharge,
		Amount:    int(proration.CreditDue.IntPart()),
		Currency:  "usd",
		Status:    api.ChargeStatusSucceeded,
		Created:   now,
		Livemode:  false,
	}
	if _, err := s.gateway.CreateCharge(charge); err != nil {
		log.Printf("Failed to create charge: %v", err)
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeChargeSucceeded, newWebhookCharge(charge))

	// 5. invoice.paid (status: paid)
	paidStatus := api.InvoiceStatusPaid
	createdInvoice.Status = &paidStatus
	if err := s.gateway.UpdateInvoice(createdInvoice.Id, createdInvoice); err != nil {
		log.Printf("Failed to mark invoice paid: %v", err)
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoicePaid, newWebhookInvoice(createdInvoice))
}
