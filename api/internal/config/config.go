package config

import (
	"os"
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
