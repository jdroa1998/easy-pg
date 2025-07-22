# Easy-PG: PostgreSQL Library with pgx for Go

Easy-PG is a minimalist and easy-to-use library for working with PostgreSQL in Go, using the native [pgx](https://github.com/jackc/pgx) library.

## Features

- ✅ Simplified PostgreSQL connection handling
- ✅ Configurable connection pool
- ✅ Transaction support with rollback capabilities
- ✅ **Database migrations system**
- ✅ Descriptive errors with context
- ✅ Easy integration with Go applications
- ✅ Lightweight implementation with minimal dependencies
- ✅ Comprehensive logging support
- ✅ Environment variable configuration
- ✅ Batch operations support

## Installation

```bash
go get github.com/jdroa1998/easy-pg
```

## Quick Start

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/jdroa1998/easy-pg/pkg/database/postgres"
)

func main() {
	pgConfig := postgres.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "postgres",
		DBName:   "example",
		SSLMode:  "disable",
		Timeout:  5 * time.Second,
		MaxConns: 10,
	}

	pgClient := postgres.NewClient(pgConfig)

	ctx := context.Background()
	if err := pgClient.Connect(ctx); err != nil {
		log.Fatalf("Error connecting to PostgreSQL: %v", err)
	}
	defer pgClient.Close()

	// Your database operations here...
}
```

## Database Client Methods

### 1. Creating a Client

#### From Configuration
```go
config := postgres.Config{
	Host:     "localhost",
	Port:     5432,
	User:     "postgres",
	Password: "postgres",
	DBName:   "example",
	SSLMode:  "disable",
	Timeout:  30 * time.Second,
	MaxConns: 10,
	Logger:   logger, // optional
}

client := postgres.NewClient(config)
```

#### From Environment Variables
```go
// Set environment variables:
// DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE, DB_TIMEOUT, DB_MAX_CONNS
client := postgres.NewClientFromEnv()
```

### 2. Connection Management

#### Connect to Database
```go
ctx := context.Background()
if err := client.Connect(ctx); err != nil {
	log.Fatal("Connection failed:", err)
}
```

#### Close Connection
```go
defer client.Close()
```

#### Ping Database
```go
if err := client.Ping(ctx); err != nil {
	log.Fatal("Database is not available:", err)
}
```

### 3. Query Operations

#### Single Row Query
```go
var userID int
var name, email string

err := client.QueryRow(ctx, 
	"SELECT id, name, email FROM users WHERE id = $1", 
	123).Scan(&userID, &name, &email)

if err != nil {
	log.Fatal("Query failed:", err)
}

fmt.Printf("User: %s (%s)\n", name, email)
```

#### Multiple Rows Query
```go
rows, err := client.QueryRows(ctx, "SELECT id, name, email FROM users")
if err != nil {
	log.Fatal("Query failed:", err)
}
defer rows.Close()

for rows.Next() {
	var id int
	var name, email string
	
	if err := rows.Scan(&id, &name, &email); err != nil {
		log.Fatal("Scan failed:", err)
	}
	
	fmt.Printf("User %d: %s (%s)\n", id, name, email)
}

if err := rows.Err(); err != nil {
	log.Fatal("Rows error:", err)
}
```

### 4. Execute Operations

#### Single Execute
```go
result, err := client.Exec(ctx, 
	"INSERT INTO users (name, email) VALUES ($1, $2)", 
	"John Doe", "john@example.com")

if err != nil {
	log.Fatal("Insert failed:", err)
}

fmt.Printf("Rows affected: %d\n", result.RowsAffected())
```

#### Batch Execute
```go
queries := []string{
	"INSERT INTO users (name, email) VALUES ($1, $2)",
	"INSERT INTO users (name, email) VALUES ($1, $2)",
	"UPDATE users SET status = $1 WHERE id = $2",
}

args := [][]any{
	{"John Doe", "john@example.com"},
	{"Jane Doe", "jane@example.com"},
	{"active", 1},
}

