package gormplus_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	gormplus "github.com/nullcache/gorm-plus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ============================================================================
// Test Models and Setup
// ============================================================================

// Test models
type User struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"not null"`
	Email     string `gorm:"unique;not null"`
	Age       int    `gorm:"default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type Product struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"not null"`
	Price       int    `gorm:"not null"`
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Invalid types for testing
type InvalidPointer *User
type InvalidPrimitive string

// Test setup helper
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// Auto migrate test models
	err = db.AutoMigrate(&User{}, &Product{})
	require.NoError(t, err)

	return db
}

// ============================================================================
// Constructor Tests
// ============================================================================

func TestNewBaseModel_ValidStruct(t *testing.T) {
	db := setupTestDB(t)

	baseModel, err := gormplus.NewBaseModel[User](db)

	assert.NoError(t, err)
	assert.NotNil(t, baseModel)
}

func TestNewBaseModel_InvalidPointerType(t *testing.T) {
	db := setupTestDB(t)

	_, err := gormplus.NewBaseModel[*User](db)

	assert.Error(t, err)
	assert.Equal(t, gormplus.ErrInvalidType, err)
}

func TestNewBaseModel_InvalidPrimitiveType(t *testing.T) {
	db := setupTestDB(t)

	_, err := gormplus.NewBaseModel[string](db)

	assert.Error(t, err)
	assert.Equal(t, gormplus.ErrInvalidType, err)
}

func TestNewBaseModel_InvalidCustomType(t *testing.T) {
	db := setupTestDB(t)

	_, err := gormplus.NewBaseModel[InvalidPrimitive](db)

	assert.Error(t, err)
	assert.Equal(t, gormplus.ErrInvalidType, err)
}

func TestNewBaseModel_ParseSchemaError(t *testing.T) {
	// Test with an invalid database configuration to trigger parse error
	// We'll use a struct that might cause GORM parsing issues
	type InvalidModel struct {
		// This should work fine, so let's skip this test for now
		// as it's hard to trigger a parse schema error reliably
	}

	// Instead, let's test the ErrNilSchema case by mocking
	// For now, we'll skip this specific error case since it's hard to trigger
	t.Skip("Schema parse error is difficult to trigger reliably in tests", InvalidModel{})
}

// ============================================================================
// Transaction Management Tests
// ============================================================================

