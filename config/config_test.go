package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/peterqlin/lazydict/config"
)

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("LAZYDICT_MW_KEY", "dict-key-from-env")
	t.Setenv("LAZYDICT_MW_THES_KEY", "thes-key-from-env")

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MWKey != "dict-key-from-env" {
		t.Errorf("MWKey = %q, want %q", cfg.MWKey, "dict-key-from-env")
	}
	if cfg.MWThesKey != "thes-key-from-env" {
		t.Errorf("MWThesKey = %q, want %q", cfg.MWThesKey, "thes-key-from-env")
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.toml")
	os.WriteFile(cfgFile, []byte("mw_key = \"file-dict-key\"\nmw_thes_key = \"file-thes-key\"\n"), 0600)

	os.Unsetenv("LAZYDICT_MW_KEY")
	os.Unsetenv("LAZYDICT_MW_THES_KEY")

	cfg, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MWKey != "file-dict-key" {
		t.Errorf("MWKey = %q, want %q", cfg.MWKey, "file-dict-key")
	}
}

func TestEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.toml")
	os.WriteFile(cfgFile, []byte("mw_key = \"file-key\"\n"), 0600)
	t.Setenv("LAZYDICT_MW_KEY", "env-key")

	cfg, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MWKey != "env-key" {
		t.Errorf("env should override file, got %q", cfg.MWKey)
	}
}

func TestMissingKey(t *testing.T) {
	os.Unsetenv("LAZYDICT_MW_KEY")
	os.Unsetenv("LAZYDICT_MW_THES_KEY")

	_, err := config.Load("")
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
}
