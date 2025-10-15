// Package gormplus provides a generic repository pattern implementation for GORM.
// It offers a type-safe wrapper around GORM operations with support for
// transactions, scoped queries, pagination, and common CRUD operations.
package gormplus

import (
	"context"
	"errors"
	"reflect"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Common errors returned by repository operations.
var (
	// ErrInvalidType is returned when the generic type T is not a struct type.
	ErrInvalidType = errors.New("generic type must be a struct type")

	// ErrNotFound is returned when a requested record is not found.
	ErrNotFound = errors.New("not found")

	// ErrTxRequired is returned when a transaction is required but not provided.
	ErrTxRequired = errors.New("tx is required")

	// ErrDangerous is returned when attempting potentially dangerous operations
	// like deleting without conditions.
	ErrDangerous = errors.New("dangerous operation is prohibited")
)

// Repo is a generic repository that provides common database operations
// for entities of type T. It wraps a GORM database instance and provides
// type-safe methods for CRUD operations, querying, and transaction handling.
type Repo[T any] struct {
	db *gorm.DB
}

// Scope represents a function that can modify a GORM database query.
// Scopes can be chained together to build complex queries in a composable way.
type Scope func(*gorm.DB) *gorm.DB

// PageResult represents the result of a paginated query.
type PageResult[T any] struct {
	Items    []T   `json:"items"`     // The items in the current page
	Total    int64 `json:"total"`     // Total number of items across all pages
	Page     int   `json:"page"`      // Current page number (1-based)
	PageSize int   `json:"page_size"` // Number of items per page
	HasNext  bool  `json:"has_next"`  // Whether there are more pages available
}

// NewRepo creates a new generic repository instance for type T.
// It validates that T is a struct type.
// Returns an error if T is not a valid struct type.
func NewRepo[T any](db *gorm.DB) (*Repo[T], error) {
	var zero T

	t := reflect.TypeOf(zero)
	if t.Kind() == reflect.Pointer {
		return nil, ErrInvalidType
	}
	if t.Kind() != reflect.Struct {
		return nil, ErrInvalidType
	}

	return &Repo[T]{
		db: db.Session(&gorm.Session{NewDB: false}),
	}, nil
}

// Transact executes the provided function within a database transaction.
// If the function returns an error, the transaction is rolled back.
// Otherwise, the transaction is committed.
func (r *Repo[T]) Transact(ctx context.Context, fn func(ctx context.Context, tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(ctx, tx) })
}

// Where creates a scope that adds a WHERE clause to the query.
// It accepts the same parameters as GORM's Where method.
func Where(query any, args ...any) Scope {
	return func(db *gorm.DB) *gorm.DB { return db.Where(query, args...) }
}

// WhereEq creates a scope that adds WHERE clauses for exact matches
// using a map of column names to values.
func WhereEq(m map[string]any) Scope {
	return func(db *gorm.DB) *gorm.DB { return db.Where(m) }
}

// Order creates a scope that adds an ORDER BY clause to the query.
func Order(order string) Scope {
	return func(db *gorm.DB) *gorm.DB { return db.Order(order) }
}

// Select creates a scope that specifies which columns to select.
func Select(cols ...string) Scope {
	return func(db *gorm.DB) *gorm.DB { return db.Select(cols) }
}

// Limit creates a scope that limits the number of returned records.
func Limit(n int) Scope {
	return func(db *gorm.DB) *gorm.DB { return db.Limit(n) }
}

// Offset creates a scope that skips the specified number of records.
func Offset(n int) Scope {
	return func(db *gorm.DB) *gorm.DB { return db.Offset(n) }
}

// WithDeleted creates a scope that includes soft-deleted records in the query.
// This is equivalent to GORM's Unscoped method.
func WithDeleted() Scope {
	return func(db *gorm.DB) *gorm.DB { return db.Unscoped() }
}

// OnlyDeleted creates a scope that returns only soft-deleted records.
func OnlyDeleted() Scope {
	return func(db *gorm.DB) *gorm.DB { return db.Unscoped().Where("deleted_at IS NOT NULL") }
}

// Create inserts a new entity into the database.
// If tx is provided, the operation is performed within that transaction.
// Otherwise, it uses the repository's default database connection.
func (r *Repo[T]) Create(ctx context.Context, tx *gorm.DB, ent *T) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Create(ent).Error
}

// Update saves the entity to the database, updating all fields.
// If tx is provided, the operation is performed within that transaction.
// Otherwise, it uses the repository's default database connection.
func (r *Repo[T]) Update(ctx context.Context, tx *gorm.DB, ent *T) error {
	db := r.db
	if tx != nil {
		db = tx
	}
	return db.WithContext(ctx).Save(ent).Error
}

// UpdateColumn updates a single column for records matching the provided scopes.
// At least one scope must be provided to prevent accidental update of all records.
// If tx is provided, the operation is performed within that transaction.
func (r *Repo[T]) UpdateColumn(ctx context.Context, tx *gorm.DB, column string, value any, scopes ...Scope) error {
	if len(scopes) == 0 {
		return ErrDangerous
	}
	return r.scWithTX(tx, ctx, scopes...).Update(column, value).Error
}

// UpdateColumns updates multiple columns for records matching the provided scopes.
// At least one scope must be provided to prevent accidental update of all records.
// If tx is provided, the operation is performed within that transaction.
// The updates parameter can be a map[string]any or a struct.
func (r *Repo[T]) UpdateColumns(ctx context.Context, tx *gorm.DB, updates any, scopes ...Scope) error {
	if len(scopes) == 0 {
		return ErrDangerous
	}
	return r.scWithTX(tx, ctx, scopes...).Updates(updates).Error
}

