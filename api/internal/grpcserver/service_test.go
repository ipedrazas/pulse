package grpcserver

import (
	"testing"

	"github.com/ipedrazas/pulse/api/internal/alerts"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNewMonitoringService(t *testing.T) {
	svc := NewMonitoringService(nil, nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.pool != nil {
		t.Fatal("expected nil pool when passed nil")
	}
}

func TestNewMonitoringService_WithPool(t *testing.T) {
	pool := &pgxpool.Pool{}
	svc := NewMonitoringService(pool, nil)
	if svc.pool != pool {
		t.Fatal("expected pool to be stored")
	}
}

func TestNewMonitoringService_WithNotifier(t *testing.T) {
	notifier := alerts.NewNotifier("http://example.com/hook", nil)
	svc := NewMonitoringService(nil, notifier)
	if svc.notifier != notifier {
		t.Fatal("expected notifier to be stored")
	}
	if svc.onlineAgents == nil {
		t.Fatal("expected onlineAgents map to be initialized")
	}
}
