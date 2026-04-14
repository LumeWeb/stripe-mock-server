package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInMemoryRepository_Create tests creating resources
func TestInMemoryRepository_Create(t *testing.T) {
	repo := NewInMemoryRepository[TestResource]()

	resource := TestResource{Name: "test", Value: 42}
	id, err := repo.Create(resource)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	retrieved, err := repo.Get(id)
	require.NoError(t, err)
	assert.Equal(t, "test", retrieved.Name)
	assert.Equal(t, 42, retrieved.Value)
}

// TestInMemoryRepository_Get tests retrieving resources
func TestInMemoryRepository_Get(t *testing.T) {
	repo := NewInMemoryRepository[TestResource]()

	t.Run("Get existing resource", func(t *testing.T) {
		resource := TestResource{Name: "existing", Value: 100}
		id, err := repo.Create(resource)
		require.NoError(t, err)

		retrieved, err := repo.Get(id)
		require.NoError(t, err)
		assert.Equal(t, "existing", retrieved.Name)
	})

	t.Run("Get non-existent resource", func(t *testing.T) {
		_, err := repo.Get("nonexistent")
		assert.Error(t, err)
		assert.Equal(t, ErrNotFound, err)
	})
}

// TestInMemoryRepository_Update tests updating resources
func TestInMemoryRepository_Update(t *testing.T) {
	repo := NewInMemoryRepository[TestResource]()

	resource := TestResource{Name: "original", Value: 1}
	id, err := repo.Create(resource)
	require.NoError(t, err)

	updated := TestResource{Name: "updated", Value: 2}
	err = repo.Update(id, updated)
	require.NoError(t, err)

	retrieved, err := repo.Get(id)
	require.NoError(t, err)
	assert.Equal(t, "updated", retrieved.Name)
	assert.Equal(t, 2, retrieved.Value)
}

// TestInMemoryRepository_Delete tests deleting resources
func TestInMemoryRepository_Delete(t *testing.T) {
	repo := NewInMemoryRepository[TestResource]()

	resource := TestResource{Name: "to-delete", Value: 999}
	id, err := repo.Create(resource)
	require.NoError(t, err)

	err = repo.Delete(id)
	require.NoError(t, err)

	_, err = repo.Get(id)
	assert.Equal(t, ErrNotFound, err)
}

// TestInMemoryRepository_List tests listing all resources
func TestInMemoryRepository_List(t *testing.T) {
	repo := NewInMemoryRepository[TestResource]()

	// Create multiple resources
	for i := 0; i < 5; i++ {
		_, err := repo.Create(TestResource{Name: "resource", Value: i})
		require.NoError(t, err)
	}

	list, err := repo.List()
	require.NoError(t, err)
	assert.Len(t, list, 5)
}

// TestInMemoryRepository_Exists tests checking resource existence
func TestInMemoryRepository_Exists(t *testing.T) {
	repo := NewInMemoryRepository[TestResource]()

	resource := TestResource{Name: "exists-test", Value: 1}
	id, err := repo.Create(resource)
	require.NoError(t, err)

	assert.True(t, repo.Exists(id))
	assert.False(t, repo.Exists("nonexistent"))
}

// TestInMemoryRepository_Clear tests clearing all resources
func TestInMemoryRepository_Clear(t *testing.T) {
	repo := NewInMemoryRepository[TestResource]()

	// Create multiple resources
	for i := 0; i < 5; i++ {
		_, err := repo.Create(TestResource{Name: "clear-test", Value: i})
		require.NoError(t, err)
	}

	// Verify resources exist
	list, err := repo.List()
	require.NoError(t, err)
	assert.Len(t, list, 5)

	// Clear the repository
	err = repo.Clear()
	require.NoError(t, err)

	// Verify all resources are gone
	list, err = repo.List()
	require.NoError(t, err)
	assert.Len(t, list, 0)

	// Verify Exists returns false for cleared resources
	assert.False(t, repo.Exists("any-id"))
}

// TestInMemoryRepository_ClearEmptyRepo tests clearing an empty repository
func TestInMemoryRepository_ClearEmptyRepo(t *testing.T) {
	repo := NewInMemoryRepository[TestResource]()

	// Clear empty repository should not error
	err := repo.Clear()
	require.NoError(t, err)

	// Verify still empty
	list, err := repo.List()
	require.NoError(t, err)
	assert.Len(t, list, 0)
}

// TestInMemoryRepository_ClearTwice tests clearing repository twice
func TestInMemoryRepository_ClearTwice(t *testing.T) {
	repo := NewInMemoryRepository[TestResource]()

	// Create resources
	id, err := repo.Create(TestResource{Name: "double-clear", Value: 1})
	require.NoError(t, err)

	// First clear
	err = repo.Clear()
	require.NoError(t, err)

	// Second clear should not error
	err = repo.Clear()
	require.NoError(t, err)

	// Verify still empty
	assert.False(t, repo.Exists(id))
}

// TestInMemoryRepository_CreateWithID tests creating resources with specific IDs
func TestInMemoryRepository_CreateWithID(t *testing.T) {
	repo := NewInMemoryRepository[TestResource]()

	resource := TestResource{Name: "with-id", Value: 42}
	err := repo.CreateWithID("custom_id_123", resource)
	require.NoError(t, err)

	retrieved, err := repo.Get("custom_id_123")
	require.NoError(t, err)
	assert.Equal(t, "with-id", retrieved.Name)
	assert.Equal(t, 42, retrieved.Value)
}

// TestInMemoryRepository_CreateWithIDAlreadyExists tests creating with duplicate ID
func TestInMemoryRepository_CreateWithIDAlreadyExists(t *testing.T) {
	repo := NewInMemoryRepository[TestResource]()

	resource := TestResource{Name: "duplicate", Value: 1}
	err := repo.CreateWithID("dupe_id", resource)
	require.NoError(t, err)

	// Try to create with same ID
	resource2 := TestResource{Name: "duplicate2", Value: 2}
	err = repo.CreateWithID("dupe_id", resource2)
	assert.Equal(t, ErrAlreadyExists, err)
}

// TestResource is a simple test struct
type TestResource struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}
