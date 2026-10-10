package main

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// resetGlobals restores the package-level state a settings change can touch.
func resetGlobals(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { setTerminalDark(true); setAccent("blue"); setScrollSpeed("normal"); cacheOn = true })
}

func optionIndex(t *testing.T, m *model, label string) int {
	t.Helper()
	for i, o := range m.settingsOptions() {
		if o.label == label {
			return i
		}
	}
	t.Fatalf("no setting called %q", label)
	return -1
}

// changeSetting opens Settings, selects an option and presses a key on it.
func changeSetting(t *testing.T, m *model, label, key string) {
	t.Helper()
	m.mode, m.sel = modeSettings, optionIndex(t, m, label)
	press(m, key)
}

func TestEveryOptionIsWellFormed(t *testing.T) {
	m := newTestModel(t, 100, 40)
	seen := map[string]bool{}
	for _, o := range m.settingsOptions() {
		if o.label == "" || o.help == "" || o.group == "" {
			t.Errorf("option needs a label, a group and help text: %+v", o.label)
		}
		if seen[o.label] {
			t.Errorf("duplicate option label %q", o.label)
		}
		seen[o.label] = true
		if o.kind == optAction {
			continue
		}
		def := defaultConfig()
		cfg := defaultConfig()
		if o.get(&cfg) == "" {
			t.Errorf("%q shows no value", o.label)
		}
		o.step(&cfg, +1)
		if reflect.DeepEqual(cfg, def) {
			t.Errorf("%q: stepping changed nothing", o.label)
		}
		o.step(&cfg, -1)
		if !reflect.DeepEqual(cfg, def) {
			t.Errorf("%q: stepping forward then back must return to the start", o.label)
		}
		o.step(&cfg, +1)
		o.reset(&cfg)
		if !reflect.DeepEqual(cfg, def) {
			t.Errorf("%q: reset must restore the default", o.label)
		}
	}
	if len(seen) < 25 {
		t.Errorf("expected a rich settings panel, got %d options", len(seen))
	}
}

func TestEveryConfigKeyHasASetting(t *testing.T) {
	// every field of Config except Keys and Homepage's raw value must be reachable from the panel
	m := newTestModel(t, 100, 40)
	covered := 0
	for _, o := range m.settingsOptions() {
		if o.kind != optAction {
			covered++
		}
	}
	fields := reflect.TypeOf(Config{}).NumField()
	// Keys (own panel), Homepage and Search engine's custom template are the only extras
	if covered < fields-3 {
		t.Errorf("%d settings for %d config fields", covered, fields)
	}
}

func TestSettingsChangesApplyAtOnceAndPersist(t *testing.T) {
	resetGlobals(t)
	m := newTestModel(t, 100, 40)
	changeSetting(t, m, "Accent color", "right")
	if m.cfg.Accent != "cyan" || accentFG != "96" {
		t.Errorf("accent: %q %q", m.cfg.Accent, accentFG)
	}
	saved, err := loadConfig()
	if err != nil || saved.Accent != "cyan" {
		t.Errorf("the change must be saved to the file: %+v %v", saved.Accent, err)
	}
	changeSetting(t, m, "Footer", "enter")
	saved, _ = loadConfig()
	if saved.Footer || m.cfg.Footer {
		t.Error("footer off must persist")
	}
	if _, _, _, fh := m.geometry(); fh != 0 {
		t.Errorf("with the footer off its rows are given back to the page: %d", fh)
	}
	if got := len(lines(m)); got != 40 {
		t.Errorf("the page must fill the terminal: %d", got)
	}
}

func TestSettingsRebuildTheDocumentWhenNeeded(t *testing.T) {
	resetGlobals(t)
	m := newTestModel(t, 100, 40)
	if !strings.Contains(strings.Join(m.doc.Plain, ""), "⣿") {
		t.Fatal("braille is the default")
	}
	changeSetting(t, m, "Braille decoration", "enter")
	if strings.Contains(strings.Join(m.doc.Plain, ""), "⣿") {
		t.Error("turning braille off must redraw the page")
	}
	changeSetting(t, m, "Links", "right")
	if m.cfg.Links != "inline" {
		t.Errorf("links=%q", m.cfg.Links)
	}
}

