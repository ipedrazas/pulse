package executor

import (
	"context"
	"strings"
	"testing"

	monitorv1 "github.com/ipedrazas/pulse/proto/monitor/v1"
)

func TestRun_DisallowedAction(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-1",
		Action:    "compose_update",
		Target:    "mystack",
	}
	allowed := map[string]bool{} // nothing allowed
	lookup := func(string) string { return "/tmp" }

	result := Run(context.Background(), cmd, allowed, lookup)

	if result.Success {
		t.Error("expected failure for disallowed action")
	}
	if !strings.Contains(result.Output, "not allowed") {
		t.Errorf("expected 'not allowed' in output, got %q", result.Output)
	}
}

func TestRun_UnknownAction(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-2",
		Action:    "nuke_everything",
		Target:    "mystack",
	}
	allowed := map[string]bool{"nuke_everything": true}
	lookup := func(string) string { return "/tmp" }

	result := Run(context.Background(), cmd, allowed, lookup)

	if result.Success {
		t.Error("expected failure for unknown action")
	}
	if !strings.Contains(result.Output, "unknown action") {
		t.Errorf("expected 'unknown action' in output, got %q", result.Output)
	}
}

func TestRun_MissingComposeDir(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-3",
		Action:    "compose_update",
		Target:    "nonexistent",
	}
	allowed := map[string]bool{"compose_update": true}
	lookup := func(string) string { return "" } // not found

	result := Run(context.Background(), cmd, allowed, lookup)

	if result.Success {
		t.Error("expected failure when compose dir not found")
	}
	if !strings.Contains(result.Output, "compose directory not found") {
		t.Errorf("expected 'compose directory not found' in output, got %q", result.Output)
	}
}

func TestTruncate(t *testing.T) {
	short := "hello"
	if truncate(short, 100) != short {
		t.Error("short string should not be truncated")
	}

	long := strings.Repeat("x", 200)
	result := truncate(long, 50)
	if len(result) > 70 { // 50 + suffix
		t.Errorf("expected truncated length ~65, got %d", len(result))
	}
	if !strings.Contains(result, "truncated") {
		t.Error("expected truncation suffix")
	}
}

func TestSplitArgs(t *testing.T) {
	args := splitArgs("up -d")
	if len(args) != 2 || args[0] != "up" || args[1] != "-d" {
		t.Errorf("expected [up -d], got %v", args)
	}
}
