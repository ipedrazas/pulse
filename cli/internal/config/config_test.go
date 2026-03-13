package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MissingFile(t *testing.T) {
	c := LoadFrom("/nonexistent/path/config.yaml")
	if c.APIAddr != "" {
		t.Errorf("expected empty APIAddr, got %q", c.APIAddr)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("api-addr: myhost:1234\n"), 0644); err != nil {
		t.Fatal(err)
	}
	c := LoadFrom(path)
	if c.APIAddr != "myhost:1234" {
		t.Errorf("APIAddr = %q, want myhost:1234", c.APIAddr)
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	c := LoadFrom(path)
	if c.APIAddr != "" {
		t.Errorf("expected empty APIAddr, got %q", c.APIAddr)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(": : invalid\n\t\tbad"), 0644); err != nil {
		t.Fatal(err)
	}
	c := LoadFrom(path)
	// Should not panic, returns zero config
	if c.APIAddr != "" {
		t.Errorf("expected empty APIAddr for invalid YAML, got %q", c.APIAddr)
	}
}