func TestSettingsDKeyRestoresTheDefault(t *testing.T) {
	m := newTestModel(t, 100, 40)
	changeSetting(t, m, "Wheel lines", "right")
	changeSetting(t, m, "Wheel lines", "right")
	if m.cfg.WheelLines != 5 {
		t.Fatalf("wheel lines=%d", m.cfg.WheelLines)
	}
	press(m, "d")
	if m.cfg.WheelLines != 3 {
		t.Errorf("d must restore the default: %d", m.cfg.WheelLines)
	}
	press(m, "left")
	if m.cfg.WheelLines != 2 {
		t.Errorf("left decrements: %d", m.cfg.WheelLines)
	}
}

func TestSettingsPanelShowsGroupsValuesAndHelp(t *testing.T) {
	m := newTestModel(t, 100, 70)
	press(m, ",")
	if m.mode != modeSettings {
		t.Fatalf("the settings key opens the panel: %v", m.mode)
	}
	v := ansi.Strip(m.render())
	for _, want := range []string{"Settings", "Layout", "Footer and scrollbar", "Mouse", "Motion", "Browsing", "Cache", "Reading width", "‹ 100 ›", "Keyboard shortcuts…"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q", want)
		}
	}
	if !strings.Contains(v, "Maximum width of the text column") {
		t.Error("the selected option's help must be shown")
	}
	press(m, "down")
	if v := ansi.Strip(m.render()); !strings.Contains(v, "Center the text column") {
		t.Error("the help follows the selection")
	}
}

func TestSettingsStartPageHomepageAction(t *testing.T) {
	m := newTestModel(t, 100, 40)
	changeSetting(t, m, "Start page", "enter")
	if m.cfg.Homepage != m.src {
		t.Errorf("homepage=%q", m.cfg.Homepage)
	}
	if saved, _ := loadConfig(); saved.Homepage != m.src {
		t.Errorf("homepage must persist: %q", saved.Homepage)
	}
	changeSetting(t, m, "Start page", "enter")
	if m.cfg.Homepage != "" {
		t.Error("choosing it again goes back to the start page")
	}
}

func TestResetAllSettingsKeepsKeyBindings(t *testing.T) {
	resetGlobals(t)
	m := newTestModel(t, 100, 40)
	changeSetting(t, m, "Accent color", "right")
	changeSetting(t, m, "Footer", "enter")
	setKeys(&m.cfg, actQuit, []string{"Q"})
	m.mode, m.sel = modeSettings, optionIndex(t, m, "Reset all settings…")
	press(m, "enter")
	if m.mode != modeConfirm || m.confirm != confirmSettings {
		t.Fatalf("it must ask first: mode=%v", m.mode)
	}
	press(m, "esc")
	if m.cfg.Accent == "blue" {
		t.Error("cancelling must change nothing")
	}
	m.mode, m.confirm = modeConfirm, confirmSettings
	press(m, "y")
	if m.cfg.Accent != "blue" || !m.cfg.Footer {
		t.Errorf("settings must be back to defaults: %+v", m.cfg)
	}
	if got := effectiveKeys(&m.cfg, actQuit); !reflect.DeepEqual(got, []string{"Q"}) {
		t.Errorf("key bindings are kept: %v", got)
	}
}

