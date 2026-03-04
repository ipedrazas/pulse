package executor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ipedrazas/pulse/agent/internal/docker"
	monitorv1 "github.com/ipedrazas/pulse/proto/monitor/v1"
)

const maxOutputBytes = 10 * 1024       // 10 KB
const maxLargeOutputBytes = 128 * 1024 // 128 KB for logs/inspect

// Result holds the outcome of a command execution.
type Result struct {
	Success    bool
	Output     string
	DurationMs int64
}

// Run executes a command and returns the result.
// dockerOps provides Docker API operations for all container and compose actions.
func Run(ctx context.Context, cmd *monitorv1.Command, allowedActions map[string]bool, dockerOps docker.DockerOps) Result {
	if !allowedActions[cmd.Action] {
		return Result{Output: fmt.Sprintf("action %q not allowed", cmd.Action)}
	}

	slog.Debug("executor.Run", "action", cmd.Action, "target", cmd.Target, "params", cmd.Params)

	switch cmd.Action {
	case "compose_update":
		return runComposeUpdate(ctx, cmd.Target, dockerOps)
	case "compose_restart":
		return runComposeRestart(ctx, cmd.Target, dockerOps)
	case "container_stop":
		return runContainerAction(ctx, cmd.Target, func() error {
			return dockerOps.StopContainer(ctx, cmd.Target)
		})
	case "container_start":
		return runContainerAction(ctx, cmd.Target, func() error {
			return dockerOps.StartContainer(ctx, cmd.Target)
		})
	case "container_restart":
		return runContainerAction(ctx, cmd.Target, func() error {
			return dockerOps.RestartContainer(ctx, cmd.Target)
		})
	case "container_logs":
		return runContainerQuery(ctx, cmd.Target, func() (string, error) {
			tail := cmd.Params["tail"]
			if tail == "" {
				tail = "100"
			}
			return dockerOps.ContainerLogs(ctx, cmd.Target, tail)
		})
	case "container_inspect":
		return runContainerQuery(ctx, cmd.Target, func() (string, error) {
			return dockerOps.InspectContainer(ctx, cmd.Target)
		})
	default:
		return Result{Output: fmt.Sprintf("unknown action %q", cmd.Action)}
	}
}

// runContainerAction executes a side-effecting container operation (stop/start/restart).
func runContainerAction(ctx context.Context, containerID string, fn func() error) Result {
	start := time.Now()
	if err := fn(); err != nil {
		return Result{
			Output:     fmt.Sprintf("container %s: %v", containerID, err),
			DurationMs: time.Since(start).Milliseconds(),
		}
	}
	return Result{
		Success:    true,
		Output:     fmt.Sprintf("container %s: ok", containerID),
		DurationMs: time.Since(start).Milliseconds(),
	}
}

// runContainerQuery executes a read-only container operation (logs/inspect).
func runContainerQuery(_ context.Context, containerID string, fn func() (string, error)) Result {
	start := time.Now()
	output, err := fn()
	if err != nil {
		return Result{
			Output:     fmt.Sprintf("container %s: %v", containerID, err),
			DurationMs: time.Since(start).Milliseconds(),
		}
	}
	return Result{
		Success:    true,
		Output:     truncate(output, maxLargeOutputBytes),
		DurationMs: time.Since(start).Milliseconds(),
	}
}

// runComposeUpdate pulls images and recreates all containers in a compose project.
// Best-effort: continues on partial failures and reports all results.
func runComposeUpdate(ctx context.Context, project string, ops docker.DockerOps) Result {
	start := time.Now()

	containers, err := ops.ListProjectContainers(ctx, project)
	if err != nil {
		return Result{
			Output:     fmt.Sprintf("listing containers for project %q: %v", project, err),
			DurationMs: time.Since(start).Milliseconds(),
		}
	}
	if len(containers) == 0 {
		return Result{
			Success:    true,
			Output:     fmt.Sprintf("no containers found for project %q", project),
			DurationMs: time.Since(start).Milliseconds(),
		}
	}

	var msgs []string
	anyErr := false

	// Pull images (deduplicated).
	pulled := make(map[string]bool)
	for _, c := range containers {
		if pulled[c.Image] {
			continue
		}
		pulled[c.Image] = true
		slog.Info("pulling image", "project", project, "image", c.Image)
		if err := ops.PullImage(ctx, c.Image); err != nil {
			anyErr = true
			msgs = append(msgs, fmt.Sprintf("pull %s: %v", c.Image, err))
		} else {
			msgs = append(msgs, fmt.Sprintf("pull %s: ok", c.Image))
		}
	}

	// Recreate each container.
	for _, c := range containers {
		slog.Info("recreating container", "project", project, "container", c.Name)
		if err := ops.RecreateContainer(ctx, c.ID); err != nil {
			anyErr = true
			msgs = append(msgs, fmt.Sprintf("recreate %s: %v", c.Name, err))
		} else {
			msgs = append(msgs, fmt.Sprintf("recreate %s: ok", c.Name))
		}
	}

	return Result{
		Success:    !anyErr,
		Output:     truncate(strings.Join(msgs, "\n"), maxOutputBytes),
		DurationMs: time.Since(start).Milliseconds(),
	}
}

// runComposeRestart restarts all containers in a compose project.
func runComposeRestart(ctx context.Context, project string, ops docker.DockerOps) Result {
	start := time.Now()

	containers, err := ops.ListProjectContainers(ctx, project)
	if err != nil {
		return Result{
			Output:     fmt.Sprintf("listing containers for project %q: %v", project, err),
			DurationMs: time.Since(start).Milliseconds(),
		}
	}
	if len(containers) == 0 {
		return Result{
			Success:    true,
			Output:     fmt.Sprintf("no containers found for project %q", project),
			DurationMs: time.Since(start).Milliseconds(),
		}
	}

	var msgs []string
	anyErr := false

	for _, c := range containers {
		slog.Info("restarting container", "project", project, "container", c.Name)
		if err := ops.RestartContainer(ctx, c.ID); err != nil {
			anyErr = true
			msgs = append(msgs, fmt.Sprintf("restart %s: %v", c.Name, err))
		} else {
			msgs = append(msgs, fmt.Sprintf("restart %s: ok", c.Name))
		}
	}

	return Result{
		Success:    !anyErr,
		Output:     truncate(strings.Join(msgs, "\n"), maxOutputBytes),
		DurationMs: time.Since(start).Milliseconds(),
	}
}

func truncate(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "\n... (truncated)"
}
