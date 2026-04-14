package spec

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed openapi.json
var embeddedSpec []byte

// GetSpec returns the embedded OpenAPI spec as a Spec object
func GetSpec() (*Spec, error) {
	var apiSpec Spec
	if err := json.Unmarshal(embeddedSpec, &apiSpec); err != nil {
		return nil, fmt.Errorf("failed to parse embedded spec: %w", err)
	}
	return &apiSpec, nil
}

// GetSpecBytes returns the raw embedded spec bytes
func GetSpecBytes() []byte {
	return embeddedSpec
}