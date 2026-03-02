package db

import (
	"context"
	"testing"
	"time"
)

func TestConnectWithRetry_InvalidURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := ConnectWithRetry(ctx, "postgres://invalid:5432/nonexistent?connect_timeout=1", 2)
	if err == nil {
		t.Fatal("expected error for invalid connection")
	}
}

func TestConnectWithRetry_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := ConnectWithRetry(ctx, "postgres://localhost:5432/test", 3)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
