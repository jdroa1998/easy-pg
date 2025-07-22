package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (c *Client) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	c.logger.DebugMsg("Executing QueryRow %s with args %v", query, args)
	return c.pool.QueryRow(ctx, query, args...)
}

func (c *Client) QueryRows(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	c.logger.DebugMsg("Executing QueryRows %s with args %v", query, args)
	return c.pool.Query(ctx, query, args...)
}

func (c *Client) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	c.logger.DebugMsg("Executing Exec %s with args %v", query, args)
	result, err := c.pool.Exec(ctx, query, args...)
	if err != nil {
		c.logger.Error().WithError(err).Msg("Error executing Exec")
		return result, err
	}
	c.logger.DebugMsg("Exec successful, rows affected: %d", result.RowsAffected())
	return result, nil
}

func (c *Client) ExecBatch(ctx context.Context, queries []string, args [][]any) error {
	c.logger.DebugMsg("Starting ExecBatch with %d queries", len(queries))

	batch := &pgx.Batch{}

	for i, query := range queries {
		var queryArgs []any
		if i < len(args) {
			queryArgs = args[i]
		}
		batch.Queue(query, queryArgs...)
	}

	batchResults := c.pool.SendBatch(ctx, batch)
	defer batchResults.Close()

	for i := range batch.Len() {
		if _, err := batchResults.Exec(); err != nil {
			c.logger.Error().WithError(err).Msg("Error in batch query %d", i)

			return fmt.Errorf("error in batch query %d: %w", i, err)
		}
	}

	c.logger.DebugMsg("ExecBatch completed successfully")
	return nil
}
