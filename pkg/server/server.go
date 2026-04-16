package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v85"
	"github.com/stripe/stripe-mock/param"
	"go.lumeweb.com/stripe-mock-server/pkg/gateway"
	"go.lumeweb.com/stripe-mock-server/pkg/generator"
	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
	"go.lumeweb.com/stripe-mock-server/pkg/spec"
	"go.lumeweb.com/stripe-mock-server/pkg/storage"
	"go.uber.org/zap"
)

// Version set in Stripe-Mock-Version response header
const Version = "mock-server-v1"

// DoubleSlashFixHandler deduplicates doubled slashes in incoming paths
type DoubleSlashFixHandler struct {
	Mux    http.Handler
	Logger *zap.Logger
}

func (h *DoubleSlashFixHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.URL.Path = strings.Replace(r.URL.Path, "//", "/", -1)
	h.Logger.Info("Incoming request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)
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
	mux            *http.ServeMux
	gateway        *gateway.Gateway
	spec           *spec.Spec
	verbose        bool
	webhook        *WebhookService
	extendedLogger *zap.Logger
	apiVersion     string
}

// NewServer creates a new Server
func NewServer(spec *spec.Spec, verbose bool, apiVersion string, logger *zap.Logger) (*Server, error) {
	s := &Server{
		extendedLogger: logger,
		mux:            http.NewServeMux(),
		spec:           spec,
		verbose:        verbose,
		gateway:        gateway.NewGateway(),
		apiVersion:     apiVersion,
	}

	// Initialize webhook service
	s.webhook = NewWebhookService(s.gateway, apiVersion)

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

// zap returns the logger to use (either injected or global)
func (s *Server) zap() *zap.Logger {
	if s.extendedLogger != nil {
		return s.extendedLogger.With(
			zap.String("component", "server"),
		)
	}
	return zap.L()
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
		s.zap().Debug("custom handler dispatched",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("pattern", verb+" "+path),
		)

		if !validateAuth(r.Header.Get("Authorization")) {
			writeError(w, http.StatusUnauthorized, invalidAuthorization)
			return
		}

		pathParams := extractPathParams(r)
		params, err := parseRequestParams(r)
		if err != nil {
			s.zap().Debug("custom handler: failed to parse request params", zap.Error(err))
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		s.zap().Debug("custom handler: parsed params",
			zap.String("path", r.URL.Path),
			zap.Any("path_params", pathParams),
			zap.Any("data_keys", func() []string {
				keys := make([]string, 0, len(params))
				for k := range params {
					keys = append(keys, k)
				}
				return keys
			}()),
		)

		statusCode, data, err := handler(r, pathParams, params)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, storage.ErrNotFound) {
				status = http.StatusNotFound
			}
			s.zap().Debug("custom handler: handler returned error",
				zap.String("path", r.URL.Path),
				zap.Int("status", status),
				zap.Error(err),
			)
			writeError(w, status, err.Error())
			return
		}
		s.zap().Debug("custom handler: handler returned success",
			zap.String("path", r.URL.Path),
			zap.Int("status", statusCode),
		)
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
		s.zap().Error("Failed to marshal resource for webhook event", zap.Error(err))
		return
	}

	// Get all webhooks that are subscribed to this event type
	webhooks, err := s.webhook.ListWebhooks(100, "")
	if err != nil {
		s.zap().Error("Failed to list webhooks", zap.Error(err))
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
				s.zap().Error("Failed to deliver webhook event",
					zap.String("webhook_id", w.Id),
					zap.String("event_type", string(eventType)),
					zap.Error(err))
			} else {
				deliveredCount++
			}
		}
	}

	if s.verbose {
		s.zap().Debug("Webhook trigger",
			zap.String("event", string(eventType)),
			zap.Int("events_delivered", deliveredCount),
			zap.Int("webhooks_checked", len(webhooks)))
	}
}

