package grpcserver

import (
	"testing"

	"github.com/ipedrazas/pulse/api/internal/alerts"
)

func TestNewMonitoringService(t *testing.T) {
	svc := NewMonitoringService(nil, nil, nil, nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewMonitoringService_WithNotifier(t *testing.T) {
	notifier := alerts.NewNotifier("http://example.com/hook", nil)
	svc := NewMonitoringService(nil, nil, nil, notifier)
	if svc.notifier != notifier {
		t.Fatal("expected notifier to be stored")
	}
	if svc.onlineAgents == nil {
		t.Fatal("expected onlineAgents map to be initialized")
	}
}
