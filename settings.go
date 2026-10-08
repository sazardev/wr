package main

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// The Settings panel. Every option is a row: you move with the arrows, change
// it with ←/→ (or Enter), and the change applies at once and is saved to
// config.toml. d restores an option's default.

type optKind int

const (
	optBool optKind = iota
	optEnum
	optInt
	optAction
)

type option struct {
	group, label, help string
	kind               optKind
	rebuild            bool                   // the document must be drawn again
	get                func(c *Config) string // the value shown
	step               func(c *Config, dir int)
	reset              func(c *Config)
	run                func(m *model) tea.Cmd // for optAction rows
}

func boolOpt(group, label, help string, f func(*Config) *bool, rebuild bool) option {
	return option{group: group, label: label, help: help, kind: optBool, rebuild: rebuild,
		get: func(c *Config) string {
			if *f(c) {
				return "on"
			}
			return "off"
		},
		step:  func(c *Config, _ int) { p := f(c); *p = !*p },
		reset: func(c *Config) { d := defaultConfig(); *f(c) = *f(&d) },
	}
}

func enumOpt(group, label, help string, f func(*Config) *string, choices []string, rebuild bool) option {
	return option{group: group, label: label, help: help, kind: optEnum, rebuild: rebuild,
		get: func(c *Config) string { return *f(c) },
		step: func(c *Config, dir int) {
			p := f(c)
			i := 0
			for j, ch := range choices {
				if ch == *p {
					i = j
				}
			}
			*p = choices[((i+dir)%len(choices)+len(choices))%len(choices)]
		},
		reset: func(c *Config) { d := defaultConfig(); *f(c) = *f(&d) },
	}
}

// listOpt cycles an integer through a fixed list of sensible values.
func listOpt(group, label, help string, f func(*Config) *int, values []int, show func(int) string, rebuild bool) option {
	if show == nil {
		show = strconv.Itoa
	}
	return option{group: group, label: label, help: help, kind: optInt, rebuild: rebuild,
		get: func(c *Config) string { return show(*f(c)) },
		step: func(c *Config, dir int) {
			p := f(c)
			i := 0
			for j, v := range values { // the nearest listed value, then move
				if v <= *p {
					i = j
				}
			}
			*p = values[((i+dir)%len(values)+len(values))%len(values)]
		},
		reset: func(c *Config) { d := defaultConfig(); *f(c) = *f(&d) },
	}
}

func rangeOpt(group, label, help string, f func(*Config) *int, lo, hi int, rebuild bool) option {
	return option{group: group, label: label, help: help, kind: optInt, rebuild: rebuild,
		get: func(c *Config) string { return strconv.Itoa(*f(c)) },
		step: func(c *Config, dir int) {
			p := f(c)
			*p = min(max(*p+dir, lo), hi)
		},
		reset: func(c *Config) { d := defaultConfig(); *f(c) = *f(&d) },
	}
}

func actionOpt(group, label, help string, show func(m *model) string, run func(m *model) tea.Cmd) option {
	return option{group: group, label: label, help: help, kind: optAction, run: run,
		get: func(*Config) string { return "" }, step: func(*Config, int) {}, reset: func(*Config) {}}
}

var (
	widthValues   = []int{0, 60, 70, 80, 90, 100, 110, 120, 140, 160, 200}
	spacingValues = []int{0, 1, 2}
	daysValues    = []int{1, 7, 14, 30, 90, 180, 365}
	historyValues = []int{100, 250, 500, 1000, 2000, 5000}
)