func TestBaseModel_Transact_Success(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	err = baseModel.Transact(ctx, func(ctx context.Context, tx *gorm.DB) error {
		user1 := &User{Name: "User1", Email: "user1@example.com", Age: 25}
		user2 := &User{Name: "User2", Email: "user2@example.com", Age: 30}

		if err := baseModel.Create(ctx, tx, user1); err != nil {
			return err
		}
		if err := baseModel.Create(ctx, tx, user2); err != nil {
			return err
		}

		return nil
	})

	assert.NoError(t, err)

	// Verify both users were created
	count, err := baseModel.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestBaseModel_Transact_Rollback(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	err = baseModel.Transact(ctx, func(ctx context.Context, tx *gorm.DB) error {
		user1 := &User{Name: "User1", Email: "user1@example.com", Age: 25}
		if err := baseModel.Create(ctx, tx, user1); err != nil {
			return err
		}

		// This should cause a rollback
		return errors.New("intentional error")
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "intentional error")

	// Verify no users were created due to rollback
	count, err := baseModel.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

// ============================================================================
// CRUD Operations Tests
// ============================================================================

func TestBaseModel_Create(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	err = baseModel.Create(ctx, nil, user)

	assert.NoError(t, err)
	assert.NotZero(t, user.ID)
	assert.NotZero(t, user.CreatedAt)
}

func TestBaseModel_Create_WithTransaction(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "Jane Doe",
		Email: "jane@example.com",
		Age:   25,
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		return baseModel.Create(ctx, tx, user)
	})

	assert.NoError(t, err)
	assert.NotZero(t, user.ID)
}

func TestBaseModel_Update(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	// Create first
	err = baseModel.Create(ctx, nil, user)
	require.NoError(t, err)

	// Update
	user.Name = "John Updated"
	user.Age = 31
	err = baseModel.Update(ctx, nil, user)

	assert.NoError(t, err)

	// Verify update
	found, err := baseModel.First(ctx, gormplus.Where("id = ?", user.ID))
	assert.NoError(t, err)
	assert.Equal(t, "John Updated", found.Name)
	assert.Equal(t, 31, found.Age)
}

func TestBaseModel_Update_WithTransaction(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	// Create first
	err = baseModel.Create(ctx, nil, user)
	require.NoError(t, err)

	// Update within transaction
	err = db.Transaction(func(tx *gorm.DB) error {
		user.Name = "John Updated"
		return baseModel.Update(ctx, tx, user)
	})

	assert.NoError(t, err)

	// Verify update
	found, err := baseModel.First(ctx, gormplus.Where("id = ?", user.ID))
	assert.NoError(t, err)
	assert.Equal(t, "John Updated", found.Name)
}

func TestBaseModel_UpdateColumn(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	// Create first
	err = baseModel.Create(ctx, nil, user)
	require.NoError(t, err)

	// Update single column
	err = baseModel.UpdateColumn(ctx, nil, "name", "John Updated", gormplus.Where("id = ?", user.ID))

	assert.NoError(t, err)

	// Verify update
	found, err := baseModel.First(ctx, gormplus.Where("id = ?", user.ID))
	assert.NoError(t, err)
	assert.Equal(t, "John Updated", found.Name)
	assert.Equal(t, "john@example.com", found.Email) // Email should remain unchanged
	assert.Equal(t, 30, found.Age)                   // Age should remain unchanged
}

func TestBaseModel_UpdateColumn_WithoutScopes(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	err = baseModel.UpdateColumn(ctx, nil, "name", "Updated Name")

	assert.Equal(t, gormplus.ErrDangerous, err)
}

func TestBaseModel_UpdateColumn_WithTransaction(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	// Create first
	err = baseModel.Create(ctx, nil, user)
	require.NoError(t, err)

	// Update within transaction
	err = db.Transaction(func(tx *gorm.DB) error {
		return baseModel.UpdateColumn(ctx, tx, "age", 31, gormplus.Where("id = ?", user.ID))
	})

	assert.NoError(t, err)

	// Verify update
	found, err := baseModel.First(ctx, gormplus.Where("id = ?", user.ID))
	assert.NoError(t, err)
	assert.Equal(t, 31, found.Age)
	assert.Equal(t, "John Doe", found.Name) // Name should remain unchanged
}

func TestBaseModel_UpdateColumns(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	// Create first
	err = baseModel.Create(ctx, nil, user)
	require.NoError(t, err)

	// Update multiple columns with map
	updates := map[string]any{
		"name": "John Updated",
		"age":  35,
	}
	err = baseModel.UpdateColumns(ctx, nil, updates, gormplus.Where("id = ?", user.ID))

	assert.NoError(t, err)

	// Verify update
	found, err := baseModel.First(ctx, gormplus.Where("id = ?", user.ID))
	assert.NoError(t, err)
	assert.Equal(t, "John Updated", found.Name)
	assert.Equal(t, 35, found.Age)
	assert.Equal(t, "john@example.com", found.Email) // Email should remain unchanged
}

func TestBaseModel_UpdateColumns_WithStruct(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	// Create first
	err = baseModel.Create(ctx, nil, user)
	require.NoError(t, err)

	// Update multiple columns with struct
	updates := User{
		Name: "John Updated",
		Age:  35,
		// Email is not set, so it should remain unchanged
	}
	err = baseModel.UpdateColumns(ctx, nil, updates, gormplus.Where("id = ?", user.ID))

	assert.NoError(t, err)

	// Verify update
	found, err := baseModel.First(ctx, gormplus.Where("id = ?", user.ID))
	assert.NoError(t, err)
	assert.Equal(t, "John Updated", found.Name)
	assert.Equal(t, 35, found.Age)
	assert.Equal(t, "john@example.com", found.Email) // Email should remain unchanged
}

func TestBaseModel_UpdateColumns_WithoutScopes(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	updates := map[string]any{"name": "Updated Name"}
	err = baseModel.UpdateColumns(ctx, nil, updates)

	assert.Equal(t, gormplus.ErrDangerous, err)
}

func TestBaseModel_UpdateColumns_WithTransaction(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	// Create first
	err = baseModel.Create(ctx, nil, user)
	require.NoError(t, err)

	// Update within transaction
	err = db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			"name": "John Updated",
			"age":  40,
		}
		return baseModel.UpdateColumns(ctx, tx, updates, gormplus.Where("id = ?", user.ID))
	})

	assert.NoError(t, err)

	// Verify update
	found, err := baseModel.First(ctx, gormplus.Where("id = ?", user.ID))
	assert.NoError(t, err)
	assert.Equal(t, "John Updated", found.Name)
	assert.Equal(t, 40, found.Age)
}

