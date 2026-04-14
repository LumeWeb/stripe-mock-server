package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap/zaptest"

	"github.com/stripe-mock-server/pkg/internal/gen/models/api"
	"github.com/stripe-mock-server/pkg/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestServer creates a minimal server for testing
func setupTestServer(t *testing.T, verbose bool) *Server {
	apiSpec, err := spec.GetSpec()
	assert.NoError(t, err)
	s, err := NewServer(apiSpec, verbose, zaptest.NewLogger(t))
	assert.NoError(t, err)
	return s
}

// makeRequest creates and executes an HTTP request
func makeRequest(method, path string, body []byte, auth string) (*httptest.ResponseRecorder, *http.Request) {
	var req *http.Request
	if len(body) > 0 {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	return rec, req
}

// parseResponseBody unmarshals the response body into a map
func parseResponseBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	var response map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	assert.NoError(t, err)
	return response
}

// TestServer_CustomHandlerRegistration tests custom handler registration
func TestServer_CustomHandlerRegistration(t *testing.T) {
	s := setupTestServer(t, false)

	handlerCalled := false
	s.RegisterCustomHandler(http.MethodGet, "/v1/custom/test", func(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
		handlerCalled = true
		return http.StatusOK, map[string]any{"custom": true}, nil
	})

	rec, req := makeRequest(http.MethodGet, "/v1/custom/test", nil, "Bearer sk_test_123")
	s.HandleRequest(rec, req)

	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, rec.Code)

	response := parseResponseBody(t, rec)
	assert.True(t, response["custom"].(bool))
}

// TestServer_CustomHandlerWithPathParams tests custom handlers with path parameters
func TestServer_CustomHandlerWithPathParams(t *testing.T) {
	s := setupTestServer(t, false)

	handlerCalled := false
	s.RegisterCustomHandler(http.MethodGet, "/v1/custom/{id}", func(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
		handlerCalled = true
		id := pathParams["id"]
		assert.Equal(t, "test123", id)
		return http.StatusOK, map[string]any{"id": id}, nil
	})

	rec, req := makeRequest(http.MethodGet, "/v1/custom/test123", nil, "Bearer sk_test_123")
	s.HandleRequest(rec, req)

	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, rec.Code)

	response := parseResponseBody(t, rec)
	assert.Equal(t, "test123", response["id"])
}

// TestServer_CustomHandlerWithBody tests custom handlers with request body
func TestServer_CustomHandlerWithBody(t *testing.T) {
	s := setupTestServer(t, false)

	handlerCalled := false
	s.RegisterCustomHandler(http.MethodPost, "/v1/custom/test", func(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
		handlerCalled = true
		return http.StatusOK, map[string]any{"received": data}, nil
	})

	reqBody := map[string]any{"name": "test", "value": 42}
	reqBodyBytes, _ := json.Marshal(reqBody)

	rec, req := makeRequest(http.MethodPost, "/v1/custom/test", reqBodyBytes, "Bearer sk_test_123")
	s.HandleRequest(rec, req)

	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, rec.Code)

	response := parseResponseBody(t, rec)
	assert.Equal(t, "test", response["received"].(map[string]any)["name"])
}