func (m *model) settingsOptions() []option {
	L, F, Mo, An, B, C := "Layout", "Footer and scrollbar", "Mouse", "Motion", "Browsing", "Cache"

	engine := enumOpt(B, "Search engine", "Where plain words go when you type them in the address bar. Put your own URL (with %s) in config.toml for a custom one.",
		func(c *Config) *string { return &c.SearchEngine }, searchPresetNames, false)
	engine.get = func(c *Config) string {
		if _, ok := searchPresets[c.SearchEngine]; ok {
			return c.SearchEngine
		}
		return "custom"
	}

	home := actionOpt(B, "Start page", "Enter makes the current page the one wr opens when you start it without an address. On the start page it goes back to the start page.",
		nil, func(m *model) tea.Cmd {
			if m.src == "" || m.cfg.Homepage == m.src {
				m.cfg.Homepage = ""
				return tea.Batch(m.applyConfig(false), m.setToast("wr will open its start page", false))
			}
			m.cfg.Homepage = m.src
			return tea.Batch(m.applyConfig(false), m.setToast("this page is now your homepage", false))
		})
	home.get = func(c *Config) string {
		if c.Homepage == "" {
			return "start page"
		}
		return hostOf(c.Homepage)
	}
	home.reset = func(c *Config) { c.Homepage = "" }

	return []option{
		listOpt(L, "Reading width", "Maximum width of the text column. Full uses the whole terminal.",
			func(c *Config) *int { return &c.Width }, widthValues, func(v int) string {
				if v == 0 {
					return "full"
				}
				return strconv.Itoa(v)
			}, true),
		boolOpt(L, "Center the column", "Center the text column in a wide terminal.", func(c *Config) *bool { return &c.Center }, false),
		listOpt(L, "Spacing", "Blank lines between paragraphs and blocks: compact, normal or airy.",
			func(c *Config) *int { return &c.Spacing }, spacingValues, func(v int) string { return []string{"compact", "normal", "airy"}[min(max(v, 0), 2)] }, true),
		boolOpt(L, "Heading rules", "A thin rule under level 1 and 2 headings.", func(c *Config) *bool { return &c.HeadingRules }, true),
		boolOpt(L, "Code frames", "The label and frame around code blocks.", func(c *Config) *bool { return &c.CodeFrame }, true),
		enumOpt(L, "Links", "Footnotes: numbered and listed at the end. Inline: the address next to the text. Hidden: text only. Links stay clickable in every mode.",
			func(c *Config) *string { return &c.Links }, linkModes, true),
		boolOpt(L, "Braille decoration", "Heading marks, scrollbar, progress bar and spinner drawn in braille. Turn off if your font lacks it.", func(c *Config) *bool { return &c.Braille }, true),
		enumOpt(L, "Accent color", "The color of panels, shortcut keys, marks and bars. One of your terminal's 16 colors.", func(c *Config) *string { return &c.Accent }, accentNames, true),

		boolOpt(F, "Scrollbar", "The scrollbar on the right edge.", func(c *Config) *bool { return &c.Scrollbar }, false),
		boolOpt(F, "Footer", "The footer row. Turn it off for a clean, full-height page.", func(c *Config) *bool { return &c.Footer }, false),
		boolOpt(F, "Footer rule", "The thin rule above the footer.", func(c *Config) *bool { return &c.FooterRule }, false),
		boolOpt(F, "Shortcut hints", "The shortcuts on the left of the footer.", func(c *Config) *bool { return &c.FooterHints }, false),
		enumOpt(F, "Reading progress", "What the footer shows on the right: bar and percentage, only one of them, or nothing.",
			func(c *Config) *string { return &c.FooterProgress }, progressModes, false),
		boolOpt(F, "Footer notes", "Small notes next to the progress: bookmark star, newer version, search count.", func(c *Config) *bool { return &c.FooterChips }, false),

		boolOpt(Mo, "Mouse support", "Wheel scrolling, clicking links, drag to select. Off hands the mouse entirely to your terminal.", func(c *Config) *bool { return &c.Mouse }, false),
		boolOpt(Mo, "Copy on select", "Copy as soon as you release the mouse. Off: select, then press y to copy.", func(c *Config) *bool { return &c.CopyOnSelect }, false),
		rangeOpt(Mo, "Wheel lines", "Lines scrolled per wheel notch.", func(c *Config) *int { return &c.WheelLines }, 1, 10, false),

		boolOpt(An, "Animations", "Master switch for every animation. Off makes everything instant.", func(c *Config) *bool { return &c.Animations }, false),
		boolOpt(An, "Smooth scrolling", "Spring-physics scrolling.", func(c *Config) *bool { return &c.AnimScroll }, false),
		enumOpt(An, "Scroll speed", "How quickly smooth scrolling catches up.", func(c *Config) *string { return &c.ScrollSpeed }, speedModes, false),
		boolOpt(An, "Page reveal", "The page wiping in when it loads.", func(c *Config) *bool { return &c.AnimReveal }, false),
		boolOpt(An, "Panel animation", "Panels opening like a shutter.", func(c *Config) *bool { return &c.AnimPanels }, false),
		boolOpt(An, "Loading wave", "The braille wave while a page downloads.", func(c *Config) *bool { return &c.AnimLoading }, false),
		boolOpt(An, "Typewriter notices", "Notices typing themselves out.", func(c *Config) *bool { return &c.AnimNotices }, false),

		engine,
		home,
		boolOpt(B, "Remember history", "Keep the pages you visit (the History panel and address-bar suggestions).", func(c *Config) *bool { return &c.History }, false),
		listOpt(B, "History size", "How many visits to keep.", func(c *Config) *int { return &c.HistoryLimit }, historyValues, nil, false),
		boolOpt(B, "Refresh in the background", "Refresh a cached page while you read it, and tell you if it changed.", func(c *Config) *bool { return &c.BackgroundRefresh }, false),

		boolOpt(C, "Cache pages", "Keep pages on disk so a page you already read opens instantly.", func(c *Config) *bool { return &c.Cache }, false),
		listOpt(C, "Keep pages for", "Days before a cached page is forgotten.", func(c *Config) *int { return &c.CacheDays }, daysValues, func(v int) string { return fmt.Sprintf("%d days", v) }, false),

		actionOpt("Other", "Keyboard shortcuts…", "See and change every key binding.", nil, func(m *model) tea.Cmd {
			m.mode, m.sel = modeKeys, 0
			return nil
		}),
		actionOpt("Other", "Reset all settings…", "Put every setting back to its default (your key bindings are kept).", nil, func(m *model) tea.Cmd {
			m.mode, m.confirm = modeConfirm, confirmSettings
			return nil
		}),
	}
}

