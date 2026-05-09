package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v85"
	"go.uber.org/zap"
	"go.lumeweb.com/stripe-mock-server/pkg/gateway"
	"go.lumeweb.com/stripe-mock-server/pkg/generator"
	"go.lumeweb.com/stripe-mock-server/pkg/internal/gen/models/api"
	"go.lumeweb.com/stripe-mock-server/pkg/storage"
)

// routeDef defines a route to register
type routeDef struct {
	method  string
	path    string
	handler CustomHandlerFunc
}

// RegisterStripeHandlers registers all Stripe API handlers needed by portal-plugin-billing
func (s *Server) RegisterStripeHandlers() error {
	routes := []routeDef{
		// Customers
		{http.MethodPost, "/v1/customers", s.handleCreateCustomer},
		{http.MethodGet, "/v1/customers", s.handleListCustomers},
		{http.MethodPost, "/v1/customers/{id}", s.handleUpdateCustomer},
		{http.MethodGet, "/v1/customers/{id}", s.handleRetrieveCustomer},

		// Checkout Sessions
		{http.MethodPost, "/v1/checkout/sessions", s.handleCreateCheckoutSession},
		{http.MethodGet, "/v1/checkout/sessions", s.handleListCheckoutSessions},
		{http.MethodGet, "/v1/checkout/sessions/{id}", s.handleRetrieveCheckoutSession},
		{http.MethodPost, "/v1/checkout/sessions/{id}/complete", s.handleCompleteCheckoutSession},

		// Billing Portal
		{http.MethodPost, "/v1/billing_portal/sessions", s.handleCreateBillingPortalSession},
		{http.MethodPost, "/v1/billing_portal/configurations", s.handleCreateBillingPortalConfiguration},
		{http.MethodGet, "/v1/billing_portal/configurations", s.handleListBillingPortalConfigurations},

		// Products & Prices
		{http.MethodPost, "/v1/products", s.handleCreateProduct},
		{http.MethodPost, "/v1/products/{id}", s.handleUpdateProduct},
		{http.MethodGet, "/v1/products", s.handleListProducts},
		{http.MethodPost, "/v1/prices", s.handleCreatePrice},
		{http.MethodGet, "/v1/prices", s.handleListPrices},

		// Invoices & Charges
		{http.MethodGet, "/v1/invoices", s.handleListInvoices},
		{http.MethodGet, "/v1/charges", s.handleListCharges},

		// Webhook Endpoints
		{http.MethodPost, "/v1/webhook_endpoints", s.handleCreateWebhookEndpoint},
		{http.MethodGet, "/v1/webhook_endpoints", s.handleListWebhookEndpoints},
		{http.MethodGet, "/v1/webhook_endpoints/{id}", s.handleGetWebhookEndpoint},

		// Subscriptions
		{http.MethodPost, "/v1/subscriptions", s.handleCreateSubscription},
		{http.MethodGet, "/v1/subscriptions", s.handleListSubscriptions},
		{http.MethodGet, "/v1/subscriptions/{id}", s.handleRetrieveSubscription},
		{http.MethodPost, "/v1/subscriptions/{id}/renew", s.handleRenewSubscription},
		{http.MethodPost, "/v1/subscriptions/{id}", s.handleUpdateSubscription},
		{http.MethodPut, "/v1/subscriptions/{id}", s.handleUpdateSubscription},
		{http.MethodDelete, "/v1/subscriptions/{id}", s.handleCancelSubscription},
		{http.MethodPost, "/v1/subscriptions/{id}/expire", s.handleExpireSubscription},
		{http.MethodPost, "/v1/subscriptions/{id}/pause", s.handlePauseSubscription},
		{http.MethodPost, "/v1/subscriptions/{id}/resume", s.handleResumeSubscription},
	}

	for _, r := range routes {
		if err := s.RegisterCustomHandler(r.method, r.path, r.handler); err != nil {
			return err
		}
	}
	return nil
}

// updateCustomerMetadata updates customer metadata with new values
func updateCustomerMetadata(customer *api.Customer, newMetadata map[string]any) {
	if newMetadata == nil || len(newMetadata) == 0 {
		return
	}

	if customer.Metadata == nil {
		var m map[string]string
		customer.Metadata = &m
	}

	// Convert new metadata to map[string]string and merge
	newMap := mapAnyToString(newMetadata)
	if *customer.Metadata == nil {
		*customer.Metadata = make(map[string]string)
	}
	for k, v := range newMap {
		(*customer.Metadata)[k] = v
	}
}

// Customer handlers

