package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ipedrazas/pulse/agent/internal/config"
	"github.com/ipedrazas/pulse/agent/internal/debounce"
	"github.com/ipedrazas/pulse/agent/internal/docker"
	"github.com/ipedrazas/pulse/agent/internal/grpcclient"
	monitorv1 "github.com/ipedrazas/pulse/proto/monitor/v1"
)

const pollInterval = 30 * time.Second

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	poller, err := docker.NewPoller()
	if err != nil {
		slog.Error("failed to create docker poller", "error", err)
		os.Exit(1)
	}
	defer poller.Close()

	client, err := grpcclient.New(cfg.ServerAddr, cfg.MonitorToken)
	if err != nil {
		slog.Error("failed to create grpc client", "error", err)
		os.Exit(1)
	}
	defer client.Close()

	tracker := debounce.NewTracker()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	slog.Info("agent starting", "server", cfg.ServerAddr, "node", cfg.NodeName)

	// Startup delay — gives other services time to become ready.
	if cfg.PollDelay > 0 {
		slog.Info("waiting before first poll", "delay", cfg.PollDelay)
		select {
		case <-time.After(cfg.PollDelay):
		case sig := <-quit:
			slog.Info("shutting down during startup delay", "signal", sig.String())
			return
		}
	}

	// Run first poll immediately, then on interval
	tick := time.NewTicker(pollInterval)
	defer tick.Stop()

	// Periodic metadata resync ticker. A nil channel is never selected, so
	// when MetadataResyncInterval is 0 the resync branch is effectively disabled.
	var resyncC <-chan time.Time
	if cfg.MetadataResyncInterval > 0 {
		resyncTicker := time.NewTicker(cfg.MetadataResyncInterval)
		defer resyncTicker.Stop()
		resyncC = resyncTicker.C
	}

	pollOnce(ctx, poller, client, tracker, cfg.NodeName)

	for {
		select {
		case <-tick.C:
			pollOnce(ctx, poller, client, tracker, cfg.NodeName)
		case <-resyncC:
			slog.Info("periodic metadata resync — clearing debounce hashes")
			tracker.Reset()
		case sig := <-quit:
			slog.Info("shutting down", "signal", sig.String())
			return
		}
	}
}

func pollOnce(ctx context.Context, poller *docker.Poller, client *grpcclient.Client, tracker *debounce.Tracker, nodeName string) {
	containers, err := poller.Poll(ctx)
	if err != nil {
		slog.Error("poll failed", "error", err)
		return
	}

	activeIDs := make(map[string]struct{}, len(containers))
	for _, c := range containers {
		activeIDs[c.ID] = struct{}{}

		// Sync metadata first — ensures the containers row exists before
		// inserting a heartbeat (FK constraint).
		if tracker.HasChanged(c) {
			mounts := make([]*monitorv1.MountInfo, len(c.Mounts))
			for i, m := range c.Mounts {
				mounts[i] = &monitorv1.MountInfo{
					Source:      m.Source,
					Destination: m.Destination,
					Mode:        m.Mode,
				}
			}

			req := &monitorv1.SyncMetadataRequest{
				ContainerId: c.ID,
				NodeName:    nodeName,
				Name:        c.Name,
				Image:       c.Image,
				Envs:        c.Envs,
				Mounts:      mounts,
				Labels:      c.Labels,
			}

			if err := client.SyncMetadata(ctx, req); err != nil {
				slog.Error("metadata sync failed", "container_id", c.ID, "error", err)
				continue
			}
			tracker.Commit(c)
			slog.Info("metadata synced", "container_id", c.ID, "name", c.Name)
		}

		// Always send heartbeat
		if err := client.ReportHeartbeat(ctx, c.ID, c.Status, c.UptimeSeconds); err != nil {
			slog.Error("heartbeat failed", "container_id", c.ID, "error", err)
		}
	}

	// Clean up hashes for removed containers
	tracker.Prune(activeIDs)

	slog.Debug("poll complete", "containers", len(containers))
}