// triggerSubscriptionLifecycle triggers the webhook waterfall for subscription lifecycle
// This simulates: subscription.created → invoice.created → invoice.finalized → charge.succeeded → invoice.paid → subscription.updated
func (s *Server) triggerSubscriptionLifecycle(subscriptionID string) {
	// Fetch fresh copy from storage to avoid race conditions
	subscription, err := s.gateway.GetSubscription(subscriptionID)
	if err != nil {
		s.zap().Error("Failed to fetch subscription for lifecycle", zap.Error(err))
		return
	}

	// Event 1: subscription.created (status: incomplete)
	s.triggerWebhookEvent(stripe.EventTypeCustomerSubscriptionCreated, newWebhookSubscription(subscription, s.gateway))

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
		Livemode:  true,
	}

	// Event 2: invoice.created (status: draft)
	createdInvoice, err := s.gateway.CreateInvoice(invoice)
	if err != nil {
		s.zap().Error("Failed to create invoice for subscription lifecycle", zap.Error(err))
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoiceCreated, newWebhookInvoice(createdInvoice, s.gateway))

	// Event 3: invoice.finalized (status: open)
	openStatus := api.InvoiceStatusOpen
	createdInvoice.Status = &openStatus
	if err := s.gateway.UpdateInvoice(createdInvoice.Id, createdInvoice); err != nil {
		s.zap().Error("Failed to update invoice status", zap.Error(err))
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoiceFinalized, newWebhookInvoice(createdInvoice, s.gateway))

	// Event 4: charge.succeeded
	chargeId := "ch_" + generator.RandomString(14)
	charge := &api.Charge{
		Id:       chargeId,
		Object:   "charge",
		Amount:   createdInvoice.AmountDue,
		Currency: createdInvoice.Currency,
		Status:   "succeeded",
		Created:  int(time.Now().Unix()),
		Livemode: true,
	}
	createdCharge, err := s.gateway.CreateCharge(charge)
	if err != nil {
		s.zap().Error("Failed to create charge", zap.Error(err))
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeChargeSucceeded, newWebhookCharge(createdCharge, s.gateway))

	// Event 5: invoice.paid (status: paid)
	paidStatus := api.InvoiceStatusPaid
	createdInvoice.Status = &paidStatus
	if err := s.gateway.UpdateInvoice(createdInvoice.Id, createdInvoice); err != nil {
		s.zap().Error("Failed to mark invoice paid", zap.Error(err))
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoicePaid, newWebhookInvoice(createdInvoice, s.gateway))

	// Event 6: subscription.updated (status: active)
	subscription.Status = api.SubscriptionStatusActive
	if err := s.gateway.UpdateSubscription(subscription.Id, subscription); err != nil {
		s.zap().Error("Failed to update subscription status", zap.Error(err))
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeCustomerSubscriptionUpdated, newWebhookSubscription(subscription, s.gateway))
}

// triggerSubscriptionRenewal triggers the 5-event renewal waterfall
// This simulates Stripe's automatic billing cycle completion when current_period_end is reached
func (s *Server) triggerSubscriptionRenewal(subscriptionID string) {
	// Fetch fresh copy from storage to avoid race conditions
	subscription, err := s.gateway.GetSubscription(subscriptionID)
	if err != nil {
		s.zap().Error("Failed to fetch subscription for renewal", zap.Error(err))
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
		Livemode:  true,
		// Mark as renewal invoice
		BillingReason: &renewReason,
	}

	createdInvoice, err := s.gateway.CreateInvoice(invoice)
	if err != nil {
		s.zap().Error("Failed to create invoice for renewal", zap.Error(err))
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoiceCreated, newWebhookInvoice(createdInvoice, s.gateway))

	// Event 2: invoice.finalized (status: open)
	openStatus := api.InvoiceStatusOpen
	createdInvoice.Status = &openStatus
	// Add hosted_invoice_url (would be real URL in production)
	// createdInvoice.HostedInvoiceUrl = "https://invoice.stripe.com/..."

	if err := s.gateway.UpdateInvoice(createdInvoice.Id, createdInvoice); err != nil {
		s.zap().Error("Failed to update invoice status", zap.Error(err))
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoiceFinalized, newWebhookInvoice(createdInvoice, s.gateway))

	// Event 3: charge.succeeded
	chargeId := "ch_" + generator.RandomString(14)
	charge := &api.Charge{
		Id:       chargeId,
		Object:   "charge",
		Amount:   createdInvoice.AmountDue,
		Currency: createdInvoice.Currency,
		Status:   "succeeded",
		Created:  int(time.Now().Unix()),
		Livemode: true,
		// Invoice: createdInvoice.Id, // Link to invoice (if field exists)
	}

	createdCharge, err := s.gateway.CreateCharge(charge)
	if err != nil {
		s.zap().Error("Failed to create charge", zap.Error(err))
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeChargeSucceeded, newWebhookCharge(createdCharge, s.gateway))

	// Event 4: invoice.paid (status: paid)
	paidStatus := api.InvoiceStatusPaid
	createdInvoice.Status = &paidStatus
	if err := s.gateway.UpdateInvoice(createdInvoice.Id, createdInvoice); err != nil {
		s.zap().Error("Failed to mark invoice paid", zap.Error(err))
		return
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoicePaid, newWebhookInvoice(createdInvoice, s.gateway))

	// Event 5: customer.subscription.updated (current_period_end advanced)
	// Subscription already updated in RenewSubscription gateway method
	s.triggerWebhookEvent(stripe.EventTypeCustomerSubscriptionUpdated, newWebhookSubscription(subscription, s.gateway))
}