func (s *Server) handleCreateCustomer(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	customer := buildCustomer("cus_"+generator.RandomString(14), data)
	created, err := s.gateway.CreateCustomer(customer)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	responseStatus, responseData, err := http.StatusOK, created, nil
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.triggerWebhookEvent(stripe.EventTypeCustomerCreated, newWebhookCustomer(created, s.gateway))
	}()
	return responseStatus, responseData, err
}

func (s *Server) handleUpdateCustomer(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	id := pathParams["id"]
	existing, err := s.gateway.GetCustomer(id)
	if err != nil {
		return http.StatusNotFound, nil, err
	}

	// Update email if provided
	if email := GetString(data, "email"); email != "" {
		existing.Email = &email
	}

	// Update metadata
	updateCustomerMetadata(existing, GetStringMap(data, "metadata"))

	if err := s.gateway.UpdateCustomer(id, existing); err != nil {
		return http.StatusInternalServerError, nil, err
	}
	responseStatus, responseData, err := http.StatusOK, existing, nil
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.triggerWebhookEvent(stripe.EventTypeCustomerUpdated, newWebhookCustomer(existing, s.gateway))
	}()
	return responseStatus, responseData, err
}

func (s *Server) handleRetrieveCustomer(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	return retrieve(s.gateway.GetCustomer, pathParams["id"])
}

// Checkout Session handlers

func (s *Server) handleCreateCheckoutSession(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	s.zap().Debug("handleCreateCheckoutSession",
		zap.String("mode", GetString(data, "mode")),
		zap.String("customer", GetString(data, "customer")),
		zap.Any("line_items", data["line_items"]),
	)
	session := buildCheckoutSession("cs_"+generator.RandomString(14), data)
	s.zap().Debug("handleCreateCheckoutSession: built session",
		zap.String("id", session.Id),
		zap.String("mode", string(session.Mode)),
		zap.Bool("has_line_items", session.LineItems != nil),
	)
	if session.LineItems != nil {
		s.zap().Debug("handleCreateCheckoutSession: line_items detail",
			zap.Int("line_item_count", len(session.LineItems.Data)),
		)
	}
	created, err := s.gateway.CreateCheckoutSession(session)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	return http.StatusOK, created, nil
}

func (s *Server) handleRetrieveCheckoutSession(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	return retrieve(s.gateway.GetCheckoutSession, pathParams["id"])
}

func (s *Server) handleCompleteCheckoutSession(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	id := pathParams["id"]

	s.zap().Debug("handleCompleteCheckoutSession",
		zap.String("session_id", id),
	)

	// Complete the checkout session
	updated, err := s.gateway.CompleteCheckoutSession(id)
	if err != nil {
		s.zap().Error("handleCompleteCheckoutSession: failed",
			zap.String("session_id", id),
			zap.Error(err),
		)
		return http.StatusBadRequest, nil, err
	}

	s.zap().Debug("handleCompleteCheckoutSession: completed",
		zap.String("session_id", id),
		zap.String("mode", string(updated.Mode)),
		zap.Bool("has_subscription", updated.Subscription != nil),
	)

	// Trigger checkout.session.completed webhook asynchronously
	// Marshal to JSON now to avoid racing with writeResponse
	sessionJSON, _ := json.Marshal(updated)
	sessionID := updated.Id
	s.wg.Add(1)
	go func(jsonBytes []byte, id string) {
		defer s.wg.Done()
		s.triggerWebhookEventBytes(stripe.EventTypeCheckoutSessionCompleted, "checkout.session", id, jsonBytes)
	}(sessionJSON, sessionID)

	// For subscription-mode checkouts, fire invoice.paid to activate the subscription
	// This follows Stripe's actual behavior where invoice.paid follows checkout completion
	if updated.Mode == api.CheckoutSessionModeEnumSubscription {
		if updated.Subscription == nil {
			s.zap().Warn("checkout session completed but subscription is nil",
				zap.String("session_id", updated.Id))
			return http.StatusOK, updated, nil
		}

		subID, err := updated.Subscription.AsCheckoutSessionSubscription0()
		if err != nil {
			s.zap().Error("failed to extract subscription ID from checkout session",
				zap.String("session_id", updated.Id),
				zap.Error(err))
			return http.StatusOK, updated, nil
		}
		if subID == "" {
			s.zap().Warn("checkout session has empty subscription ID",
				zap.String("session_id", updated.Id))
			return http.StatusOK, updated, nil
		}

		// Fire invoice.paid asynchronously
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			// Small delay to ensure checkout.session.completed is processed first
			time.Sleep(100 * time.Millisecond)
			s.triggerInvoicePaidForSubscription(subID, api.InvoiceBillingReasonEnumSubscriptionCreate)
		}()
	}

	return http.StatusOK, updated, nil
}

