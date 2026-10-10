package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config is read from ~/.config/wr/config.toml (optional) and edited live from
// the Settings panel, which saves it back. Anything missing uses its default,
// and a broken file never prevents reading: a warning is shown and the
// defaults are used.
type Config struct {
	// Layout
	Width        int    `toml:"width"`         // max width of the reading column (0 = full width)
	Center       bool   `toml:"center"`        // center that column
	Spacing      int    `toml:"spacing"`       // blank lines between blocks: 0, 1 or 2
	HeadingRules bool   `toml:"heading_rules"` // a rule under h1 / h2
	CodeFrame    bool   `toml:"code_frame"`    // label and frame around code blocks
	Links        string `toml:"links"`         // footnotes | inline | hidden
	Braille      bool   `toml:"braille"`       // braille decoration
	Accent       string `toml:"accent"`        // color of the UI chrome

	// Scrollbar and footer
	Scrollbar      bool   `toml:"scrollbar"`
	Footer         bool   `toml:"footer"`
	FooterRule     bool   `toml:"footer_rule"`
	FooterHints    bool   `toml:"footer_hints"`
	FooterProgress string `toml:"footer_progress"` // both | percent | bar | off
	FooterChips    bool   `toml:"footer_chips"`    // search count, newer version, bookmark star

	// Mouse
	Mouse        bool `toml:"mouse"`
	CopyOnSelect bool `toml:"copy_on_select"`
	WheelLines   int  `toml:"wheel_lines"`

	// Motion
	Animations  bool   `toml:"animations"` // master switch
	AnimScroll  bool   `toml:"anim_scroll"`
	AnimReveal  bool   `toml:"anim_reveal"`
	AnimPanels  bool   `toml:"anim_panels"`
	AnimLoading bool   `toml:"anim_loading"`
	AnimNotices bool   `toml:"anim_notices"`
	ScrollSpeed string `toml:"scroll_speed"` // slow | normal | fast

	// Browsing
	SearchEngine string `toml:"search_engine"` // a preset name or a URL template with %s
	Homepage     string `toml:"homepage"`      // "" = the start page
	History      bool   `toml:"history"`
	HistoryLimit int    `toml:"history_limit"`

	// Cache
	Cache             bool `toml:"cache"`
	CacheDays         int  `toml:"cache_days"`
	BackgroundRefresh bool `toml:"background_refresh"`

	// Key bindings that differ from the defaults: action -> keys
	Keys map[string][]string `toml:"keys"`
}

func defaultConfig() Config {
	return Config{
		Width: 100, Center: true, Spacing: 1, HeadingRules: true, CodeFrame: true,
		Links: "footnotes", Braille: true, Accent: "blue",
		Scrollbar: true, Footer: true, FooterRule: true, FooterHints: true,
		FooterProgress: "both", FooterChips: true,
		Mouse: true, CopyOnSelect: true, WheelLines: 3,
		Animations: true, AnimScroll: true, AnimReveal: true, AnimPanels: true,
		AnimLoading: true, AnimNotices: true, ScrollSpeed: "normal",
		SearchEngine: "duckduckgo", History: true, HistoryLimit: 1000,
		Cache: true, CacheDays: 30, BackgroundRefresh: true,
	}
}

var (
	linkModes     = []string{"footnotes", "inline", "hidden"}
	progressModes = []string{"both", "percent", "bar", "off"}
	speedModes    = []string{"slow", "normal", "fast"}
	searchPresets = map[string]string{
		"duckduckgo": "https://html.duckduckgo.com/html/?q=%s",
		"wikipedia":  "https://en.wikipedia.org/w/index.php?search=%s",
		"github":     "https://github.com/search?q=%s",
	}
	searchPresetNames = []string{"duckduckgo", "wikipedia", "github"}
)

func oneOf(v string, set []string) bool {
	for _, s := range set {
		if v == s {
			return true
		}
	}
	return false
}