// Delete removes records from the database based on the provided conditions.
// At least one scope must be provided to prevent accidental deletion of all records.
// If tx is provided, the operation is performed within that transaction.
func (r *Repo[T]) Delete(ctx context.Context, tx *gorm.DB, scopes ...Scope) error {
	if len(scopes) == 0 {
		return ErrDangerous
	}
	return r.scWithTX(tx, ctx, scopes...).Delete(new(T)).Error
}

// BatchInsert performs a batch insert operation for multiple entities.
// If tx is provided, the operation is performed within that transaction.
// The optional batchSize parameter controls how many records are inserted in each batch.
// If not specified or zero, defaults to 1000 records per batch.
func (r *Repo[T]) BatchInsert(ctx context.Context, tx *gorm.DB, ents []*T, batchSize ...int) error {
	if len(ents) == 0 {
		return nil
	}
	db := r.db
	if tx != nil {
		db = tx
	}

	size := 1000
	if len(batchSize) > 0 {
		size = batchSize[0]
	}
	if size == 0 {
		size = 1000
	}
	return db.WithContext(ctx).CreateInBatches(ents, size).Error
}

// First retrieves the first record that matches the provided scopes.
// Returns ErrNotFound if no record is found.
func (r *Repo[T]) First(ctx context.Context, scopes ...Scope) (T, error) {
	var out T
	if err := r.sc(ctx, scopes...).First(&out).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return out, ErrNotFound
		}
		return out, err
	}
	return out, nil
}

// List retrieves all records that match the provided scopes.
// Consider using Limit and Order scopes to control the result set size and ordering.
func (r *Repo[T]) List(ctx context.Context, scopes ...Scope) ([]T, error) {
	var out []T
	if err := r.sc(ctx, scopes...).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// Count returns the number of records that match the provided scopes.
func (r *Repo[T]) Count(ctx context.Context, scopes ...Scope) (int64, error) {
	var total int64
	if err := r.sc(ctx, scopes...).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// Exists checks whether any record matching the provided scopes exists.
// Returns true if at least one record exists, false otherwise.
func (r *Repo[T]) Exists(ctx context.Context, scopes ...Scope) (bool, error) {
	var count int64
	err := r.sc(ctx, scopes...).Limit(1).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// FirstForUpdate retrieves the first record that matches the provided scopes
// with a SELECT FOR UPDATE lock. This method requires a transaction to be provided.
// Returns ErrNotFound if no record is found, ErrTxRequired if no transaction is provided.
func (r *Repo[T]) FirstForUpdate(ctx context.Context, tx *gorm.DB, scopes ...Scope) (T, error) {
	var zero T
	if tx == nil {
		return zero, ErrTxRequired
	}

	scopes = append(scopes, func(d *gorm.DB) *gorm.DB {
		return d.Clauses(clause.Locking{Strength: "UPDATE"})
	})

	var v T
	if err := r.scWithTX(tx, ctx, scopes...).First(&v).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return zero, ErrNotFound
		}
		return zero, err
	}
	return v, nil
}

// FindForUpdate retrieves all records that match the provided scopes
// with a SELECT FOR UPDATE lock. This method requires a transaction to be provided.
// Returns ErrTxRequired if no transaction is provided.
func (r *Repo[T]) FindForUpdate(ctx context.Context, tx *gorm.DB, scopes ...Scope) ([]T, error) {
	var zero []T
	if tx == nil {
		return zero, ErrTxRequired
	}

	scopes = append(scopes, func(d *gorm.DB) *gorm.DB {
		return d.Clauses(clause.Locking{Strength: "UPDATE"})
	})

	var out []T
	if err := r.scWithTX(tx, ctx, scopes...).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// Page retrieves a paginated result set based on the provided scopes.
// Page numbers are 1-based. If page <= 0, defaults to 1.
// If pageSize <= 0, defaults to 20. Maximum pageSize is capped at 1000.
func (r *Repo[T]) Page(ctx context.Context, page, pageSize int, scopes ...Scope) (PageResult[T], error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	// Cap the page size to prevent excessive resource usage
	if pageSize > 1000 {
		pageSize = 1000
	}

	// First, get the total count
	total, err := r.Count(ctx, scopes...)
	if err != nil {
		return PageResult[T]{}, err
	}

	// Then, fetch the data for the current page
	offset := (page - 1) * pageSize
	var items []T
	q := append(scopes, Limit(pageSize), Offset(offset))
	if err := r.sc(ctx, q...).Find(&items).Error; err != nil {
		return PageResult[T]{}, err
	}

	return PageResult[T]{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasNext:  int64(page*pageSize) < total,
	}, nil
}

// sc creates a base query with context and model, then applies the provided scopes.
// This is the unified starting point for all query operations.
func (r *Repo[T]) sc(ctx context.Context, scopes ...Scope) *gorm.DB {
	db := r.db.WithContext(ctx).Model(new(T))
	for _, s := range scopes {
		if s != nil {
			db = s(db)
		}
	}
	return db
}

// scWithTX creates a base query with context and model using the provided transaction,
// then applies the provided scopes. If db is nil, falls back to the repository's default DB.
func (r *Repo[T]) scWithTX(db *gorm.DB, ctx context.Context, scopes ...Scope) *gorm.DB {
	if db == nil {
		db = r.db
	}
	q := db.WithContext(ctx).Model(new(T))
	for _, s := range scopes {
		if s != nil {
			q = s(q)
		}
	}
	return q
}
