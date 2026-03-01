package config

import (
	"fmt"
	"os"
)

type Config struct {
	DBURL        string
	GRPCPort     string
	HTTPPort     string
	MonitorToken string
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

	return c, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