// Billing Portal handlers

func (s *Server) handleCreateBillingPortalSession(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	sessionID := "bps_" + generator.RandomString(14)
	customerID := GetString(data, "customer")

	session := &api.BillingPortalSession{
		Id:       sessionID,
		Object:   api.BillingPortalSessionObjectEnumBillingPortalSession,
		Created:  int(time.Now().Unix()),
		Customer: customerID,
		Url:      fmt.Sprintf("https://billing.stripe.com/p/session/%s", sessionID),
		Livemode: true,
	}

	// Set return_url if provided
	if returnURL, ok := GetStringWithEmpty(data, "return_url"); ok {
		session.ReturnUrl = returnURL
	}

	// Set configuration if provided
	if configID := GetString(data, "configuration"); configID != "" {
		var config api.BillingPortalSession_Configuration
		_ = config.FromBillingPortalSessionConfiguration0(configID)
		session.Configuration = config
	}

	// Set customer_account if provided
	if customerAccount, ok := GetStringWithEmpty(data, "customer_account"); ok {
		session.CustomerAccount = customerAccount
	}

	// Set on_behalf_of if provided
	if onBehalfOf, ok := GetStringWithEmpty(data, "on_behalf_of"); ok {
		session.OnBehalfOf = onBehalfOf
	}

	// Set locale if provided
	if locale := GetString(data, "locale"); locale != "" {
		l := api.BillingPortalSessionLocale(locale)
		session.Locale = &l
	}

	return http.StatusOK, session, nil
}

func (s *Server) handleCreateBillingPortalConfiguration(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	configID := "bpc_" + generator.RandomString(14)

	// Build configuration from input data
	config := &api.BillingPortalConfiguration{
		Id:        configID,
		Object:    api.BillingPortalConfigurationObjectEnumBillingPortalConfiguration,
		Created:   int(time.Now().Unix()),
		Livemode:  true,
		IsDefault: false,
		Active:    true,
		Updated:   int(time.Now().Unix()),
		LoginPage: api.PortalLoginPage{
			Enabled: false,
		},
	}

	// Set name if provided
	if name, ok := GetStringWithEmpty(data, "name"); ok {
		config.Name = name
	}

	// Set default_return_url if provided
	if returnURL, ok := GetStringWithEmpty(data, "default_return_url"); ok {
		config.DefaultReturnUrl = returnURL
	}

	// Set metadata if provided
	if metadata := GetStringMap(data, "metadata"); len(metadata) > 0 {
		m := mapAnyToString(metadata)
		config.Metadata = &m
	}

	// Build business_profile if provided
	if businessProfile := GetStringMap(data, "business_profile"); businessProfile != nil {
		config.BusinessProfile = buildPortalBusinessProfile(businessProfile)
	}

	// Build features (required)
	if features := GetStringMap(data, "features"); features != nil {
		config.Features = buildPortalFeatures(features)
	} else {
		// Set empty default features if not provided
		config.Features = api.PortalFeatures{}
	}

	// Handle application if provided (union type)
	if application := GetString(data, "application"); application != "" {
		var app api.BillingPortalConfiguration_Application
		_ = app.FromBillingPortalConfigurationApplication0(application)
		config.Application = &app
	}

	if _, err := s.gateway.CreateBillingPortalConfiguration(config); err != nil {
		return http.StatusInternalServerError, nil, err
	}
	return http.StatusOK, config, nil
}

// Product & Price handlers

func (s *Server) handleCreateProduct(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	product := buildProduct("prod_"+generator.RandomString(14), data)
	created, err := s.gateway.CreateProduct(product)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	responseStatus, responseData, err := http.StatusOK, created, nil
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.triggerWebhookEvent(stripe.EventTypeProductCreated, newWebhookProduct(created, s.gateway))
	}()
	return responseStatus, responseData, err
}

func (s *Server) handleCreatePrice(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	price := buildPrice("price_"+generator.RandomString(14), data)
	created, err := s.gateway.CreatePrice(price)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	responseStatus, responseData, err := http.StatusOK, created, nil
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.triggerWebhookEvent(stripe.EventTypePriceCreated, newWebhookPrice(created, s.gateway))
	}()
	return responseStatus, responseData, err
}