func TestLinksFlagIsNotWrittenToTheFile(t *testing.T) {
	isolateAll(t)
	cfg := defaultConfig()
	cfg.Animations = false
	_ = saveConfig(cfg)  // the file says footnotes
	cfg.Links = "inline" // but this session was started with a different links mode (-L uses hidden)
	m := loaded(newModel("https://t.test/x", cfg, false, nil), 100, 40)
	if !m.linksFromFlag {
		t.Fatal("a links value that differs from the file comes from a flag")
	}
	changeSetting(t, m, "Wheel lines", "right")
	if saved, _ := loadConfig(); saved.Links != "footnotes" {
		t.Errorf("-L must not be saved: %q", saved.Links)
	}
	changeSetting(t, m, "Links", "right") // an explicit choice in Settings is saved
	if saved, _ := loadConfig(); saved.Links == "footnotes" {
		t.Errorf("an explicit change must be saved: %q", saved.Links)
	}
}

// ---- what each option actually does ----

func withCfg(t *testing.T, edit func(*Config), w, h int) *model {
	t.Helper()
	isolateAll(t)
	resetGlobals(t)
	cfg := defaultConfig()
	cfg.Animations = false
	edit(&cfg)
	applyRuntime(cfg)
	return loaded(newModel("https://t.test/x", cfg, false, nil), w, h)
}

func TestSpacingChangesTheLineCount(t *testing.T) {
	count := func(s int) int {
		return len(withCfg(t, func(c *Config) { c.Spacing = s }, 100, 40).doc.Lines)
	}
	if c, n, a := count(0), count(1), count(2); !(c < n && n < a) {
		t.Errorf("compact < normal < airy expected: %d %d %d", c, n, a)
	}
}

func TestHeadingRulesAndCodeFramesCanBeTurnedOff(t *testing.T) {
	md := "# Title\n\ntext\n\n```go\nx := 1\n```\n"
	on := renderDoc(md, 60, defaultConfig())
	cfg := defaultConfig()
	cfg.HeadingRules, cfg.CodeFrame = false, false
	off := renderDoc(md, 60, cfg)
	all := func(d *Doc) string { return strings.Join(d.Plain, "\n") }
	if !strings.Contains(all(on), "⣀⣀") || !strings.Contains(all(on), "╭─ go") {
		t.Fatal("rules and frames are on by default")
	}
	if strings.Contains(all(off), "⣀⣀") || strings.Contains(all(off), "╭─") || strings.Contains(all(off), "╰─") {
		t.Errorf("rules and frames must disappear:\n%s", all(off))
	}
	if !strings.Contains(all(off), "x := 1") {
		t.Error("the code itself stays")
	}
	// copying still works without the frame
	if len(off.Meta) != len(off.Lines) {
		t.Error("metadata must still cover every line")
	}
}

func TestFullWidthUsesTheWholeTerminal(t *testing.T) {
	m := withCfg(t, func(c *Config) { c.Width = 0; c.Center = false }, 160, 30)
	cw, left, _, _ := m.geometry()
	if cw < 150 || left > 3 {
		t.Errorf("full width: content=%d left=%d", cw, left)
	}
	m = withCfg(t, func(c *Config) { c.Width = 80 }, 160, 30)
	if cw, _, _, _ := m.geometry(); cw != 80 {
		t.Errorf("width 80: %d", cw)
	}
}

func TestCenterOffAlignsLeft(t *testing.T) {
	m := withCfg(t, func(c *Config) { c.Center = false }, 160, 30)
	if _, left, _, _ := m.geometry(); left != 2 {
		t.Errorf("left=%d", left)
	}
}

