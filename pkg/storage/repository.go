package storage

import (
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"time"

	"github.com/stripe-mock-server/pkg/generator")

// ErrNotFound is returned when a resource is not found
var ErrNotFound = errors.New("not found")

// ErrAlreadyExists is returned when trying to create a resource that already exists
var ErrAlreadyExists = errors.New("already exists")

// Repository defines the interface for stateful storage with type safety
type Repository[T any] interface {
	Create(data T) (string, error)
	Get(id string) (*T, error)
	Update(id string, data T) error
	Delete(id string) error
	List() ([]T, error)
	Exists(id string) bool
	Clear() error
}

// InMemoryRepository implements Repository[T] using in-memory storage
type InMemoryRepository[T any] struct {
	resources   map[string]map[string]json.RawMessage // resourceType → id → raw JSON data
	locks       map[string]*sync.RWMutex
	locksMutex  sync.RWMutex // protects locks map access
}

// NewInMemoryRepository creates a new in-memory repository
func NewInMemoryRepository[T any]() *InMemoryRepository[T] {
	return &InMemoryRepository[T]{
		resources: make(map[string]map[string]json.RawMessage),
		locks:     make(map[string]*sync.RWMutex),
	}
}

// Create stores a new resource and returns its ID
func (r *InMemoryRepository[T]) Create(data T) (string, error) {
	typename := typeToString[T]()
	resourceType := r.extractPrefixFromType()
	lock := r.getLock(resourceType)
	lock.Lock()
	defer lock.Unlock()

	if r.resources[resourceType] == nil {
		r.resources[resourceType] = make(map[string]json.RawMessage)
	}

	// Generate ID
	id := generator.GenerateID(resourceType)

	// Set metadata
	dataMap, err := toMap(data)
	if err != nil {
		return "", err
	}
	dataMap["id"] = id
	dataMap["created"] = time.Now().Unix()
	if _, hasObject := dataMap["object"]; !hasObject {
		dataMap["object"] = typename
	}

	// Serialize to JSON and store as raw message
	jsonBytes, err := json.Marshal(dataMap)
	if err != nil {
		return "", err
	}

	r.resources[resourceType][id] = jsonBytes
	return id, nil
}

// CreateWithID creates a new resource with a specific ID
func (r *InMemoryRepository[T]) CreateWithID(id string, data T) error {
	typename := typeToString[T]()
	resourceType := r.extractPrefixFromType()
	lock := r.getLock(resourceType)
	lock.Lock()
	defer lock.Unlock()

	if r.resources[resourceType] == nil {
		r.resources[resourceType] = make(map[string]json.RawMessage)
	}

	// Check if already exists
	if _, exists := r.resources[resourceType][id]; exists {
		return ErrAlreadyExists
	}

	// Set metadata if not already set
	dataMap, err := toMap(data)
	if err != nil {
		return err
	}
	if _, hasID := dataMap["id"]; !hasID {
		dataMap["id"] = id
	}
	if _, hasCreated := dataMap["created"]; !hasCreated {
		dataMap["created"] = time.Now().Unix()
	}
	if _, hasObject := dataMap["object"]; !hasObject {
		dataMap["object"] = typename
	}

	// Serialize to JSON and store as raw message
	jsonBytes, err := json.Marshal(dataMap)
	if err != nil {
		return err
	}

	r.resources[resourceType][id] = jsonBytes
	return nil
}