err := client.ExecBatch(ctx, queries, args)
if err != nil {
	log.Fatal("Batch execution failed:", err)
}
```

### 5. Transactions

#### Simple Transaction
```go
err := client.ExecTx(ctx, func(tx pgx.Tx) error {
	// Insert user
	_, err := tx.Exec(ctx, 
		"INSERT INTO users (name, email) VALUES ($1, $2)", 
		"John", "john@example.com")
	if err != nil {
		return err // Automatic rollback
	}
	
	// Update account balance
	_, err = tx.Exec(ctx, 
		"UPDATE accounts SET balance = balance - $1 WHERE user_id = $2", 
		100, 1)
	if err != nil {
		return err // Automatic rollback
	}
	
	return nil // Automatic commit
})

if err != nil {
	log.Fatal("Transaction failed:", err)
}
```

#### Complex Transaction with Query
```go
err := client.ExecTx(ctx, func(tx pgx.Tx) error {
	// Get current balance
	var balance float64
	err := tx.QueryRow(ctx, 
		"SELECT balance FROM accounts WHERE user_id = $1", 
		userID).Scan(&balance)
	if err != nil {
		return err
	}
	
	// Check if sufficient funds
	if balance < transferAmount {
		return fmt.Errorf("insufficient funds")
	}
	
	// Perform transfer
	_, err = tx.Exec(ctx, 
		"UPDATE accounts SET balance = balance - $1 WHERE user_id = $2", 
		transferAmount, fromUserID)
	if err != nil {
		return err
	}
	
	_, err = tx.Exec(ctx, 
		"UPDATE accounts SET balance = balance + $1 WHERE user_id = $2", 
		transferAmount, toUserID)
	
	return err
})
```

## Database Migrations

Easy-PG includes a powerful migration system that allows you to manage database schema changes.

### Creating Migration Files

Create SQL files in your migrations directory with the naming convention: `{version}_{description}.sql`

Example:
```
migrations/
├── 001_create_users.sql
├── 002_add_user_status.sql
└── 003_create_posts.sql
```

### Migration File Example

```sql
-- 001_create_users.sql
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
```

### Migration Methods

#### Load Migrations from Directory
```go
migrations, err := postgres.LoadMigrationsFromPath("./migrations")
if err != nil {
    log.Fatal("Error loading migrations:", err)
}
```

#### Create Migration Manager
```go
migrationManager := postgres.NewMigrationManager(client)
```

#### Apply All Pending Migrations
```go
err := migrationManager.Migrate(ctx, migrations)
if err != nil {
    log.Fatal("Error applying migrations:", err)
}
```

#### Check Applied Migrations
```go
appliedMigrations, err := migrationManager.GetAppliedMigrations(ctx)
if err != nil {
    log.Fatal("Error getting applied migrations:", err)
}

for version, appliedAt := range appliedMigrations {
    fmt.Printf("Migration %d applied at %v\n", version, appliedAt)
}
```

#### Apply Single Migration
```go
migration := postgres.Migration{
    Version:     1,
    Description: "001_create_users",
    SQL:         "CREATE TABLE users...",
}

err := migrationManager.ApplyMigration(ctx, migration)
if err != nil {
    log.Fatal("Error applying migration:", err)
}
```

#### Rollback Last Migration
```go
rollbacks := map[int]string{
    3: "DROP TABLE posts;",
    2: "ALTER TABLE users DROP COLUMN status;",
    1: "DROP TABLE users;",
}

