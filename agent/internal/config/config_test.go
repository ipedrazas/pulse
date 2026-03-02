package config

import (
	"testing"
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