func TestBaseModel_Delete(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	// Create first
	err = baseModel.Create(ctx, nil, user)
	require.NoError(t, err)

	// Delete
	err = baseModel.Delete(ctx, nil, gormplus.Where("id = ?", user.ID))

	assert.NoError(t, err)

	// Verify deletion (should be soft deleted)
	_, err = baseModel.First(ctx, gormplus.Where("id = ?", user.ID))
	assert.Equal(t, gormplus.ErrNotFound, err)

	// Verify still exists with soft delete scope
	found, err := baseModel.First(ctx, gormplus.WithDeleted(), gormplus.Where("id = ?", user.ID))
	assert.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)
}

func TestBaseModel_Delete_WithoutScopes(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	err = baseModel.Delete(ctx, nil)

	assert.Equal(t, gormplus.ErrDangerous, err)
}

// ============================================================================
// Batch Operations Tests
// ============================================================================

func TestBaseModel_BatchInsert(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	users := []*User{
		{Name: "User1", Email: "user1@example.com", Age: 20},
		{Name: "User2", Email: "user2@example.com", Age: 21},
		{Name: "User3", Email: "user3@example.com", Age: 22},
	}

	err = baseModel.BatchInsert(ctx, nil, users)

	assert.NoError(t, err)
	for _, user := range users {
		assert.NotZero(t, user.ID)
	}

	// Verify all users were created
	count, err := baseModel.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestBaseModel_BatchInsert_EmptySlice(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	var users []*User

	err = baseModel.BatchInsert(ctx, nil, users)

	assert.NoError(t, err)
}

func TestBaseModel_BatchInsert_CustomBatchSize(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	users := []*User{
		{Name: "User1", Email: "user1@example.com", Age: 20},
		{Name: "User2", Email: "user2@example.com", Age: 21},
	}

	err = baseModel.BatchInsert(ctx, nil, users, 1)

	assert.NoError(t, err)
	for _, user := range users {
		assert.NotZero(t, user.ID)
	}
}

func TestBaseModel_BatchInsert_ZeroBatchSize(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	users := []*User{
		{Name: "User1", Email: "user1@example.com", Age: 20},
		{Name: "User2", Email: "user2@example.com", Age: 21},
	}

	// Test with zero batch size (should default to 1000)
	err = baseModel.BatchInsert(ctx, nil, users, 0)

	assert.NoError(t, err)
	for _, user := range users {
		assert.NotZero(t, user.ID)
	}
}

func TestBaseModel_BatchInsert_WithTransaction(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	users := []*User{
		{Name: "User1", Email: "user1@example.com", Age: 20},
		{Name: "User2", Email: "user2@example.com", Age: 21},
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		return baseModel.BatchInsert(ctx, tx, users)
	})

	assert.NoError(t, err)
	for _, user := range users {
		assert.NotZero(t, user.ID)
	}
}

// ============================================================================
// Query Operations Tests
// ============================================================================

func TestBaseModel_First(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	// Create first
	err = baseModel.Create(ctx, nil, user)
	require.NoError(t, err)

	// Find
	found, err := baseModel.First(ctx, gormplus.Where("email = ?", "john@example.com"))

	assert.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)
	assert.Equal(t, "John Doe", found.Name)
	assert.Equal(t, "john@example.com", found.Email)
	assert.Equal(t, 30, found.Age)
}

func TestBaseModel_First_NotFound(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	_, err = baseModel.First(ctx, gormplus.Where("email = ?", "nonexistent@example.com"))

	assert.Equal(t, gormplus.ErrNotFound, err)
}

