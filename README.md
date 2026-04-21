# Stripe Mock Server

A stateful HTTP mock server for Stripe-like API testing. Implements subscription lifecycle, checkout sessions, webhooks, and proration with OpenAPI 3.0-driven validation.

## Features

- **Subscription Lifecycle**: Create, update, cancel, renew, and expire subscriptions
- **Checkout Sessions**: Full checkout session workflow with completion
- **Webhook Delivery**: Concurrent delivery with worker pool and retry logic
- **Proration**: Accurate proration calculations for subscription changes
- **Billing Portal**: Customer self-serve billing configuration
- **OpenAPI 3.0 Driven**: Embedded spec with request/response validation
- **Stateful Storage**: Type-safe generic repository with in-memory persistence
- **Stripe-like IDs**: Time-based ID generation matching Stripe's format

## Project Structure

```
stripe-mock-server/
├── cmd/stripe-mock-server/main.go   # Entry point
├── pkg/
│   ├── gateway/                    # Business logic layer
│   │   ├── gateway.go              # Typed repositories and operations
│   │   └── proration.go            # Subscription proration calculations
│   ├── server/                     # HTTP server and handlers
│   │   ├── server.go               # Routing and request handling
│   │   ├── stripe_handlers.go      # Stripe API handlers
│   │   ├── webhook_service.go     # Webhook delivery with worker pool
│   │   └── helpers.go              # Response formatting
│   ├── storage/repository.go       # Generic Repository[T] interface
│   ├── spec/                       # OpenAPI schema and validation
│   │   ├── openapi.json            # Embedded Stripe OpenAPI spec
│   │   └── validate.go             # Request validation
│   ├── generator/                  # Stripe-like ID generation
│   └── internal/gen/models/        # Generated Go types from OpenAPI
├── scripts/normalize_openapi.py    # OpenAPI spec preprocessing
├── go.mod
└── go.sum
```

## Building

```bash
go build -o bin/server ./cmd/stripe-mock-server/
```

## Running

```bash
./bin/server
# Server runs on port 8080 by default
# OpenAPI spec is embedded - no external spec file needed
```

### Options

- `-port`: Port to listen on (default: 8080)
- `-verbose`: Enable verbose logging (default: false)

## API Usage

### Authentication

All requests require an `Authorization` header:

```bash
Authorization: Bearer sk_test_123
```

### Create a Customer

```bash
curl -X POST http://localhost:8080/v1/customers \
  -H "Authorization: Bearer sk_test_123" \
  -d "name=John Doe&email=john@example.com"
```

### Create a Product and Price

```bash
curl -X POST http://localhost:8080/v1/products \
  -H "Authorization: Bearer sk_test_123" \
  -d "name=Pro Plan"

curl -X POST http://localhost:8080/v1/prices \
  -H "Authorization: Bearer sk_test_123" \
  -d "product=prod_xxx&unit_amount=1999&currency=usd&recurring[interval]=month"
```

### Create a Subscription

```bash
curl -X POST http://localhost:8080/v1/subscriptions \
  -H "Authorization: Bearer sk_test_123" \
  -d "customer=cus_xxx&items[0][price]=price_xxx"
```

### Create a Checkout Session

```bash
curl -X POST http://localhost:8080/v1/checkout/sessions \
  -H "Authorization: Bearer sk_test_123" \
  -d "customer=cus_xxx&success_url=https://example.com/success&cancel_url=https://example.com/cancel"
```

### Create a Webhook Endpoint

```bash
curl -X POST http://localhost:8080/v1/webhook_endpoints \
  -H "Authorization: Bearer sk_test_123" \
  -d "url=https://example.com/webhooks&enabled_events[]=invoice.paid"
```

## Webhook Delivery

Webhooks are delivered asynchronously via a worker pool:

- **Concurrent delivery**: Multiple webhooks processed in parallel
- **Retry logic**: Up to 5 attempts with exponential backoff
- **Stripe signatures**: Proper webhook signing with `Stripe-Signature` header
- **Event ordering**: Incrementing timestamps for proper event sequencing

## Subscription Proration

The server supports Stripe-compatible proration behaviors:

- `none`: No proration credits or charges
- `create_prorations`: Generate proration line items
- `always_invoice`: Immediately invoice proration items

Proration calculations handle cross-cadence changes (e.g., monthly to yearly) using precise decimal arithmetic.

## ID Generation

Stripe-like IDs with format: `<prefix>_<time5chars><random10chars>`

Examples:
- Customers: `cus_TLVphx7YrQyLtWP`
- Subscriptions: `sub_TLVphWXlcPZZ9sI`
- Charges: `ch_TLVphWXlcPZZ9sI`

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    HTTP Layer                           │
│  Go 1.22+ path variables + stripe-mock/param parsing    │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│              Stripe Handlers (pkg/server)               │
│  Custom handlers for each Stripe operation              │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│              Gateway (pkg/gateway)                      │
│  Business logic: subscriptions, checkout, proration    │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│          Repository[T] (pkg/storage)                   │
│  Type-safe generic storage with per-resource locking    │
└─────────────────────────────────────────────────────────┘
```

## Testing

```bash
go test ./...

# Test specific packages
go test ./pkg/gateway
go test ./pkg/server
go test ./pkg/storage
```

## Regenerating Types

Go types are generated from the OpenAPI spec:

```bash
go generate ./pkg/internal/gen/models/...
```

## Dependencies

- `github.com/stripe/stripe-mock/param` - Rack-style parameter parsing
- `github.com/stripe/stripe-go/v85` - Stripe SDK types and webhook signing
- `github.com/lestrrat-go/jsschema` + `jsval` - JSON Schema validation
- `github.com/gammazero/workerpool` - Concurrent webhook delivery
- `github.com/avast/retry-go/v5` - Webhook retry logic
- `github.com/shopspring/decimal` - Precise proration calculations

## License

MIT License - Copyright (c) 2026 Lume Web
