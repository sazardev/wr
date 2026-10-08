package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is read from ~/.config/wr/config.toml (optional). Anything missing
// uses its default, and a broken file never prevents reading: a warning is
// shown and the defaults are used.
type Config struct {
	Width      int    `toml:"width"`      // max width of the reading column
	Center     bool   `toml:"center"`     // center that column
	Braille    bool   `toml:"braille"`    // braille decoration (headings, bars)
	Scrollbar  bool   `toml:"scrollbar"`  // scrollbar on the right
	Footer     bool   `toml:"footer"`     // footer with progress and shortcuts
	Links      string `toml:"links"`      // footnotes | inline | hidden
	Mouse      bool   `toml:"mouse"`      // wheel scrolling and drag-to-copy selection
	Animations bool   `toml:"animations"` // spring scrolling, reveal, panel and loading animations
	CacheDays  int    `toml:"cache_days"` // days a page stays in the cache
}

func defaultConfig() Config {
	return Config{Width: 100, Center: true, Braille: true, Scrollbar: true,
		Footer: true, Links: "footnotes", Mouse: true, Animations: true, CacheDays: 30}
}

func (c *Config) normalize() {
	c.Width = min(max(c.Width, 40), 200)
	c.CacheDays = min(max(c.CacheDays, 1), 3650)
	switch c.Links {
	case "footnotes", "inline", "hidden":
	default:
		c.Links = "footnotes"
	}
}

func configPath() (string, error) {
	d, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "wr", "config.toml"), nil
}

// loadConfig never fails outright: it always returns a usable configuration.
func loadConfig() (Config, error) {
	cfg := defaultConfig()
	path, err := configPath()
	if err != nil {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return defaultConfig(), fmt.Errorf("config.toml: %w", err)
	}
	cfg.normalize()
	return cfg, nil
}

const configTemplate = `# wr configuration. Everything is optional: whatever you delete uses its default.

# Max width of the reading column (40-200).
width = 100

# Center that column in the terminal.
center = true

# Braille decoration: heading marks, progress bar, scrollbar, spinner.
# false = plain glyphs (more compatible with fonts without braille).
braille = true

# Scrollbar on the right.
scrollbar = true

# Footer with reading progress and shortcuts.
footer = true

# Links: "footnotes" (numbered and listed at the end), "inline" (URL next to
# the text) or "hidden" (text only).
links = "footnotes"

# Mouse support: wheel scrolling and drag-to-copy selection (double click = word,
# triple click = line). Hold Shift while dragging to use your terminal's own
# selection instead. false = the terminal handles the mouse entirely.
mouse = true

# Spring-physics scrolling, reveal, panel and loading animations. They only run
# while something is moving; at rest the program uses no CPU.
animations = true

# Days a page is kept in the cache (1-3650).
cache_days = 30
`

// ensureConfigFile creates the file with commented defaults if it is missing.
func ensureConfigFile() (string, error) {
	path, err := configPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(configTemplate), 0o644)
}