func (s *Server) handleUpdateProduct(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	id := pathParams["id"]
	existing, err := s.gateway.GetProduct(id)
	if err != nil {
		return http.StatusNotFound, nil, err
	}

	// Update default_price if provided
	if defaultPrice := GetString(data, "default_price"); defaultPrice != "" {
		var dp api.Product_DefaultPrice
		_ = dp.FromProductDefaultPrice0(defaultPrice)
		existing.DefaultPrice = &dp
	}

	// Update name if provided
	if name := GetString(data, "name"); name != "" {
		existing.Name = name
	}

	// Update description if provided
	if desc, ok := GetStringWithEmpty(data, "description"); ok {
		existing.Description = desc
	}

	// Update active if provided
	if _, ok := GetStringWithEmpty(data, "active"); ok {
		existing.Active = GetBool(data, "active")
	}

	// Update metadata if provided
	if metadata := GetStringMap(data, "metadata"); len(metadata) > 0 {
		existing.Metadata = mapAnyToString(metadata)
	}

	// Update updated timestamp for API response
	// Note: Repository marshals data; Updated timestamp will be preserved
	existing.Updated = int(time.Now().Unix())

	if err := s.gateway.UpdateProduct(id, existing); err != nil {
		return http.StatusInternalServerError, nil, err
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.triggerWebhookEvent(stripe.EventTypeProductUpdated, newWebhookProduct(existing, s.gateway))
	}()
	return http.StatusOK, existing, nil
}

func (s *Server) handleListProducts(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	products, err := s.gateway.ListProducts()
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	// Return in Stripe list format
	return http.StatusOK, map[string]any{
		"object":   "list",
		"data":     products,
		"has_more": false,
	}, nil
}

func (s *Server) handleListPrices(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	prices, err := s.gateway.ListPrices()
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	// Return in Stripe list format
	return http.StatusOK, map[string]any{
		"object":   "list",
		"data":     prices,
		"has_more": false,
	}, nil
}

func (s *Server) handleListCustomers(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	customers, err := s.gateway.ListCustomers()
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	// Return in Stripe list format
	return http.StatusOK, map[string]any{
		"object":   "list",
		"data":     customers,
		"has_more": false,
	}, nil
}

func (s *Server) handleListSubscriptions(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	subscriptions, err := s.gateway.ListSubscriptions()
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	// Return in Stripe list format
	return http.StatusOK, map[string]any{
		"object":   "list",
		"data":     subscriptions,
		"has_more": false,
	}, nil
}

func (s *Server) handleListInvoices(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	invoices, err := s.gateway.ListInvoices()
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	// Return in Stripe list format
	return http.StatusOK, map[string]any{
		"object":   "list",
		"data":     invoices,
		"has_more": false,
	}, nil
}

func (s *Server) handleListCharges(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	charges, err := s.gateway.ListCharges()
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	// Return in Stripe list format
	return http.StatusOK, map[string]any{
		"object":   "list",
		"data":     charges,
		"has_more": false,
	}, nil
}

func (s *Server) handleListCheckoutSessions(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	sessions, err := s.gateway.ListCheckoutSessions()
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	// Return in Stripe list format
	return http.StatusOK, map[string]any{
		"object":   "list",
		"data":     sessions,
		"has_more": false,
	}, nil
}

func (s *Server) handleListBillingPortalConfigurations(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	configs, err := s.gateway.ListBillingPortalConfigurations()
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	// Return in Stripe list format
	return http.StatusOK, map[string]any{
		"object":   "list",
		"data":     configs,
		"has_more": false,
	}, nil
}

func (s *Server) handleListWebhookEndpoints(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	endpoints, err := s.gateway.ListWebhookEndpoints(0, "")
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	// Return in Stripe list format
	return http.StatusOK, map[string]any{
		"object":   "list",
		"data":     endpoints,
		"has_more": false,
	}, nil
}

// Subscription handlers

func (s *Server) handleRetrieveSubscription(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	id := pathParams["id"]

	// Parse expand[] parameters from query string
	expand := parseExpandParams(r)

	s.zap().Debug("handleRetrieveSubscription",
		zap.String("id", id),
		zap.Strings("expand", expand),
	)

	subscription, err := s.gateway.GetSubscription(id)
	if err != nil {
		if err == storage.ErrNotFound {
			s.zap().Debug("handleRetrieveSubscription: not found, returning mock",
				zap.String("id", id),
			)
			// Return a mock subscription for testing
			now := int(time.Now().Unix())
			var customer api.Subscription_Customer
			_ = customer.FromSubscriptionCustomer0("cus_mock")
			sub := &api.Subscription{
				Id:       id,
				Object:   api.SubscriptionObjectEnumSubscription,
				Status:   "active",
				Created:  now - 86400,
				Livemode: true,
				Customer: customer,
				Metadata: map[string]string{},
			}
			// Apply expansions
			s.expandSubscription(sub, expand)
			return http.StatusOK, sub, nil
		}
		return http.StatusNotFound, nil, err
	}

	s.zap().Debug("handleRetrieveSubscription: found subscription",
		zap.String("id", id),
		zap.String("status", string(subscription.Status)),
		zap.Int("item_count", len(subscription.Items.Data)),
	)

	// Apply expansions
	s.expandSubscription(subscription, expand)

	return http.StatusOK, subscription, nil
}