err := migrationManager.RollbackLastMigration(ctx, rollbacks)
if err != nil {
    log.Fatal("Error rolling back migration:", err)
}
```

## Configuration Options

```go
config := postgres.Config{
	Host:     "localhost",        // Database host
	Port:     5432,              // Database port
	User:     "postgres",        // Database user
	Password: "password",        // Database password
	DBName:   "mydb",           // Database name
	SSLMode:  "disable",        // SSL mode (disable, require, verify-ca, verify-full)
	Timeout:  30 * time.Second, // Connection timeout
	MaxConns: 10,               // Maximum number of connections in pool
	Logger:   logger,           // Optional logger (compatible with easy-logger)
}
```

### Environment Variables

When using `NewClientFromEnv()`, the following environment variables are supported:

- `DB_HOST` (default: "localhost")
- `DB_PORT` (default: 5432)
- `DB_USER` (default: "postgres")
- `DB_PASSWORD` (default: "postgres")
- `DB_NAME` (default: "postgres")
- `DB_SSLMODE` (default: "disable")
- `DB_TIMEOUT` (default: 30 seconds)
- `DB_MAX_CONNS` (default: 50)

## Complete Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jdroa1998/easy-pg/pkg/database/postgres"
)

type User struct {
	ID    int
	Name  string
	Email string
}

func main() {
	// Configure database
	config := postgres.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "postgres",
		DBName:   "example",
		SSLMode:  "disable",
		Timeout:  30 * time.Second,
		MaxConns: 10,
	}

	client := postgres.NewClient(config)
	ctx := context.Background()

	// Connect
	if err := client.Connect(ctx); err != nil {
		log.Fatal("Connection failed:", err)
	}
	defer client.Close()

	// Load and apply migrations
	migrations, err := postgres.LoadMigrationsFromPath("./migrations")
	if err != nil {
		log.Fatal("Failed to load migrations:", err)
	}

	migrationManager := postgres.NewMigrationManager(client)
	if err := migrationManager.Migrate(ctx, migrations); err != nil {
		log.Fatal("Failed to apply migrations:", err)
	}

	// Create user
	userID := createUser(ctx, client, "John Doe", "john@example.com")
	fmt.Printf("Created user with ID: %d\n", userID)

	// Get user
	user := getUser(ctx, client, userID)
	fmt.Printf("Retrieved user: %+v\n", user)
}

func createUser(ctx context.Context, client *postgres.Client, name, email string) int {
	var userID int
	err := client.QueryRow(ctx,
		"INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id",
		name, email).Scan(&userID)
	if err != nil {
		log.Fatal("Failed to create user:", err)
	}
	return userID
}

func getUser(ctx context.Context, client *postgres.Client, id int) User {
	var user User
	err := client.QueryRow(ctx,
		"SELECT id, name, email FROM users WHERE id = $1",
		id).Scan(&user.ID, &user.Name, &user.Email)
	if err != nil {
		log.Fatal("Failed to get user:", err)
	}
	return user
}
```

## API Reference

### Client Methods

- `NewClient(config Config) *Client` - Create client from configuration
- `NewClientFromEnv() *Client` - Create client from environment variables
- `Connect(ctx context.Context) error` - Establish database connection
- `Close()` - Close database connection
- `Ping(ctx context.Context) error` - Test database connectivity
- `Pool() *pgxpool.Pool` - Get the underlying connection pool

### Query Methods

- `QueryRow(ctx context.Context, query string, args ...any) pgx.Row` - Execute single row query
- `QueryRows(ctx context.Context, query string, args ...any) (pgx.Rows, error)` - Execute multiple rows query
- `Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)` - Execute command
- `ExecBatch(ctx context.Context, queries []string, args [][]any) error` - Execute batch commands
- `ExecTx(ctx context.Context, fn func(pgx.Tx) error) error` - Execute transaction

### Migration Methods

- `LoadMigrationsFromPath(path string) ([]Migration, error)` - Load migrations from directory
- `NewMigrationManager(client *Client) *MigrationManager` - Create migration manager
- `InitMigrationTable(ctx context.Context) error` - Initialize migration tracking table
- `GetAppliedMigrations(ctx context.Context) (map[int]time.Time, error)` - Get applied migrations
- `ApplyMigration(ctx context.Context, migration Migration) error` - Apply single migration
- `Migrate(ctx context.Context, migrations []Migration) error` - Apply all pending migrations
- `RollbackLastMigration(ctx context.Context, rollbacks map[int]string) error` - Rollback last migration

## Contributing

Contributions are welcome! Please submit a pull request or open an issue to discuss proposed changes.

## Examples

For hands-on examples and advanced usage patterns, check the `cmd/example/` directory:

- **`cmd/example/main.go`** - Complete CRUD operations example
- **`cmd/example/migrations_example.go`** - Migration system demonstration
- **`cmd/example/migrations/`** - Sample migration files

### Running the Examples

1. **Start PostgreSQL:**
   ```bash
   docker-compose up -d
   ```

2. **Run the basic example:**
   ```bash
   go run cmd/example/main.go
   ```

The examples demonstrate real-world usage patterns including:
- Database connection setup
- CRUD operations with proper error handling
- Transaction management
- Migration system usage
- Batch operations

## License

MIT License - see [LICENSE](LICENSE) file for details.