func (c *Config) normalize() {
	d := defaultConfig()
	if c.Width != 0 {
		c.Width = min(max(c.Width, 40), 400)
	}
	c.Spacing = min(max(c.Spacing, 0), 2)
	c.WheelLines = min(max(c.WheelLines, 1), 20)
	c.CacheDays = min(max(c.CacheDays, 1), 3650)
	c.HistoryLimit = min(max(c.HistoryLimit, 50), 20000)
	if !oneOf(c.Links, linkModes) {
		c.Links = d.Links
	}
	if !oneOf(c.FooterProgress, progressModes) {
		c.FooterProgress = d.FooterProgress
	}
	if !oneOf(c.ScrollSpeed, speedModes) {
		c.ScrollSpeed = d.ScrollSpeed
	}
	if _, ok := accents[c.Accent]; !ok {
		c.Accent = d.Accent
	}
	if c.SearchEngine == "" || (searchPresets[c.SearchEngine] == "" && !strings.Contains(c.SearchEngine, "%s")) {
		c.SearchEngine = d.SearchEngine
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

// saveConfig writes the whole configuration (with comments) atomically.
func saveConfig(c Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	_, werr := tmp.WriteString(configText(c))
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(tmp.Name())
		return werr
	}
	_ = os.Chmod(tmp.Name(), 0o644)
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

// ensureConfigFile creates the file with commented defaults if it is missing.
func ensureConfigFile() (string, error) {
	path, err := configPath()
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return path, saveConfig(defaultConfig())
}

func tomlBool(b bool) string  { return strconv.FormatBool(b) }
func tomlStr(s string) string { return strconv.Quote(s) }

// configText renders a configuration as a commented TOML file.
func configText(c Config) string {
	var b strings.Builder
	kv := func(comment, key, val string) {
		for _, l := range strings.Split(comment, "\n") {
			b.WriteString("# " + l + "\n")
		}
		b.WriteString(key + " = " + val + "\n\n")
	}
	section := func(name string) { b.WriteString("# ---- " + name + " ----\n\n") }

	b.WriteString("# wr configuration. Everything is optional: whatever you delete uses its\n")
	b.WriteString("# default. Most of it can be changed live from the Settings panel (m), which\n")
	b.WriteString("# saves back to this file (comments you add here are not kept when it does).\n\n")

	section("Layout")
	kv("Max width of the reading column, in columns (40-400). 0 = use the full width.", "width", strconv.Itoa(c.Width))
	kv("Center that column in the terminal.", "center", tomlBool(c.Center))
	kv("Blank lines between blocks: 0 compact, 1 normal, 2 airy.", "spacing", strconv.Itoa(c.Spacing))
	kv("A rule under level 1 and 2 headings.", "heading_rules", tomlBool(c.HeadingRules))
	kv("The label and frame around code blocks.", "code_frame", tomlBool(c.CodeFrame))
	kv("Links: \"footnotes\" (numbered, listed at the end), \"inline\" (URL next to the\ntext) or \"hidden\" (text only). Links are clickable in every mode.", "links", tomlStr(c.Links))
	kv("Braille decoration: heading marks, progress bar, scrollbar, spinner.\nfalse = plain glyphs (for fonts without braille).", "braille", tomlBool(c.Braille))
	kv("Accent color of the interface: blue, cyan, green, magenta, yellow, red, gray, white (your\nterminal's own 16 colors, so they follow its theme) or orange, violet, teal, pink, lime,\nslate (fixed 256-color presets). The headings, the quote bars and the rules follow it too.", "accent", tomlStr(c.Accent))

	section("Scrollbar and footer")
	kv("Scrollbar on the right.", "scrollbar", tomlBool(c.Scrollbar))
	kv("The footer row at the bottom.", "footer", tomlBool(c.Footer))
	kv("The thin rule above the footer.", "footer_rule", tomlBool(c.FooterRule))
	kv("Shortcut hints on the left of the footer.", "footer_hints", tomlBool(c.FooterHints))
	kv("Reading progress on the right of the footer: \"both\", \"percent\", \"bar\" or \"off\".", "footer_progress", tomlStr(c.FooterProgress))
	kv("Small notes beside the progress: search count, newer version, bookmark star.", "footer_chips", tomlBool(c.FooterChips))

	section("Mouse")
	kv("Mouse support: wheel scrolling, click links, drag to select. Hold Shift while\ndragging to use your terminal's own selection. false = the terminal handles it.", "mouse", tomlBool(c.Mouse))
	kv("Copy a selection as soon as you release the mouse. false = copy with y.", "copy_on_select", tomlBool(c.CopyOnSelect))
	kv("Lines per wheel notch (1-20).", "wheel_lines", strconv.Itoa(c.WheelLines))

	section("Motion")
	kv("Master switch for every animation. They only run while something moves; at\nrest the program uses no CPU.", "animations", tomlBool(c.Animations))
	kv("Spring-physics smooth scrolling.", "anim_scroll", tomlBool(c.AnimScroll))
	kv("The page wiping in when it loads.", "anim_reveal", tomlBool(c.AnimReveal))
	kv("Panels opening like a shutter.", "anim_panels", tomlBool(c.AnimPanels))
	kv("The braille wave while a page downloads.", "anim_loading", tomlBool(c.AnimLoading))
	kv("Notices typing themselves out.", "anim_notices", tomlBool(c.AnimNotices))
	kv("How quickly smooth scrolling catches up: \"slow\", \"normal\" or \"fast\".", "scroll_speed", tomlStr(c.ScrollSpeed))

	section("Browsing")
	kv("Where plain words go when you type them in the address bar (o): a preset\n(\"duckduckgo\", \"wikipedia\", \"github\") or your own URL with %s for the query.", "search_engine", tomlStr(c.SearchEngine))
	kv("Page to open when you start wr without an address. \"\" = the start page.", "homepage", tomlStr(c.Homepage))
	kv("Remember the pages you visit (the History panel and address-bar suggestions).", "history", tomlBool(c.History))
	kv("How many visits to keep (50-20000).", "history_limit", strconv.Itoa(c.HistoryLimit))

	section("Cache")
	kv("Keep pages on disk so a page you already read opens instantly.", "cache", tomlBool(c.Cache))
	kv("Days a page stays in the cache (1-3650).", "cache_days", strconv.Itoa(c.CacheDays))
	kv("Refresh a cached page in the background while you read it.", "background_refresh", tomlBool(c.BackgroundRefresh))

	// Only the bindings that differ from the defaults are written.
	if len(c.Keys) > 0 {
		b.WriteString("# ---- Key bindings ----\n#\n# Only the actions you changed. Customize them from the menu: Keyboard shortcuts.\n\n[keys]\n")
		ids := make([]string, 0, len(c.Keys))
		for id := range c.Keys {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			q := make([]string, len(c.Keys[id]))
			for i, k := range c.Keys[id] {
				q[i] = tomlStr(k)
			}
			b.WriteString(id + " = [" + strings.Join(q, ", ") + "]\n")
		}
	}
	return b.String()
}
