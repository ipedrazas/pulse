package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectWithRetry creates a connection pool, retrying with exponential backoff
// if the database is not yet available. This is useful during container startup
// when the database may still be initializing.
func ConnectWithRetry(ctx context.Context, dbURL string, maxRetries int) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	backoff := time.Second

	for attempt := range maxRetries {
		var err error
		pool, err = pgxpool.New(ctx, dbURL)
		if err != nil {
			slog.Warn("failed to create pool, retrying", "attempt", attempt+1, "backoff", backoff, "error", err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, 30*time.Second)
			continue
		}

		if err = pool.Ping(ctx); err != nil {
			pool.Close()
			slog.Warn("failed to ping database, retrying", "attempt", attempt+1, "backoff", backoff, "error", err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, 30*time.Second)
			continue
		}

		slog.Info("database connected", "attempts", attempt+1)
		return pool, nil
	}

	return nil, fmt.Errorf("failed to connect after %d attempts", maxRetries)
}
