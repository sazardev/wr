package main

import (
	"strings"
	"testing"
)

func typeText(m *model, s string) {
	for _, r := range s {
		press(m, string(r))
	}
}

func filterModel(t *testing.T) *model {
	t.Helper()
	return newTestModel(t, 110, 40)
}

func TestMatchesFilterIsWordBasedAndCaseInsensitive(t *testing.T) {
	for _, c := range []struct {
		hay, q string
		want   bool
	}{
		{"Clear all cache", "cache", true},
		{"Clear all cache", "CLEAR cache", true},
		{"Clear all cache", "cache clear", true}, // any order
		{"Clear all cache", "cache bookmarks", false},
		{"Settings…", "", true},
		{"Settings…", "  ", true},
	} {
		if got := matchesFilter(c.hay, c.q); got != c.want {
			t.Errorf("matchesFilter(%q, %q) = %v, want %v", c.hay, c.q, got, c.want)
		}
	}
}

func TestSlashFiltersTheMenuAndEnterRunsTheEntry(t *testing.T) {
	m := filterModel(t)
	press(m, "m", "/")
	if !m.pf.typing {
		t.Fatal("/ must start the filter")
	}
	typeText(m, "history")
	s := screen(m)
	if !strings.Contains(s, "History…") || strings.Contains(s, "Section index") || strings.Contains(s, "Reload page") {
		t.Errorf("only the matching entries may stay:\n%s", s)
	}
	if !strings.Contains(s, "/ history") {
		t.Errorf("the filter prompt must show what was typed:\n%s", s)
	}
	press(m, "enter")
	if m.mode != modeList || m.list.kind != listHistory {
		t.Errorf("enter must run the entry found: mode=%v", m.mode)
	}
}

func TestEscClearsTheFilterBeforeClosingThePanel(t *testing.T) {
	m := filterModel(t)
	press(m, "m", "/")
	typeText(m, "zzzz")
	if s := screen(m); !strings.Contains(s, "nothing matches") {
		t.Errorf("no match must say so:\n%s", s)
	}
	press(m, "enter") // nothing to run: stays open
	if m.mode != modeMenu {
		t.Fatalf("enter on an empty result must not close or run anything: mode=%v", m.mode)
	}
	press(m, "esc")
	if m.mode != modeMenu || m.pf.text != "" {
		t.Errorf("the first esc only clears the filter: mode=%v text=%q", m.mode, m.pf.text)
	}
	if !strings.Contains(screen(m), "Section index") {
		t.Error("clearing the filter must bring every entry back")
	}
	press(m, "esc")
	if m.mode != modeRead {
		t.Error("the second esc closes the menu")
	}
}

func TestTypingQOrJInTheFilterIsTextNotACommand(t *testing.T) {
	m := filterModel(t)
	press(m, "m", "/")
	typeText(m, "quit")
	if m.mode != modeMenu || m.pf.text != "quit" {
		t.Fatalf("q must be typed, not quit the menu: mode=%v text=%q", m.mode, m.pf.text)
	}
	if s := screen(m); !strings.Contains(s, "Quit") || strings.Contains(s, "Reload page") {
		t.Errorf("filtering by 'quit':\n%s", s)
	}
	press(m, "backspace", "backspace", "backspace", "backspace")
	press(m, "backspace") // on an empty filter backspace leaves typing mode
	if m.pf.typing {
		t.Error("backspace on an empty filter must stop typing")
	}
}

func TestSettingsFilterFindsByNameGroupAndHelp(t *testing.T) {
	m := filterModel(t)
	press(m, ",", "/")
	typeText(m, "braille")
	s := screen(m)
	if !strings.Contains(s, "Braille decoration") || strings.Contains(s, "Mouse support") {
		t.Errorf("filter by name:\n%s", s)
	}
	press(m, "esc", "/")
	typeText(m, "cache")
	s = screen(m)
	if !strings.Contains(s, "Cache pages") || !strings.Contains(s, "Keep pages for") || strings.Contains(s, "Spacing") {
		t.Errorf("filter by group/name:\n%s", s)
	}
	if !strings.Contains(s, "Cache") || strings.Contains(s, "Motion") {
		t.Errorf("only groups with matches keep their header:\n%s", s)
	}
}

func TestSettingsEnterAcceptsTheFilterThenChangesTheFoundOption(t *testing.T) {
	m := filterModel(t)
	press(m, ",", "/")
	typeText(m, "braille")
	press(m, "enter")
	if m.pf.typing || m.pf.text != "braille" {
		t.Fatalf("enter accepts the filter and keeps the results: typing=%v text=%q", m.pf.typing, m.pf.text)
	}
	if m.mode != modeSettings {
		t.Fatal("still in Settings")
	}
	before := m.cfg.Braille
	press(m, "enter") // now acts on the entry found
	if m.cfg.Braille == before {
		t.Error("the found option must toggle")
	}
	press(m, "esc")
	if m.pf.text != "" || m.mode != modeSettings {
		t.Errorf("esc clears the results first: mode=%v text=%q", m.mode, m.pf.text)
	}
}

