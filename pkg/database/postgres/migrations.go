package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type Migration struct {
	Version     int
	Description string
	SQL         string
	Timestamp   time.Time
}

type MigrationManager struct {
	client *Client
}

func NewMigrationManager(client *Client) *MigrationManager {
	return &MigrationManager{
		client: client,
	}
}

func (m *MigrationManager) InitMigrationTable(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS migrations (
		id SERIAL PRIMARY KEY,
		version INT NOT NULL UNIQUE,
		description TEXT NOT NULL,
		applied_at TIMESTAMP NOT NULL DEFAULT NOW()
	);`

	_, err := m.client.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("error creating migrations table: %w", err)
	}
	return nil
}

func (m *MigrationManager) GetAppliedMigrations(ctx context.Context) (map[int]time.Time, error) {
	query := `
	SELECT version, applied_at
	FROM migrations
	ORDER BY version;`

	rows, err := m.client.QueryRows(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying migrations: %w", err)
	}
	defer rows.Close()

	result := make(map[int]time.Time)
	for rows.Next() {
		var version int
		var appliedAt time.Time
		if err := rows.Scan(&version, &appliedAt); err != nil {
			return nil, fmt.Errorf("error scanning migration: %w", err)
		}
		result[version] = appliedAt
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating migrations: %w", err)
	}

	return result, nil
}

func (m *MigrationManager) ApplyMigration(ctx context.Context, migration Migration) error {
	return m.client.ExecTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, migration.SQL)
		if err != nil {
			return fmt.Errorf("error applying migration %d: %w", migration.Version, err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO migrations (version, description, applied_at)
			VALUES ($1, $2, $3);
		`, migration.Version, migration.Description, time.Now())
		if err != nil {
			return fmt.Errorf("error registering migration %d: %w", migration.Version, err)
		}

		return nil
	})
}

func (m *MigrationManager) Migrate(ctx context.Context, migrations []Migration) error {
	if err := m.InitMigrationTable(ctx); err != nil {
		return err
	}

	applied, err := m.GetAppliedMigrations(ctx)
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		if _, exists := applied[migration.Version]; exists {
			continue
		}

		if err := m.ApplyMigration(ctx, migration); err != nil {
			return err
		}

		fmt.Printf("Applied migration %d: %s\n", migration.Version, migration.Description)
	}

	return nil
}

func (m *MigrationManager) RollbackLastMigration(ctx context.Context, rollbacks map[int]string) error {
	var lastVersion int
	var lastAppliedAt time.Time

	err := m.client.QueryRow(ctx, `
		SELECT version, applied_at
		FROM migrations
		ORDER BY version DESC
		LIMIT 1
	`).Scan(&lastVersion, &lastAppliedAt)

	if err != nil {
		return fmt.Errorf("error getting last migration: %w", err)
	}

	rollbackSQL, exists := rollbacks[lastVersion]
	if !exists {
		return fmt.Errorf("no rollback for migration %d", lastVersion)
	}

	return m.client.ExecTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, rollbackSQL)
		if err != nil {
			return fmt.Errorf("error applying rollback %d: %w", lastVersion, err)
		}

		_, err = tx.Exec(ctx, `
			DELETE FROM migrations
			WHERE version = $1
		`, lastVersion)
		if err != nil {
			return fmt.Errorf("error deleting migration record %d: %w", lastVersion, err)
		}

		return nil
	})
}

// LoadMigrationsFromPath loads migrations from SQL files in the specified directory path.
// The function expects SQL files to be named with the pattern: {version}_{description}.sql
// where version is a numeric identifier and description is the migration description.
// example: 001_create_users.sql
func LoadMigrationsFromPath(migrationsPath string) ([]Migration, error) {
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("error reading migrations directory: %w", err)
	}

	var migrations []Migration
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			parts := strings.SplitN(entry.Name(), "_", 2)
			if len(parts) < 2 {
				continue
			}

			version, err := strconv.Atoi(parts[0])
			if err != nil {
				continue
			}

			content, err := os.ReadFile(filepath.Join(migrationsPath, entry.Name()))
			if err != nil {
				return nil, fmt.Errorf("error reading migration file %s: %w", entry.Name(), err)
			}

			description := strings.TrimSuffix(entry.Name(), ".sql")

			migration := Migration{
				Version:     version,
				Description: description,
				SQL:         string(content),
				Timestamp:   time.Now(),
			}
			migrations = append(migrations, migration)
		}
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}
