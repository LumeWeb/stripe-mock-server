# AGENTS.md
This file provides guidance to various AI agents when working with code in this repository.

## Common Commands

### Build
```bash
go build -o bin/server ./cmd/server/
```

### Run
```bash
./bin/server
# Server runs on port 8080 by default
```

### Test
```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./pkg/gateway
go test ./pkg/server
go test ./pkg/storage
```

### Generate Models
```bash
# Regenerate Go types from OpenAPI spec
go generate ./pkg/internal/gen/models/...
```

### Clean
```bash
go clean
rm -rf bin/server
```

## High-Level Architecture

This is a stateful HTTP mock server for APIs, specifically designed to mimic Stripe-like API behavior. The server is driven by an embedded OpenAPI 3.0 specification.

### Architecture Flow
```
HTTP Request (net/http)
       ↓
Server.ServeHTTP: Route matching via Go 1.22+ path variables
       ↓
param.ParseParams: Parse query/body (Rack-style encoding from stripe-mock)
       ↓
StripeHandlers: Custom handlers for Stripe operations
       ↓
Gateway: Business logic layer with typed repositories
       ↓
storage.Repository[T]: Generic in-memory CRUD operations
       ↓
Response: JSON response + Stripe-like formatting
```

### Package Structure

**`pkg/spec/`**: OpenAPI schema types and validation
- `spec.go`: Core types (Spec, Schema, Operation, Parameter, Response, etc.)
- `validate.go`: Converts OpenAPI 3 schemas to JSON Schema and builds validators
- `query.go`: Builds query parameter schemas from operation definitions
- `embed.go`: Embeds OpenAPI spec (openapi.json) using `go:embed`
- Uses `github.com/lestrrat-go/jsschema` and `github.com/lestrrat-go/jsval` for validation

**`pkg/gateway/`**: Business logic layer for Stripe operations
- `gateway.go`: Gateway struct with typed repositories for all resources (Customer, Subscription, Invoice, Charge, etc.)
- `proration.go`: Proration calculations for subscription changes using `github.com/shopspring/decimal`
- Handles subscription lifecycle: create, update, cancel, renew, expire
- Manages checkout sessions and billing portal

**`pkg/internal/gen/models/`**: Generated Go types from OpenAPI spec
- `generate.go`: `go:generate` directive for oapi-codegen
- `config.yaml`: oapi-codegen configuration
- `api/`: Generated types (Customer, Subscription, Invoice, Charge, Product, Price, etc.)

**`pkg/server/`**: Main HTTP server and handlers
- `server.go`: Server struct with routing via Go 1.22+ path variables, uses stripe-mock/param for parsing
- `stripe_handlers.go`: Custom handlers for all Stripe operations (customers, subscriptions, checkout sessions, etc.)
- `webhook_service.go`: Webhook delivery with worker pool and retry logic
- `helpers.go`: Helper functions for response formatting
- `api_helpers.go`: API-specific helper utilities
- Uses `github.com/stripe/stripe-mock/param` for Rack-style parameter parsing

**`pkg/storage/`**: Generic repository with in-memory implementation
- `Repository[T]` interface: Generic `Create()`, `Get()`, `Update()`, `Delete()`, `List()`, `Exists()`
- `InMemoryRepository[T]`: Thread-safe per-resource-type locking using `sync.RWMutex`
- Stores data as `map[string]map[string]json.RawMessage` (resourceType → id → raw JSON)
- Automatically adds `id`, `created`, `object` fields
- Uses `pkg/generator` for Stripe-like IDs

**`pkg/generator/`**: Stripe-like ID generation
- `GenerateID(prefix)`: Returns `<prefix>_<time5chars><random10chars>`
- Time encoding uses base-62 with reference timestamp `1342389380` (from stripe-mock)
- Characters: `0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz`

**`cmd/server/`**: Entry point
- `main.go`: Creates server, listens on port 8080

## Key Implementation Details

### Rack-style Parameter Encoding
Uses `github.com/stripe/stripe-mock/param` for parsing nested structures:
- `invoice_lines[]=line1&invoice_lines[]=line2` → `{"invoice_lines": ["line1", "line2"]}`
- `metadata[key1]=val1&metadata[key2]=val2` → `{"metadata": {"key1": "val1", "key2": "val2"}}`
- `obj[][foo]=bar` → Array of objects with nested properties

### Type Coercion
Form-encoding only supports strings, so param package converts them based on schema:
- boolean type: `strconv.ParseBool()`
- integer type: `strconv.Atoi()`
- number type: `strconv.ParseFloat()`
- Arrays may come as integer-indexed maps: `{"0": "a", "1": "b"}` → `["a", "b"]`

### OpenAPI Spec Handling
- Spec is embedded in `pkg/spec/openapi.json` using `go:embed`
- **WARNING**: Never read `pkg/spec/openapi.json` directly. The file is extremely large and will overload agent context. Use the `pkg/spec` package APIs to interact with the spec programmatically.
- OpenAPI "nullable: true" is converted to JSON Schema `anyOf` with null type
- Schema refs use `#/components/schemas/Name` format
- Query parameters are extracted from `operation.Parameters` and built into schemas

### Generic Repository Pattern
- `Repository[T]` is a generic interface for type-safe storage
- Each resource type has its own repository instance
- Thread-safe with per-resource-type `sync.RWMutex`
- Returns copies of stored data to prevent external modifications

### Webhook Delivery
- `WebhookService` manages webhook endpoints and delivers events
- Uses `github.com/gammazero/workerpool` for concurrent delivery
- Retry logic via `github.com/avast/retry-go/v5` (5 attempts max)
- Events delivered with proper Stripe signature headers

### Proration
- Supports Stripe proration behaviors: `none`, `create_prorations`, `always_invoice`
- Uses `github.com/shopspring/decimal` for precise monetary calculations
- Handles cross-cadence subscription changes

## Dependencies

**External imports from stripe-mock:**
- `github.com/stripe/stripe-mock/param` - Parameter parsing (do not modify)
- `github.com/stripe/stripe-mock/spec` - Spec types (do not modify)

**Key dependencies:**
- `github.com/stripe/stripe-go/v85` - Stripe SDK types and webhook signing
- `github.com/lestrrat-go/jsschema` and `jsval` - JSON Schema validation
- `github.com/gammazero/workerpool` - Worker pool for webhook delivery
- `github.com/avast/retry-go/v5` - Retry logic
- `github.com/shopspring/decimal` - Precise decimal arithmetic

## Testing

Run tests to validate OpenAPI spec changes:
```bash
go test ./pkg/...
```

## License
MIT License - Copyright (c) 2026 Lume Web