func TestFooterPartsCanBeTurnedOffIndependently(t *testing.T) {
	m := withCfg(t, func(c *Config) { c.FooterRule = false }, 100, 30)
	if _, _, _, fh := m.geometry(); fh != 1 {
		t.Errorf("no rule: footer is one row, got %d", fh)
	}
	m = withCfg(t, func(c *Config) { c.FooterHints = false }, 100, 30)
	if strings.Contains(lastRow(m), "quit") || !strings.Contains(lastRow(m), "%") {
		t.Errorf("no hints, progress stays: %q", lastRow(m))
	}
	m = withCfg(t, func(c *Config) { c.FooterProgress = "off" }, 100, 30)
	if strings.Contains(lastRow(m), "%") || !strings.Contains(lastRow(m), "quit") {
		t.Errorf("no progress, hints stay: %q", lastRow(m))
	}
	m = withCfg(t, func(c *Config) { c.FooterProgress = "percent" }, 100, 30)
	if r := lastRow(m); !strings.Contains(r, "%") || strings.ContainsAny(r, "⣀⣿") && strings.Contains(strings.TrimSpace(strings.SplitN(r, "quit", 2)[1]), "⣀") {
		t.Errorf("percent only: %q", r)
	}
	m = withCfg(t, func(c *Config) { c.FooterProgress = "bar" }, 100, 30)
	if r := lastRow(m); strings.Contains(r, "%") || !strings.ContainsAny(r, "⣀⣿") {
		t.Errorf("bar only: %q", r)
	}
	m = withCfg(t, func(c *Config) { c.FooterChips = false }, 100, 30)
	m.bookmarked = true
	if strings.Contains(lastRow(m), "★") {
		t.Error("notes off: no star")
	}
}

func TestScrollbarCanBeTurnedOff(t *testing.T) {
	m := withCfg(t, func(c *Config) { c.Scrollbar = false }, 100, 30)
	if strings.ContainsAny(strings.Join(lines(m)[:10], ""), "⡇⠿") {
		t.Error("no scrollbar glyphs expected")
	}
}

func TestWheelLinesSetting(t *testing.T) {
	m := withCfg(t, func(c *Config) { c.WheelLines = 7 }, 100, 30)
	m.Update(tea.MouseWheelMsg{X: 5, Y: 5, Button: tea.MouseWheelDown})
	if m.y != 7 {
		t.Errorf("one notch is 7 lines: %d", m.y)
	}
}

func TestCopyOnSelectCanBeTurnedOff(t *testing.T) {
	var copied []string
	old := clipboardExternal
	clipboardExternal = func(s string) error { copied = append(copied, s); return nil }
	t.Cleanup(func() { clipboardExternal = old })
	m := withCfg(t, func(c *Config) { c.CopyOnSelect = false }, 100, 30)
	l := lineOf(t, m, "filler")
	if cmd := drag(m, point{l, 0}, point{l, 5}); cmd != nil {
		t.Error("with copy-on-select off, releasing the mouse must not copy")
	}
	if !m.mouseSel.active {
		t.Error("but the selection stays highlighted")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}); cmd == nil {
		t.Error("y copies it")
	}
}

func TestMouseCanBeTurnedOff(t *testing.T) {
	m := withCfg(t, func(c *Config) { c.Mouse = false }, 100, 30)
	if m.View().MouseMode != tea.MouseModeNone {
		t.Error("with mouse support off the terminal keeps the mouse")
	}
	m = withCfg(t, func(c *Config) {}, 100, 30)
	if m.View().MouseMode == tea.MouseModeNone {
		t.Error("mouse support is on by default")
	}
}

func TestGranularAnimationSwitches(t *testing.T) {
	// the master switch is on, but smooth scrolling alone is off
	isolateAll(t)
	cfg := defaultConfig()
	cfg.AnimScroll = false
	m := loaded(newModel("https://t.test/x", cfg, false, nil), 100, 30)
	m.revealStart = time0()
	press(m, "G")
	if m.viewY() != m.maxY() {
		t.Error("with smooth scrolling off the view must follow at once")
	}
	// and the other way round: master off beats everything
	cfg = defaultConfig()
	cfg.Animations = false
	m = loaded(newModel("https://t.test/x", cfg, false, nil), 100, 30)
	press(m, "G")
	if m.viewY() != m.maxY() {
		t.Error("master off means instant")
	}
	if active, _ := m.animating(); active {
		t.Error("nothing animates with the master switch off")
	}
}

