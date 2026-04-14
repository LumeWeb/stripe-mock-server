//go:generate go generate ./pkg/internal/gen/models

package main

import (
	"flag"
	"net/http"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/stripe-mock-server/pkg/server"
	"github.com/stripe-mock-server/pkg/spec"
)

func main() {
	// Parse command-line flags
	port := flag.String("port", "8080", "Port to listen on")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	// Create logger
	logger := setupLogger(*verbose)
	defer logger.Sync() // Flush any buffered log entries

	logger.Info("Starting stateful mock server",
		zap.String("port", *port),
		zap.Bool("verbose", *verbose),
	)

	logger.Info("Loading embedded spec")

	apiSpec, err := spec.GetSpec()
	if err != nil {
		logger.Fatal("Failed to get spec", zap.Error(err))
	}
	s, err := server.NewServer(apiSpec, *verbose)
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
	var development bool
	if verbose {
		development = true
	}

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Development:      development,
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