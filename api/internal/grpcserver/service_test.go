package grpcserver

import (
	"testing"

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
