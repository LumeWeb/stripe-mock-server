//go:generate go generate ./pkg/internal/gen/models

package main

import (
	"log"
	"net/http"

	"github.com/stripe-mock-server/pkg/server"
	"github.com/stripe-mock-server/pkg/spec"
)

func main() {
	log.Printf("Starting stateful mock server on port 8080")
	log.Printf("Loading embedded spec")

	apiSpec, err := spec.GetSpec()
	if err != nil {
		log.Fatalf("Failed to get spec: %v", err)
	}
	s, err := server.NewServer(apiSpec, false)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Register Stripe API handlers
	if err := s.RegisterStripeHandlers(); err != nil {
		log.Fatalf("Failed to register Stripe handlers: %v", err)
	}

	log.Printf("Server ready")

	// Use DoubleSlashFixHandler for proper URL handling
	handler := s.HandleHTTP()

	addr := ":8080"
	if err := logAndServe(addr, handler); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// logAndServe wraps http.ListenAndServe with log output
func logAndServe(addr string, handler http.Handler) error {
	log.Printf("Listening on %s", addr)
	return http.ListenAndServe(addr, handler)
}