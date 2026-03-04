package maintenance

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ipedrazas/pulse/api/internal/alerts"
	"github.com/ipedrazas/pulse/api/internal/repository"
)

// --- minimal mocks ---

type mockContainerRepo struct {
	sweepCount int64
	sweepErr   error
}

func (m *mockContainerRepo) SweepStale(_ context.Context, _ time.Duration) (int64, error) {
	return m.sweepCount, m.sweepErr
}

func (m *mockContainerRepo) ListContainers(_ context.Context, _, _ int) ([]repository.ContainerStatus, error) {
	return nil, nil
}
func (m *mockContainerRepo) GetContainer(_ context.Context, _ string) (repository.ContainerStatus, error) {
	return repository.ContainerStatus{}, nil
}
func (m *mockContainerRepo) ListContainersByNode(_ context.Context, _ string, _, _ int) ([]repository.ContainerStatus, error) {
	return nil, nil
}
func (m *mockContainerRepo) ListContainersByNodeForStacks(_ context.Context, _ string, _, _ int) ([]repository.ContainerStatus, error) {
	return nil, nil
}
func (m *mockContainerRepo) GetComposeDir(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (m *mockContainerRepo) UpsertMetadata(_ context.Context, _ repository.ContainerMetadata) error {
	return nil
}
func (m *mockContainerRepo) InsertHeartbeat(_ context.Context, _, _ string, _ int64) error {
	return nil
}
func (m *mockContainerRepo) GetPreviousStatus(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (m *mockContainerRepo) GetContainerInfoForRemoval(_ context.Context, _ string, _ []string) ([]repository.ContainerInfo, error) {
	return nil, nil
}
func (m *mockContainerRepo) GetContainerMetadataForEvent(_ context.Context, _ string) (repository.ContainerInfo, string, error) {
	return repository.ContainerInfo{}, "", nil
}
func (m *mockContainerRepo) MarkRemoved(_ context.Context, _ string, _ []string) (int64, error) {
	return 0, nil
}

type mockAgentRepo struct {
	nodes []string
	err   error
}

func (m *mockAgentRepo) ListOnlineAgents(_ context.Context) ([]string, error) {
	return m.nodes, m.err
}
func (m *mockAgentRepo) UpsertAgent(_ context.Context, _, _ string) error { return nil }
func (m *mockAgentRepo) ListAgents(_ context.Context) ([]repository.AgentStatus, error) {
	return nil, nil
}

// --- tests ---

func TestNewScheduler(t *testing.T) {
	s := NewScheduler(nil, nil, nil)
	if s == nil {
		t.Fatal("expected non-nil scheduler")
	}
	if s.onlineAgents == nil {
		t.Fatal("expected onlineAgents map to be initialized")
	}
}

func TestSweepStaleContainers_Success(t *testing.T) {
	repo := &mockContainerRepo{sweepCount: 5}
	s := NewScheduler(repo, nil, nil)

	count, err := s.SweepStaleContainers(context.Background(), 48*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 swept, got %d", count)
	}
}

func TestSweepStaleContainers_Error(t *testing.T) {
	repo := &mockContainerRepo{sweepErr: errors.New("db error")}
	s := NewScheduler(repo, nil, nil)

	_, err := s.SweepStaleContainers(context.Background(), 48*time.Hour)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSweepStaleContainers_ZeroSwept(t *testing.T) {
	repo := &mockContainerRepo{sweepCount: 0}
	s := NewScheduler(repo, nil, nil)

	count, err := s.SweepStaleContainers(context.Background(), 48*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 swept, got %d", count)
	}
}

func TestCheckAgentStatus_ListError(t *testing.T) {
	agents := &mockAgentRepo{err: errors.New("db error")}
	s := NewScheduler(nil, agents, nil)

	// Should not panic; error is logged internally.
	s.CheckAgentStatus(context.Background())
}

func TestCheckAgentStatus_NoNotifier(t *testing.T) {
	agents := &mockAgentRepo{nodes: []string{"node-a"}}
	s := NewScheduler(nil, agents, nil)

	s.CheckAgentStatus(context.Background())

	// State should still be updated even without a notifier.
	s.onlineMu.Lock()
	defer s.onlineMu.Unlock()
	if !s.onlineAgents["node-a"] {
		t.Error("expected node-a to be tracked as online")
	}
}

func TestCheckAgentStatus_DetectsNewlyOnline(t *testing.T) {
	var mu sync.Mutex
	var received []alerts.Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var evt alerts.Event
		json.NewDecoder(r.Body).Decode(&evt)
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	notifier := alerts.NewNotifier(srv.URL, nil)
	agents := &mockAgentRepo{nodes: []string{"node-a", "node-b"}}
	s := NewScheduler(nil, agents, notifier)

	s.CheckAgentStatus(context.Background())

	// Wait briefly for async webhook dispatch.
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 2 {
		t.Fatalf("expected 2 online events, got %d", len(received))
	}

	eventTypes := map[string]bool{}
	nodeNames := map[string]bool{}
	for _, evt := range received {
		eventTypes[evt.EventType] = true
		nodeNames[evt.NodeName] = true
	}

	if !eventTypes[alerts.EventAgentOnline] {
		t.Error("expected agent_online events")
	}
	if !nodeNames["node-a"] || !nodeNames["node-b"] {
		t.Error("expected events for node-a and node-b")
	}
}

func TestCheckAgentStatus_DetectsNewlyOffline(t *testing.T) {
	var mu sync.Mutex
	var received []alerts.Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var evt alerts.Event
		json.NewDecoder(r.Body).Decode(&evt)
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	notifier := alerts.NewNotifier(srv.URL, nil)
	agents := &mockAgentRepo{}
	s := NewScheduler(nil, agents, notifier)

	// Seed initial state: node-a and node-b are online.
	s.onlineMu.Lock()
	s.onlineAgents = map[string]bool{"node-a": true, "node-b": true}
	s.onlineMu.Unlock()

	// Now no agents are online.
	agents.nodes = nil
	s.CheckAgentStatus(context.Background())

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 2 {
		t.Fatalf("expected 2 offline events, got %d", len(received))
	}

	for _, evt := range received {
		if evt.EventType != alerts.EventAgentOffline {
			t.Errorf("expected agent_offline, got %s", evt.EventType)
		}
	}
}

func TestCheckAgentStatus_MixedTransitions(t *testing.T) {
	var mu sync.Mutex
	var received []alerts.Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var evt alerts.Event
		json.NewDecoder(r.Body).Decode(&evt)
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	notifier := alerts.NewNotifier(srv.URL, nil)
	agents := &mockAgentRepo{}
	s := NewScheduler(nil, agents, notifier)

	// Seed: node-a is online.
	s.onlineMu.Lock()
	s.onlineAgents = map[string]bool{"node-a": true}
	s.onlineMu.Unlock()

	// Now node-a goes offline and node-b comes online.
	agents.nodes = []string{"node-b"}
	s.CheckAgentStatus(context.Background())

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}

	gotOnline := false
	gotOffline := false
	for _, evt := range received {
		switch {
		case evt.EventType == alerts.EventAgentOnline && evt.NodeName == "node-b":
			gotOnline = true
		case evt.EventType == alerts.EventAgentOffline && evt.NodeName == "node-a":
			gotOffline = true
		}
	}

	if !gotOnline {
		t.Error("expected agent_online event for node-b")
	}
	if !gotOffline {
		t.Error("expected agent_offline event for node-a")
	}
}

func TestCheckAgentStatus_NoChangeNoEvents(t *testing.T) {
	var mu sync.Mutex
	var received []alerts.Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var evt alerts.Event
		json.NewDecoder(r.Body).Decode(&evt)
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	notifier := alerts.NewNotifier(srv.URL, nil)
	agents := &mockAgentRepo{nodes: []string{"node-a"}}
	s := NewScheduler(nil, agents, notifier)

	// First call: node-a appears (online event).
	s.CheckAgentStatus(context.Background())
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	received = nil // Reset.
	mu.Unlock()

	// Second call: node-a still online — no new events.
	s.CheckAgentStatus(context.Background())
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 0 {
		t.Errorf("expected 0 events on stable state, got %d", len(received))
	}
}

func TestCheckAgentStatus_EmptyToEmpty(t *testing.T) {
	var mu sync.Mutex
	var received []alerts.Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var evt alerts.Event
		json.NewDecoder(r.Body).Decode(&evt)
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	notifier := alerts.NewNotifier(srv.URL, nil)
	agents := &mockAgentRepo{nodes: nil}
	s := NewScheduler(nil, agents, notifier)

	s.CheckAgentStatus(context.Background())
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) != 0 {
		t.Errorf("expected 0 events for empty-to-empty, got %d", len(received))
	}
}
