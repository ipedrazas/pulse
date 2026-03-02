package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultRedactPatterns = "PASSWORD,SECRET,KEY,TOKEN,CREDENTIAL"

type Config struct {
	ServerAddr             string
	MonitorToken           string
	NodeName               string
	PollDelay              time.Duration
	MetadataResyncInterval time.Duration
	RedactPatterns         []string
}

func Load() (*Config, error) {
	pollDelay, err := parseDurationSeconds("POLL_DELAY_SECONDS", "0")
	if err != nil {
		return nil, err
	}

	resyncInterval, err := parseDurationSeconds("METADATA_RESYNC_SECONDS", "3600")
	if err != nil {
		return nil, err
	}

	c := &Config{
		ServerAddr:             getEnv("SERVER_ADDR", ""),
		MonitorToken:           getEnv("MONITOR_TOKEN", ""),
		NodeName:               getEnv("PROXMOX_NODE_NAME", ""),
		PollDelay:              pollDelay,
		MetadataResyncInterval: resyncInterval,
		RedactPatterns:         parseRedactPatterns(getEnv("ENV_REDACT_PATTERNS", defaultRedactPatterns)),
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

func parseRedactPatterns(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	patterns := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			patterns = append(patterns, strings.ToUpper(p))
		}
	}
	return patterns
}

func parseDurationSeconds(envKey, fallback string) (time.Duration, error) {
	raw := getEnv(envKey, fallback)
	secs, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid integer %q: %w", envKey, raw, err)
	}
	return time.Duration(secs) * time.Second, nil
}