func TestScrollSpeedChangesTheSpring(t *testing.T) {
	resetGlobals(t)
	frames := func(speed string) int {
		setScrollSpeed(speed)
		s := spring{target: 100}
		n := 0
		for !s.settled() && n < 1000 {
			s.step(1)
			n++
		}
		return n
	}
	slow, normal, fast := frames("slow"), frames("normal"), frames("fast")
	if !(fast < normal && normal < slow) {
		t.Errorf("fast < normal < slow expected: %d %d %d", fast, normal, slow)
	}
}

func TestCacheSettingIsHonored(t *testing.T) {
	resetGlobals(t)
	isolateAll(t)
	cfg := defaultConfig()
	cfg.Cache = false
	applyRuntime(cfg)
	if cacheOn {
		t.Fatal("cache off must turn the package switch off")
	}
	if cmd := loadCmd(1, "https://t.test/cached", false, cfg); cmd == nil {
		t.Fatal("loadCmd")
	}
	_ = cachePut("https://t.test/cached", "# From cache\n")
	// a cache hit must be ignored when the cache is off: the load goes to the
	// network, which is unreachable here, so it errors instead of returning the cached text
	msg := runCmd(loadCmd(1, "https://127.0.0.1:1/never", false, cfg))
	if lm, ok := msg.(loadedMsg); ok && lm.fromCache {
		t.Error("cache off: never serve from cache")
	}
}

func TestAccentColorsThePanels(t *testing.T) {
	resetGlobals(t)
	setAccent("red")
	b := box("T", []boxRow{{"row", ""}}, 0, 20)
	if !strings.Contains(b[0], "\x1b[91m") || !strings.Contains(b[1], "\x1b[30;101m") {
		t.Errorf("red accent: %q %q", b[0], b[1])
	}
	setAccent("nonsense")
	if accentFG != "94" {
		t.Error("an unknown accent falls back to blue")
	}
}

func TestThePaletteHasTwoShades(t *testing.T) {
	resetGlobals(t)
	dark := func() [4]string {
		setAccent("blue")
		return [4]string{accentFG, accentBG, accentDim, accentAlt}
	}
	before := dark()
	if want := [4]string{"94", "104", "34", "96"}; before != want {
		t.Errorf("on a dark terminal: %v, want %v", before, want)
	}
	if !setTerminalDark(false) {
		t.Fatal("a light background must re-tint the palette")
	}
	setAccent("blue")
	light := [4]string{accentFG, accentBG, accentDim, accentAlt}
	if want := [4]string{"34", "104", "94", "36"}; light != want {
		t.Errorf("on a light terminal: %v, want %v", light, want)
	}
	if light[0] == light[2] || light[0] == before[0] {
		t.Error("the light shade must be its own, and different from the dark one")
	}
	if setTerminalDark(false) {
		t.Error("the same background twice changes nothing")
	}
}

func TestPresetAccentsDoNotFollowTheTerminal(t *testing.T) {
	resetGlobals(t)
	setAccent("orange")
	preset := [4]string{accentFG, accentBG, accentDim, accentAlt}
	if want := [4]string{"38;5;208", "48;5;208", "38;5;130", "38;5;214"}; preset != want {
		t.Errorf("orange: %v, want %v", preset, want)
	}
	setTerminalDark(false)
	setAccent("orange")
	if got := [4]string{accentFG, accentBG, accentDim, accentAlt}; got != preset {
		t.Errorf("a preset must not move with the background: %v vs %v", got, preset)
	}
}

func TestHeadingsWalkTheAccent(t *testing.T) {
	resetGlobals(t)
	setAccent("blue")
	if want := [6]string{"94", "96", "92", "93", "95", "97"}; headColor != want {
		t.Errorf("blue walks %v, want %v", headColor, want)
	}
	setAccent("green")
	if headColor[0] != accentFG || headColor[1] != accentAlt {
		t.Errorf("the accent and its partner must lead: %v", headColor)
	}
	seen := map[string]bool{}
	for _, c := range headColor {
		if seen[c] {
			t.Errorf("a repeated heading color in %v", headColor)
		}
		seen[c] = true
	}
	setTerminalDark(false)
	setAccent("green")
	if strings.HasPrefix(headColor[0], "9") {
		t.Errorf("a light terminal wants the plain half: %v", headColor)
	}
}

