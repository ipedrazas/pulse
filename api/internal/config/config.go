package config

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DBURL          string
	GRPCAddr       string
	RESTAddr       string
	TLSCert        string
	TLSKey         string
	TLSCA          string
	WebhookURL     string
	StaleThreshold time.Duration
	LogLevel       slog.Level

	// Database pool
	DBPoolMaxConns         int32
	DBPoolMinConns         int32
	DBPoolMaxIdleTime      time.Duration
	DBPoolHealthCheckPeriod time.Duration
}

func Load() Config {
	return Config{
		DBURL:          envOr("PULSE_DB_URL", "postgres://pulse:pulse@localhost:5432/pulse?sslmode=disable"),
		GRPCAddr:       envOr("PULSE_GRPC_ADDR", ":9090"),
		RESTAddr:       envOr("PULSE_REST_ADDR", ":8080"),
		TLSCert:        os.Getenv("PULSE_TLS_CERT"),
		TLSKey:         os.Getenv("PULSE_TLS_KEY"),
		TLSCA:          os.Getenv("PULSE_TLS_CA"),
		WebhookURL:     os.Getenv("PULSE_WEBHOOK_URL"),
		StaleThreshold: parseDuration("PULSE_STALE_THRESHOLD", 5*time.Minute),
		LogLevel:       parseLogLevel(envOr("PULSE_LOG_LEVEL", "info")),

		DBPoolMaxConns:          parseInt32("PULSE_DB_POOL_MAX_CONNS", 10),
		DBPoolMinConns:          parseInt32("PULSE_DB_POOL_MIN_CONNS", 2),
		DBPoolMaxIdleTime:       parseDuration("PULSE_DB_POOL_MAX_IDLE_TIME", 5*time.Minute),
		DBPoolHealthCheckPeriod: parseDuration("PULSE_DB_POOL_HEALTH_CHECK_PERIOD", 30*time.Second),
	}
}

func parseInt32(key string, fallback int32) int32 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseInt(v, 10, 32); err == nil {
			return int32(n)
		}
	}
	return fallback
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

// Validate checks that required configuration values are well-formed.
func (c Config) Validate() error {
	if !strings.HasPrefix(c.DBURL, "postgres://") && !strings.HasPrefix(c.DBURL, "postgresql://") {
		return fmt.Errorf("PULSE_DB_URL must start with postgres:// or postgresql://, got %q", c.DBURL)
	}
	if _, _, err := net.SplitHostPort(c.GRPCAddr); err != nil {
		return fmt.Errorf("PULSE_GRPC_ADDR is not a valid host:port (%q): %w", c.GRPCAddr, err)
	}
	if _, _, err := net.SplitHostPort(c.RESTAddr); err != nil {
		return fmt.Errorf("PULSE_REST_ADDR is not a valid host:port (%q): %w", c.RESTAddr, err)
	}
	if (c.TLSCert != "") != (c.TLSKey != "") {
		return fmt.Errorf("PULSE_TLS_CERT and PULSE_TLS_KEY must both be set or both be empty")
	}
	if c.StaleThreshold <= 0 {
		return fmt.Errorf("PULSE_STALE_THRESHOLD must be positive, got %v", c.StaleThreshold)
	}
	return nil
}