func (s *Server) handleCreateSubscription(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	id := "sub_" + generator.RandomString(14)
	subscription := buildSubscription(id, data)
	
	created, err := s.gateway.CreateSubscription(subscription)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	
	
	// Trigger subscription lifecycle webhook waterfall asynchronously
	go s.triggerSubscriptionLifecycle(created.Id)
	
	return http.StatusOK, created, nil
}

func (s *Server) handleRenewSubscription(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	// Extract subscription ID from path
	id := pathParams["id"]
	if id == "" {
		return http.StatusBadRequest, nil, fmt.Errorf("subscription ID required")
	}
	
	// Perform renewal (updates current_period_end if field available)
	updated, err := s.gateway.RenewSubscription(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return http.StatusNotFound, nil, err
		}
		return http.StatusBadRequest, nil, err
	}
	
	// Trigger webhook waterfall asynchronously
	go s.triggerSubscriptionRenewal(updated.Id)
	
	// Return updated subscription
	return http.StatusOK, updated, nil
}

func (s *Server) handleUpdateSubscription(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	id := pathParams["id"]
	if id == "" {
		return http.StatusBadRequest, nil, fmt.Errorf("subscription ID required")
	}

	// Get existing subscription
	sub, err := s.gateway.GetSubscription(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return http.StatusNotFound, nil, err
		}
		return http.StatusBadRequest, nil, err
	}

	// Track if we need to fire subscription.updated webhook
	// (items update fires it via triggerSubscriptionPlanChange)
	var needsWebhook bool

	// Handle cancel_at_period_end flag
	if _, ok := data["cancel_at_period_end"]; ok {
		val := GetBool(data, "cancel_at_period_end")
		if val && !sub.CancelAtPeriodEnd {
			// Schedule cancellation at end of period
			sub, err = s.gateway.SetCancelAtPeriodEnd(id)
			if err != nil {
				return http.StatusBadRequest, nil, err
			}
			needsWebhook = true
		} else if !val && sub.CancelAtPeriodEnd {
			// Cancel scheduled cancellation
			sub.CancelAtPeriodEnd = false
			sub.CancelAt = nil
			if err := s.gateway.UpdateSubscription(id, sub); err != nil {
				return http.StatusBadRequest, nil, err
			}
			sub, err = s.gateway.GetSubscription(id)
			if err != nil {
				return http.StatusInternalServerError, nil, fmt.Errorf("failed to fetch updated subscription: %w", err)
			}
			needsWebhook = true
		}
	}

	// Handle pause_collection
	if _, ok := data["pause_collection"]; ok {
		pcMap, isMap := data["pause_collection"].(map[string]any)
		if isMap {
			behavior := GetString(pcMap, "behavior")
			if behavior != "" {
				var resumesAt *int
				if ra := GetInt64(pcMap, "resumes_at"); ra > 0 {
					v := int(ra)
					resumesAt = &v
				}

				pauseCollection := api.SubscriptionsResourcePauseCollection{
					Behavior: api.SubscriptionsResourcePauseCollectionBehavior(behavior),
				}
				if resumesAt != nil {
					pauseCollection.ResumesAt = resumesAt
				}

				var pcUnion api.Subscription_PauseCollection
				_ = pcUnion.FromSubscriptionsResourcePauseCollection(pauseCollection)
				sub.PauseCollection = &pcUnion

				if err := s.gateway.UpdateSubscription(id, sub); err != nil {
					return http.StatusBadRequest, nil, err
				}
				sub, err = s.gateway.GetSubscription(id)
				if err != nil {
					return http.StatusInternalServerError, nil, fmt.Errorf("failed to fetch updated subscription: %w", err)
				}
				needsWebhook = true
			} else {
				// Empty behavior in map → clear pause_collection
				sub.PauseCollection = nil
				if err := s.gateway.UpdateSubscription(id, sub); err != nil {
					return http.StatusBadRequest, nil, err
				}
				sub, err = s.gateway.GetSubscription(id)
				if err != nil {
					return http.StatusInternalServerError, nil, fmt.Errorf("failed to fetch updated subscription: %w", err)
				}
				needsWebhook = true
			}
		} else {
			// pause_collection= (empty string) → clear pause_collection
			sub.PauseCollection = nil
			if err := s.gateway.UpdateSubscription(id, sub); err != nil {
				return http.StatusBadRequest, nil, err
			}
			sub, err = s.gateway.GetSubscription(id)
			if err != nil {
				return http.StatusInternalServerError, nil, fmt.Errorf("failed to fetch updated subscription: %w", err)
			}
			needsWebhook = true
		}
	}

	// Handle items update (plan change)
	items := GetMapSlice(data, "items")
	if len(items) > 0 {
		// Build update list
		var updates []gateway.SubscriptionItemUpdate
		for _, item := range items {
			update := gateway.SubscriptionItemUpdate{
				ID:       GetString(item, "id"),
				PriceID:  GetString(item, "price"),
				Quantity: int(GetInt64(item, "quantity")),
			}

			// Check for deletion
			if _, ok := item["deleted"]; ok {
				if GetBool(item, "deleted") {
					update.Deleted = true
				}
			}

			updates = append(updates, update)
		}

		// Get proration behavior
		prorationBehavior := GetString(data, "proration_behavior")
		if prorationBehavior == "" {
			prorationBehavior = "create_prorations"
		}

		// Perform update
		var prorationResult *gateway.ProrationResult
		sub, prorationResult, err = s.gateway.UpdateSubscriptionItems(id, updates, prorationBehavior)
		if err != nil {
			return http.StatusBadRequest, nil, err
		}

		// Trigger plan change webhooks asynchronously (fires subscription.updated)
		go s.triggerSubscriptionPlanChange(sub, prorationResult, prorationBehavior)
		// Already fired webhook, don't fire again at end
		needsWebhook = false
	}

	// If no changes made, just return the subscription
	if !needsWebhook && len(items) == 0 {
		return http.StatusOK, sub, nil
	}

	// Trigger subscription.updated webhook (only for cancel_at_period_end changes)
	if needsWebhook {
		s.wg.Add(1)
		go func(s *Server, sub *api.Subscription) {
			defer s.wg.Done()
			s.triggerSubscriptionUpdate(sub)
		}(s, sub)
	}

	return http.StatusOK, sub, nil
}

