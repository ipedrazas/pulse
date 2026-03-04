package maintenance

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ipedrazas/pulse/api/internal/alerts"
	"github.com/ipedrazas/pulse/api/internal/repository"
)

// Scheduler runs background maintenance tasks: stale container sweeping
// and agent online/offline detection.
type Scheduler struct {
	containers repository.ContainerRepository
	agents     repository.AgentRepository
	notifier   *alerts.Notifier

	onlineMu     sync.Mutex
	onlineAgents map[string]bool
}

// NewScheduler creates a new Scheduler.
func NewScheduler(containers repository.ContainerRepository, agents repository.AgentRepository, notifier *alerts.Notifier) *Scheduler {
	return &Scheduler{
		containers:   containers,
		agents:       agents,
		notifier:     notifier,
		onlineAgents: make(map[string]bool),
	}
}

// SweepStaleContainers marks containers as removed if they haven't had a
// heartbeat in the given duration. Returns the number of containers swept.
func (s *Scheduler) SweepStaleContainers(ctx context.Context, maxAge time.Duration) (int64, error) {
	return s.containers.SweepStale(ctx, maxAge)
}

// CheckAgentStatus queries the agents table and fires agent_online/agent_offline
// webhook events for nodes that have changed state since the last check.
func (s *Scheduler) CheckAgentStatus(ctx context.Context) {
	nodes, err := s.agents.ListOnlineAgents(ctx)
	if err != nil {
		slog.Error("failed to query agent status", "error", err)
		return
	}

	currentOnline := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		currentOnline[n] = true
	}

	s.onlineMu.Lock()
	prev := s.onlineAgents
	s.onlineAgents = currentOnline
	s.onlineMu.Unlock()

	if s.notifier == nil {
		return
	}

	// Detect newly online agents.
	for node := range currentOnline {
		if !prev[node] {
			s.notifier.Send(alerts.Event{
				EventType: alerts.EventAgentOnline,
				NodeName:  node,
			})
		}
	}

	// Detect newly offline agents.
	for node := range prev {
		if !currentOnline[node] {
			s.notifier.Send(alerts.Event{
				EventType: alerts.EventAgentOffline,
				NodeName:  node,
			})
		}
	}
}
