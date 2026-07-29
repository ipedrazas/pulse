package cmd

import (
	"bytes"
	"context"
	"testing"
)

// --- stats: flag validation ---

func TestStatsCmd_InvalidSince(t *testing.T) {
	_, _, err := executeCmd("stats", "--since", "not-a-duration")
	if err == nil {
		t.Fatal("expected error for invalid --since duration")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("invalid --since duration")) {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- diff: flag validation ---

func TestDiffCmd_MissingSince(t *testing.T) {
	_, _, err := executeCmd("diff")
	if err == nil {
		t.Fatal("expected error for missing --since")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("--since is required")) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDiffCmd_InvalidSince(t *testing.T) {
	_, _, err := executeCmd("diff", "--since", "not-a-duration")
	if err == nil {
		t.Fatal("expected error for invalid --since duration")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("invalid --since duration")) {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- resolveNode ---

func TestResolveNode_FlagValueWins(t *testing.T) {
	got, err := resolveNode(context.Background(), nil, "worker-1")
	if err != nil {
		t.Fatalf("resolveNode returned error: %v", err)
	}
	if got != "worker-1" {
		t.Errorf("resolveNode = %q, want worker-1", got)
	}
}

func TestResolveNode_DefaultNode(t *testing.T) {
	defaultNode = "default-node"
	defer func() { defaultNode = "" }()

	got, err := resolveNode(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("resolveNode returned error: %v", err)
	}
	if got != "default-node" {
		t.Errorf("resolveNode = %q, want default-node", got)
	}
}
