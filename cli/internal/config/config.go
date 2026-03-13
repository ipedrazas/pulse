package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	APIAddr string `yaml:"api-addr"`
}

// Load reads config from ~/.pulse/config.yaml. Missing file is not an error.
func Load() Config {
	var c Config

	home, err := os.UserHomeDir()
	if err != nil {
		return c
	}
	data, err := os.ReadFile(filepath.Join(home, ".pulse", "config.yaml"))
	if err != nil {
		return c
	}
	_ = yaml.Unmarshal(data, &c)
	return c
}

// LoadFrom reads config from a specific path. Missing file is not an error.
func LoadFrom(path string) Config {
	var c Config
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	_ = yaml.Unmarshal(data, &c)
	return c
}
