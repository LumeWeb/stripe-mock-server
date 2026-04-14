package generator

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/stripe-mock-server/pkg/spec"
)

// Version set in Stripe-Mock-Version response header
const Version = "mock-server-v1"

// ComponentsForValidation wraps spec components for validation
type ComponentsForValidation struct {
	Schemas map[string]*spec.Schema
}

// Generator generates mock responses
type Generator struct {
	Components ComponentsForValidation
	Verbose    bool
}

// GenerateParams is a parameters structure for generating responses
type GenerateParams struct {
	Expansions    map[string]interface{}
	PathParams    map[string]string
	RequestData   map[string]interface{}
	RequestMethod string
	RequestPath   string
	Schema        *spec.Schema
}

// Generate generates a mock response based on the schema
func (g *Generator) Generate(params *GenerateParams) (interface{}, error) {
	start := time.Now()
	if g.Verbose {
		log.Printf("Generating response for schema: %s", params.Schema)
	}

	result, err := g.generateFromSchema(params.Schema, params)
	if err != nil {
		return nil, err
	}

	if g.Verbose {
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			panic(err)
		}
		log.Printf("Generated data: %s", jsonBytes)
		log.Printf("Response elapsed: %v", time.Since(start))
	}

	return result, nil
}

// generateFromSchema generates data from a schema
func (g *Generator) generateFromSchema(schema *spec.Schema, params *GenerateParams) (interface{}, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema is nil")
	}

	// Handle $ref
	if schema.Ref != "" {
		refName := strings.TrimPrefix(schema.Ref, "#/components/schemas/")
		refSchema, ok := g.Components.Schemas[refName]
		if !ok {
			return nil, fmt.Errorf("reference not found: %s", refName)
		}
		return g.generateFromSchema(refSchema, params)
	}

	// Handle type
	switch schema.Type {
	case spec.TypeString:
		return g.generateString(schema)
	case spec.TypeInteger:
		return g.generateInteger(schema)
	case spec.TypeNumber:
		return g.generateNumber(schema)
	case spec.TypeBoolean:
		return g.generateBoolean(schema)
	case spec.TypeArray:
		return g.generateArray(schema, params)
	case spec.TypeObject:
		return g.generateObject(schema, params)
	default:
		return nil, fmt.Errorf("unsupported type: %s", schema.Type)
	}
}

// generateString generates a string value
func (g *Generator) generateString(schema *spec.Schema) (string, error) {
	// Handle enum
	if len(schema.Enum) > 0 {
		return schema.Enum[rand.Intn(len(schema.Enum))].(string), nil
	}

	// Generate a reasonable string based on pattern or format
	if schema.Pattern != "" {
		return "value", nil
	}

	if schema.Format == "email" {
		return "test@example.com", nil
	}

	return "test_value", nil
}

// generateInteger generates an integer value
func (g *Generator) generateInteger(schema *spec.Schema) (int, error) {
	if len(schema.Enum) > 0 {
		return schema.Enum[rand.Intn(len(schema.Enum))].(int), nil
	}

	// Generate a random integer
	return rand.Intn(1000), nil
}

// generateNumber generates a float value
func (g *Generator) generateNumber(schema *spec.Schema) (float64, error) {
	if len(schema.Enum) > 0 {
		return schema.Enum[rand.Intn(len(schema.Enum))].(float64), nil
	}

	// Generate a random number
	return rand.Float64() * 100.0, nil
}

// generateBoolean generates a boolean value
func (g *Generator) generateBoolean(schema *spec.Schema) (bool, error) {
	if len(schema.Enum) > 0 {
		// Filter to only boolean values and pick randomly, consistent with other generators
		var boolEnums []bool
		for _, e := range schema.Enum {
			if b, ok := e.(bool); ok {
				boolEnums = append(boolEnums, b)
			}
		}
		if len(boolEnums) > 0 {
			return boolEnums[rand.Intn(len(boolEnums))], nil
		}
	}

	return rand.Intn(2) == 0, nil
}

// generateArray generates an array value
func (g *Generator) generateArray(schema *spec.Schema, params *GenerateParams) ([]interface{}, error) {
	items := schema.Items
	if items == nil {
		return []interface{}{}, nil
	}

	// Generate 1-3 elements
	count := 1 + rand.Intn(3)

	result := make([]interface{}, 0, count)
	for i := 0; i < count; i++ {
		val, err := g.generateFromSchema(items, params)
		if err != nil {
			return nil, err
		}
		result = append(result, val)
	}

	return result, nil
}

// generateObject generates an object value
func (g *Generator) generateObject(schema *spec.Schema, params *GenerateParams) (map[string]interface{}, error) {
	if schema.Properties == nil {
		return map[string]interface{}{}, nil
	}

	result := make(map[string]interface{})
	keys := make([]string, 0, len(schema.Properties))
	for k := range schema.Properties {
		keys = append(keys, k)
	}

	// Shuffle keys for randomness
	rand.Shuffle(len(keys), func(i, j int) {
		keys[i], keys[j] = keys[j], keys[i]
	})

	// Generate values for each property
	for _, key := range keys {
		propertySchema := schema.Properties[key]

		// Check if required
		if !g.isRequired(schema, key) {
			continue
		}

		val, err := g.generateFromSchema(propertySchema, params)
		if err != nil {
			return nil, err
		}

		result[key] = val
	}

	return result, nil
}

// isRequired checks if a property is required
func (g *Generator) isRequired(schema *spec.Schema, propertyName string) bool {
	for _, req := range schema.Required {
		if req == propertyName {
			return true
		}
	}
	return false
}
