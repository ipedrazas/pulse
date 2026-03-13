package db

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStartHealthCheck_SetsUnhealthyWhenPoolInvalid(t *testing.T) {
	// Create a pool pointing at a non-existent database.
	// ParseConfig succeeds even with a bad host; the pool only fails on use.
	cfg, err := pgxpool.ParseConfig("postgres://invalid:invalid@127.0.0.1:1/noexist?connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	// Minimise pool size and connect timeout so the test is fast.
	cfg.MaxConns = 1

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Skipf("cannot create pool config: %v", err)
	}
	defer pool.Close()

	var healthy atomic.Bool
	StartHealthCheck(ctx, pool, 50*time.Millisecond, &healthy)

	// healthy starts as true
	if !healthy.Load() {
		t.Fatal("expected healthy=true initially")
	}

	// Wait for the first tick to fail
	time.Sleep(200 * time.Millisecond)

	if healthy.Load() {
		t.Fatal("expected healthy=false after failed ping")
	}
}

func TestStartHealthCheck_StopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var healthy atomic.Bool
	// Pass nil pool — we cancel before the first tick, so it's never used.
	// We need a valid pool to avoid nil panics. Create one pointing nowhere.
	cfg, _ := pgxpool.ParseConfig("postgres://x:x@127.0.0.1:1/x?connect_timeout=1")
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Skipf("cannot create pool: %v", err)
	}
	defer pool.Close()

	StartHealthCheck(ctx, pool, time.Hour, &healthy)

	// Cancel immediately — the goroutine should exit without ticking.
	cancel()
	time.Sleep(50 * time.Millisecond)

	// healthy should still be true (never ticked)
	if !healthy.Load() {
		t.Fatal("expected healthy=true when cancelled before first tick")
	}
}