func TestEveryAccentResolves(t *testing.T) {
	resetGlobals(t)
	for _, name := range accentNames {
		setAccent(name)
		if accentFG == "" || accentBG == "" || accentDim == "" || accentAlt == "" {
			t.Fatalf("%s: an empty code", name)
		}
		seen := map[string]bool{}
		for _, c := range headColor {
			if seen[c] {
				t.Fatalf("%s: a repeated heading color in %v", name, headColor)
			}
			seen[c] = true
		}
		if !strings.Contains(acc2("x"), accentAlt) {
			t.Errorf("%s: acc2 must use the partner color", name)
		}
		// the quiet end of the gradient and the bars drawn with it must be
		// visible: never plain black on a dark terminal
		if accentDim == "30" {
			t.Errorf("%s: the muted end is invisible on a dark terminal", name)
		}
	}
	cfg := defaultConfig()
	for _, name := range accentNames {
		cfg.Accent = name
		cfg.normalize()
		if cfg.Accent != name {
			t.Errorf("%s must be a valid config value", name)
		}
	}
	setTerminalDark(false)
	for _, name := range accentNames {
		setAccent(name)
		if accentFG == "" || headColor[0] == "" {
			t.Errorf("%s: the light shade is not resolved", name)
		}
	}
}

func TestConfigNormalizeClampsEverything(t *testing.T) {
	c := Config{Width: 5, Spacing: 9, WheelLines: 0, CacheDays: -3, HistoryLimit: 1, Links: "x", FooterProgress: "x",
		ScrollSpeed: "x", Accent: "x", SearchEngine: "x"}
	c.normalize()
	d := defaultConfig()
	if c.Width != 40 || c.Spacing != 2 || c.WheelLines != 1 || c.CacheDays != 1 || c.HistoryLimit != 50 ||
		c.Links != d.Links || c.FooterProgress != d.FooterProgress || c.ScrollSpeed != d.ScrollSpeed ||
		c.Accent != d.Accent || c.SearchEngine != d.SearchEngine {
		t.Errorf("%+v", c)
	}
	c = Config{Width: 0, SearchEngine: "https://s.test/?q=%s"}
	c.normalize()
	if c.Width != 0 || c.SearchEngine != "https://s.test/?q=%s" {
		t.Errorf("width 0 (full) and a custom search URL are valid: %+v", c)
	}
}

func TestConfigFileRoundTrip(t *testing.T) {
	isolateAll(t)
	c := defaultConfig()
	c.Width, c.Accent, c.Spacing, c.Footer, c.SearchEngine, c.Homepage = 0, "magenta", 2, false, "https://s.test/?q=%s", "https://home.test"
	setKeys(&c, actQuit, []string{"Q", "ctrl+q"})
	if err := saveConfig(c); err != nil {
		t.Fatal(err)
	}
	got, err := loadConfig()
	if err != nil || !reflect.DeepEqual(got, c) {
		t.Errorf("round trip failed:\n got %+v\nwant %+v (%v)", got, c, err)
	}
	text := configText(c)
	if !strings.Contains(text, "[keys]") || !strings.Contains(text, `quit = ["Q", "ctrl+q"]`) {
		t.Errorf("only changed keys are written:\n%s", text)
	}
	if strings.Contains(configText(defaultConfig()), "[keys]") {
		t.Error("the default config has no [keys] table")
	}
	path, _ := configPath()
	if fi, err := osStat(path); err != nil || fi.Mode().Perm() != 0o644 {
		t.Errorf("config perms: %v %v", fi, err)
	}
}

func osStat(p string) (os.FileInfo, error) { return os.Stat(p) }
func time0() time.Time                     { return time.Time{} }
