package main

import (
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

const selDoc = "# Title\n\n" +
	"alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron pi rho sigma tau upsilon\n\n" +
	"```go\nfunc main() {\n\tprintln(\"hello\")\n}\n```\n\n" +
	"- first item\n- second item\n"

const paragraph = "alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron pi rho sigma tau upsilon"

// selModel is a model with a narrow reading column (so the paragraph wraps) and
// a fake clipboard that records what would be copied.
func selModel(t *testing.T) (*model, *[]string) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	cfg := defaultConfig()
	cfg.Animations, cfg.Width = false, 40
	m := newModel("https://t.test/x", cfg, false, nil)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(loadedMsg{md: selDoc})

	var mu sync.Mutex
	var copied []string
	old := clipboardExternal
	clipboardExternal = func(s string) error { mu.Lock(); copied = append(copied, s); mu.Unlock(); return nil }
	t.Cleanup(func() { clipboardExternal = old })
	return m, &copied
}

// lineOf finds the first document line containing text.
func lineOf(t *testing.T, m *model, text string) int {
	t.Helper()
	for i, l := range m.doc.Plain {
		if strings.Contains(l, text) {
			return i
		}
	}
	t.Fatalf("no line contains %q in:\n%s", text, strings.Join(m.doc.Plain, "\n"))
	return -1
}

// screenPos converts a document position to a screen position.
func screenPos(m *model, p point) (x, y int) {
	_, left, _, _ := m.geometry()
	r := []rune(m.doc.Plain[p.line])
	return left + ansi.StringWidth(string(r[:min(p.col, len(r))])), p.line - m.viewY()
}

func click(m *model, p point) tea.Cmd {
	x, y := screenPos(m, p)
	_, cmd := m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	return cmd
}

func dragTo(m *model, p point) {
	x, y := screenPos(m, p)
	m.Update(tea.MouseMotionMsg{X: x, Y: y, Button: tea.MouseLeft})
}

func release(m *model, p point) tea.Cmd {
	x, y := screenPos(m, p)
	_, cmd := m.Update(tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
	return cmd
}

// drag selects from a to b and returns the commands produced on release.
func drag(m *model, a, b point) tea.Cmd {
	click(m, a)
	dragTo(m, b)
	return release(m, b)
}

func endOf(m *model, line int) point {
	return point{line, utf8.RuneCountInString(m.doc.Plain[line]) - 1}
}

func TestSelectionCopiesWrappedParagraphAsOneLine(t *testing.T) {
	m, _ := selModel(t)
	first := lineOf(t, m, "alpha")
	last := lineOf(t, m, "upsilon")
	if last == first {
		t.Fatal("the paragraph was expected to wrap")
	}
	drag(m, point{first, 0}, endOf(m, last))
	if got := m.selectedText(); got != paragraph {
		t.Errorf("a wrapped paragraph must copy as written:\n got %q\nwant %q", got, paragraph)
	}
}

func TestSelectionCopiesCodeWithoutFrameOrGutter(t *testing.T) {
	m, _ := selModel(t)
	top := lineOf(t, m, "╭─ go")
	bottom := lineOf(t, m, "╰─")
	drag(m, point{top, 0}, endOf(m, bottom))
	want := "func main() {\n    println(\"hello\")\n}"
	if got := m.selectedText(); got != want {
		t.Errorf("code must copy without its frame:\n got %q\nwant %q", got, want)
	}
	if strings.ContainsAny(m.selectedText(), "│╭╰") {
		t.Error("frame characters leaked into the copied text")
	}
}

func TestSelectionInsideOneCodeLine(t *testing.T) {
	m, _ := selModel(t)
	l := lineOf(t, m, "println")
	start := strings.Index(m.doc.Plain[l], "println")
	r := []rune(m.doc.Plain[l])
	startCol := utf8.RuneCountInString(m.doc.Plain[l][:start])
	drag(m, point{l, startCol}, point{l, startCol + 6})
	if got := m.selectedText(); got != "println" {
		t.Errorf("got %q", got)
	}
	_ = r
}

func TestDoubleClickSelectsAWordAndCopies(t *testing.T) {
	m, copied := selModel(t)
	l := lineOf(t, m, "alpha")
	click(m, point{l, 1}) // first click
	release(m, point{l, 1})
	cmd := click(m, point{l, 1}) // second click on the same cell, within the double-click window
	if got := m.selectedText(); got != "alpha" {
		t.Errorf("double click must select the word: %q", got)
	}
	if cmd == nil {
		t.Error("a double click must copy at once")
	}
	if !strings.Contains(m.toast, "copied 5 characters") {
		t.Errorf("toast=%q", m.toast)
	}
	_ = copied
}

func TestTripleClickSelectsTheLine(t *testing.T) {
	m, _ := selModel(t)
	l := lineOf(t, m, "first item")
	for i := 0; i < 3; i++ {
		click(m, point{l, 4})
		release(m, point{l, 4})
	}
	if got := m.selectedText(); !strings.Contains(got, "first item") || strings.Contains(got, "second") {
		t.Errorf("triple click must select exactly the line: %q", got)
	}
}

func TestPlainClickSelectsNothing(t *testing.T) {
	m, _ := selModel(t)
	l := lineOf(t, m, "alpha")
	click(m, point{l, 3})
	if cmd := release(m, point{l, 3}); cmd != nil {
		t.Error("a plain click must not copy")
	}
	if m.mouseSel.active || m.selectedText() != "" {
		t.Error("a plain click must not leave a selection")
	}
}

func TestDragCopiesOnReleaseAndToasts(t *testing.T) {
	m, _ := selModel(t)
	l := lineOf(t, m, "alpha")
	cmd := drag(m, point{l, 0}, point{l, 4})
	if cmd == nil {
		t.Fatal("releasing a selection must produce the copy commands")
	}
	if !strings.Contains(m.toast, "copied 5 characters") {
		t.Errorf("toast=%q", m.toast)
	}
}

func TestSelectionIsHighlightedAndCleared(t *testing.T) {
	m, _ := selModel(t)
	l := lineOf(t, m, "alpha")
	drag(m, point{l, 0}, point{l, 4})
	if !strings.Contains(m.render(), selOpen) {
		t.Error("the selection must be painted")
	}
	press(m, "esc")
	if m.mouseSel.active || strings.Contains(m.render(), selOpen) {
		t.Error("esc must clear the selection")
	}
}

func TestYankCopiesTheSelectionAgain(t *testing.T) {
	m, _ := selModel(t)
	l := lineOf(t, m, "alpha")
	drag(m, point{l, 0}, point{l, 4})
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}); cmd == nil {
		t.Error("y must copy the current selection")
	}
	press(m, "esc")
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"}); cmd != nil {
		t.Error("y without a selection must do nothing")
	}
}

