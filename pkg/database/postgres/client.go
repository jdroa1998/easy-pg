package postgres

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jdroa1998/easy-logger/logger"
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
	Timeout  time.Duration
	MaxConns int32
	Logger   *logger.Logger
}

type Client struct {
	pool   *pgxpool.Pool
	cfg    Config
	logger *logger.Logger
}

func NewClientFromEnv() *Client {
	cfg := Config{
		Host:     getEnvString("DB_HOST", "localhost"),
		Port:     getEnvInt("DB_PORT", 5432),
		User:     getEnvString("DB_USER", "postgres"),
		Password: getEnvString("DB_PASSWORD", "postgres"),
		DBName:   getEnvString("DB_NAME", "postgres"),
		SSLMode:  getEnvString("DB_SSLMODE", "disable"),
		Timeout:  time.Duration(getEnvInt("DB_TIMEOUT", 30)) * time.Second,
		MaxConns: int32(getEnvInt("DB_MAX_CONNS", 50)),
		Logger:   logger.NewFromEnv(),
	}

	return NewClient(cfg)
}

func NewClient(cfg Config) *Client {
	if cfg.Port == 0 {
		cfg.Port = 5432
	}
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.MaxConns == 0 {
		cfg.MaxConns = 10
	}

	if cfg.Logger == nil {
		cfg.Logger = logger.NewFromEnv()
	}

	return &Client{
		cfg:    cfg,
		logger: cfg.Logger,
	}
}

func (c *Client) Connect(ctx context.Context) error {
	connString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.cfg.Host, c.cfg.Port, c.cfg.User, c.cfg.Password, c.cfg.DBName, c.cfg.SSLMode,
	)

	c.logger.DebugMsg("Starting connection to PostgreSQL... %s", connString)

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		c.logger.Error().WithError(err).Msg("failed to parse connection string")

		return fmt.Errorf("failed to parse connection string: %w", err)
	}

	poolConfig.MaxConns = c.cfg.MaxConns

	ctxWithTimeout, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctxWithTimeout, poolConfig)
	if err != nil {
		c.logger.Error().WithError(err).Msg("failed to create connection pool")
		return fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctxWithTimeout); err != nil {
		c.logger.Error().WithError(err).Msg("failed to ping database")
		return fmt.Errorf("failed to ping database: %w", err)
	}

	c.pool = pool
	c.logger.DebugMsg("Successfully connected to PostgreSQL %s:%d/%s", c.cfg.Host, c.cfg.Port, c.cfg.DBName)

	return nil
}

func (c *Client) Close() {
	if c.pool != nil {
		c.pool.Close()
		c.logger.DebugMsg("Successfully closed connection to PostgreSQL")
	}
}

func (c *Client) Pool() *pgxpool.Pool {
	return c.pool
}

func (c *Client) Ping(ctx context.Context) error {
	if c.pool == nil {
		c.logger.Error().Msg("connection pool is not initialized")

		return fmt.Errorf("connection pool is not initialized")
	}

	err := c.pool.Ping(ctx)
	if err != nil {
		c.logger.Error().WithError(err).Msg("failed to ping database")
		return err
	}

	c.logger.InfoMsg("Successfully pinged database")
	return nil
}

func (c *Client) ExecTx(ctx context.Context, fn func(pgx.Tx) error) error {
	c.logger.DebugMsg("Starting transaction")

	tx, err := c.pool.Begin(ctx)
	if err != nil {
		c.logger.Error().WithError(err).Msg("failed to begin transaction")

		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			c.logger.Error().WithError(err).Msg("failed to rollback transaction")

			return fmt.Errorf("tx failed: %v, rollback failed: %v", err, rbErr)
		}
		c.logger.Error().WithError(err).Msg("transaction failed, rollback successful")
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		c.logger.Error().WithError(err).Msg("failed to commit transaction")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	c.logger.DebugMsg("Transaction completed successfully")
	return nil
}

func getEnvString(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := parseInt(value); err == nil {
			return intValue
		}
	}
	return fallback
}

func parseInt(s string) (int, error) {
	n := 0
	for _, ch := range s {
		ch -= '0'
		if ch > 9 {
			return 0, fmt.Errorf("invalid number: %s", s)
		}
		n = n*10 + int(ch)
	}
	return n, nil
}
