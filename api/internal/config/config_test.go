package config

import (
	"log/slog"
	"os"
	"testing"
)

func TestLoad_RequiresDBURL(t *testing.T) {
	os.Unsetenv("DB_URL")
	os.Unsetenv("MONITOR_TOKEN")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing DB_URL")
	}
}

func TestLoad_RequiresMonitorToken(t *testing.T) {
	t.Setenv("DB_URL", "postgres://localhost/test")
	t.Setenv("MONITOR_TOKEN", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing MONITOR_TOKEN")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DB_URL", "postgres://localhost/test")
	t.Setenv("MONITOR_TOKEN", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GRPCPort != "50051" {
		t.Errorf("expected default GRPC_PORT 50051, got %s", cfg.GRPCPort)
	}
	if cfg.HTTPPort != "8080" {
		t.Errorf("expected default HTTP_PORT 8080, got %s", cfg.HTTPPort)
	}
}

func TestLoad_CustomPorts(t *testing.T) {
	t.Setenv("DB_URL", "postgres://localhost/test")
	t.Setenv("MONITOR_TOKEN", "secret")
	t.Setenv("GRPC_PORT", "9090")
	t.Setenv("HTTP_PORT", "3000")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GRPCPort != "9090" {
		t.Errorf("expected GRPC_PORT 9090, got %s", cfg.GRPCPort)
	}
	if cfg.HTTPPort != "3000" {
		t.Errorf("expected HTTP_PORT 3000, got %s", cfg.HTTPPort)
	}
}

func TestLoad_LogLevelDefault(t *testing.T) {
	t.Setenv("DB_URL", "postgres://localhost/test")
	t.Setenv("MONITOR_TOKEN", "secret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("expected default LogLevel Info, got %v", cfg.LogLevel)
	}
}

func TestLoad_LogLevelCustom(t *testing.T) {
	t.Setenv("DB_URL", "postgres://localhost/test")
	t.Setenv("MONITOR_TOKEN", "secret")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("expected LogLevel Debug, got %v", cfg.LogLevel)
	}
}

func TestLoad_LogLevelInvalid(t *testing.T) {
	t.Setenv("DB_URL", "postgres://localhost/test")
	t.Setenv("MONITOR_TOKEN", "secret")
	t.Setenv("LOG_LEVEL", "not-a-level")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid LOG_LEVEL")
	}
}
