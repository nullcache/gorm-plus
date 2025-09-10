# GORM Plus

A type-safe generic repository pattern implementation for GORM, providing a clean and consistent interface for common database operations.

## Features

- **Type-safe operations**: Generic implementation ensures compile-time type safety
- **Transaction support**: Built-in transaction management with automatic rollback
- **Flexible querying**: Composable scope functions for building complex queries
- **Pagination**: Built-in pagination support with configurable page sizes
- **Soft delete support**: Full support for GORM's soft delete functionality
- **Row locking**: SELECT FOR UPDATE support for concurrent access control
- **Batch operations**: Efficient batch insert operations with configurable batch sizes

## Installation

```bash
go get github.com/nullcache/gorm-plus
```

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/nullcache/gorm-plus"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

type User struct {
    ID    uint   `gorm:"primaryKey"`
    Name  string `gorm:"not null"`
    Email string `gorm:"unique;not null"`
    Age   int    `gorm:"default:0"`
}

func main() {
    // Initialize database
    db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
    if err != nil {
        log.Fatal(err)
    }

    // Auto migrate
    db.AutoMigrate(&User{})

    // Create repository
    userRepo, err := gormplus.NewRepo[User](db)
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    // Create a user
    user := &User{Name: "John Doe", Email: "john@example.com", Age: 30}
    err = userRepo.Create(ctx, nil, user)
    if err != nil {
        log.Fatal(err)
    }

    // Find user by email
    foundUser, err := userRepo.First(ctx, gormplus.Where("email = ?", "john@example.com"))
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Found user: %+v", foundUser)
}
```

## Core Components

### Repository

The `Repo[T]` type is the main component that provides database operations for entities of type T.

```go
// Create a new repository instance
userRepo, err := gormplus.NewRepo[User](db)
```

### Scopes

Scopes are composable functions that modify GORM queries:

```go
// Basic conditions
users, err := userRepo.List(ctx,
    gormplus.Where("age > ?", 25),
    gormplus.Order("name ASC"),
    gormplus.Limit(10),
)

// Map-based conditions
users, err := userRepo.List(ctx,
    gormplus.WhereEq(map[string]any{
        "active": true,
        "role":   "admin",
    }),
)
```

### Available Scopes

- `Where(query, args...)` - Add WHERE conditions
- `WhereEq(map[string]any)` - Add equality conditions from map
- `Order(string)` - Add ORDER BY clause
- `Select(columns...)` - Select specific columns
- `Limit(int)` - Limit number of results
- `Offset(int)` - Skip number of results
- `WithDeleted()` - Include soft-deleted records
- `OnlyDeleted()` - Only soft-deleted records

## Operations

### CRUD Operations

```go
// Create
user := &User{Name: "Jane Doe", Email: "jane@example.com"}
err := userRepo.Create(ctx, nil, user)

// Update
user.Age = 25
err = userRepo.Update(ctx, nil, user)

// Delete (soft delete if DeletedAt field exists)
err = userRepo.Delete(ctx, nil, gormplus.Where("id = ?", user.ID))
```

### Query Operations

```go
// Find first record
user, err := userRepo.First(ctx, gormplus.Where("email = ?", "john@example.com"))

// List records
users, err := userRepo.List(ctx,
    gormplus.Where("age > ?", 18),
    gormplus.Order("name ASC"),
)

// Count records
count, err := userRepo.Count(ctx, gormplus.Where("active = ?", true))

// Check existence
exists, err := userRepo.Exists(ctx, gormplus.Where("email = ?", "test@example.com"))
```

### Pagination

```go
// Get paginated results
result, err := userRepo.Page(ctx, 1, 20, // page 1, 20 items per page
    gormplus.Where("active = ?", true),
    gormplus.Order("created_at DESC"),
)

// Access pagination info
fmt.Printf("Page: %d/%d, Total: %d, HasNext: %v",
    result.Page,
    (result.Total + int64(result.PageSize) - 1) / int64(result.PageSize),
    result.Total,
    result.HasNext,
)
```

### Batch Operations

```go
users := []*User{
    {Name: "User 1", Email: "user1@example.com"},
    {Name: "User 2", Email: "user2@example.com"},
    {Name: "User 3", Email: "user3@example.com"},
}

// Batch insert with default batch size (1000)
err := userRepo.BatchInsert(ctx, nil, users)

// Batch insert with custom batch size
err = userRepo.BatchInsert(ctx, nil, users, 100)
```

### Transactions

```go
// Manual transaction management
err := userRepo.Transact(ctx, func(ctx context.Context, tx *gorm.DB) error {
    user1 := &User{Name: "User 1", Email: "user1@example.com"}
    if err := userRepo.Create(ctx, tx, user1); err != nil {
        return err
    }

    user2 := &User{Name: "User 2", Email: "user2@example.com"}
    if err := userRepo.Create(ctx, tx, user2); err != nil {
        return err
    }

    return nil // Commit transaction
})

// Row locking (requires transaction)
err = db.Transaction(func(tx *gorm.DB) error {
    user, err := userRepo.FirstForUpdate(ctx, tx, gormplus.Where("id = ?", 1))
    if err != nil {
        return err
    }

    // Modify user safely
    user.Balance += 100
    return userRepo.Update(ctx, tx, &user)
})
```

## Error Handling

The library defines several standard errors:

```go
// Check for specific errors
user, err := userRepo.First(ctx, gormplus.Where("id = ?", 999))
if errors.Is(err, gormplus.ErrNotFound) {
    // Handle not found case
}

// Available errors:
// - gormplus.ErrInvalidType: Invalid generic type
// - gormplus.ErrNotFound: Record not found
// - gormplus.ErrTxRequired: Transaction required for operation
// - gormplus.ErrDangerous: Dangerous operation (e.g., delete without conditions)
```

## Best Practices

1. **Always use contexts**: Pass context for cancellation and timeout support
2. **Handle transactions properly**: Use the `Transact` method for complex operations
3. **Validate inputs**: Check for required conditions before dangerous operations
4. **Use appropriate scopes**: Combine scopes to build precise queries
5. **Handle errors**: Always check for specific error types when needed

## Requirements

- Go 1.19 or higher
- GORM v1.25.0 or higher

## License

MIT License. See LICENSE file for details.

## Contributing

Contributions are welcome. Please ensure all tests pass and follow the existing code style.