func TestDraggingPastTheEdgeScrolls(t *testing.T) {
	m, _ := selModel(t)
	// a long document so there is something to scroll to
	m.Update(loadedMsg{md: longDoc()})
	l := m.viewY() + 2
	click(m, point{l, 2})
	before := m.y
	_, _, bodyH, _ := m.geometry()
	m.Update(tea.MouseMotionMsg{X: 20, Y: bodyH + 2, Button: tea.MouseLeft}) // below the body
	if m.y != before+1 {
		t.Errorf("dragging below the body must scroll down: y=%d (was %d)", m.y, before)
	}
}

func TestSelectionClearedOnRebuild(t *testing.T) {
	m, _ := selModel(t)
	l := lineOf(t, m, "alpha")
	drag(m, point{l, 0}, point{l, 4})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30}) // same reading width: layout unchanged
	if !m.mouseSel.active {
		t.Error("a resize that does not change the layout must keep the selection")
	}
	m.Update(tea.WindowSizeMsg{Width: 30, Height: 30}) // narrower column: lines re-wrap
	if m.mouseSel.active {
		t.Error("a re-wrap invalidates the selection positions: it must clear")
	}
}

func TestWheelStillScrolls(t *testing.T) {
	m, _ := selModel(t)
	m.Update(loadedMsg{md: longDoc()})
	m.Update(tea.MouseWheelMsg{X: 10, Y: 5, Button: tea.MouseWheelDown})
	if m.y == 0 {
		t.Error("the wheel must keep scrolling")
	}
}

func TestClickOnFooterOrOutsideDoesNothing(t *testing.T) {
	m, _ := selModel(t)
	if cmd := m.mouseDown(5, 29); cmd != nil || m.mouseSel.dragging {
		t.Error("clicking the footer must not start a selection")
	}
}

func TestRuneAtCellHandlesWideCharacters(t *testing.T) {
	// "a" 1 cell, "世" 2 cells, "b" 1 cell
	for cell, want := range map[int]int{0: 0, 1: 1, 2: 1, 3: 2, 4: 3, 9: 3} {
		if got := runeAtCell("a世b", cell); got != want {
			t.Errorf("cell %d -> rune %d, want %d", cell, got, want)
		}
	}
}

func TestWordBounds(t *testing.T) {
	for _, c := range []struct {
		s        string
		col      int
		from, to int
	}{
		{"hello world", 1, 0, 5},
		{"hello world", 6, 6, 11},
		{"hello   world", 6, 5, 8}, // a run of spaces
		{"a_b-c", 1, 0, 3},         // underscore belongs to the word
		{"foo.bar", 3, 3, 4},       // punctuation: itself
		{"", 0, 0, 0},
	} {
		if f, e := wordBounds(c.s, c.col); f != c.from || e != c.to {
			t.Errorf("wordBounds(%q,%d)=(%d,%d) want (%d,%d)", c.s, c.col, f, e, c.from, c.to)
		}
	}
}

func TestEncodeUTF16LE(t *testing.T) {
	got := encodeUTF16LE("a😀")
	want := []byte{'a', 0, 0x3d, 0xd8, 0x00, 0xde} // no BOM: clip.exe would keep it as a character
	if string(got) != string(want) {
		t.Errorf("got % x want % x", got, want)
	}
}

func TestSelectionSpanOn(t *testing.T) {
	s := selection{active: true, a: point{2, 3}, b: point{4, 1}}
	if sp, ok := s.spanOn(2, 10); !ok || sp != (span{3, 10}) {
		t.Errorf("first line: %v %v", sp, ok)
	}
	if sp, ok := s.spanOn(3, 10); !ok || sp != (span{0, 10}) {
		t.Errorf("middle line: %v %v", sp, ok)
	}
	if sp, ok := s.spanOn(4, 10); !ok || sp != (span{0, 2}) { // focus cell is inclusive
		t.Errorf("last line: %v %v", sp, ok)
	}
	if _, ok := s.spanOn(5, 10); ok {
		t.Error("outside the selection")
	}
	// the same selection made backwards is identical
	r := selection{active: true, a: point{4, 1}, b: point{2, 3}}
	if sp, _ := r.spanOn(2, 10); sp != (span{3, 10}) {
		t.Errorf("backwards selection differs: %v", sp)
	}
}
