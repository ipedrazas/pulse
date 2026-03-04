package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
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

// Run executes a command and returns the result. It looks up the compose
// working directory from the containers on this node via the dirLookup function.
//
// dirLookup takes a compose project name and returns the working directory
// where the compose file lives (from com.docker.compose.project.working_dir).
//
// dockerOps provides direct Docker API operations for container-level actions.
func Run(ctx context.Context, cmd *monitorv1.Command, allowedActions map[string]bool, dirLookup func(project string) string, dockerOps docker.DockerOps) Result {
	if !allowedActions[cmd.Action] {
		return Result{Output: fmt.Sprintf("action %q not allowed", cmd.Action)}
	}

	switch cmd.Action {
	case "compose_update":
		return runCompose(ctx, cmd.Target, dirLookup, "pull", "up -d")
	case "compose_restart":
		return runCompose(ctx, cmd.Target, dirLookup, "restart")
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

func runCompose(ctx context.Context, project string, dirLookup func(string) string, subcommands ...string) Result {
	dir := dirLookup(project)
	if dir == "" {
		return Result{Output: fmt.Sprintf("compose directory not found for project %q", project)}
	}

	var combined bytes.Buffer
	start := time.Now()

	for _, sub := range subcommands {
		args := []string{"compose"}
		args = append(args, splitArgs(sub)...)

		c := exec.CommandContext(ctx, "docker", args...)
		c.Dir = dir
		c.Stdout = &combined
		c.Stderr = &combined

		if err := c.Run(); err != nil {
			combined.WriteString(fmt.Sprintf("\n--- error: %v\n", err))
			return Result{
				Output:     truncate(combined.String(), maxOutputBytes),
				DurationMs: time.Since(start).Milliseconds(),
			}
		}
	}

	return Result{
		Success:    true,
		Output:     truncate(combined.String(), maxOutputBytes),
		DurationMs: time.Since(start).Milliseconds(),
	}
}

func splitArgs(s string) []string {
	// Simple space split — sufficient for our known subcommands.
	var args []string
	for _, a := range bytes.Fields([]byte(s)) {
		args = append(args, string(a))
	}
	return args
}

func truncate(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "\n... (truncated)"
}
