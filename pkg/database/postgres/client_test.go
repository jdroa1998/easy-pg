package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestClient(t *testing.T) {
	dbHost := os.Getenv("TEST_DB_HOST")
	if dbHost == "" {
		t.Skip("Pruebas de base de datos desactivadas. Establece TEST_DB_HOST para activarlas")
	}

	pgConfig := Config{
		Host:     dbHost,
		Port:     getEnvInt("TEST_DB_PORT", 5432),
		User:     getEnvString("TEST_DB_USER", "postgres"),
		Password: getEnvString("TEST_DB_PASSWORD", "postgres"),
		DBName:   getEnvString("TEST_DB_NAME", "example"),
		SSLMode:  "disable",
		Timeout:  30 * time.Second,
		MaxConns: 50,
	}

	client := NewClient(pgConfig)

	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Error conectando a PostgreSQL: %v", err)
	}
	defer client.Close()

	t.Run("Ping", func(t *testing.T) {
		if err := client.Ping(ctx); err != nil {
			t.Errorf("Ping falló: %v", err)
		}
	})

	t.Run("CreateTable", func(t *testing.T) {
		_, err := client.Exec(ctx, `
			DROP TABLE IF EXISTS test_users;
			CREATE TABLE test_users (
				id SERIAL PRIMARY KEY,
				name TEXT NOT NULL,
				email TEXT NOT NULL UNIQUE,
				created_at TIMESTAMP NOT NULL DEFAULT NOW()
			);
		`)
		if err != nil {
			t.Errorf("Error creando tabla: %v", err)
		}
	})

	t.Run("Insert", func(t *testing.T) {
		_, err := client.Exec(ctx, `
			INSERT INTO test_users (name, email) 
			VALUES ('Test User', 'test@example.com')
		`)
		if err != nil {
			t.Errorf("Error insertando: %v", err)
		}
	})

	t.Run("Query", func(t *testing.T) {
		var name, email string
		err := client.QueryRow(ctx, `
			SELECT name, email FROM test_users WHERE email = $1
		`, "test@example.com").Scan(&name, &email)

		if err != nil {
			t.Errorf("Error consultando: %v", err)
		}

		if name != "Test User" {
			t.Errorf("Nombre esperado: %s, obtenido: %s", "Test User", name)
		}

		if email != "test@example.com" {
			t.Errorf("Email esperado: %s, obtenido: %s", "test@example.com", email)
		}
	})

	t.Run("SuccessfulTransaction", func(t *testing.T) {
		err := client.ExecTx(ctx, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO test_users (name, email) 
				VALUES ('Transaction User', 'transaction@example.com')
			`)
			return err
		})

		if err != nil {
			t.Errorf("Error en transacción: %v", err)
		}

		var count int
		err = client.QueryRow(ctx, `
			SELECT COUNT(*) FROM test_users WHERE email = $1
		`, "transaction@example.com").Scan(&count)

		if err != nil {
			t.Errorf("Error consultando: %v", err)
		}

		if count != 1 {
			t.Errorf("Esperaba 1 registro, encontré %d", count)
		}
	})

	t.Run("FailedTransaction", func(t *testing.T) {
		_ = client.ExecTx(ctx, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO test_users (name, email) 
				VALUES ('Rollback User', 'rollback@example.com')
			`)
			return err
		})

		err := client.ExecTx(ctx, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `
				INSERT INTO test_users (name, email) 
				VALUES ('Another User', 'rollback@example.com')
			`)
			return err
		})

		if err == nil {
			t.Errorf("Se esperaba un error por duplicación, pero la transacción fue exitosa")
		}

		var count int
		err = client.QueryRow(ctx, `
			SELECT COUNT(*) FROM test_users WHERE email = $1
		`, "rollback@example.com").Scan(&count)

		if err != nil {
			t.Errorf("Error consultando: %v", err)
		}

		if count != 1 {
			t.Errorf("Esperaba 1 registro, encontré %d", count)
		}
	})

	t.Run("Cleanup", func(t *testing.T) {
		_, err := client.Exec(ctx, "DROP TABLE IF EXISTS test_users")
		if err != nil {
			t.Errorf("Error eliminando tabla: %v", err)
		}
	})
}
