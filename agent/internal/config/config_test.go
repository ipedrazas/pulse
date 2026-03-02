package config

import (
	"testing"
	"time"
)

func TestLoad_RequiresServerAddr(t *testing.T) {
	t.Setenv("SERVER_ADDR", "")
	t.Setenv("MONITOR_TOKEN", "secret")
	t.Setenv("PROXMOX_NODE_NAME", "node1")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing SERVER_ADDR")
	}
}

func TestLoad_RequiresMonitorToken(t *testing.T) {
	t.Setenv("SERVER_ADDR", "localhost:50051")
	t.Setenv("MONITOR_TOKEN", "")
	t.Setenv("PROXMOX_NODE_NAME", "node1")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing MONITOR_TOKEN")
	}
}

func TestLoad_RequiresNodeName(t *testing.T) {
	t.Setenv("SERVER_ADDR", "localhost:50051")
	t.Setenv("MONITOR_TOKEN", "secret")
	t.Setenv("PROXMOX_NODE_NAME", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing PROXMOX_NODE_NAME")
	}
}

func TestLoad_Success(t *testing.T) {
	t.Setenv("SERVER_ADDR", "api:50051")
	t.Setenv("MONITOR_TOKEN", "secret")
	t.Setenv("PROXMOX_NODE_NAME", "pve1")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ServerAddr != "api:50051" {
		t.Errorf("expected ServerAddr 'api:50051', got %s", cfg.ServerAddr)
	}
	if cfg.MonitorToken != "secret" {
		t.Errorf("expected MonitorToken 'secret', got %s", cfg.MonitorToken)
	}
	if cfg.NodeName != "pve1" {
		t.Errorf("expected NodeName 'pve1', got %s", cfg.NodeName)
	}
}

func TestGetEnv_Fallback(t *testing.T) {
	t.Setenv("TEST_GETENV_KEY", "")
	if got := getEnv("TEST_GETENV_NONEXISTENT", "default"); got != "default" {
		t.Errorf("expected fallback 'default', got %s", got)
	}
}

func TestGetEnv_Value(t *testing.T) {
	t.Setenv("TEST_GETENV_KEY", "myval")
	if got := getEnv("TEST_GETENV_KEY", "default"); got != "myval" {
		t.Errorf("expected 'myval', got %s", got)
	}
}

// --- PollDelay tests ---

func TestLoad_PollDelayDefault(t *testing.T) {
	setRequiredEnvs(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PollDelay != 0 {
		t.Errorf("expected PollDelay 0, got %v", cfg.PollDelay)
	}
}

func TestLoad_PollDelayCustom(t *testing.T) {
	setRequiredEnvs(t)
	t.Setenv("POLL_DELAY_SECONDS", "10")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PollDelay != 10*time.Second {
		t.Errorf("expected PollDelay 10s, got %v", cfg.PollDelay)
	}
}

func TestLoad_PollDelayInvalid(t *testing.T) {
	setRequiredEnvs(t)
	t.Setenv("POLL_DELAY_SECONDS", "abc")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid POLL_DELAY_SECONDS")
	}
}

// --- MetadataResyncInterval tests ---

func TestLoad_MetadataResyncDefault(t *testing.T) {
	setRequiredEnvs(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MetadataResyncInterval != 3600*time.Second {
		t.Errorf("expected MetadataResyncInterval 3600s, got %v", cfg.MetadataResyncInterval)
	}
}

func TestLoad_MetadataResyncCustom(t *testing.T) {
	setRequiredEnvs(t)
	t.Setenv("METADATA_RESYNC_SECONDS", "120")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MetadataResyncInterval != 120*time.Second {
		t.Errorf("expected MetadataResyncInterval 120s, got %v", cfg.MetadataResyncInterval)
	}
}

func TestLoad_MetadataResyncInvalid(t *testing.T) {
	setRequiredEnvs(t)
	t.Setenv("METADATA_RESYNC_SECONDS", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid METADATA_RESYNC_SECONDS")
	}
}

func setRequiredEnvs(t *testing.T) {
	t.Helper()
	t.Setenv("SERVER_ADDR", "api:50051")
	t.Setenv("MONITOR_TOKEN", "secret")
	t.Setenv("PROXMOX_NODE_NAME", "pve1")
}
