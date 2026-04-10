package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds runtime configuration.
type Config struct {
	MWKey     string // MW Dictionary API key
	MWThesKey string // MW Thesaurus API key (optional, falls back to MWKey)
}

type fileConfig struct {
	MWKey     string `toml:"mw_key"`
	MWThesKey string `toml:"mw_thes_key"`
}

// DefaultPath returns the default config file path.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "lazydict", "config.toml")
}

// Load reads config from the given file path (may be empty) then overrides
// with environment variables. Returns an error if MWKey is still unset.
func Load(path string) (*Config, error) {
	var fc fileConfig
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			if _, err := toml.DecodeFile(path, &fc); err != nil {
				return nil, fmt.Errorf("config: parse %s: %w", path, err)
			}
		}
	}

	cfg := &Config{
		MWKey:     fc.MWKey,
		MWThesKey: fc.MWThesKey,
	}

	if v := os.Getenv("LAZYDICT_MW_KEY"); v != "" {
		cfg.MWKey = v
	}
	if v := os.Getenv("LAZYDICT_MW_THES_KEY"); v != "" {
		cfg.MWThesKey = v
	}

	// Thesaurus key falls back to dictionary key
	if cfg.MWThesKey == "" {
		cfg.MWThesKey = cfg.MWKey
	}

	if cfg.MWKey == "" {
		return nil, fmt.Errorf(
			"lazydict: MW API key not set. Set $LAZYDICT_MW_KEY or add mw_key to %s",
			DefaultPath(),
		)
	}

	return cfg, nil
}
