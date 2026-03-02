package main

import (
	"context"
	"testing"
	"time"

	grpcserver "github.com/ipedrazas/pulse/api/internal/grpcserver"

	"github.com/ipedrazas/pulse/api/internal/config"
)

func TestSetupGRPC_NoTLS(t *testing.T) {
	cfg := &config.Config{
		MonitorToken: "test-token",
	}

	srv, svc, err := setupGRPC(cfg, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	srv.Stop()
}

func TestSetupGRPC_InvalidTLS(t *testing.T) {
	cfg := &config.Config{
		MonitorToken: "test-token",
		TLSCertFile:  "/nonexistent/cert.pem",
		TLSKeyFile:   "/nonexistent/key.pem",
	}

	_, _, err := setupGRPC(cfg, nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid TLS files")
	}
}

func TestStartSweeper_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	svc := grpcserver.NewMonitoringService(nil, nil)

	done := make(chan struct{})
	go func() {
		startSweeper(ctx, svc)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// sweeper returned after context cancellation
	case <-time.After(2 * time.Second):
		t.Fatal("startSweeper did not return after context cancellation")
	}
}
