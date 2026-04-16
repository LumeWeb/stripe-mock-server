//go:generate go generate ./pkg/internal/gen/models

package main

import (
	"flag"
	"net/http"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"go.lumeweb.com/stripe-mock-server/pkg/server"
	"go.lumeweb.com/stripe-mock-server/pkg/spec"
)

func main() {
	// Parse command-line flags
	port := flag.String("port", "8080", "Port to listen on")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	stripeAPIVersion := flag.String("stripe-api-version", "", "Stripe API version to use (defaults to spec version if not specified)")
	flag.Parse()

	// Create logger
	logger := setupLogger(*verbose)
	defer logger.Sync() // Flush any buffered log entries

	// Set global logger so zap.L() works in gateway/storage/helpers
	zap.ReplaceGlobals(logger)

	logger.Info("Loading embedded spec")

	apiSpec, err := spec.GetSpec()
	if err != nil {
		logger.Fatal("Failed to get spec", zap.Error(err))
	}

	// Determine API version to use
	apiVersion := *stripeAPIVersion
	if apiVersion == "" && apiSpec.Info != nil {
		apiVersion = apiSpec.Info.Version
	}
	if apiVersion == "" {
		apiVersion = "2020-08-27" // Fallback default
	}

	logger.Info("Starting stateful mock server",
		zap.String("port", *port),
		zap.Bool("verbose", *verbose),
		zap.String("stripe-api-version", apiVersion),
	)

	s, err := server.NewServer(apiSpec, *verbose, apiVersion, logger)
	if err != nil {
		logger.Fatal("Failed to create server", zap.Error(err))
	}

	// Register Stripe API handlers
	if err := s.RegisterStripeHandlers(); err != nil {
		logger.Fatal("Failed to register Stripe handlers", zap.Error(err))
	}

	logger.Info("Server ready")

	// Note: The custom logger is passed to DoubleSlashFixHandler in the server initialization
	handler := s.HandleHTTP()

	logger.Info("Listening on", zap.String("address", ":"+*port))

	if err := http.ListenAndServe(":"+*port, handler); err != nil {
		logger.Fatal("Server error", zap.Error(err))
	}
}

// setupLogger configures a zap logger based on the verbose flag
func setupLogger(verbose bool) *zap.Logger {
	level := zapcore.InfoLevel
	if verbose {
		level = zapcore.DebugLevel
	}

	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(level),
		Development: verbose,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := config.Build()
	if err != nil {
		panic(err) // Use panic here since SetupLogger is typically initialization code
	}
	return logger
}