// handleCancelSubscription handles DELETE /v1/subscriptions/{id}
// Immediately cancels a subscription.
func (s *Server) handleCancelSubscription(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	id := pathParams["id"]
	if id == "" {
		return http.StatusBadRequest, nil, fmt.Errorf("subscription ID required")
	}

	// Cancel the subscription
	canceled, err := s.gateway.CancelSubscription(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return http.StatusNotFound, nil, err
		}
		return http.StatusBadRequest, nil, err
	}

	// Trigger subscription.deleted webhook
	go s.triggerSubscriptionDeleted(canceled)

	return http.StatusOK, canceled, nil
}

// handleExpireSubscription handles POST /v1/subscriptions/{id}/expire
// Finalizes a subscription scheduled for cancellation (when current_period_end is reached).
func (s *Server) handleExpireSubscription(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	id := pathParams["id"]
	if id == "" {
		return http.StatusBadRequest, nil, fmt.Errorf("subscription ID required")
	}

	// Expire the subscription
	expired, err := s.gateway.ExpireCanceledSubscription(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return http.StatusNotFound, nil, err
		}
		return http.StatusBadRequest, nil, err
	}

	// Trigger subscription.deleted webhook
	go s.triggerSubscriptionDeleted(expired)

	return http.StatusOK, expired, nil
}

// parseExpandParams extracts expand[] parameters from the request query string
func parseExpandParams(r *http.Request) []string {
	if r == nil || r.URL == nil {
		return nil
	}

	query := r.URL.Query()
	var result []string

	// Handle both expand[]=foo and expand=foo formats
	if expands, ok := query["expand[]"]; ok {
		result = append(result, expands...)
	}
	if expands, ok := query["expand"]; ok {
		result = append(result, expands...)
	}

	return result
}

// expandSubscription expands related objects in a subscription based on expand parameters
func (s *Server) expandSubscription(sub *api.Subscription, expand []string) {
	s.zap().Debug("expandSubscription",
		zap.String("subscription_id", sub.Id),
		zap.Strings("expand", expand),
		zap.Int("item_count", len(sub.Items.Data)),
	)
	for _, exp := range expand {
		switch exp {
		case "items.data.price.product":
			// First expand the price to get metadata, then expand product
			s.expandSubscriptionItemsPrice(sub)
			s.expandSubscriptionItemsProduct(sub)
		case "items.data.price":
			s.expandSubscriptionItemsPrice(sub)
		case "customer":
			s.expandSubscriptionCustomer(sub)
		case "latest_invoice":
			s.expandSubscriptionLatestInvoice(sub)
		}
	}
}

