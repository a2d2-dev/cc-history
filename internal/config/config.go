// Package config handles persistent TUI preferences for cc-history.
// Config is stored at ~/.config/cc-history/config.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds persistent TUI preferences.
type Config struct {
	// ShowTools controls whether tool call lines are visible in the TUI.
	ShowTools bool `json:"show_tools"`
	// SortKey is the session list sort field (e.g. "time", "id").
	SortKey string `json:"sort_key"`
	// SortOrder is "asc" or "desc".
	SortOrder string `json:"sort_order"`
	// GroupedMode groups sessions by project in the session picker.
	GroupedMode bool `json:"grouped_mode"`
	// FilterPath restricts displayed sessions to a specific project path.
	FilterPath string `json:"filter_path"`
	// WatcherEnabled controls the file-watcher for live session updates.
	WatcherEnabled bool `json:"watcher_enabled"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		ShowTools: true,
		SortKey:   "time",
		SortOrder: "desc",
	}
}

// dir returns the directory for the config file.
func dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "cc-history"), nil
}

// Load reads the config file. Returns defaults on missing or corrupt file.
func Load() Config {
	cfg := DefaultConfig()
	d, err := dir()
	if err != nil {
		return cfg
	}
	data, err := os.ReadFile(filepath.Join(d, "config.json"))
	if err != nil {
		// Missing file is expected on first run.
		return cfg
	}
	if jsonErr := json.Unmarshal(data, &cfg); jsonErr != nil {
		// Corrupt config — silently return defaults.
		return DefaultConfig()
	}
	return cfg
}

// Save writes cfg to disk, creating the config directory if necessary.
func Save(cfg Config) error {
	d, err := dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(d, "config.json"), data, 0o644)
}
