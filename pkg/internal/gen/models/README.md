# Stripe Mock Server - Model Generation

This directory contains the auto-generated API models from the OpenAPI specification.

## Setup

The models are generated using `oapi-codegen` based on the only-models example.

## Generation

To regenerate the API models, run:

```bash
cd pkg/internal/gen/models && go generate
```

Or run from the root of the repository:

```bash
go generate ./pkg/internal/gen/models
```

## Generated Files

- `api/gen.go` - Generated API models from the OpenAPI spec

## Usage

Import and use the generated API types:

```go
import "github.com/stripe-mock-server/pkg/internal/gen/models/api"

// Use generated types
var myType api.SomeType
```

## Notes

- The `api/gen.go` file is auto-generated and should not be manually edited.
- Regeneration should be done whenever the OpenAPI specification changes.
- Duplicate type declarations in the generated file are expected due to the Stripe OpenAPI spec having duplicate schema names.