// TestServer_CustomHandlerError tests error handling in custom handlers
func TestServer_CustomHandlerError(t *testing.T) {
	s := setupTestServer(t, false)

	s.RegisterCustomHandler(http.MethodGet, "/v1/error", func(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
		return http.StatusInternalServerError, nil, assert.AnError
	})

	rec, req := makeRequest(http.MethodGet, "/v1/error", nil, "Bearer sk_test_123")
	s.HandleRequest(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// TestServer_AuthenticationRequired tests authentication requirement
func TestServer_AuthenticationRequired(t *testing.T) {
	s := setupTestServer(t, false)

	s.RegisterCustomHandler(http.MethodGet, "/v1/auth-test", func(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
		return http.StatusOK, map[string]any{"authenticated": true}, nil
	})

	// Test without auth
	rec, req := makeRequest(http.MethodGet, "/v1/auth-test", nil, "")
	s.HandleRequest(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	// Test with valid auth
	rec, req = makeRequest(http.MethodGet, "/v1/auth-test", nil, "Bearer sk_test_123")
	s.HandleRequest(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Test with invalid auth
	rec, req = makeRequest(http.MethodGet, "/v1/auth-test", nil, "Bearer invalid_key")
	s.HandleRequest(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestServer_ResponseHeaders tests response headers
func TestServer_ResponseHeaders(t *testing.T) {
	s := setupTestServer(t, false)

	s.RegisterCustomHandler(http.MethodGet, "/v1/headers", func(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
		return http.StatusOK, map[string]any{"ok": true}, nil
	})

	rec, req := makeRequest(http.MethodGet, "/v1/headers", nil, "Bearer sk_test_123")
	s.HandleRequest(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.NotEmpty(t, rec.Header().Get("Request-Id"))
	assert.NotEmpty(t, rec.Header().Get("Stripe-Mock-Version"))
}

// TestServer_WithoutCustomHandler tests 404 when no handler matches
func TestServer_WithoutCustomHandler(t *testing.T) {
	s := setupTestServer(t, false)

	rec, req := makeRequest(http.MethodGet, "/v1/notfound", nil, "Bearer sk_test_123")
	s.HandleRequest(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestServer_MultipleCustomHandlers tests registration of multiple custom handlers
func TestServer_MultipleCustomHandlers(t *testing.T) {
	s := setupTestServer(t, false)

	handlerCount := 0
	s.RegisterCustomHandler(http.MethodGet, "/v1/test/a", func(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
		handlerCount++
		return http.StatusOK, map[string]any{"handler": "a"}, nil
	})

	s.RegisterCustomHandler(http.MethodGet, "/v1/test/b", func(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
		handlerCount++
		return http.StatusOK, map[string]any{"handler": "b"}, nil
	})

	// Test handler A
	recA, reqA := makeRequest(http.MethodGet, "/v1/test/a", nil, "Bearer sk_test_123")
	s.HandleRequest(recA, reqA)
	assert.Equal(t, http.StatusOK, recA.Code)
	responseA := parseResponseBody(t, recA)
	assert.Equal(t, "a", responseA["handler"])
	assert.Equal(t, 1, handlerCount)

	// Test handler B
	recB, reqB := makeRequest(http.MethodGet, "/v1/test/b", nil, "Bearer sk_test_123")
	s.HandleRequest(recB, reqB)
	assert.Equal(t, http.StatusOK, recB.Code)
	responseB := parseResponseBody(t, recB)
	assert.Equal(t, "b", responseB["handler"])
	assert.Equal(t, 2, handlerCount)
}

// TestServer_DoubleSlashFix tests double slash handling
func TestServer_DoubleSlashFix(t *testing.T) {
	s := setupTestServer(t, false)

	handlerCalled := false
	s.RegisterCustomHandler(http.MethodGet, "/v1/test", func(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
		handlerCalled = true
		return http.StatusOK, map[string]any{"ok": true}, nil
	})

	// Test with double slash
	rec, req := makeRequest(http.MethodGet, "//v1/test", nil, "Bearer sk_test_123")
	s.HandleRequest(rec, req)

	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, rec.Code)
}

// TestServer_StatefulOperations tests stateful gateway operations
func TestServer_StatefulOperations(t *testing.T) {
	s := setupTestServer(t, false)

	// Test customer creation
	s.RegisterCustomHandler(http.MethodPost, "/v1/customers", func(r *http.Request, pathParams map[string]string, data map[string]any) (int, any, error) {
		customer := data
		customer["object"] = "customer"
		customer["created"] = 0
		customer["email"] = data["email"]
		return http.StatusCreated, customer, nil
	})

	reqBody := map[string]any{"email": "test@example.com"}
	reqBodyBytes, _ := json.Marshal(reqBody)

	rec, req := makeRequest(http.MethodPost, "/v1/customers", reqBodyBytes, "Bearer sk_test_123")
	s.HandleRequest(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	response := parseResponseBody(t, rec)
	assert.Equal(t, "test@example.com", response["email"])
	assert.Equal(t, "customer", response["object"])
}

// TestServer_ResetEndpoint tests the reset endpoint
func TestServer_ResetEndpoint(t *testing.T) {
	s := setupTestServer(t, false)

	// Create a webhook endpoint directly via gateway (should survive reset)
	webhook, err := s.gateway.CreateWebhookEndpoint(&api.WebhookEndpoint{
		Url:           "https://example.com/webhook",
		EnabledEvents: []string{"customer.created"},
	})
	require.NoError(t, err)
	webhookID := webhook.Id

	// Create a customer directly via gateway (should be cleared by reset)
	name := "Test Customer"
	customer, err := s.gateway.CreateCustomer(&api.Customer{Name: &name})
	require.NoError(t, err)
	customerID := customer.Id

	// Verify resources exist before reset
	assert.True(t, s.gateway.CustomerExists(customerID))
	assert.True(t, s.gateway.WebhookEndpointExists(webhookID))

	// Reset the server
	rec, req := makeRequest(http.MethodPost, "/v1/reset", nil, "Bearer sk_test_123")
	s.HandleRequest(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	resetResponse := parseResponseBody(t, rec)
	assert.Equal(t, "reset", resetResponse["object"])
	assert.True(t, resetResponse["deleted"].(bool))

	// Customer should be gone
	assert.False(t, s.gateway.CustomerExists(customerID))

	// Webhook should still exist
	assert.True(t, s.gateway.WebhookEndpointExists(webhookID))
}

// TestServer_ResetEndpointRequiresAuth tests that reset requires authentication
func TestServer_ResetEndpointRequiresAuth(t *testing.T) {
	s := setupTestServer(t, false)

	// Reset without auth should fail
	rec, req := makeRequest(http.MethodPost, "/v1/reset", nil, "")
	s.HandleRequest(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestServer_ResetEndpointRequiresPost tests that reset only accepts POST
func TestServer_ResetEndpointRequiresPost(t *testing.T) {
	s := setupTestServer(t, false)

	// GET should return 405 (Method Not Allowed)
	rec, req := makeRequest(http.MethodGet, "/v1/reset", nil, "Bearer sk_test_123")
	s.HandleRequest(rec, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}