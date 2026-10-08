package main

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestDefaultKeysAreUniqueAndComplete(t *testing.T) {
	owner := map[string]action{}
	for _, info := range actionList {
		keys, ok := defaultKeys[info.id]
		if !ok || len(keys) == 0 {
			t.Errorf("%s has no default key", info.id)
		}
		for _, k := range keys {
			if prev, dup := owner[k]; dup {
				t.Errorf("key %q is bound to both %s and %s", k, prev, info.id)
			}
			owner[k] = info.id
		}
	}
	if len(defaultKeys) != len(actionList) {
		t.Errorf("%d default entries for %d actions", len(defaultKeys), len(actionList))
	}
	for _, reserved := range []string{"esc", "ctrl+c"} {
		if _, taken := owner[reserved]; taken {
			t.Errorf("%q is reserved and must not be a default binding", reserved)
		}
	}
}

func TestBuildKeymapAndOverrides(t *testing.T) {
	c := defaultConfig()
	km := buildKeymap(&c)
	if km["q"] != actQuit || km["j"] != actDown || km["H"] != actBack || km["tab"] != actNextLink {
		t.Errorf("defaults: %v", km)
	}
	c.Keys = map[string][]string{"quit": {"Q"}, "down": {}}
	km = buildKeymap(&c)
	if km["Q"] != actQuit || km["q"] != "" || km["j"] != "" || km["down"] != "" {
		t.Errorf("overrides replace the defaults, and an empty list unbinds: %v", km)
	}
	if got := keyList(&c, actDown); got != "unbound" {
		t.Errorf("keyList: %q", got)
	}
}

func TestSetKeysStealsFromOtherActionsAndDropsDefaults(t *testing.T) {
	c := defaultConfig()
	setKeys(&c, actBack, []string{"j"})
	if got := effectiveKeys(&c, actDown); !reflect.DeepEqual(got, []string{"down"}) {
		t.Errorf("down must lose j: %v", got)
	}
	if got := effectiveKeys(&c, actBack); !reflect.DeepEqual(got, []string{"j"}) {
		t.Errorf("back: %v", got)
	}
	km := buildKeymap(&c)
	if km["j"] != actBack {
		t.Errorf("j is now back: %v", km["j"])
	}
	// setting an action back to its defaults removes its override
	setKeys(&c, actBack, append([]string{}, defaultKeys[actBack]...))
	setKeys(&c, actDown, append([]string{}, defaultKeys[actDown]...))
	if len(c.Keys) != 0 {
		t.Errorf("no overrides should remain: %v", c.Keys)
	}
	resetKeys(&c)
	if c.Keys != nil {
		t.Error("reset clears all overrides")
	}
}

func TestPrettyKeys(t *testing.T) {
	for k, want := range map[string]string{"down": "↓", "ctrl+f": "^F", "shift+tab": "⇧tab", "enter": "↵", "j": "j", "alt+left": "⌥←", "space": "space"} {
		if got := prettyKey(k); got != want {
			t.Errorf("%q -> %q want %q", k, got, want)
		}
	}
}

func TestKeysPanelRebindFlow(t *testing.T) {
	m := newTestModel(t, 100, 70)
	m.mode, m.sel = modeKeys, 0
	for i, a := range actionList {
		if a.id == actQuit {
			m.sel = i
		}
	}
	press(m, "enter")
	if m.capture != string(actQuit) {
		t.Fatalf("enter starts capturing: %q", m.capture)
	}
	if !strings.Contains(ansi.Strip(m.render()), "press a key") {
		t.Error("the panel must say it is waiting for a key")
	}
	press(m, "Q")
	if m.capture != "" {
		t.Error("capture ends after one key")
	}
	if got := effectiveKeys(&m.cfg, actQuit); !reflect.DeepEqual(got, []string{"Q"}) {
		t.Errorf("quit: %v", got)
	}
	if m.keymap["Q"] != actQuit || m.keymap["q"] != "" {
		t.Error("the live keymap must update")
	}
	if saved, _ := loadConfig(); !reflect.DeepEqual(saved.Keys["quit"], []string{"Q"}) {
		t.Errorf("the binding must be saved: %v", saved.Keys)
	}

	// the new key works while reading, the old one no longer does
	m.mode = modeRead
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'Q', Text: "Q"}); cmd == nil {
		t.Error("Q now quits")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); cmd != nil {
		t.Error("q no longer quits")
	}
	// and the footer shows the keys you actually have
	if r := lastRow(m); !strings.Contains(r, "Q quit") {
		t.Errorf("the hints follow your bindings: %q", r)
	}
}

