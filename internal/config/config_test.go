package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.ShowTools {
		t.Error("expected ShowTools=true by default")
	}
	if cfg.SortKey != "time" {
		t.Errorf("expected SortKey=time, got %q", cfg.SortKey)
	}
	if cfg.SortOrder != "desc" {
		t.Errorf("expected SortOrder=desc, got %q", cfg.SortOrder)
	}
}

func TestSaveLoad(t *testing.T) {
	// Override home dir to a temp directory.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	original := DefaultConfig()
	original.ShowTools = false
	original.SortKey = "id"
	original.SortOrder = "asc"
	original.GroupedMode = true
	original.FilterPath = "/some/path"
	original.WatcherEnabled = true

	if err := Save(original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Confirm file was created.
	configFile := filepath.Join(tmp, ".config", "cc-history", "config.json")
	if _, err := os.Stat(configFile); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	loaded := Load()
	if loaded.ShowTools != original.ShowTools {
		t.Errorf("ShowTools: want %v, got %v", original.ShowTools, loaded.ShowTools)
	}
	if loaded.SortKey != original.SortKey {
		t.Errorf("SortKey: want %q, got %q", original.SortKey, loaded.SortKey)
	}
	if loaded.SortOrder != original.SortOrder {
		t.Errorf("SortOrder: want %q, got %q", original.SortOrder, loaded.SortOrder)
	}
	if loaded.GroupedMode != original.GroupedMode {
		t.Errorf("GroupedMode: want %v, got %v", original.GroupedMode, loaded.GroupedMode)
	}
	if loaded.FilterPath != original.FilterPath {
		t.Errorf("FilterPath: want %q, got %q", original.FilterPath, loaded.FilterPath)
	}
	if loaded.WatcherEnabled != original.WatcherEnabled {
		t.Errorf("WatcherEnabled: want %v, got %v", original.WatcherEnabled, loaded.WatcherEnabled)
	}
}

func TestLoadMissingFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// No config file exists — should return defaults without error.
	cfg := Load()
	def := DefaultConfig()
	if cfg.ShowTools != def.ShowTools {
		t.Errorf("expected ShowTools=%v, got %v", def.ShowTools, cfg.ShowTools)
	}
}

func TestLoadCorruptFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Write corrupt JSON.
	configDir := filepath.Join(tmp, ".config", "cc-history")
	os.MkdirAll(configDir, 0o755) //nolint:errcheck
	os.WriteFile(filepath.Join(configDir, "config.json"), []byte("not-json{{"), 0o644) //nolint:errcheck

	cfg := Load()
	def := DefaultConfig()
	if cfg.ShowTools != def.ShowTools {
		t.Errorf("expected ShowTools=%v after corrupt config, got %v", def.ShowTools, cfg.ShowTools)
	}
}
