package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDefaultsWhenMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := loadConfig()
	if err != nil || cfg != defaultConfig() {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
}

func TestConfigPartialOverridesAndClamps(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	_ = os.MkdirAll(filepath.Join(dir, "wr"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "wr", "config.toml"),
		[]byte("width = 5000\nbraille = false\nlinks = \"raro\"\ncache_days = 0\n"), 0o644)
	cfg, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 200 || cfg.Braille || cfg.Links != "footnotes" || cfg.CacheDays != 1 || !cfg.Scrollbar {
		t.Errorf("cfg=%+v", cfg)
	}
}

func TestConfigBrokenFileFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	_ = os.MkdirAll(filepath.Join(dir, "wr"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "wr", "config.toml"), []byte("this is not toml ==="), 0o644)
	cfg, err := loadConfig()
	if err == nil || cfg != defaultConfig() {
		t.Fatalf("must warn and use the defaults: cfg=%+v err=%v", cfg, err)
	}
}

func TestEnsureConfigFileCreatesCommentedDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, err := ensureConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "braille = true") {
		t.Errorf("plantilla: %s", data)
	}
	// the template must read back as the default configuration
	cfg, err := loadConfig()
	if err != nil || cfg != defaultConfig() {
		t.Errorf("the template differs from the defaults: %+v %v", cfg, err)
	}
	// and it never overwrites an existing file
	_ = os.WriteFile(path, []byte("width = 70\n"), 0o644)
	if _, err := ensureConfigFile(); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != "width = 70\n" {
		t.Error("ensureConfigFile overwrote the user's file")
	}
}