// registerWebhookRoutes registers webhook endpoint routes

// handleReset handles the reset endpoint
func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	if !validateAuth(r.Header.Get("Authorization")) {
		writeError(w, http.StatusUnauthorized, invalidAuthorization)
		return
	}

	if err := s.gateway.Reset(); err != nil {
		s.zap().Error("Failed to reset state", zap.Error(err))
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
	return &DoubleSlashFixHandler{
		Mux:    s.mux,
		Logger: s.extendedLogger,
	}
}

// HandleRequest handles a single HTTP request (for testing)
func (s *Server) HandleRequest(w http.ResponseWriter, r *http.Request) {
	s.HandleHTTP().ServeHTTP(w, r)
}

// handleOpenAPIRoute handles OpenAPI-generated routes
func (s *Server) handleOpenAPIRoute(w http.ResponseWriter, r *http.Request, operation *spec.Operation) {
	start := time.Now()
	s.zap().Info("Incoming request", zap.String("method", r.Method), zap.String("path", r.URL.Path))

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
		s.zap().Error("Couldn't generate response", zap.Error(err))
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

	encodedData, err := json.Marshal(data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	w.Header().Set("Stripe-Mock-Version", Version)
	w.Header().Set("Request-Id", "req_"+generator.RandomString(10))
	w.WriteHeader(status)

	_, err = w.Write(encodedData)
	if err != nil {
		// Log writing error - but we don't have access to logger in helper function
	}

	if !start.IsZero() {
		// Debug info logged by caller
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

// triggerInvoicePaidForSubscription fires invoice.paid event for a subscription.
// This is used by checkout completion to activate subscriptions in the portal.
// The invoice is created with subscription ID in lines so portal can look up the subscriber.
func (s *Server) triggerInvoicePaidForSubscription(subscriptionID string, billingReason api.InvoiceBillingReasonEnum) {
	subscription, err := s.gateway.GetSubscription(subscriptionID)
	if err != nil {
		s.zap().Error("Failed to fetch subscription for invoice.paid", zap.Error(err))
		return
	}

	// Get customer and amount from subscription
	customerID, _ := subscription.Customer.AsSubscriptionCustomer0()
	amount := 1000 // default
	currency := "usd"
	if len(subscription.Items.Data) > 0 {
		if subscription.Items.Data[0].Price.UnitAmount != nil {
			amount = *subscription.Items.Data[0].Price.UnitAmount
		}
		currency = subscription.Items.Data[0].Price.Currency
	}

	// Create paid invoice linked to subscription
	now := int(time.Now().Unix())
	paidStatus := api.InvoiceStatusPaid
	invoice := &api.Invoice{
		Id:            "in_" + generator.RandomString(14),
		Object:        api.InvoiceObjectEnumInvoice,
		Status:        &paidStatus,
		AmountDue:     amount,
		AmountPaid:    amount,
		Currency:      currency,
		Created:       now,
		Livemode:      true,
		BillingReason: &billingReason,
	}

	if customerID != "" {
		var custUnion api.Invoice_Customer
		_ = custUnion.FromInvoiceCustomer0(customerID)
		invoice.Customer = custUnion
	}

	// Add subscription to invoice lines (portal uses this to find subscriber)
	lineItem := api.LineItem{
		Id:           "il_" + generator.RandomString(14),
		Object:       api.LineItemObjectEnum("line_item"),
		Amount:       amount,
		Currency:     currency,
		Description:  &[]string{"Subscription"}[0],
		Discountable: false,
		Livemode:     true,
		Metadata:     map[string]string{},
		Period: api.InvoiceLineItemPeriod{
			Start: now,
			End:   now + 2592000,
		},
		Subtotal: amount,
	}
	lineItem.Subscription = &api.LineItem_Subscription{}
	_ = lineItem.Subscription.FromLineItemSubscription0(subscriptionID)

	invoice.Lines.Data = []api.LineItem{lineItem}
	invoice.Lines.Object = api.InvoiceLinesListObjectList

	createdInvoice, err := s.gateway.CreateInvoice(invoice)
	if err != nil {
		s.zap().Error("Failed to create invoice for subscription", zap.Error(err))
		return
	}

	s.triggerWebhookEvent(stripe.EventTypeInvoicePaid, newWebhookInvoice(createdInvoice, s.gateway))
}

// triggerSubscriptionUpdate triggers subscription.updated webhook event
func (s *Server) triggerSubscriptionUpdate(subscription *api.Subscription) {
	s.triggerSubscriptionEvent(stripe.EventTypeCustomerSubscriptionUpdated, subscription)
}

// triggerSubscriptionDeleted triggers subscription.deleted webhook event
func (s *Server) triggerSubscriptionDeleted(subscription *api.Subscription) {
	s.triggerSubscriptionEvent(stripe.EventTypeCustomerSubscriptionDeleted, subscription)
}

// triggerSubscriptionPaused triggers subscription.paused webhook event
func (s *Server) triggerSubscriptionPaused(subscription *api.Subscription) {
	s.triggerSubscriptionEvent(stripe.EventTypeCustomerSubscriptionPaused, subscription)
}

// triggerSubscriptionResumed triggers subscription.resumed webhook event
func (s *Server) triggerSubscriptionResumed(subscription *api.Subscription) {
	s.triggerSubscriptionEvent(stripe.EventTypeCustomerSubscriptionResumed, subscription)
}

// triggerSubscriptionEvent is a helper for subscription webhook events
func (s *Server) triggerSubscriptionEvent(eventType stripe.EventType, subscription *api.Subscription) {
	s.triggerWebhookEvent(eventType, newWebhookSubscription(subscription, s.gateway))
}

// triggerSubscriptionPlanChange handles webhook waterfall for subscription plan changes.
// This unified helper handles all proration behaviors:
//   - none: only fires subscription.updated
//   - create_prorations: fires subscription.updated → invoice.paid
//   - always_invoice: fires subscription.updated → invoice.created → invoice.finalized → charge.succeeded → invoice.paid
func (s *Server) triggerSubscriptionPlanChange(
	subscription *api.Subscription,
	proration *gateway.ProrationResult,
	prorationBehavior string,
) {
	// 1. Always fire subscription.updated first
	s.triggerWebhookEvent(stripe.EventTypeCustomerSubscriptionUpdated, newWebhookSubscription(subscription, s.gateway))

	// 2. Handle proration invoicing based on behavior
	if prorationBehavior == "none" {
		return
	}

	// Skip if no proration amount
	if proration == nil || proration.CreditDue.IsZero() {
		return
	}

	// For create_prorations, fire simplified invoice.paid (portal only needs this)
	// For always_invoice, fire full invoice waterfall
	if prorationBehavior == "create_prorations" {
		s.triggerInvoicePaidForSubscription(subscription.Id, api.InvoiceBillingReasonEnumSubscriptionUpdate)
	} else if prorationBehavior == "always_invoice" {
		s.triggerInvoiceWaterfall(int(proration.CreditDue.IntPart()), "usd", subscription, nil)
	}
}

// triggerInvoiceWaterfall fires the full invoice waterfall: invoice.created → invoice.finalized → charge.succeeded → invoice.paid
// If subscription is provided, it's added to invoice lines for portal lookup.
func (s *Server) triggerInvoiceWaterfall(amount int, currency string, subscription *api.Subscription, billingReason *api.InvoiceBillingReasonEnum) *api.Invoice {
	now := int(time.Now().Unix())

	// 1. invoice.created (status: draft)
	draftStatus := api.InvoiceStatusDraft
	invoice := &api.Invoice{
		Id:        "in_" + generator.RandomString(14),
		Object:    api.InvoiceObjectEnumInvoice,
		Status:    &draftStatus,
		AmountDue: amount,
		Currency:  currency,
		Created:   now,
		Livemode:  true,
	}
	if billingReason != nil {
		invoice.BillingReason = billingReason
	}

	// Add customer and subscription to invoice
	if subscription != nil {
		if customerID, err := subscription.Customer.AsSubscriptionCustomer0(); err == nil && customerID != "" {
			var custUnion api.Invoice_Customer
			_ = custUnion.FromInvoiceCustomer0(customerID)
			invoice.Customer = custUnion
		}

		// Add subscription to invoice lines (portal uses this to find subscriber)
		invoice.Lines.Data = []api.LineItem{
			{
				Id:           "il_" + generator.RandomString(14),
				Object:       api.LineItemObjectEnum("line_item"),
				Amount:       amount,
				Currency:     currency,
				Description:  new("Subscription"),
				Discountable: false,
				Livemode:     true,
				Metadata:     map[string]string{},
				Period: api.InvoiceLineItemPeriod{
					Start: now,
					End:   now + 2592000,
				},
				Subtotal: amount,
			},
		}
		invoice.Lines.Data[0].Subscription = &api.LineItem_Subscription{}
		_ = invoice.Lines.Data[0].Subscription.FromLineItemSubscription0(subscription.Id)
	}

	createdInvoice, err := s.gateway.CreateInvoice(invoice)
	if err != nil {
		s.zap().Error("Failed to create invoice", zap.Error(err))
		return nil
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoiceCreated, newWebhookInvoice(createdInvoice, s.gateway))

	// 2. invoice.finalized (status: open)
	openStatus := api.InvoiceStatusOpen
	createdInvoice.Status = &openStatus
	if err := s.gateway.UpdateInvoice(createdInvoice.Id, createdInvoice); err != nil {
		s.zap().Error("Failed to finalize invoice", zap.Error(err))
		return nil
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoiceFinalized, newWebhookInvoice(createdInvoice, s.gateway))

	// 3. charge.succeeded
	charge := &api.Charge{
		Id:       "ch_" + generator.RandomString(14),
		Object:   api.ChargeObjectEnumCharge,
		Amount:   amount,
		Currency: currency,
		Status:   api.ChargeStatusSucceeded,
		Created:  now,
		Livemode: true,
	}
	if _, err := s.gateway.CreateCharge(charge); err != nil {
		s.zap().Error("Failed to create charge", zap.Error(err))
		return nil
	}
	s.triggerWebhookEvent(stripe.EventTypeChargeSucceeded, newWebhookCharge(charge, s.gateway))

	// 4. invoice.paid (status: paid)
	paidStatus := api.InvoiceStatusPaid
	createdInvoice.Status = &paidStatus
	if err := s.gateway.UpdateInvoice(createdInvoice.Id, createdInvoice); err != nil {
		s.zap().Error("Failed to mark invoice paid", zap.Error(err))
		return nil
	}
	s.triggerWebhookEvent(stripe.EventTypeInvoicePaid, newWebhookInvoice(createdInvoice, s.gateway))

	return createdInvoice
}