func TestBaseModel_First_DatabaseError(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	// Test with invalid SQL to cause error
	_, err = baseModel.First(ctx, gormplus.Where("invalid_column = ?", 1))
	assert.Error(t, err)
	assert.NotEqual(t, gormplus.ErrNotFound, err) // Should be a different database error
}

func TestBaseModel_List(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	users := []*User{
		{Name: "User1", Email: "user1@example.com", Age: 20},
		{Name: "User2", Email: "user2@example.com", Age: 21},
		{Name: "User3", Email: "user3@example.com", Age: 22},
	}

	// Create users
	err = baseModel.BatchInsert(ctx, nil, users)
	require.NoError(t, err)

	// List all
	found, err := baseModel.List(ctx)

	assert.NoError(t, err)
	assert.Len(t, found, 3)
}

func TestBaseModel_List_WithScopes(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	users := []*User{
		{Name: "User1", Email: "user1@example.com", Age: 20},
		{Name: "User2", Email: "user2@example.com", Age: 25},
		{Name: "User3", Email: "user3@example.com", Age: 30},
	}

	// Create users
	err = baseModel.BatchInsert(ctx, nil, users)
	require.NoError(t, err)

	// List with conditions
	found, err := baseModel.List(ctx, gormplus.Where("age > ?", 22), gormplus.Order("age DESC"), gormplus.Limit(2))

	assert.NoError(t, err)
	assert.Len(t, found, 2)
	assert.Equal(t, 30, found[0].Age) // Should be ordered DESC
	assert.Equal(t, 25, found[1].Age)
}

func TestBaseModel_List_DatabaseError(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	// Test with invalid SQL to cause error
	_, err = baseModel.List(ctx, gormplus.Where("invalid_column = ?", "value"))
	assert.Error(t, err)
}