// expandSubscriptionItemsProduct expands items[].price.product to full product objects
func (s *Server) expandSubscriptionItemsProduct(sub *api.Subscription) {
	for i := range sub.Items.Data {
		// Get the product ID from the price
		productID := extractProductIDFromPrice(&sub.Items.Data[i].Price)
		if productID == "" {
			s.zap().Debug("expandSubscriptionItemsProduct: no product ID on price",
				zap.Int("item_index", i),
				zap.String("price_id", sub.Items.Data[i].Price.Id),
			)
			continue
		}

		// Try to get the product from storage
		product, err := s.gateway.GetProduct(productID)
		if err != nil {
			s.zap().Debug("expandSubscriptionItemsProduct: product not found in storage, creating minimal",
				zap.Int("item_index", i),
				zap.String("product_id", productID),
			)
			// Create a minimal product if not found
			product = &api.Product{
				Id:       productID,
				Object:   api.ProductObjectEnumProduct,
				Created:  int(time.Now().Unix()),
				Livemode: true,
				Metadata: map[string]string{},
			}
		} else {
			s.zap().Debug("expandSubscriptionItemsProduct: expanded product from storage",
				zap.Int("item_index", i),
				zap.String("product_id", productID),
				zap.String("product_name", product.Name),
				zap.Any("product_metadata", product.Metadata),
			)
		}

		// Set expanded product on the price
		sub.Items.Data[i].Price.Product.FromProduct(*product)
	}
}

// expandSubscriptionItemsPrice expands items[].price to full price objects with metadata
func (s *Server) expandSubscriptionItemsPrice(sub *api.Subscription) {
	for i := range sub.Items.Data {
		priceID := sub.Items.Data[i].Price.Id
		if priceID == "" {
			s.zap().Debug("expandSubscriptionItemsPrice: item has no price ID", zap.Int("item_index", i))
			continue
		}

		// Try to get the full price from storage
		fullPrice, err := s.gateway.GetPrice(priceID)
		if err != nil {
			s.zap().Debug("expandSubscriptionItemsPrice: price not found in storage",
				zap.Int("item_index", i),
				zap.String("price_id", priceID),
				zap.Error(err),
			)
			// Price not in storage, keep the minimal price
			continue
		}

		s.zap().Debug("expandSubscriptionItemsPrice: expanded price from storage",
			zap.Int("item_index", i),
			zap.String("price_id", priceID),
			zap.Any("price_metadata", fullPrice.Metadata),
		)

		// Check if the minimal price has an expanded product (not just an ID)
		_, minPriceErr := sub.Items.Data[i].Price.Product.AsProduct()
		hasExpandedProduct := minPriceErr == nil

		// Replace minimal price with full price data
		sub.Items.Data[i].Price = *fullPrice

		// If the minimal price already had an expanded product, restore it
		// Otherwise, use the product reference from the stored price
		if hasExpandedProduct {
			s.zap().Debug("expandSubscriptionItemsPrice: minimal price already had expanded product, preserving",
				zap.Int("item_index", i),
			)
			// The caller may have already expanded the product, keep that
			// (this shouldn't normally happen, but handle it gracefully)
		}
		// Otherwise, the fullPrice from storage has the product reference (ID or expanded)
	}
}

// expandSubscriptionCustomer expands the customer field to a full customer object
func (s *Server) expandSubscriptionCustomer(sub *api.Subscription) {
	customerID := extractCustomerIDFromSubscription(sub)
	if customerID == "" {
		return
	}

	customer, err := s.gateway.GetCustomer(customerID)
	if err != nil {
		// Create a minimal customer if not found
		m := map[string]string{}
		customer = &api.Customer{
			Id:       customerID,
			Object:   api.CustomerObjectEnumCustomer,
			Created:  int(time.Now().Unix()),
			Livemode: true,
			Metadata: &m,
		}
	}

	// Set expanded customer
	sub.Customer.FromCustomer(*customer)
}

// expandSubscriptionLatestInvoice expands the latest_invoice field to a full invoice object
func (s *Server) expandSubscriptionLatestInvoice(sub *api.Subscription) {
	if sub.LatestInvoice == nil {
		return
	}

	invoiceID := extractInvoiceIDFromUnion(sub.LatestInvoice)
	if invoiceID == "" {
		return
	}

	invoice, err := s.gateway.GetInvoice(invoiceID)
	if err != nil {
		// Create a minimal invoice if not found
		invoice = &api.Invoice{
			Id:       invoiceID,
			Object:   api.InvoiceObjectEnumInvoice,
			Created:  int(time.Now().Unix()),
			Livemode: true,
		}
	}

	// Set expanded invoice
	sub.LatestInvoice.FromInvoice(*invoice)
}

