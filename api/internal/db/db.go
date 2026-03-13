package db

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// PoolConfig holds tunables for the database connection pool.
type PoolConfig struct {
	MaxConns          int32
	MinConns          int32
	MaxIdleTime       time.Duration
	HealthCheckPeriod time.Duration
}

func NewPool(ctx context.Context, dbURL string, pc *PoolConfig) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}
	if pc != nil {
		if pc.MaxConns > 0 {
			config.MaxConns = pc.MaxConns
		}
		if pc.MinConns > 0 {
			config.MinConns = pc.MinConns
		}
		if pc.MaxIdleTime > 0 {
			config.MaxConnIdleTime = pc.MaxIdleTime
		}
		if pc.HealthCheckPeriod > 0 {
			config.HealthCheckPeriod = pc.HealthCheckPeriod
		}
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return pool, nil
}

// StartHealthCheck runs a background goroutine that periodically pings the
// database and logs connectivity changes. It stops when ctx is cancelled.
// The healthy atomic can be read by callers to check current status.
func StartHealthCheck(ctx context.Context, pool *pgxpool.Pool, interval time.Duration, healthy *atomic.Bool) {
	healthy.Store(true)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		wasHealthy := true
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				err := pool.Ping(pingCtx)
				cancel()
				if err != nil {
					healthy.Store(false)
					if wasHealthy {
						slog.Error("database health check failed", "error", err)
					}
					wasHealthy = false
				} else {
					healthy.Store(true)
					if !wasHealthy {
						slog.Info("database connection recovered")
					}
					wasHealthy = true
				}
			}
		}
	}()
}

func RunMigrations(dbURL string) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("create migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", source, dbURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}
	slog.Info("migrations complete")
	return nil
}