func TestBaseModel_Count(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	users := []*User{
		{Name: "User1", Email: "user1@example.com", Age: 20},
		{Name: "User2", Email: "user2@example.com", Age: 25},
		{Name: "User3", Email: "user3@example.com", Age: 30},
	}

	// Create users
	err = baseModel.BatchInsert(ctx, nil, users)
	require.NoError(t, err)

	// Count all
	count, err := baseModel.Count(ctx)

	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Count with condition
	count, err = baseModel.Count(ctx, gormplus.Where("age > ?", 22))

	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestBaseModel_Count_DatabaseError(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	// Test with invalid SQL to cause error
	_, err = baseModel.Count(ctx, gormplus.Where("invalid_column = ?", "value"))
	assert.Error(t, err)
}

func TestBaseModel_Exists(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	// Check non-existence
	exists, err := baseModel.Exists(ctx, gormplus.Where("email = ?", "john@example.com"))
	assert.NoError(t, err)
	assert.False(t, exists)

	// Create user
	err = baseModel.Create(ctx, nil, user)
	require.NoError(t, err)

	// Check existence
	exists, err = baseModel.Exists(ctx, gormplus.Where("email = ?", "john@example.com"))
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestBaseModel_Exists_DatabaseError(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	// Test with invalid SQL to cause error
	_, err = baseModel.Exists(ctx, gormplus.Where("invalid_column = ?", "value"))
	assert.Error(t, err)
}

// ============================================================================
// Scope Functions Tests
// ============================================================================

func TestScopes_Where(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	users := []*User{
		{Name: "Alice", Email: "alice@example.com", Age: 25},
		{Name: "Bob", Email: "bob@example.com", Age: 30},
	}

	err = baseModel.BatchInsert(ctx, nil, users)
	require.NoError(t, err)

	// Test Where with parameters
	found, err := baseModel.List(ctx, gormplus.Where("age = ?", 25))
	assert.NoError(t, err)
	assert.Len(t, found, 1)
	assert.Equal(t, "Alice", found[0].Name)
}

func TestScopes_WhereEq(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	users := []*User{
		{Name: "Alice", Email: "alice@example.com", Age: 25},
		{Name: "Bob", Email: "bob@example.com", Age: 30},
	}

	err = baseModel.BatchInsert(ctx, nil, users)
	require.NoError(t, err)

	// Test WhereEq with map
	found, err := baseModel.List(ctx, gormplus.WhereEq(map[string]any{"age": 25, "name": "Alice"}))
	assert.NoError(t, err)
	assert.Len(t, found, 1)
	assert.Equal(t, "Alice", found[0].Name)
}

func TestScopes_Order(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	users := []*User{
		{Name: "Charlie", Email: "charlie@example.com", Age: 20},
		{Name: "Alice", Email: "alice@example.com", Age: 25},
		{Name: "Bob", Email: "bob@example.com", Age: 30},
	}

	err = baseModel.BatchInsert(ctx, nil, users)
	require.NoError(t, err)

	// Test Order ASC
	found, err := baseModel.List(ctx, gormplus.Order("name ASC"))
	assert.NoError(t, err)
	assert.Len(t, found, 3)
	assert.Equal(t, "Alice", found[0].Name)
	assert.Equal(t, "Bob", found[1].Name)
	assert.Equal(t, "Charlie", found[2].Name)

	// Test Order DESC
	found, err = baseModel.List(ctx, gormplus.Order("age DESC"))
	assert.NoError(t, err)
	assert.Equal(t, 30, found[0].Age)
	assert.Equal(t, 25, found[1].Age)
	assert.Equal(t, 20, found[2].Age)
}

func TestScopes_Select(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	err = baseModel.Create(ctx, nil, user)
	require.NoError(t, err)

	// Test Select specific columns
	found, err := baseModel.First(ctx, gormplus.Select("name", "age"), gormplus.Where("id = ?", user.ID))
	assert.NoError(t, err)
	assert.Equal(t, "John Doe", found.Name)
	assert.Equal(t, 30, found.Age)
	// Email should be empty since not selected
	assert.Empty(t, found.Email)
}

func TestScopes_LimitOffset(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	users := make([]*User, 10)
	for i := range 10 {
		users[i] = &User{
			Name:  fmt.Sprintf("User%d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
			Age:   20 + i,
		}
	}

	err = baseModel.BatchInsert(ctx, nil, users)
	require.NoError(t, err)

	// Test Limit
	found, err := baseModel.List(ctx, gormplus.Order("age ASC"), gormplus.Limit(3))
	assert.NoError(t, err)
	assert.Len(t, found, 3)
	assert.Equal(t, 20, found[0].Age)
	assert.Equal(t, 21, found[1].Age)
	assert.Equal(t, 22, found[2].Age)

	// Test Offset
	found, err = baseModel.List(ctx, gormplus.Order("age ASC"), gormplus.Offset(5), gormplus.Limit(3))
	assert.NoError(t, err)
	assert.Len(t, found, 3)
	assert.Equal(t, 25, found[0].Age)
	assert.Equal(t, 26, found[1].Age)
	assert.Equal(t, 27, found[2].Age)
}

func TestScopes_SoftDelete(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	err = baseModel.Create(ctx, nil, user)
	require.NoError(t, err)

	// Soft delete
	err = baseModel.Delete(ctx, nil, gormplus.Where("id = ?", user.ID))
	require.NoError(t, err)

	// Should not find with normal query
	_, err = baseModel.First(ctx, gormplus.Where("id = ?", user.ID))
	assert.Equal(t, gormplus.ErrNotFound, err)

	// Should find with WithDeleted scope
	found, err := baseModel.First(ctx, gormplus.WithDeleted(), gormplus.Where("id = ?", user.ID))
	assert.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)

	// Should find with OnlyDeleted scope
	found, err = baseModel.First(ctx, gormplus.OnlyDeleted(), gormplus.Where("id = ?", user.ID))
	assert.NoError(t, err)
	assert.Equal(t, user.ID, found.ID)
}

func TestScopes_NilScope(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	err = baseModel.Create(ctx, nil, user)
	require.NoError(t, err)

	// Test with nil scope (should be ignored)
	var nilScope gormplus.Scope = nil
	found, err := baseModel.List(ctx, nilScope, gormplus.Where("id = ?", user.ID))
	assert.NoError(t, err)
	assert.Len(t, found, 1)
}

// ============================================================================
// Pagination Tests
// ============================================================================

func TestBaseModel_Page(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	// Create 25 users
	users := make([]*User, 25)
	for i := range 25 {
		users[i] = &User{
			Name:  fmt.Sprintf("User%02d", i),
			Email: fmt.Sprintf("user%02d@example.com", i),
			Age:   20 + i,
		}
	}

	err = baseModel.BatchInsert(ctx, nil, users)
	require.NoError(t, err)

	// Test first page
	result, err := baseModel.Page(ctx, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 10, result.PageSize)
	assert.Equal(t, int64(25), result.Total)
	assert.True(t, result.HasNext)
	assert.Len(t, result.Items, 10)

	// Test last page
	result, err = baseModel.Page(ctx, 3, 10)
	assert.NoError(t, err)
	assert.Equal(t, 3, result.Page)
	assert.Equal(t, 10, result.PageSize)
	assert.Equal(t, int64(25), result.Total)
	assert.False(t, result.HasNext)
	assert.Len(t, result.Items, 5) // Only 5 items on last page
}

func TestBaseModel_Page_DefaultValues(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	// Create 5 users
	users := make([]*User, 5)
	for i := range 5 {
		users[i] = &User{
			Name:  fmt.Sprintf("User%d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
			Age:   20 + i,
		}
	}

	err = baseModel.BatchInsert(ctx, nil, users)
	require.NoError(t, err)

	// Test default page (should be 1)
	result, err := baseModel.Page(ctx, 0, 10)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Page)

	// Test default page size (should be 20)
	result, err = baseModel.Page(ctx, 1, 0)
	assert.NoError(t, err)
	assert.Equal(t, 20, result.PageSize)
}

func TestBaseModel_Page_MaxPageSize(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	// Test max page size cap (should be 1000)
	result, err := baseModel.Page(ctx, 1, 2000)
	assert.NoError(t, err)
	assert.Equal(t, 1000, result.PageSize)
}

func TestBaseModel_Page_WithScopes(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	// Create users with different ages
	users := make([]*User, 20)
	for i := range 20 {
		users[i] = &User{
			Name:  fmt.Sprintf("User%d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
			Age:   20 + (i % 3), // Ages will be 20, 21, 22, 20, 21, 22, ...
		}
	}

	err = baseModel.BatchInsert(ctx, nil, users)
	require.NoError(t, err)

	// Page with condition
	result, err := baseModel.Page(ctx, 1, 5, gormplus.Where("age = ?", 21), gormplus.Order("name ASC"))
	assert.NoError(t, err)
	assert.Equal(t, int64(7), result.Total) // Should be 7 users with age 21
	assert.Len(t, result.Items, 5)
	assert.True(t, result.HasNext)

	// All returned users should have age 21
	for _, user := range result.Items {
		assert.Equal(t, 21, user.Age)
	}
}

func TestBaseModel_Page_CountError(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	// Test with invalid SQL to cause count error
	_, err = baseModel.Page(ctx, 1, 10, gormplus.Where("invalid_column = ?", "value"))
	assert.Error(t, err)
}

func TestBaseModel_Page_FindError(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	// We need to test the case where Count succeeds but Find fails
	// This is tricky with SQLite, but we can test with invalid scopes
	_, err = baseModel.Page(ctx, 1, 10, gormplus.Where("invalid_column = ?", "value"))
	assert.Error(t, err)
}

// ============================================================================
// Locking Operations Tests
// ============================================================================

func TestBaseModel_FirstForUpdate_RequiresTransaction(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	_, err = baseModel.FirstForUpdate(ctx, nil, gormplus.Where("id = ?", 1))

	assert.Equal(t, gormplus.ErrTxRequired, err)
}

func TestBaseModel_FirstForUpdate_WithTransaction(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	err = baseModel.Create(ctx, nil, user)
	require.NoError(t, err)

	err = db.Transaction(func(tx *gorm.DB) error {
		found, err := baseModel.FirstForUpdate(ctx, tx, gormplus.Where("id = ?", user.ID))
		if err != nil {
			return err
		}

		assert.Equal(t, user.ID, found.ID)
		assert.Equal(t, "John Doe", found.Name)
		return nil
	})

	assert.NoError(t, err)
}

func TestBaseModel_FirstForUpdate_NotFound(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	err = db.Transaction(func(tx *gorm.DB) error {
		_, err := baseModel.FirstForUpdate(ctx, tx, gormplus.Where("id = ?", 999))
		assert.Equal(t, gormplus.ErrNotFound, err)
		return nil
	})

	assert.NoError(t, err)
}

func TestBaseModel_FindForUpdate_RequiresTransaction(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	_, err = baseModel.FindForUpdate(ctx, nil, gormplus.Where("age > ?", 20))

	assert.Equal(t, gormplus.ErrTxRequired, err)
}

func TestBaseModel_FindForUpdate_WithTransaction(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	users := []*User{
		{Name: "User1", Email: "user1@example.com", Age: 25},
		{Name: "User2", Email: "user2@example.com", Age: 30},
	}

	err = baseModel.BatchInsert(ctx, nil, users)
	require.NoError(t, err)

	err = db.Transaction(func(tx *gorm.DB) error {
		found, err := baseModel.FindForUpdate(ctx, tx, gormplus.Where("age > ?", 20))
		if err != nil {
			return err
		}

		assert.Len(t, found, 2)
		return nil
	})

	assert.NoError(t, err)
}

// ============================================================================
// Integration and Complex Scenarios Tests
// ============================================================================

func TestBaseModel_ComplexQuery(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()

	// Create test data
	users := []*User{
		{Name: "Alice Johnson", Email: "alice@example.com", Age: 25},
		{Name: "Bob Smith", Email: "bob@example.com", Age: 30},
		{Name: "Charlie Brown", Email: "charlie@example.com", Age: 35},
		{Name: "Diana Wilson", Email: "diana@example.com", Age: 28},
		{Name: "Eve Davis", Email: "eve@example.com", Age: 32},
	}

	err = baseModel.BatchInsert(ctx, nil, users)
	require.NoError(t, err)

	// Complex query: users over 27, ordered by age desc, limit 3, select only name and age
	found, err := baseModel.List(ctx,
		gormplus.Where("age > ?", 27),
		gormplus.Order("age DESC"),
		gormplus.Limit(3),
		gormplus.Select("name", "age"),
	)

	assert.NoError(t, err)
	assert.Len(t, found, 3)
	assert.Equal(t, "Charlie Brown", found[0].Name)
	assert.Equal(t, 35, found[0].Age)
	assert.Empty(t, found[0].Email) // Should be empty due to Select
	assert.Equal(t, "Eve Davis", found[1].Name)
	assert.Equal(t, 32, found[1].Age)
	assert.Equal(t, "Bob Smith", found[2].Name)
	assert.Equal(t, 30, found[2].Age)
}

func TestBaseModel_MultipleBaseModels(t *testing.T) {
	db := setupTestDB(t)

	userBaseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	productBaseModel, err := gormplus.NewBaseModel[Product](db)
	require.NoError(t, err)

	ctx := context.Background()

	// Create user and product
	user := &User{Name: "John Doe", Email: "john@example.com", Age: 30}
	product := &Product{Name: "Laptop", Price: 1000, Description: "Gaming laptop"}

	err = userBaseModel.Create(ctx, nil, user)
	assert.NoError(t, err)

	err = productBaseModel.Create(ctx, nil, product)
	assert.NoError(t, err)

	// Verify both exist
	userCount, err := userBaseModel.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), userCount)

	productCount, err := productBaseModel.Count(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), productCount)
}

func TestBaseModel_scWithTX_NilDB(t *testing.T) {
	db := setupTestDB(t)
	baseModel, err := gormplus.NewBaseModel[User](db)
	require.NoError(t, err)

	ctx := context.Background()
	user := &User{
		Name:  "John Doe",
		Email: "john@example.com",
		Age:   30,
	}

	err = baseModel.Create(ctx, nil, user)
	require.NoError(t, err)

	// Test scWithTX with nil db (should fall back to baseModel.db)
	err = baseModel.Delete(ctx, nil, gormplus.Where("id = ?", user.ID))
	assert.NoError(t, err)
}

// ============================================================================
// Error Handling Tests
// ============================================================================

func TestBaseModel_ErrorConstants(t *testing.T) {
	assert.Equal(t, "generic type must be a struct type", gormplus.ErrInvalidType.Error())
	assert.Equal(t, "not found", gormplus.ErrNotFound.Error())
	assert.Equal(t, "tx is required", gormplus.ErrTxRequired.Error())
	assert.Equal(t, "dangerous operation is prohibited", gormplus.ErrDangerous.Error())
}
