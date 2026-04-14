package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetString tests getting string values from a map
func TestGetString(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		key      string
		expected string
	}{
		{"string value", map[string]any{"name": "John"}, "name", "John"},
		{"value not found", map[string]any{"name": "John"}, "missing", ""},
		{"empty string", map[string]any{"email": ""}, "email", ""},
		{"nil map", nil, "name", ""},
		{"non-string value (converted)", map[string]any{"count": 123}, "count", "123"}, // koanf converts to string
		{"float value (converted)", map[string]any{"price": 12.34}, "price", "12.34"},   // koanf converts to string
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetString(tt.m, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetInt64 tests getting int64 values from a map
func TestGetInt64(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		key      string
		expected int64
	}{
		{"int64 value", map[string]any{"count": int64(100)}, "count", 100},
		{"int value", map[string]any{"count": 200}, "count", 200},
		{"int32 value", map[string]any{"count": int32(300)}, "count", 300},
		{"float64 value", map[string]any{"count": 400.0}, "count", 400},
		{"value not found", map[string]any{"name": "John"}, "count", 0},
		{"nil map", nil, "count", 0},
		{"negative value", map[string]any{"balance": -50}, "balance", -50},
		{"large value", map[string]any{"amount": 9999999999}, "amount", 9999999999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetInt64(tt.m, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetBool tests getting boolean values from a map
func TestGetBool(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		key      string
		expected bool
	}{
		{"true value", map[string]any{"active": true}, "active", true},
		{"false value", map[string]any{"active": false}, "active", false},
		{"value not found", map[string]any{"name": "John"}, "active", false},
		{"nil map", nil, "active", false},
		{"string 'true'", map[string]any{"active": "true"}, "active", true},
		{"string 'false'", map[string]any{"active": "false"}, "active", false},
		{"non-convertible string", map[string]any{"active": "yes"}, "active", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetBool(tt.m, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetStringMap tests getting nested map from a map
func TestGetStringMap(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		key      string
		expected map[string]any
	}{
		{
			name: "nested map found",
			m: map[string]any{
				"metadata": map[string]any{"key1": "val1", "key2": "val2"},
			},
			key:      "metadata",
			expected: map[string]any{"key1": "val1", "key2": "val2"},
		},
		{
			name: "value not found",
			m:    map[string]any{"name": "John"},
			key:  "metadata",
			expected: nil,
		},
		{
			name: "empty map",
			m:    map[string]any{"metadata": map[string]any{}},
			key:  "metadata",
			expected: map[string]any{},
		},
		{
			name: "nil map",
			m:    nil,
			key:  "metadata",
			expected: nil,
		},
		{
			name:     "non-map value",
			m:        map[string]any{"metadata": "not a map"},
			key:      "metadata",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStringMap(tt.m, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetMapSlice tests getting slice of maps from a map
func TestGetMapSlice(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		key      string
		expected []map[string]any
	}{
		{
			name: "slice of maps found",
			m: map[string]any{
				"items": []any{
					map[string]any{"id": "1", "name": "Item 1"},
					map[string]any{"id": "2", "name": "Item 2"},
				},
			},
			key: "items",
			expected: []map[string]any{
				{"id": "1", "name": "Item 1"},
				{"id": "2", "name": "Item 2"},
			},
		},
		{
			name: "slice of maps from Go literal - []map[string]any",
			m: map[string]any{
				"items": []map[string]any{
					{"id": "1", "name": "Item 1"},
					{"id": "2", "name": "Item 2"},
				},
			},
			key: "items",
			expected: []map[string]any{
				{"id": "1", "name": "Item 1"},
				{"id": "2", "name": "Item 2"},
			},
		},
		{
			name:     "value not found",
			m:        map[string]any{"name": "John"},
			key:      "items",
			expected: nil,
		},
		{
			name:     "empty slice",
			m:        map[string]any{"items": []any{}},
			key:      "items",
			expected: nil, // koanf returns nil for empty slices
		},
		{
			name: "mixed slice with some non-maps",
			m: map[string]any{
				"items": []any{
					map[string]any{"id": "1"},
					"string",
					123,
					map[string]any{"id": "2"},
				},
			},
			key: "items",
			expected: []map[string]any{
				{"id": "1"},
				{"id": "2"},
			},
		},
		{
			name:     "nil map",
			m:        nil,
			key:      "items",
			expected: nil,
		},
		{
			name:     "non-slice value",
			m:        map[string]any{"items": "not a slice"},
			key:      "items",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMapSlice(tt.m, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetStringSlice tests getting slice of strings from a map
func TestGetStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		m        map[string]any
		key      string
		expected []string
	}{
		{
			name:     "slice of strings found",
			m:        map[string]any{"tags": []any{"tag1", "tag2", "tag3"}},
			key:      "tags",
			expected: []string{"tag1", "tag2", "tag3"},
		},
		{
			name:     "value not found",
			m:        map[string]any{"name": "John"},
			key:      "tags",
			expected: []string{}, // koanf returns empty slice when key not found
		},
		{
			name:     "empty slice",
			m:        map[string]any{"tags": []any{}},
			key:      "tags",
			expected: []string{}, // koanf returns empty slice for empty slice
		},
		{
			name: "mixed slice with some non-strings",
			m: map[string]any{
				"tags": []any{"tag1", 123, "tag2", true, "tag3"},
			},
			key:      "tags",
			expected: []string{"tag1", "123", "tag2", "true", "tag3"}, // koanf converts all to strings
		},
		{
			name:     "nil map",
			m:        nil,
			key:      "tags",
			expected: []string{}, // koanf returns empty slice for nil map
		},
		{
			name:     "non-slice value",
			m:        map[string]any{"tags": "not a slice"},
			key:      "tags",
			expected: []string{}, // koanf returns empty slice for non-slice
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetStringSlice(tt.m, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestGetStringWithEmpty tests getting string pointer from a map
func TestGetStringWithEmpty(t *testing.T) {
	tests := []struct {
		name       string
		m          map[string]any
		key        string
		wantString string
		wantBool   bool
	}{
		{
			name:       "non-empty string",
			m:          map[string]any{"name": "John"},
			key:        "name",
			wantString: "John",
			wantBool:   true,
		},
		{
			name:       "empty string",
			m:          map[string]any{"name": ""},
			key:        "name",
			wantString: "",
			wantBool:   false,
		},
		{
			name:       "value not found",
			m:          map[string]any{"email": "test@example.com"},
			key:        "name",
			wantString: "",
			wantBool:   false,
		},
		{
			name:       "nil map",
			m:          nil,
			key:        "name",
			wantString: "",
			wantBool:   false,
		},
		{
			name:       "non-string value (converted)",
			m:          map[string]any{"name": 123},
			key:        "name",
			wantString: "123",
			wantBool:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ptr, ok := GetStringWithEmpty(tt.m, tt.key)
			assert.Equal(t, tt.wantBool, ok)
			if tt.wantBool {
				require.NotNil(t, ptr)
				assert.Equal(t, tt.wantString, *ptr)
			} else {
				assert.Nil(t, ptr)
			}
		})
	}
}

// TestMapAnyToString tests converting map[string]any to map[string]string
func TestMapAnyToString(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]string
	}{
		{
			name: "all string values",
			input: map[string]any{
				"key1": "val1",
				"key2": "val2",
			},
			expected: map[string]string{
				"key1": "val1",
				"key2": "val2",
			},
		},
		{
			name: "mixed types",
			input: map[string]any{
				"key1": "val1",
				"key2": 123,
				"key3": true,
				"key4": int64(456),
			},
			expected: map[string]string{
				"key1": "val1",
				"key2": "123",
				"key3": "true",
				"key4": "456",
			},
		},
		{
			name:     "empty map",
			input:    map[string]any{},
			expected: map[string]string{},
		},
		{
			name:     "nil map",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapAnyToString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMapStringToAny tests converting map[string]string to map[string]any
func TestMapStringToAny(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]any
	}{
		{
			name: "simple conversion",
			input: map[string]string{
				"key1": "val1",
				"key2": "val2",
			},
			expected: map[string]any{
				"key1": "val1",
				"key2": "val2",
			},
		},
		{
			name:     "empty map",
			input:    map[string]string{},
			expected: map[string]any{},
		},
		{
			name:     "nil map",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapStringToAny(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMergeMetadata tests merging metadata maps
func TestMergeMetadata(t *testing.T) {
	tests := []struct {
		name     string
		existing map[string]string
		newMeta  map[string]any
		expected map[string]string
	}{
		{
			name: "merge new keys",
			existing: map[string]string{
				"existing": "value",
			},
			newMeta: map[string]any{
				"new": "newValue",
			},
			expected: map[string]string{
				"existing": "value",
				"new": "newValue",
			},
		},
		{
			name: "overwrite existing keys",
			existing: map[string]string{
				"key1": "old",
				"key2": "stay",
			},
			newMeta: map[string]any{
				"key1": "new",
			},
			expected: map[string]string{
				"key1": "new",
				"key2": "stay",
			},
		},
		{
			name: "nil existing",
			existing: nil,
			newMeta: map[string]any{
				"key1": "value1",
			},
			expected: map[string]string{
				"key1": "value1",
			},
		},
		{
			name:     "nil new meta",
			existing: map[string]string{"key": "value"},
			newMeta:  nil,
			expected: map[string]string{"key": "value"},
		},
		{
			name:     "both nil",
			existing: nil,
			newMeta:  nil,
			expected: nil,
		},
		{
			name: "type conversion in new meta",
			existing: map[string]string{
				"key": "value",
			},
			newMeta: map[string]any{
				"number": 123,
				"bool":   true,
			},
			expected: map[string]string{
				"key":    "value",
				"number": "123",
				"bool":   "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeMetadata(tt.existing, tt.newMeta)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMapStringToBool tests converting string to boolean
func TestMapStringToBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		{"yes", false},
		{"no", false},
		{"1", false},
		{"0", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapStringToBool(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestStringToUnion tests converting string to json.RawMessage
func TestStringToUnion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // The string representation of the JSON
	}{
		{
			name:     "simple string",
			input:    "test_value",
			expected: `"test_value"`,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "string with spaces",
			input:    "hello world",
			expected: `"hello world"`,
		},
		{
			name:     "string with special chars",
			input:    `{"key": "value"}`,
			expected: `"{\"key\": \"value\"}"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stringToUnion(tt.input)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

// TestParseCustomerUpdateAllowedUpdates tests parsing customer update allowed updates
func TestParseCustomerUpdateAllowedUpdates(t *testing.T) {
	tests := []struct {
		name     string
		vals     []string
		expected []string // For comparison, convert back to strings
	}{
		{
			name: "single value",
			vals: []string{"address"},
			expected: []string{"address"},
		},
		{
			name: "multiple values",
			vals: []string{"address", "email", "name", "phone"},
			expected: []string{"address", "email", "name", "phone"},
		},
		{
			name:     "empty slice",
			vals:     []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCustomerUpdateAllowedUpdates(tt.vals)
			require.Equal(t, len(tt.expected), len(result))
			for i, val := range result {
				assert.Equal(t, tt.expected[i], string(val))
			}
		})
	}
}

// TestParseSubscriptionUpdateDefaultAllowedUpdates tests parsing subscription update default allowed updates
func TestParseSubscriptionUpdateDefaultAllowedUpdates(t *testing.T) {
	tests := []struct {
		name     string
		vals     []string
		expected []string
	}{
		{
			name: "single value",
			vals: []string{"price"},
			expected: []string{"price"},
		},
		{
			name: "multiple values",
			vals: []string{"price", "promotion_code", "quantity"},
			expected: []string{"price", "promotion_code", "quantity"},
		},
		{
			name:     "empty slice",
			vals:     []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSubscriptionUpdateDefaultAllowedUpdates(tt.vals)
			require.Equal(t, len(tt.expected), len(result))
			for i, val := range result {
				assert.Equal(t, tt.expected[i], string(val))
			}
		})
	}
}

// TestParseSubscriptionCancelReasonOptions tests parsing subscription cancel reason options
func TestParseSubscriptionCancelReasonOptions(t *testing.T) {
	tests := []struct {
		name     string
		vals     []string
		expected []string
	}{
		{
			name: "single value",
			vals: []string{"too_expensive"},
			expected: []string{"too_expensive"},
		},
		{
			name: "multiple values",
			vals: []string{"too_expensive", "missing_features", "switched_service"},
			expected: []string{"too_expensive", "missing_features", "switched_service"},
		},
		{
			name:     "empty slice",
			vals:     []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSubscriptionCancelReasonOptions(tt.vals)
			require.Equal(t, len(tt.expected), len(result))
			for i, val := range result {
				assert.Equal(t, tt.expected[i], string(val))
			}
		})
	}
}

// TestBuildCustomer tests building a Customer from input data
func TestBuildCustomer(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		data     map[string]any
		checkers []func(*testing.T, any) // Custom checkers for complex fields
	}{
		{
			name: "minimal customer",
			id:   "cus_123",
			data: map[string]any{},
		},
		{
			name: "customer with basic fields",
			id:   "cus_456",
			data: map[string]any{
				"email":       "test@example.com",
				"name":        "John Doe",
				"description": "Test customer",
				"phone":       "+1234567890",
				"currency":    "usd",
			},
		},
		{
			name: "customer with balance",
			id:   "cus_789",
			data: map[string]any{
				"balance": int64(-500),
			},
		},
		{
			name: "customer with address",
			id:   "cus_abc",
			data: map[string]any{
				"address": map[string]any{
					"line1":       "123 Main St",
					"city":        "San Francisco",
					"state":       "CA",
					"postal_code": "94105",
					"country":     "US",
				},
			},
		},
		{
			name: "customer with shipping",
			id:   "cus_def",
			data: map[string]any{
				"shipping": map[string]any{
					"name":            "John Doe",
					"phone":           "+1234567890",
					"carrier":         "UPS",
					"tracking_number": "1Z999AA10123456784",
					"address": map[string]any{
						"line1": "123 Main St",
						"city":  "San Francisco",
					},
				},
			},
		},
		{
			name: "customer with metadata",
			id:   "cus_ghi",
			data: map[string]any{
				"metadata": map[string]any{
					"user_id": "123",
					"tier":    "premium",
				},
			},
		},
		{
			name: "customer with all fields",
			id:   "cus_jkl",
			data: map[string]any{
				"email":       "full@example.com",
				"name":        "Full Customer",
				"description": "Complete customer",
				"phone":       "+9876543210",
				"currency":    "usd",
				"balance":     int64(100),
				"address": map[string]any{
					"line1":       "456 Oak Ave",
					"city":        "New York",
					"state":       "NY",
					"postal_code": "10001",
					"country":     "US",
				},
				"shipping": map[string]any{
					"name":    "Full Customer",
					"phone":   "+9876543210",
					"carrier": "FedEx",
					"address": map[string]any{
						"line1": "456 Oak Ave",
						"city":  "New York",
					},
				},
				"metadata": map[string]any{
					"key1": "value1",
					"key2": "value2",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			customer := buildCustomer(tt.id, tt.data)

			assert.NotNil(t, customer)
			assert.Equal(t, tt.id, customer.Id)
			assert.Equal(t, "customer", string(customer.Object))
			assert.False(t, customer.Livemode)

			// Check basic fields
			if email, ok := tt.data["email"].(string); ok && email != "" {
				require.NotNil(t, customer.Email)
				assert.Equal(t, email, *customer.Email)
			}

			if name, ok := tt.data["name"].(string); ok && name != "" {
				require.NotNil(t, customer.Name)
				assert.Equal(t, name, *customer.Name)
			}

			if desc, ok := tt.data["description"].(string); ok && desc != "" {
				require.NotNil(t, customer.Description)
				assert.Equal(t, desc, *customer.Description)
			}

			if phone, ok := tt.data["phone"].(string); ok && phone != "" {
				require.NotNil(t, customer.Phone)
				assert.Equal(t, phone, *customer.Phone)
			}

			if currency, ok := tt.data["currency"].(string); ok && currency != "" {
				require.NotNil(t, customer.Currency)
				assert.Equal(t, currency, *customer.Currency)
			}

			if balance, ok := tt.data["balance"].(int64); ok && balance != 0 {
				require.NotNil(t, customer.Balance)
				assert.Equal(t, int(balance), *customer.Balance)
			}

			if metadata, ok := tt.data["metadata"].(map[string]any); ok && len(metadata) > 0 {
				require.NotNil(t, customer.Metadata)
				expectedMeta := mapAnyToString(metadata)
				assert.Equal(t, expectedMeta, *customer.Metadata)
			}
		})
	}
}

// TestBuildProduct tests building a Product from input data
func TestBuildProduct(t *testing.T) {
	tests := []struct {
		name string
		id   string
		data map[string]any
	}{
		{
			name: "minimal product",
			id:   "prod_123",
			data: map[string]any{},
		},
		{
			name: "product with name and description",
			id:   "prod_456",
			data: map[string]any{
				"name":        "Premium Plan",
				"description": "Our premium subscription",
			},
		},
		{
			name: "product with active flag",
			id:   "prod_789",
			data: map[string]any{
				"name":   "Inactive Plan",
				"active": false,
			},
		},
		{
			name: "product with images",
			id:   "prod_abc",
			data: map[string]any{
				"name":   "Product with Images",
				"images": []any{"image1.jpg", "image2.jpg"},
			},
		},
		{
			name: "product with URL",
			id:   "prod_def",
			data: map[string]any{
				"name": "Product with URL",
				"url":  "https://example.com/product",
			},
		},
		{
			name: "product with unit label",
			id:   "prod_ghi",
			data: map[string]any{
				"name":       "Subscription",
				"unit_label": "seat",
			},
		},
		{
			name: "product with statement descriptor",
			id:   "prod_jkl",
			data: map[string]any{
				"name":                 "Product",
				"statement_descriptor": "COMPANY INC",
			},
		},
		{
			name: "product with shippable flag",
			id:   "prod_mno",
			data: map[string]any{
				"name":      "Physical Product",
				"shippable": true,
			},
		},
		{
			name: "product with default price",
			id:   "prod_pqr",
			data: map[string]any{
				"name":          "Product",
				"default_price": "price_123",
			},
		},
		{
			name: "product with metadata",
			id:   "prod_stu",
			data: map[string]any{
				"name": "Product",
				"metadata": map[string]any{
					"category": "electronics",
					"brand":    "example",
				},
			},
		},
		{
			name: "product with all fields",
			id:   "prod_full",
			data: map[string]any{
				"name":                 "Complete Product",
				"description":           "Full featured product",
				"active":               true,
				"images":               []any{"img1.jpg", "img2.jpg", "img3.jpg"},
				"url":                  "https://example.com/full",
				"unit_label":           "item",
				"statement_descriptor": "FULL PRODUCT",
				"shippable":            true,
				"default_price":        "price_456",
				"metadata": map[string]any{
					"version": "2.0",
					"tier":    "premium",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			product := buildProduct(tt.id, tt.data)

			assert.NotNil(t, product)
			assert.Equal(t, tt.id, product.Id)
			assert.Equal(t, "product", string(product.Object))
			assert.False(t, product.Livemode)
			assert.NotNil(t, product.Created)
			assert.NotNil(t, product.Updated)

			// Check name field (required for valid product)
			if name, ok := tt.data["name"].(string); ok && name != "" {
				assert.Equal(t, name, product.Name)
			}

			// Check description
			if desc, ok := tt.data["description"].(string); ok && desc != "" {
				require.NotNil(t, product.Description)
				assert.Equal(t, desc, *product.Description)
			}

			// Check active flag
			if active, ok := tt.data["active"].(bool); ok {
				assert.Equal(t, active, product.Active)
			}

			// Check images
			if images, ok := tt.data["images"].([]any); ok && len(images) > 0 {
				expectedImages := GetStringSlice(tt.data, "images")
				assert.Equal(t, expectedImages, product.Images)
			}

			// Check URL
			if url, ok := tt.data["url"].(string); ok && url != "" {
				require.NotNil(t, product.Url)
				assert.Equal(t, url, *product.Url)
			}

			// Check unit label
			if unitLabel, ok := tt.data["unit_label"].(string); ok && unitLabel != "" {
				require.NotNil(t, product.UnitLabel)
				assert.Equal(t, unitLabel, *product.UnitLabel)
			}

			// Check statement descriptor
			if stmtDesc, ok := tt.data["statement_descriptor"].(string); ok && stmtDesc != "" {
				require.NotNil(t, product.StatementDescriptor)
				assert.Equal(t, stmtDesc, *product.StatementDescriptor)
			}

			// Check shippable
			if shippable, ok := tt.data["shippable"].(bool); ok {
				require.NotNil(t, product.Shippable)
				assert.Equal(t, shippable, *product.Shippable)
			}

			// Check metadata
			if metadata, ok := tt.data["metadata"].(map[string]any); ok && len(metadata) > 0 {
				expectedMeta := mapAnyToString(metadata)
				assert.Equal(t, expectedMeta, product.Metadata)
			}
		})
	}
}

// TestBuildPrice tests building a Price from input data
func TestBuildPrice(t *testing.T) {
	tests := []struct {
		name string
		id   string
		data map[string]any
	}{
		{
			name: "minimal price",
			id:   "price_123",
			data: map[string]any{},
		},
		{
			name: "price with currency and amount",
			id:   "price_456",
			data: map[string]any{
				"currency":    "usd",
				"unit_amount": int64(2999),
			},
		},
		{
			name: "price with decimal amount",
			id:   "price_789",
			data: map[string]any{
				"currency":           "eur",
				"unit_amount_decimal": "29.99",
			},
		},
		{
			name: "price with product",
			id:   "price_abc",
			data: map[string]any{
				"currency":    "usd",
				"unit_amount": int64(999),
				"product":     "prod_123",
			},
		},
		{
			name: "price with nickname",
			id:   "price_def",
			data: map[string]any{
				"currency": "usd",
				"nickname": "Monthly Basic",
			},
		},
		{
			name: "price with lookup key",
			id:   "price_ghi",
			data: map[string]any{
				"currency":   "usd",
				"lookup_key": "monthly_basic",
			},
		},
		{
			name: "price with billing scheme",
			id:   "price_jkl",
			data: map[string]any{
				"currency":       "usd",
				"billing_scheme": "tiered",
			},
		},
		{
			name: "price with tax behavior",
			id:   "price_mno",
			data: map[string]any{
				"currency":      "usd",
				"tax_behavior":  "exclusive",
			},
		},
		{
			name: "price with type",
			id:   "price_pqr",
			data: map[string]any{
				"currency": "usd",
				"type":     "recurring",
			},
		},
		{
			name: "price with recurring",
			id:   "price_stu",
			data: map[string]any{
				"currency": "usd",
				"recurring": map[string]any{
					"interval":       "month",
					"interval_count": 1,
					"usage_type":     "licensed",
				},
			},
		},
		{
			name: "price with all fields",
			id:   "price_full",
			data: map[string]any{
				"currency":            "usd",
				"unit_amount":         int64(4999),
				"unit_amount_decimal": "49.99",
				"product":             "prod_789",
				"nickname":            "Monthly Premium",
				"lookup_key":          "monthly_premium",
				"billing_scheme":      "per_unit",
				"tiers_mode":          "graduated",
				"tax_behavior":        "exclusive",
				"type":                "recurring",
				"recurring": map[string]any{
					"interval":       "month",
					"interval_count": 3,
					"usage_type":     "metered",
				},
				"metadata": map[string]any{
					"promotional": "true",
					"region":      "us",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price := buildPrice(tt.id, tt.data)

			assert.NotNil(t, price)
			assert.Equal(t, tt.id, price.Id)
			assert.Equal(t, "price", string(price.Object))
			assert.False(t, price.Livemode)
			assert.True(t, price.Active)
			assert.NotNil(t, price.Created)

			// Check currency
			if currency, ok := tt.data["currency"].(string); ok && currency != "" {
				assert.Equal(t, currency, price.Currency)
			}

			// Check unit amount
			if amount, ok := tt.data["unit_amount"].(int64); ok && amount != 0 {
				require.NotNil(t, price.UnitAmount)
				assert.Equal(t, int(amount), *price.UnitAmount)
			}

			// Check unit amount decimal
			if decimal, ok := tt.data["unit_amount_decimal"].(string); ok && decimal != "" {
				require.NotNil(t, price.UnitAmountDecimal)
				assert.Equal(t, decimal, *price.UnitAmountDecimal)
			}

			// Check product (union type)
			if product, ok := tt.data["product"].(string); ok && product != "" {
				// Extract value from union
				productID, err := price.Product.AsPriceProduct0()
				assert.NoError(t, err)
				assert.Equal(t, product, productID)
			}

			// Check nickname
			if nickname, ok := tt.data["nickname"].(string); ok && nickname != "" {
				require.NotNil(t, price.Nickname)
				assert.Equal(t, nickname, *price.Nickname)
			}

			// Check lookup key
			if lookupKey, ok := tt.data["lookup_key"].(string); ok && lookupKey != "" {
				require.NotNil(t, price.LookupKey)
				assert.Equal(t, lookupKey, *price.LookupKey)
			}

			// Check billing scheme
			if billingScheme, ok := tt.data["billing_scheme"].(string); ok && billingScheme != "" {
				assert.Equal(t, billingScheme, string(price.BillingScheme))
			}

			// Check tax behavior
			if taxBehavior, ok := tt.data["tax_behavior"].(string); ok && taxBehavior != "" {
				require.NotNil(t, price.TaxBehavior)
				assert.Equal(t, taxBehavior, string(*price.TaxBehavior))
			}

			// Check type
			if priceType, ok := tt.data["type"].(string); ok && priceType != "" {
				assert.Equal(t, priceType, string(price.Type))
			}

			// Check recurring (union type)
			if recurring, ok := tt.data["recurring"].(map[string]any); ok && len(recurring) > 0 {
				require.NotNil(t, price.Recurring)
				// Verify recurring was set - check if it has data
				rec, err := price.Recurring.AsRecurring()
				if err == nil {
					// Verify the recurring interval was set correctly
					if interval, ok := recurring["interval"].(string); ok {
						assert.Equal(t, interval, string(rec.Interval))
					}
				}
			}

			// Check metadata
			if metadata, ok := tt.data["metadata"].(map[string]any); ok && len(metadata) > 0 {
				expectedMeta := mapAnyToString(metadata)
				assert.Equal(t, expectedMeta, price.Metadata)
			}
		})
	}
}

// TestBuildCheckoutSession tests building a CheckoutSession from input data
func TestBuildCheckoutSession(t *testing.T) {
	tests := []struct {
		name string
		id   string
		data map[string]any
	}{
		{
			name: "minimal checkout session",
			id:   "cs_123",
			data: map[string]any{},
		},
		{
			name: "checkout session with mode",
			id:   "cs_456",
			data: map[string]any{
				"mode": "subscription",
			},
		},
		{
			name: "checkout session with customer",
			id:   "cs_789",
			data: map[string]any{
				"customer": "cus_123",
			},
		},
		{
			name: "checkout session with customer email",
			id:   "cs_abc",
			data: map[string]any{
				"customer_email": "customer@example.com",
			},
		},
		{
			name: "checkout session with URLs",
			id:   "cs_def",
			data: map[string]any{
				"success_url": "https://example.com/success",
				"cancel_url":  "https://example.com/cancel",
			},
		},
		{
			name: "checkout session with currency",
			id:   "cs_ghi",
			data: map[string]any{
				"currency": "eur",
			},
		},
		{
			name: "checkout session with locale",
			id:   "cs_jkl",
			data: map[string]any{
				"locale": "fr-FR",
			},
		},
		{
			name: "checkout session with submit type",
			id:   "cs_mno",
			data: map[string]any{
				"submit_type": "subscribe",
			},
		},
		{
			name: "checkout session with billing address collection",
			id:   "cs_pqr",
			data: map[string]any{
				"billing_address_collection": "required",
			},
		},
		{
			name: "checkout session with allow promotion codes",
			id:   "cs_stu",
			data: map[string]any{
				"allow_promotion_codes": true,
			},
		},
		{
			name: "checkout session with metadata",
			id:   "cs_vwx",
			data: map[string]any{
				"metadata": map[string]any{
					"source":   "web",
					"campaign": "summer2024",
				},
			},
		},
		{
			name: "checkout session with client reference id",
			id:   "cs_yza",
			data: map[string]any{
				"client_reference_id": "order_123",
			},
		},
		{
			name: "checkout session with all fields",
			id:   "cs_full",
			data: map[string]any{
				"mode":                      "subscription",
				"customer":                  "cus_456",
				"customer_email":            "customer@example.com",
				"client_reference_id":       "checkout_789",
				"success_url":               "https://example.com/success",
				"cancel_url":                "https://example.com/cancel",
				"currency":                  "usd",
				"locale":                    "en-US",
				"submit_type":               "pay",
				"billing_address_collection": "required",
				"allow_promotion_codes":     true,
				"metadata": map[string]any{
					"source":   "mobile",
					"campaign": "spring2024",
					"utm":      "google",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := buildCheckoutSession(tt.id, tt.data)

			assert.NotNil(t, session)
			assert.Equal(t, tt.id, session.Id)
			assert.Equal(t, "checkout.session", string(session.Object))
			assert.False(t, session.Livemode)
			assert.NotNil(t, session.Created)
			assert.NotNil(t, session.ExpiresAt)
			assert.NotNil(t, session.PaymentMethodCollection)
			assert.Equal(t, "open", string(*session.Status))
			assert.Equal(t, "unpaid", string(session.PaymentStatus))
			assert.False(t, session.AutomaticTax.Enabled)

			// Check mode
			if mode, ok := tt.data["mode"].(string); ok && mode != "" {
				assert.Equal(t, mode, string(session.Mode))
			}

			// Check customer (union type)
			if customer, ok := tt.data["customer"].(string); ok && customer != "" {
				require.NotNil(t, session.Customer)
				// Extract value from union
				customerID, err := session.Customer.AsCheckoutSessionCustomer0()
				assert.NoError(t, err)
				assert.Equal(t, customer, customerID)
			}

			// Check customer email
			if customerEmail, ok := tt.data["customer_email"].(string); ok && customerEmail != "" {
				require.NotNil(t, session.CustomerEmail)
				assert.Equal(t, customerEmail, *session.CustomerEmail)
			}

			// Check client reference id
			if clientRef, ok := tt.data["client_reference_id"].(string); ok && clientRef != "" {
				require.NotNil(t, session.ClientReferenceId)
				assert.Equal(t, clientRef, *session.ClientReferenceId)
			}

			// Check success URL
			if successURL, ok := tt.data["success_url"].(string); ok && successURL != "" {
				require.NotNil(t, session.SuccessUrl)
				assert.Equal(t, successURL, *session.SuccessUrl)
			}

			// Check cancel URL
			if cancelURL, ok := tt.data["cancel_url"].(string); ok && cancelURL != "" {
				require.NotNil(t, session.CancelUrl)
				assert.Equal(t, cancelURL, *session.CancelUrl)
			}

			// Check currency
			if currency, ok := tt.data["currency"].(string); ok && currency != "" {
				require.NotNil(t, session.Currency)
				assert.Equal(t, currency, *session.Currency)
			}

			// Check locale
			if locale, ok := tt.data["locale"].(string); ok && locale != "" {
				require.NotNil(t, session.Locale)
				assert.Equal(t, locale, string(*session.Locale))
			}

			// Check submit type
			if submitType, ok := tt.data["submit_type"].(string); ok && submitType != "" {
				require.NotNil(t, session.SubmitType)
				assert.Equal(t, submitType, string(*session.SubmitType))
			}

			// Check billing address collection
			if billingAddr, ok := tt.data["billing_address_collection"].(string); ok && billingAddr != "" {
				require.NotNil(t, session.BillingAddressCollection)
				assert.Equal(t, billingAddr, string(*session.BillingAddressCollection))
			}

			// Check allow promotion codes
			if allowPromo, ok := tt.data["allow_promotion_codes"].(bool); ok {
				require.NotNil(t, session.AllowPromotionCodes)
				assert.Equal(t, allowPromo, *session.AllowPromotionCodes)
			}

			// Check metadata
			if metadata, ok := tt.data["metadata"].(map[string]any); ok && len(metadata) > 0 {
				require.NotNil(t, session.Metadata)
				expectedMeta := mapAnyToString(metadata)
				assert.Equal(t, expectedMeta, *session.Metadata)
			}
		})
	}
}

// TestBuildAddress tests building an Address from input data
func TestBuildAddress(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]any
		expected map[string]any
	}{
		{
			name:     "empty address",
			data:     map[string]any{},
			expected: map[string]any{},
		},
		{
			name: "address with line1",
			data: map[string]any{
				"line1": "123 Main St",
			},
			expected: map[string]any{
				"line1": "123 Main St",
			},
		},
		{
			name: "address with line1 and line2",
			data: map[string]any{
				"line1": "123 Main St",
				"line2": "Apt 4B",
			},
			expected: map[string]any{
				"line1": "123 Main St",
				"line2": "Apt 4B",
			},
		},
		{
			name: "address with city",
			data: map[string]any{
				"city": "San Francisco",
			},
			expected: map[string]any{
				"city": "San Francisco",
			},
		},
		{
			name: "address with state",
			data: map[string]any{
				"state": "CA",
			},
			expected: map[string]any{
				"state": "CA",
			},
		},
		{
			name: "address with postal code",
			data: map[string]any{
				"postal_code": "94105",
			},
			expected: map[string]any{
				"postal_code": "94105",
			},
		},
		{
			name: "address with country",
			data: map[string]any{
				"country": "US",
			},
			expected: map[string]any{
				"country": "US",
			},
		},
		{
			name: "complete address",
			data: map[string]any{
				"line1":       "456 Oak Ave",
				"line2":       "Suite 100",
				"city":        "New York",
				"state":       "NY",
				"postal_code": "10001",
				"country":     "US",
			},
			expected: map[string]any{
				"line1":       "456 Oak Ave",
				"line2":       "Suite 100",
				"city":        "New York",
				"state":       "NY",
				"postal_code": "10001",
				"country":     "US",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address := buildAddress(tt.data)

			assert.NotNil(t, address)

			// Check each field
			if line1, ok := tt.expected["line1"].(string); ok && line1 != "" {
				require.NotNil(t, address.Line1)
				assert.Equal(t, line1, *address.Line1)
			} else {
				assert.Nil(t, address.Line1)
			}

			if line2, ok := tt.expected["line2"].(string); ok && line2 != "" {
				require.NotNil(t, address.Line2)
				assert.Equal(t, line2, *address.Line2)
			} else {
				assert.Nil(t, address.Line2)
			}

			if city, ok := tt.expected["city"].(string); ok && city != "" {
				require.NotNil(t, address.City)
				assert.Equal(t, city, *address.City)
			} else {
				assert.Nil(t, address.City)
			}

			if state, ok := tt.expected["state"].(string); ok && state != "" {
				require.NotNil(t, address.State)
				assert.Equal(t, state, *address.State)
			} else {
				assert.Nil(t, address.State)
			}

			if postalCode, ok := tt.expected["postal_code"].(string); ok && postalCode != "" {
				require.NotNil(t, address.PostalCode)
				assert.Equal(t, postalCode, *address.PostalCode)
			} else {
				assert.Nil(t, address.PostalCode)
			}

			if country, ok := tt.expected["country"].(string); ok && country != "" {
				require.NotNil(t, address.Country)
				assert.Equal(t, country, *address.Country)
			} else {
				assert.Nil(t, address.Country)
			}
		})
	}
}

// TestBuildCustomerAddress tests building a Customer_Address from input data
func TestBuildCustomerAddress(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "empty customer address",
			data: map[string]any{},
		},
		{
			name: "customer address with all fields",
			data: map[string]any{
				"line1":       "789 Pine St",
				"line2":       "Floor 5",
				"city":        "Chicago",
				"state":       "IL",
				"postal_code": "60601",
				"country":     "US",
			},
		},
		{
			name: "customer address with partial fields",
			data: map[string]any{
				"line1": "321 Elm St",
				"city":  "Austin",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address := buildCustomerAddress(tt.data)

			assert.NotNil(t, address)
			// Verify the union was set by checking we can read it back
			addr, err := address.AsAddress()
			if err == nil {
				// Verify fields were set correctly
				if line1, ok := tt.data["line1"].(string); ok && line1 != "" {
					require.NotNil(t, addr.Line1)
					assert.Equal(t, line1, *addr.Line1)
				}
			}
		})
	}
}

// TestBuildCustomerShipping tests building a Customer_Shipping from input data
func TestBuildCustomerShipping(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "empty shipping",
			data: map[string]any{},
		},
		{
			name: "shipping with all fields",
			data: map[string]any{
				"name":            "Jane Smith",
				"phone":           "+5555551234",
				"carrier":         "FedEx",
				"tracking_number": "123456789012345",
				"address": map[string]any{
					"line1":       "999 Park Ave",
					"city":        "Miami",
					"state":       "FL",
					"postal_code": "33101",
					"country":     "US",
				},
			},
		},
		{
			name: "shipping with minimal fields",
			data: map[string]any{
				"name": "Bob Johnson",
				"address": map[string]any{
					"line1": "555 Broadway",
					"city":  "Seattle",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shipping := buildCustomerShipping(tt.data)

			assert.NotNil(t, shipping)
			// Verify the union was set by checking we can read it back
			ship, err := shipping.AsShipping()
			if err == nil {
				// Verify name field
				if name, ok := tt.data["name"].(string); ok && name != "" {
					require.NotNil(t, ship.Name)
					assert.Equal(t, name, *ship.Name)
				}
			}
		})
	}
}

// TestBuildPortalBusinessProfile tests building a PortalBusinessProfile
func TestBuildPortalBusinessProfile(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "empty business profile",
			data: map[string]any{},
		},
		{
			name: "business profile with headline",
			data: map[string]any{
				"headline": "Acme Inc",
			},
		},
		{
			name: "business profile with all fields",
			data: map[string]any{
				"headline":           "Acme Inc",
				"privacy_policy_url":  "https://example.com/privacy",
				"terms_of_service_url": "https://example.com/terms",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := buildPortalBusinessProfile(tt.data)
			assert.NotNil(t, bp)

			if headline, ok := tt.data["headline"].(string); ok && headline != "" {
				require.NotNil(t, bp.Headline)
				assert.Equal(t, headline, *bp.Headline)
			}
			if privacyURL, ok := tt.data["privacy_policy_url"].(string); ok && privacyURL != "" {
				require.NotNil(t, bp.PrivacyPolicyUrl)
				assert.Equal(t, privacyURL, *bp.PrivacyPolicyUrl)
			}
			if termsURL, ok := tt.data["terms_of_service_url"].(string); ok && termsURL != "" {
				require.NotNil(t, bp.TermsOfServiceUrl)
				assert.Equal(t, termsURL, *bp.TermsOfServiceUrl)
			}
		})
	}
}

// TestBuildPortalCustomerUpdate tests building a PortalCustomerUpdate
func TestBuildPortalCustomerUpdate(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "disabled customer update",
			data: map[string]any{
				"enabled": false,
			},
		},
		{
			name: "enabled customer update with allowed updates",
			data: map[string]any{
				"enabled":         true,
				"allowed_updates": []any{"address", "email", "name", "phone"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cu := buildPortalCustomerUpdate(tt.data)
			assert.NotNil(t, cu)

			if enabled, ok := tt.data["enabled"].(bool); ok {
				assert.Equal(t, enabled, cu.Enabled)
			}
			if allowedUpdates, ok := tt.data["allowed_updates"].([]any); ok {
				assert.Equal(t, len(allowedUpdates), len(cu.AllowedUpdates))
			}
		})
	}
}

// TestBuildPortalInvoiceList tests building a PortalInvoiceList
func TestBuildPortalInvoiceList(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "disabled invoice list",
			data: map[string]any{
				"enabled": false,
			},
		},
		{
			name: "enabled invoice list with default limit",
			data: map[string]any{
				"enabled":       true,
				"default_limit": 50,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			il := buildPortalInvoiceList(tt.data)
			assert.NotNil(t, il)

			if enabled, ok := tt.data["enabled"].(bool); ok {
				assert.Equal(t, enabled, il.Enabled)
			}
		})
	}
}

// TestBuildPortalPaymentMethodUpdate tests building a PortalPaymentMethodUpdate
func TestBuildPortalPaymentMethodUpdate(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "disabled payment method update",
			data: map[string]any{
				"enabled": false,
			},
		},
		{
			name: "enabled payment method update",
			data: map[string]any{
				"enabled": true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pmu := buildPortalPaymentMethodUpdate(tt.data)
			assert.NotNil(t, pmu)

			if enabled, ok := tt.data["enabled"].(bool); ok {
				assert.Equal(t, enabled, pmu.Enabled)
			}
		})
	}
}

// TestBuildPortalSubscriptionCancellationReason tests building a PortalSubscriptionCancellationReason
func TestBuildPortalSubscriptionCancellationReason(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "disabled cancellation reason",
			data: map[string]any{
				"enabled": false,
			},
		},
		{
			name: "enabled cancellation reason with options",
			data: map[string]any{
				"enabled": true,
				"options": []any{
					"too_expensive",
					"missing_features",
					"switched_service",
					"too_complex",
					"low_quality",
					"other",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := buildPortalSubscriptionCancellationReason(tt.data)
			assert.NotNil(t, cr)

			if enabled, ok := tt.data["enabled"].(bool); ok {
				assert.Equal(t, enabled, cr.Enabled)
			}

			if options, ok := tt.data["options"].([]any); ok {
				assert.Equal(t, len(options), len(cr.Options))
			}
		})
	}
}

// TestBuildPortalSubscriptionCancel tests building a PortalSubscriptionCancel
func TestBuildPortalSubscriptionCancel(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "disabled subscription cancel",
			data: map[string]any{
				"enabled": false,
			},
		},
		{
			name: "enabled subscription cancel with mode",
			data: map[string]any{
				"enabled": true,
				"mode":    "at_once",
			},
		},
		{
			name: "subscription cancel with proration and reason",
			data: map[string]any{
				"enabled":            true,
				"mode":               "at_period_end",
				"proration_behavior": "create_prorations",
				"cancellation_reason": map[string]any{
					"enabled": true,
					"options": []any{"too_expensive"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := buildPortalSubscriptionCancel(tt.data)
			assert.NotNil(t, sc)

			if enabled, ok := tt.data["enabled"].(bool); ok {
				assert.Equal(t, enabled, sc.Enabled)
			}

			if mode, ok := tt.data["mode"].(string); ok && mode != "" {
				assert.Equal(t, mode, string(sc.Mode))
			}

			if proration, ok := tt.data["proration_behavior"].(string); ok && proration != "" {
				assert.Equal(t, proration, string(sc.ProrationBehavior))
			}
		})
	}
}

// TestBuildPortalResourceScheduleUpdateAtPeriodEnd tests building a PortalResourceScheduleUpdateAtPeriodEnd
func TestBuildPortalResourceScheduleUpdateAtPeriodEnd(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "empty schedule update",
			data: map[string]any{},
		},
		{
			name: "schedule update with conditions",
			data: map[string]any{
				"conditions": []any{
					map[string]any{"type": "past_due"},
					map[string]any{"type": "subscription_active"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := buildPortalResourceScheduleUpdateAtPeriodEnd(tt.data)
			assert.NotNil(t, rs)

			if conditions, ok := tt.data["conditions"].([]any); ok {
				assert.Equal(t, len(conditions), len(rs.Conditions))
			}
		})
	}
}

// TestBuildPortalSubscriptionUpdate tests building a PortalSubscriptionUpdate
func TestBuildPortalSubscriptionUpdate(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "disabled subscription update",
			data: map[string]any{
				"enabled": false,
			},
		},
		{
			name: "enabled subscription update with default allowed updates",
			data: map[string]any{
				"enabled":                true,
				"default_allowed_updates": []any{"price", "quantity"},
			},
		},
		{
			name: "subscription update with all fields",
			data: map[string]any{
				"enabled":                true,
				"default_allowed_updates": []any{"price", "promotion_code", "quantity"},
				"proration_behavior":      "create_prorations",
				"products": []any{
					map[string]any{"product": "prod_123", "prices": []any{"price_123"}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			su := buildPortalSubscriptionUpdate(tt.data)
			assert.NotNil(t, su)

			if enabled, ok := tt.data["enabled"].(bool); ok {
				assert.Equal(t, enabled, su.Enabled)
			}

			if defaultUpdates, ok := tt.data["default_allowed_updates"].([]any); ok {
				assert.Equal(t, len(defaultUpdates), len(su.DefaultAllowedUpdates))
			}

			if proration, ok := tt.data["proration_behavior"].(string); ok && proration != "" {
				assert.Equal(t, proration, string(su.ProrationBehavior))
			}

			if products, ok := tt.data["products"].([]any); ok && su.Products != nil {
				assert.Equal(t, len(products), len(*su.Products))
			}
		})
	}
}

// TestBuildPortalSubscriptionUpdateProduct tests building a PortalSubscriptionUpdateProduct
func TestBuildPortalSubscriptionUpdateProduct(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "product update with product id",
			data: map[string]any{
				"product": "prod_123",
			},
		},
		{
			name: "product update with product and prices",
			data: map[string]any{
				"product": "prod_456",
				"prices":  []any{"price_123", "price_456"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pup := buildPortalSubscriptionUpdateProduct(tt.data)
			assert.NotNil(t, pup)

			if product, ok := tt.data["product"].(string); ok {
				assert.Equal(t, product, pup.Product)
			}

			if prices, ok := tt.data["prices"].([]any); ok {
				assert.Equal(t, len(prices), len(pup.Prices))
			}
		})
	}
}

// TestBuildPortalFeatures tests building complete PortalFeatures
func TestBuildPortalFeatures(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
	}{
		{
			name: "minimal features",
			data: map[string]any{
				"business_profile": map[string]any{
					"name": "Test Inc",
				},
			},
		},
		{
			name: "complete features",
			data: map[string]any{
				"business_profile": map[string]any{
					"headline":            "Acme Inc",
					"privacy_policy_url":  "https://example.com/privacy",
					"terms_of_service_url": "https://example.com/terms",
				},
				"customer_update": map[string]any{
					"enabled":         true,
					"allowed_updates": []any{"address", "email", "name", "phone"},
				},
				"invoice_history": map[string]any{
					"enabled":       true,
					"default_limit": 50,
				},
				"payment_method_update": map[string]any{
					"enabled": true,
				},
				"subscription_cancel": map[string]any{
					"enabled":            true,
					"mode":               "at_period_end",
					"proration_behavior": "create_prorations",
					"cancellation_reason": map[string]any{
						"enabled": true,
						"options": []any{"too_expensive", "missing_features"},
					},
				},
				"subscription_update": map[string]any{
					"enabled":                true,
					"default_allowed_updates": []any{"price", "quantity"},
					"proration_behavior":      "create_prorations",
					"products": []any{
						map[string]any{
							"product": "prod_123",
							"prices":  []any{"price_123"},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features := buildPortalFeatures(tt.data)
			assert.NotNil(t, features)

			// Verify customer update if present
			if _, ok := tt.data["customer_update"]; ok {
				assert.True(t, features.CustomerUpdate.Enabled || len(features.CustomerUpdate.AllowedUpdates) > 0)
			}

			// Verify invoice history if present
			if ihData, ok := tt.data["invoice_history"].(map[string]any); ok {
				if enabled, ok := ihData["enabled"].(bool); ok {
					assert.Equal(t, enabled, features.InvoiceHistory.Enabled)
				}
			}

			// Verify payment method update if present
			if _, ok := tt.data["payment_method_update"]; ok {
				assert.True(t, features.PaymentMethodUpdate.Enabled)
			}

			// Verify subscription cancel if present
			if _, ok := tt.data["subscription_cancel"]; ok {
				assert.True(t, features.SubscriptionCancel.Enabled)
			}

			// Verify subscription update if present
			if _, ok := tt.data["subscription_update"]; ok {
				assert.True(t, features.SubscriptionUpdate.Enabled)
			}
		})
	}
}
