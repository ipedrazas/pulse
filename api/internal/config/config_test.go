package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// Clear any env that might be set
	for _, k := range []string{"PULSE_DB_URL", "PULSE_GRPC_ADDR", "PULSE_REST_ADDR", "PULSE_TLS_CERT", "PULSE_TLS_KEY", "PULSE_TLS_CA", "PULSE_WEBHOOK_URL", "PULSE_STALE_THRESHOLD"} {
		t.Setenv(k, "")
	}
	// envOr treats "" as unset, but os.Getenv returns "". We need to actually unset.
	// t.Setenv sets the var; to get defaults we set to empty string which envOr treats as fallback.

	cfg := Load()
	if cfg.DBURL != "postgres://pulse:pulse@localhost:5432/pulse?sslmode=disable" {
		t.Errorf("DBURL = %q, want default", cfg.DBURL)
	}
	if cfg.GRPCAddr != ":9090" {
		t.Errorf("GRPCAddr = %q, want :9090", cfg.GRPCAddr)
	}
	if cfg.RESTAddr != ":8080" {
		t.Errorf("RESTAddr = %q, want :8080", cfg.RESTAddr)
	}
	if cfg.StaleThreshold != 5*time.Minute {
		t.Errorf("StaleThreshold = %v, want 5m", cfg.StaleThreshold)
	}
}

func TestLoadCustomValues(t *testing.T) {
	t.Setenv("PULSE_DB_URL", "postgres://custom:custom@db:5432/mydb")
	t.Setenv("PULSE_GRPC_ADDR", ":7070")
	t.Setenv("PULSE_REST_ADDR", ":3000")
	t.Setenv("PULSE_TLS_CERT", "/cert.pem")
	t.Setenv("PULSE_TLS_KEY", "/key.pem")
	t.Setenv("PULSE_TLS_CA", "/ca.pem")
	t.Setenv("PULSE_WEBHOOK_URL", "https://hooks.example.com/test")

	cfg := Load()
	if cfg.DBURL != "postgres://custom:custom@db:5432/mydb" {
		t.Errorf("DBURL = %q", cfg.DBURL)
	}
	if cfg.GRPCAddr != ":7070" {
		t.Errorf("GRPCAddr = %q", cfg.GRPCAddr)
	}
	if cfg.RESTAddr != ":3000" {
		t.Errorf("RESTAddr = %q", cfg.RESTAddr)
	}
	if cfg.TLSCert != "/cert.pem" {
		t.Errorf("TLSCert = %q", cfg.TLSCert)
	}
	if cfg.TLSKey != "/key.pem" {
		t.Errorf("TLSKey = %q", cfg.TLSKey)
	}
	if cfg.TLSCA != "/ca.pem" {
		t.Errorf("TLSCA = %q", cfg.TLSCA)
	}
	if cfg.WebhookURL != "https://hooks.example.com/test" {
		t.Errorf("WebhookURL = %q", cfg.WebhookURL)
	}
}

func TestEnvOr_WithValue(t *testing.T) {
	t.Setenv("TEST_KEY", "custom")
	got := envOr("TEST_KEY", "default")
	if got != "custom" {
		t.Errorf("envOr = %q, want custom", got)
	}
}

func TestEnvOr_WithFallback(t *testing.T) {
	t.Setenv("TEST_KEY", "")
	got := envOr("TEST_KEY", "fallback")
	if got != "fallback" {
		t.Errorf("envOr = %q, want fallback", got)
	}
}

func TestParseDuration_Valid(t *testing.T) {
	t.Setenv("TEST_DUR", "10m")
	got := parseDuration("TEST_DUR", time.Hour)
	if got != 10*time.Minute {
		t.Errorf("parseDuration = %v, want 10m", got)
	}
}

func TestParseDuration_Invalid(t *testing.T) {
	t.Setenv("TEST_DUR", "notaduration")
	got := parseDuration("TEST_DUR", 5*time.Minute)
	if got != 5*time.Minute {
		t.Errorf("parseDuration = %v, want 5m (fallback)", got)
	}
}

func TestParseDuration_Unset(t *testing.T) {
	t.Setenv("TEST_DUR", "")
	got := parseDuration("TEST_DUR", 3*time.Minute)
	if got != 3*time.Minute {
		t.Errorf("parseDuration = %v, want 3m (fallback)", got)
	}
}
