package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	monitorv1 "github.com/ipedrazas/pulse/proto/monitor/v1"
)

const maxOutputBytes = 10 * 1024 // 10 KB

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
func Run(ctx context.Context, cmd *monitorv1.Command, allowedActions map[string]bool, dirLookup func(project string) string) Result {
	if !allowedActions[cmd.Action] {
		return Result{Output: fmt.Sprintf("action %q not allowed", cmd.Action)}
	}

	switch cmd.Action {
	case "compose_update":
		return runCompose(ctx, cmd.Target, dirLookup, "pull", "up -d")
	case "compose_restart":
		return runCompose(ctx, cmd.Target, dirLookup, "restart")
	default:
		return Result{Output: fmt.Sprintf("unknown action %q", cmd.Action)}
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
