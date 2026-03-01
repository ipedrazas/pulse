package config

import (
	"fmt"
	"os"
)

type Config struct {
	ServerAddr   string
	MonitorToken string
	NodeName     string
}

func Load() (*Config, error) {
	c := &Config{
		ServerAddr:   getEnv("SERVER_ADDR", ""),
		MonitorToken: getEnv("MONITOR_TOKEN", ""),
		NodeName:     getEnv("PROXMOX_NODE_NAME", ""),
	}

	if c.ServerAddr == "" {
		return nil, fmt.Errorf("SERVER_ADDR is required")
	}
	if c.MonitorToken == "" {
		return nil, fmt.Errorf("MONITOR_TOKEN is required")
	}
	if c.NodeName == "" {
		return nil, fmt.Errorf("PROXMOX_NODE_NAME is required")
	}

	return c, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
