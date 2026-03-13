package config

import (
	"log/slog"
	"os"
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
	}
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
