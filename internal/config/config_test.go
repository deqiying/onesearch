package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigDirUsesUserHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(home, ".config", "onesearch")
	if got := defaultConfigDir(); got != want {
		t.Fatalf("defaultConfigDir() = %q, want %q", got, want)
	}
}

func TestResolveConfigDirPrefersEnvironmentOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ONESEARCH_CONFIG_DIR", dir)

	got, source := resolveConfigDir()
	if got != filepath.Clean(dir) {
		t.Fatalf("resolveConfigDir() dir = %q, want %q", got, filepath.Clean(dir))
	}
	if source != "environment" {
		t.Fatalf("resolveConfigDir() source = %q, want environment", source)
	}
}