func TestSelectionKeepsItsEntryWhenTheFilterIsCleared(t *testing.T) {
	m := filterModel(t)
	press(m, ",", "/")
	typeText(m, "cache")
	press(m, "enter")
	found := m.sel
	if !matchesFilter(m.panelHay(found), "cache") {
		t.Fatalf("selection must land on a match, got %q", m.settingsOptions()[found].label)
	}
	press(m, "esc")
	if m.sel != found {
		t.Errorf("clearing the filter must leave the same entry selected: %d -> %d", found, m.sel)
	}
}

func TestArrowsStepOverHiddenEntries(t *testing.T) {
	m := filterModel(t)
	press(m, ",", "/")
	typeText(m, "cache")
	press(m, "enter")
	vis := m.visibleEntries()
	if len(vis) < 2 {
		t.Fatalf("expected several cache entries, got %d", len(vis))
	}
	m.sel = vis[0]
	press(m, "down")
	if m.sel != vis[1] {
		t.Errorf("down must go to the next visible entry (%d), got %d", vis[1], m.sel)
	}
	press(m, "up", "up")
	if m.sel != vis[len(vis)-1] {
		t.Errorf("up wraps among the visible entries: %d", m.sel)
	}
}

func TestShortcutsPanelFiltersButCaptureKeepsSlashBindable(t *testing.T) {
	m := filterModel(t)
	press(m, "m")
	pickMenu(t, m, "Keyboard shortcuts")
	press(m, "/")
	typeText(m, "bookmark")
	s := screen(m)
	if !strings.Contains(s, "Bookmark this page") || strings.Contains(s, "Line down") {
		t.Errorf("shortcut filter:\n%s", s)
	}
	press(m, "esc")
	if m.pf.text != "" {
		t.Fatal("filter should be clear")
	}
	// capture mode: "/" is a key like any other
	m.sel = 0
	press(m, "enter")
	if m.capture == "" {
		t.Fatal("enter must start capturing a key")
	}
	press(m, "/")
	if got := effectiveKeys(&m.cfg, actionList[0].id); len(got) != 1 || got[0] != "/" {
		t.Errorf("'/' must be bindable while capturing, got %v", got)
	}
	if m.pf.typing {
		t.Error("capturing a key must not open the filter")
	}
}

func TestSectionIndexFilterJumpsOnEnter(t *testing.T) {
	m := filterModel(t)
	press(m, "t", "/")
	typeText(m, "section 30")
	if s := screen(m); !strings.Contains(s, "Section 30") || strings.Contains(s, "Section 2\n") {
		t.Errorf("index filter:\n%s", s)
	}
	press(m, "enter")
	if m.mode != modeRead {
		t.Fatalf("enter in the index jumps and closes: mode=%v", m.mode)
	}
	if m.y == 0 {
		t.Error("the view must have moved to the section")
	}
}

func TestFilterIsResetEveryTimeAPanelOpens(t *testing.T) {
	m := filterModel(t)
	press(m, "m", "/")
	typeText(m, "hist")
	press(m, "esc", "esc") // clear, then close
	press(m, "m")
	if m.pf.typing || m.pf.text != "" {
		t.Errorf("a freshly opened panel starts unfiltered: %+v", m.pf)
	}
	// and a different panel never inherits it
	press(m, "/")
	typeText(m, "x")
	press(m, "esc", "esc")
	press(m, ",")
	if m.pf.text != "" || m.pf.typing {
		t.Errorf("Settings must not inherit the menu's filter: %+v", m.pf)
	}
}

func TestFooterHintsFollowTheFilterState(t *testing.T) {
	m := filterModel(t)
	press(m, "m")
	if !strings.Contains(lastRow(m), "/ find") {
		t.Errorf("a filterable panel advertises '/': %q", lastRow(m))
	}
	press(m, "/")
	if !strings.Contains(lastRow(m), "to filter") || !strings.Contains(lastRow(m), "esc clear") {
		t.Errorf("typing hints: %q", lastRow(m))
	}
	typeText(m, "set")
	press(m, "down")
	// still typing: hints unchanged
	if !strings.Contains(lastRow(m), "to filter") {
		t.Errorf("hints while typing: %q", lastRow(m))
	}
	// panels without the filter keep the plain hints
	press(m, "esc", "esc")
	pickMenu(t, m, "Clear all cache")
	if strings.Contains(lastRow(m), "/ find") {
		t.Errorf("the confirm box has no filter: %q", lastRow(m))
	}
}

func TestNumbersStillPickMenuEntriesWhenNotFiltering(t *testing.T) {
	m := filterModel(t)
	press(m, "m")
	if !strings.Contains(screen(m), "1  Open address or search") {
		t.Errorf("unfiltered menu keeps its numbers:\n%s", screen(m))
	}
	press(m, "/")
	typeText(m, "back")
	if strings.Contains(screen(m), "1  Back") {
		t.Errorf("numbers are hidden while filtering (digits are text now):\n%s", screen(m))
	}
}