func TestKeysPanelAddUnbindDefaultsAndReset(t *testing.T) {
	m := newTestModel(t, 100, 70)
	idx := func(a action) int {
		for i, x := range actionList {
			if x.id == a {
				return i
			}
		}
		return -1
	}
	m.mode, m.sel = modeKeys, idx(actSearch)
	press(m, "a", "s")
	if got := effectiveKeys(&m.cfg, actSearch); !reflect.DeepEqual(got, []string{"/", "s"}) {
		t.Errorf("a adds another key: %v", got)
	}
	press(m, "x")
	if got := effectiveKeys(&m.cfg, actSearch); len(got) != 0 {
		t.Errorf("x unbinds: %v", got)
	}
	press(m, "d")
	if got := effectiveKeys(&m.cfg, actSearch); !reflect.DeepEqual(got, defaultKeys[actSearch]) {
		t.Errorf("d restores the defaults: %v", got)
	}
	setKeys(&m.cfg, actQuit, []string{"Q"})
	press(m, "R")
	if m.cfg.Keys != nil {
		t.Error("R resets every binding")
	}
}

func TestKeyCaptureRules(t *testing.T) {
	m := newTestModel(t, 100, 70)
	m.mode, m.sel = modeKeys, 0
	press(m, "enter", "esc")
	if m.capture != "" || m.cfg.Keys != nil {
		t.Error("esc cancels a capture without changing anything")
	}
	press(m, "enter")
	m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if m.cfg.Keys != nil || !strings.Contains(m.toast, "cannot be rebound") {
		t.Errorf("ctrl+c is reserved: keys=%v toast=%q", m.cfg.Keys, m.toast)
	}
	// ctrl+c quits even from a panel when nothing is being captured
	m.mode = modeKeys
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}); cmd == nil {
		t.Error("ctrl+c always quits")
	}
}

func TestReadingKeysFollowTheKeymap(t *testing.T) {
	m := newTestModel(t, 100, 30)
	setKeys(&m.cfg, actTop, []string{"T"})
	m.keymap = buildKeymap(&m.cfg)
	press(m, "G")
	if m.y == 0 {
		t.Fatal("G goes to the bottom")
	}
	press(m, "T")
	if m.y != 0 {
		t.Errorf("T is now top: y=%d", m.y)
	}
	press(m, "g") // no longer bound to anything
	setKeys(&m.cfg, actDown, []string{"n"})
	m.keymap = buildKeymap(&m.cfg)
	before := m.y
	press(m, "n")
	if m.y != before+1 {
		t.Errorf("n now scrolls down: y=%d", m.y)
	}
}

func TestEveryActionDoesSomething(t *testing.T) {
	// each default binding must reach a handler (no action is bound but ignored)
	a, _ := site(t)
	for _, info := range actionList {
		if info.id == actQuit {
			continue
		}
		m := browseModel(t, a)
		key := defaultKeys[info.id][0]
		var msg tea.KeyPressMsg
		switch key {
		case "space":
			msg = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "tab":
			msg = tea.KeyPressMsg{Code: tea.KeyTab}
		case "shift+tab":
			msg = tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
		case "backspace":
			msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
		default:
			msg = tea.KeyPressMsg{Code: []rune(key)[0], Text: key}
		}
		if got := m.keymap[msg.String()]; got != info.id {
			t.Errorf("%s: the key %q reaches %q", info.id, key, got)
		}
	}
}