// extractProductIDFromPrice extracts the product ID from a Price's Product union field
func extractProductIDFromPrice(price *api.Price) string {
	if price == nil {
		return ""
	}

	// Try to get as string (ID only)
	id, err := price.Product.AsPriceProduct0()
	if err == nil && id != "" {
		zap.L().Debug("extractProductIDFromPrice: found string ID", zap.String("product_id", id))
		return id
	}

	// Try to get as Product object and extract ID
	product, err := price.Product.AsProduct()
	if err == nil {
		zap.L().Debug("extractProductIDFromPrice: found expanded product", zap.String("product_id", product.Id), zap.String("product_name", product.Name))
		return product.Id
	}

	zap.L().Debug("extractProductIDFromPrice: could not extract product ID from price",
		zap.String("price_id", price.Id),
		zap.Error(err),
	)
	return ""
}

// extractCustomerIDFromSubscription extracts the customer ID from a Subscription's Customer union field
func extractCustomerIDFromSubscription(sub *api.Subscription) string {
	if sub == nil {
		return ""
	}

	// Try to get as string (ID only)
	id, err := sub.Customer.AsSubscriptionCustomer0()
	if err == nil && id != "" {
		return id
	}

	// Try to get as Customer object and extract ID
	customer, err := sub.Customer.AsCustomer()
	if err == nil {
		return customer.Id
	}

	return ""
}

	// Extract invoice ID from union field
func extractInvoiceIDFromUnion(invoice *api.Subscription_LatestInvoice) string {
	if invoice == nil {
		return ""
	}

	// Try to get as string (ID only)
	id, err := invoice.AsSubscriptionLatestInvoice0()
	if err == nil && id != "" {
		return id
	}

	// Try to get as Invoice object and extract ID
	inv, err := invoice.AsInvoice()
	if err == nil {
		return inv.Id
	}

	return ""
}

// handlePauseSubscription handles POST /v1/subscriptions/{id}/pause
// Pauses a subscription (similar to cancellation but with ability to resume)
func (s *Server) handlePauseSubscription(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	id := pathParams["id"]
	if id == "" {
		return http.StatusBadRequest, nil, fmt.Errorf("subscription ID required")
	}

	// Pause the subscription
	paused, err := s.gateway.PauseSubscription(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return http.StatusNotFound, nil, err
		}
		return http.StatusBadRequest, nil, err
	}

	// Trigger subscription.paused webhook
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.triggerSubscriptionPaused(paused)
	}()

	return http.StatusOK, paused, nil
}

// handleResumeSubscription handles POST /v1/subscriptions/{id}/resume
// Resumes a paused subscription
func (s *Server) handleResumeSubscription(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	id := pathParams["id"]
	if id == "" {
		return http.StatusBadRequest, nil, fmt.Errorf("subscription ID required")
	}

	// Resume the subscription
	resumed, err := s.gateway.ResumeSubscription(id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return http.StatusNotFound, nil, err
		}
		return http.StatusBadRequest, nil, err
	}

	// Trigger subscription.resumed webhook
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.triggerSubscriptionResumed(resumed)
	}()

	return http.StatusOK, resumed, nil
}

// Webhook Endpoint handlers

func (s *Server) handleCreateWebhookEndpoint(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	id := "we_" + generator.RandomString(14)
	
	// Build webhook endpoint from request data
	wbe := &api.WebhookEndpoint{
		Id:        id,
		Object:    api.WebhookEndpointObjectEnumWebhookEndpoint,
		Created:   int(time.Now().Unix()),
		Livemode:  true,
		Url:       GetString(data, "url"),
		Status:    "enabled",
		Metadata:  map[string]string{},
	}
	
	// Set description if provided
	if desc := GetString(data, "description"); desc != "" {
		wbe.Description = &desc
	}
	
	// Set enabled_events if provided
	if events := GetStringSlice(data, "enabled_events"); len(events) > 0 {
		wbe.EnabledEvents = events
	}
	
	// Set api_version if provided
	if apiVersion := GetString(data, "api_version"); apiVersion != "" {
		wbe.ApiVersion = &apiVersion
	}
	
	// Generate a webhook secret
	secret := "whsec_" + generator.RandomString(24)
	wbe.Secret = &secret
	
	created, err := s.gateway.CreateWebhookEndpoint(wbe)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	
	return http.StatusOK, created, nil
}

func (s *Server) handleGetWebhookEndpoint(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
	id := pathParams["id"]
	if id == "" {
		return http.StatusBadRequest, nil, fmt.Errorf("webhook endpoint ID required")
	}
	
	wbe, err := s.gateway.GetWebhookEndpoint(id)
	if err != nil {
		return http.StatusNotFound, nil, err
	}
	
	return http.StatusOK, wbe, nil
}
