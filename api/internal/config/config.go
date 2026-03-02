package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DBURL         string
	GRPCPort      string
	HTTPPort      string
	MonitorToken  string
	RESTToken     string
	TLSCertFile   string
	TLSKeyFile    string
	WebhookURL    string
	WebhookEvents []string
}

func Load() (*Config, error) {
	c := &Config{
		DBURL:        getEnv("DB_URL", ""),
		GRPCPort:     getEnv("GRPC_PORT", "50051"),
		HTTPPort:     getEnv("HTTP_PORT", "8080"),
		MonitorToken: getEnv("MONITOR_TOKEN", ""),
	}

	if c.DBURL == "" {
		return nil, fmt.Errorf("DB_URL is required")
	}
	if c.MonitorToken == "" {
		return nil, fmt.Errorf("MONITOR_TOKEN is required")
	}

	restToken := getEnv("REST_TOKEN", "")
	if restToken == "" {
		restToken = c.MonitorToken
	}
	c.RESTToken = restToken

	c.TLSCertFile = getEnv("TLS_CERT_FILE", "")
	c.TLSKeyFile = getEnv("TLS_KEY_FILE", "")

	if (c.TLSCertFile == "") != (c.TLSKeyFile == "") {
		return nil, fmt.Errorf("TLS_CERT_FILE and TLS_KEY_FILE must both be set or both be empty")
	}

	c.WebhookURL = getEnv("WEBHOOK_URL", "")
	if evts := getEnv("WEBHOOK_EVENTS", ""); evts != "" {
		for _, e := range strings.Split(evts, ",") {
			if t := strings.TrimSpace(e); t != "" {
				c.WebhookEvents = append(c.WebhookEvents, t)
			}
		}
	}

	return c, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
