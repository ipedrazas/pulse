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
	"github.com/ipedrazas/pulse/agent/internal/executor"
	"github.com/ipedrazas/pulse/agent/internal/grpcclient"
	"github.com/ipedrazas/pulse/agent/internal/redact"
	monitorv1 "github.com/ipedrazas/pulse/proto/monitor/v1"
)

const pollInterval = 30 * time.Second

// version is set via -ldflags at build time, e.g.:
//
//	go build -ldflags "-X main.version=1.0.0" ./agent/cmd/agent
var version = "dev"

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

	client, err := grpcclient.New(cfg.ServerAddr, cfg.MonitorToken, cfg.TLSCAFile)
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

	pollOnce(ctx, poller, client, tracker, cfg.NodeName, cfg.RedactPatterns)
	executeCommands(ctx, client, poller, cfg)

	for {
		select {
		case <-tick.C:
			pollOnce(ctx, poller, client, tracker, cfg.NodeName, cfg.RedactPatterns)
			executeCommands(ctx, client, poller, cfg)
		case <-resyncC:
			slog.Info("periodic metadata resync — clearing debounce hashes")
			tracker.Reset()
		case sig := <-quit:
			slog.Info("shutting down", "signal", sig.String())
			return
		}
	}
}

func pollOnce(ctx context.Context, poller *docker.Poller, client *grpcclient.Client, tracker *debounce.Tracker, nodeName string, redactPatterns []string) {
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
				Envs:        redact.Envs(c.Envs, redactPatterns),
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

	// Clean up hashes for removed containers and report them to the hub
	removedIDs := tracker.Prune(activeIDs)
	if len(removedIDs) > 0 {
		slog.Info("containers removed from node", "count", len(removedIDs), "ids", removedIDs)
		if err := client.ReportRemovedContainers(ctx, nodeName, removedIDs); err != nil {
			slog.Error("failed to report removed containers", "error", err)
		}
	}

	// Send agent-level heartbeat after container heartbeats.
	if err := client.AgentHeartbeat(ctx, nodeName, version); err != nil {
		slog.Error("agent heartbeat failed", "error", err)
	}

	slog.Debug("poll complete", "containers", len(containers))
}

func executeCommands(ctx context.Context, client *grpcclient.Client, poller *docker.Poller, cfg *config.Config) {
	commands, err := client.GetPendingCommands(ctx, cfg.NodeName)
	if err != nil {
		slog.Error("failed to fetch pending commands", "error", err)
		return
	}
	if len(commands) == 0 {
		return
	}

	// Build a lookup from compose project → working directory using
	// the labels already available from local Docker containers.
	dirLookup := buildComposeDirLookup(ctx, poller)

	for _, cmd := range commands {
		slog.Info("executing command", "command_id", cmd.CommandId, "action", cmd.Action, "target", cmd.Target)

		result := executor.Run(ctx, cmd, cfg.AllowedActions, dirLookup)

		if err := client.ReportCommandResult(ctx, &monitorv1.ReportCommandResultRequest{
			CommandId:  cmd.CommandId,
			NodeName:   cfg.NodeName,
			Success:    result.Success,
			Output:     result.Output,
			DurationMs: result.DurationMs,
		}); err != nil {
			slog.Error("failed to report command result", "command_id", cmd.CommandId, "error", err)
		}

		level := slog.LevelInfo
		if !result.Success {
			level = slog.LevelError
		}
		slog.Log(ctx, level, "command completed", "command_id", cmd.CommandId, "success", result.Success, "duration_ms", result.DurationMs)
	}
}

// buildComposeDirLookup polls local Docker containers and builds a map
// from compose project name to the working directory where the compose file lives.
func buildComposeDirLookup(ctx context.Context, poller *docker.Poller) func(string) string {
	containers, err := poller.Poll(ctx)
	if err != nil {
		slog.Error("failed to poll containers for dir lookup", "error", err)
		return func(string) string { return "" }
	}

	dirs := make(map[string]string)
	for _, c := range containers {
		project := c.Labels["com.docker.compose.project"]
		dir := c.Labels["com.docker.compose.project.working_dir"]
		if project != "" && dir != "" {
			dirs[project] = dir
		}
	}

	return func(project string) string {
		return dirs[project]
	}
}