// applyConfig makes m.cfg take effect and saves it. Called after every change
// made from a panel.
func (m *model) applyConfig(rebuild bool) tea.Cmd {
	m.cfg.normalize()
	m.keymap = buildKeymap(&m.cfg)
	applyRuntime(m.cfg)
	if rebuild {
		m.rebuild()
	}
	m.clampY()
	save := m.cfg
	if m.linksFromFlag { // -L was a one-session choice: do not write it to the file
		save.Links = m.fileLinks
	}
	if err := saveConfig(save); err != nil {
		return m.setToast("could not save settings: "+err.Error(), true)
	}
	return nil
}

func (m *model) changeOption(o option, dir int) tea.Cmd {
	if o.kind == optAction {
		return o.run(m)
	}
	o.step(&m.cfg, dir)
	if o.label == "Links" {
		m.linksFromFlag = false
	}
	return m.applyConfig(o.rebuild)
}

func (m *model) keySettings(k string) (tea.Model, tea.Cmd) {
	opts := m.settingsOptions()
	n := len(opts)
	switch k {
	case "esc", "q":
		m.mode = modeRead
	case "up", "k":
		m.sel = (m.sel - 1 + n) % n
	case "down", "j":
		m.sel = (m.sel + 1) % n
	case "pgup":
		m.sel = max(m.sel-6, 0)
	case "pgdown":
		m.sel = min(m.sel+6, n-1)
	case "home", "g":
		m.sel = 0
	case "end", "G":
		m.sel = n - 1
	case "left", "h":
		return m, m.changeOption(opts[m.sel], -1)
	case "right", "l", "enter", "space", " ":
		return m, m.changeOption(opts[m.sel], +1)
	case "d", "backspace": // back to this option's default
		o := opts[m.sel]
		if o.kind != optAction {
			o.reset(&m.cfg)
			return m, m.applyConfig(o.rebuild)
		}
	}
	return m, nil
}

// ---- keyboard shortcuts panel ----

func (m *model) keyKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	n := len(actionList)
	if m.capture != "" { // waiting for the key to bind
		a := action(m.capture)
		m.capture = ""
		switch k {
		case "esc":
			return m, nil
		case "ctrl+c":
			return m, m.setToast("ctrl+c always quits and cannot be rebound", true)
		}
		keys := []string{k}
		if m.captureAdd {
			keys = append(append([]string{}, effectiveKeys(&m.cfg, a)...), k)
		}
		setKeys(&m.cfg, a, dedupe(keys))
		return m, tea.Batch(m.applyConfig(false), m.setToast(fmt.Sprintf("%s → %s", labelOf(a), prettyKey(k)), false))
	}
	switch k {
	case "esc", "q":
		m.mode = modeRead
	case "up", "k":
		m.sel = (m.sel - 1 + n) % n
	case "down", "j":
		m.sel = (m.sel + 1) % n
	case "pgup":
		m.sel = max(m.sel-6, 0)
	case "pgdown":
		m.sel = min(m.sel+6, n-1)
	case "enter", "space", " ", "right", "l": // rebind
		m.capture, m.captureAdd = string(actionList[m.sel].id), false
	case "a": // add another key
		m.capture, m.captureAdd = string(actionList[m.sel].id), true
	case "x": // unbind
		setKeys(&m.cfg, actionList[m.sel].id, []string{})
		return m, m.applyConfig(false)
	case "d", "backspace": // back to this action's default keys
		id := actionList[m.sel].id
		setKeys(&m.cfg, id, append([]string{}, defaultKeys[id]...))
		return m, m.applyConfig(false)
	case "R":
		resetKeys(&m.cfg)
		return m, tea.Batch(m.applyConfig(false), m.setToast("key bindings reset", false))
	}
	return m, nil
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func labelOf(a action) string {
	for _, i := range actionList {
		if i.id == a {
			return strings.ToLower(i.label)
		}
	}
	return string(a)
}
