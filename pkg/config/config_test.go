package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nexus.yaml")

	if err := os.WriteFile(path, []byte("server:\n  name: \"Nexus\"\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Server.Version == "" || cfg.Server.LogLevel == "" {
		t.Fatalf("expected defaults for version/log level")
	}
	if cfg.Modules.Kubernetes.Kubeconfig == "" {
		t.Fatalf("expected default kubeconfig")
	}
	if cfg.Modules.Prometheus.URL == "" {
		t.Fatalf("expected default prometheus url")
	}
}

func TestLoadConfigMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatalf("expected error for missing config")
	}
	if cfg.Server.Name != "Nexus" {
		t.Fatalf("expected default config returned on error")
	}
}