// Get retrieves a resource by ID
func (r *InMemoryRepository[T]) Get(id string) (*T, error) {
	resourceType := r.extractPrefixFromType()
	lock := r.getLock(resourceType)
	lock.RLock()
	defer lock.RUnlock()

	if r.resources[resourceType] == nil {
		return nil, ErrNotFound
	}

	jsonData, ok := r.resources[resourceType][id]
	if !ok {
		return nil, ErrNotFound
	}

	// Unmarshal JSON to T
	var result T
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Update updates an existing resource
func (r *InMemoryRepository[T]) Update(id string, data T) error {
	resourceType := r.extractPrefixFromType()
	lock := r.getLock(resourceType)
	lock.Lock()
	defer lock.Unlock()

	if r.resources[resourceType] == nil {
		return ErrNotFound
	}

	_, ok := r.resources[resourceType][id]
	if !ok {
		return ErrNotFound
	}

	// Marshal new data directly and store
	newJson, err := json.Marshal(data)
	if err != nil {
		return err
	}

	r.resources[resourceType][id] = newJson
	return nil
}

// Delete removes a resource
func (r *InMemoryRepository[T]) Delete(id string) error {
	resourceType := r.extractPrefixFromType()
	lock := r.getLock(resourceType)
	lock.Lock()
	defer lock.Unlock()

	if r.resources[resourceType] == nil {
		return ErrNotFound
	}

	if _, ok := r.resources[resourceType][id]; !ok {
		return ErrNotFound
	}

	delete(r.resources[resourceType], id)
	return nil
}

// List returns all resources
func (r *InMemoryRepository[T]) List() ([]T, error) {
	resourceType := r.extractPrefixFromType()
	lock := r.getLock(resourceType)
	lock.RLock()
	defer lock.RUnlock()

	if r.resources[resourceType] == nil {
		return []T{}, nil
	}

	result := make([]T, 0, len(r.resources[resourceType]))
	for _, jsonData := range r.resources[resourceType] {
		var data T
		if err := json.Unmarshal(jsonData, &data); err != nil {
			continue
		}
		result = append(result, data)
	}

	return result, nil
}

// Exists checks if a resource exists
func (r *InMemoryRepository[T]) Exists(id string) bool {
	resourceType := r.extractPrefixFromType()
	lock := r.getLock(resourceType)
	lock.RLock()
	defer lock.RUnlock()

	if r.resources[resourceType] == nil {
		return false
	}

	_, ok := r.resources[resourceType][id]
	return ok
}

// Clear removes all resources of this type
func (r *InMemoryRepository[T]) Clear() error {
	resourceType := r.extractPrefixFromType()
	lock := r.getLock(resourceType)
	lock.Lock()
	defer lock.Unlock()

	r.resources[resourceType] = make(map[string]json.RawMessage)
	return nil
}

// getLock returns or creates a lock for the given resource type
func (r *InMemoryRepository[T]) getLock(resourceType string) *sync.RWMutex {
	// First check with read lock (fast path)
	r.locksMutex.RLock()
	lock, ok := r.locks[resourceType]
	r.locksMutex.RUnlock()

	if ok {
		return lock
	}

	// Need to create lock - use write lock
	r.locksMutex.Lock()
	defer r.locksMutex.Unlock()

	// Double-check after acquiring write lock
	if lock, ok := r.locks[resourceType]; ok {
		return lock
	}

	lock = &sync.RWMutex{}
	r.locks[resourceType] = lock
	return lock
}

// extractPrefixFromType maps type to ID prefix
// We use type name through reflection
func (r *InMemoryRepository[T]) extractPrefixFromType() string {
	// Using string representation of type name for prefix
	typename := typeToString[T]()
	if typename == "" {
		return "res"
	}

	prefixMap := map[string]string{
		"Charge":           "ch",
		"Customer":         "cus",
		"Invoice":          "in",
		"Payment":          "pay",
		"Refund":           "re",
		"Card":             "card",
		"Account":          "acct",
		"Balance":          "bal",
		"Product":          "prod",
		"Price":            "prc",
		"CheckoutSession":  "cs",
		"Subscription":     "sub",
		"WebhookEndpoint":  "we",
		"Event":            "evt",
	}

	if prefix, ok := prefixMap[typename]; ok {
		return prefix
	}

	// Generate from first 3 chars of type name
	if len(typename) >= 3 {
		return typename[:3]
	}
	return "res"
}

// toMap converts any type to map[string]any
func toMap[T any](data T) (map[string]any, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal(bytes, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// typeToString uses reflection to get the type name
func typeToString[T any]() string {
	typ := reflect.TypeOf(*new(T))
	if typ == nil {
		return ""
	}
	return typ.String()
}