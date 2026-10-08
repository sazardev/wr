package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Mouse text selection with copy to the clipboard.
//
//   - drag: select; the text is copied when you release the button
//   - double click: a word; triple click: a line
//   - code blocks copy without their frame and gutter, and paragraphs that were
//     word-wrapped on screen are re-joined into one line
//   - Shift+drag still gives your terminal's own selection

const multiClick = 450 * time.Millisecond

// point is a position in the document: a line and a rune column inside it.
type point struct{ line, col int }

func (p point) before(q point) bool {
	return p.line < q.line || (p.line == q.line && p.col < q.col)
}

type selection struct {
	active   bool // there is something selected to show
	dragging bool
	a, b     point // anchor and focus
	clicks   int
	lastAt   time.Time
	lastAtPt point
}

func (s *selection) clear() { *s = selection{lastAt: s.lastAt, lastAtPt: s.lastAtPt, clicks: s.clicks} }

// ordered returns the start and the (exclusive) end of the selection.
func (s *selection) ordered() (start, end point) {
	start, end = s.a, s.b
	if end.before(start) {
		start, end = end, start
	}
	end.col++ // the cell under the focus is part of the selection
	return start, end
}

// spanOn returns the selected rune range on a line (ok=false if none).
func (s *selection) spanOn(line int, runes int) (span, bool) {
	if !s.active {
		return span{}, false
	}
	start, end := s.ordered()
	if line < start.line || line > end.line {
		return span{}, false
	}
	from, to := 0, runes
	if line == start.line {
		from = start.col
	}
	if line == end.line {
		to = min(end.col, runes)
	}
	from = min(max(from, 0), runes)
	if to <= from {
		return span{}, false
	}
	return span{from, to}, true
}

// runeAtCell converts a screen cell offset inside a line to a rune index.
func runeAtCell(plain string, cell int) int {
	acc, idx := 0, 0
	for _, r := range plain {
		w := ansi.StringWidth(string(r))
		if cell < acc+w {
			return idx
		}
		acc += w
		idx++
	}
	return idx
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// wordBounds returns the [from, to) rune range of the word at col; on
// whitespace or punctuation it selects that run of identical characters.
func wordBounds(plain string, col int) (int, int) {
	r := []rune(plain)
	if len(r) == 0 {
		return 0, 0
	}
	col = min(max(col, 0), len(r)-1)
	same := func(a, b rune) bool {
		switch {
		case isWordRune(a):
			return isWordRune(b)
		case unicode.IsSpace(a):
			return unicode.IsSpace(b)
		}
		return a == b
	}
	from, to := col, col+1
	for from > 0 && same(r[col], r[from-1]) {
		from--
	}
	for to < len(r) && same(r[col], r[to]) {
		to++
	}
	return from, to
}

// pointAt converts a screen position to a document position. ok is false when
// the position is not on the reading area.
func (m *model) pointAt(x, y int) (point, bool) {
	cw, left, bodyH, _ := m.geometry()
	if m.doc == nil || y < 0 || y >= bodyH {
		return point{}, false
	}
	line := m.viewY() + y
	if line >= len(m.doc.Lines) {
		line = len(m.doc.Lines) - 1
	}
	cell := min(max(x-left, 0), cw)
	return point{line, runeAtCell(m.doc.Plain[line], cell)}, true
}

// selectedText extracts the selected text the way you would want to paste it.
func (m *model) selectedText() string {
	if m.doc == nil || !m.mouseSel.active {
		return ""
	}
	start, end := m.mouseSel.ordered()
	var out []string
	for l := start.line; l <= end.line && l < len(m.doc.Plain); l++ {
		mt := m.doc.Meta[l]
		if mt.Kind == kindCodeTop || mt.Kind == kindCodeBottom {
			continue // the frame is decoration
		}
		r := []rune(m.doc.Plain[l])
		from, to := 0, len(r)
		if l == start.line {
			from = start.col
		}
		if l == end.line {
			to = min(end.col, len(r))
		}
		if mt.Kind == kindCodeBody {
			from = max(from, mt.Gutter+2) // skip "│ "
		}
		from = min(max(from, 0), len(r))
		to = max(to, from)
		text := string(r[from:to])

		if mt.Soft && len(out) > 0 {
			if mt.Kind == kindCodeBody {
				out[len(out)-1] += text // a long source line wrapped on screen
			} else {
				out[len(out)-1] = strings.TrimRight(out[len(out)-1], " ") + " " + strings.TrimLeft(text, " ")
			}
			continue
		}
		if mt.Kind != kindCodeBody {
			text = strings.TrimRight(text, " ")
		}
		out = append(out, text)
	}
	return strings.Join(out, "\n")
}

// ---- mouse handling ----

func (m *model) mouseDown(x, y int) tea.Cmd {
	p, ok := m.pointAt(x, y)
	if !ok {
		return nil
	}
	if !m.mouseSel.lastAt.IsZero() && m.now.Sub(m.mouseSel.lastAt) < multiClick &&
		p.line == m.mouseSel.lastAtPt.line && abs(p.col-m.mouseSel.lastAtPt.col) <= 1 {
		m.mouseSel.clicks++
	} else {
		m.mouseSel.clicks = 1
	}
	m.mouseSel.lastAt, m.mouseSel.lastAtPt = m.now, p

	switch m.mouseSel.clicks {
	case 2: // word
		from, to := wordBounds(m.doc.Plain[p.line], p.col)
		m.mouseSel.a, m.mouseSel.b = point{p.line, from}, point{p.line, max(to-1, from)}
		m.mouseSel.active, m.mouseSel.dragging = to > from, false
		return m.copyIfSet()
	case 3: // line
		n := utf8.RuneCountInString(m.doc.Plain[p.line])
		m.mouseSel.a, m.mouseSel.b = point{p.line, 0}, point{p.line, max(n-1, 0)}
		m.mouseSel.active, m.mouseSel.dragging = n > 0, false
		return m.copyIfSet()
	}
	m.mouseSel.a, m.mouseSel.b = p, p
	m.mouseSel.active, m.mouseSel.dragging = false, true
	return nil
}

func (m *model) mouseDrag(x, y int) {
	_, _, bodyH, _ := m.geometry()
	switch { // dragging past an edge scrolls
	case y < 0:
		m.y--
	case y >= bodyH:
		m.y++
	}
	m.clampY()
	p, ok := m.pointAt(x, min(max(y, 0), bodyH-1))
	if !ok {
		return
	}
	m.mouseSel.b = p
	m.mouseSel.active = p != m.mouseSel.a
}

func (m *model) mouseUp() tea.Cmd {
	m.mouseSel.dragging = false
	if !m.mouseSel.active {
		// a plain click: nothing selected, but it may have landed on a link
		if id, ok := m.linkAt(m.mouseSel.a); ok {
			return m.openLink(id)
		}
		return nil
	}
	return m.copyIfSet()
}

// copyIfSet copies the selection right away unless copy-on-select is off (then
// the selection stays highlighted and y copies it).
func (m *model) copyIfSet() tea.Cmd {
	if !m.cfg.CopyOnSelect {
		return nil
	}
	return m.copySelection()
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// ---- clipboard ----

// clipboardExternal is swapped out in tests.
var clipboardExternal = copyExternal

// copySelection copies the selection two ways: OSC 52 (works through SSH and in
// most modern terminals) and a system clipboard tool (works where the terminal
// ignores OSC 52).
func (m *model) copySelection() tea.Cmd {
	text := m.selectedText()
	if text == "" {
		return nil
	}
	n := utf8.RuneCountInString(text)
	word := "characters"
	if n == 1 {
		word = "character"
	}
	return tea.Batch(
		tea.SetClipboard(text),
		func() tea.Msg { _ = clipboardExternal(text); return nil },
		m.setToast(fmt.Sprintf("✓ copied %d %s", n, word), false),
	)
}

type clipTool struct {
	name  string
	args  []string
	utf16 bool // feed UTF-16LE (no BOM): clip.exe would keep a BOM as a stray character
}

// windowsExe finds a Windows program from WSL, which does not always have the
// Windows directories in PATH.
func windowsExe(name, fallback string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	if _, err := os.Stat(fallback); err == nil {
		return fallback
	}
	return ""
}

const psSetClipboard = "[Console]::InputEncoding=[Text.Encoding]::UTF8; Set-Clipboard -Value ([Console]::In.ReadToEnd())"

func clipTools() []clipTool {
	var tools []clipTool
	if isWSL() {
		// PowerShell reads UTF-8 exactly; clip.exe is the lighter fallback.
		if p := windowsExe("powershell.exe", "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"); p != "" {
			tools = append(tools, clipTool{name: p, args: []string{"-NoProfile", "-NonInteractive", "-Command", psSetClipboard}})
		}
		if p := windowsExe("clip.exe", "/mnt/c/Windows/System32/clip.exe"); p != "" {
			tools = append(tools, clipTool{name: p, utf16: true})
		}
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		tools = append(tools, clipTool{name: "wl-copy"})
	}
	if os.Getenv("DISPLAY") != "" {
		tools = append(tools, clipTool{name: "xclip", args: []string{"-selection", "clipboard"}},
			clipTool{name: "xsel", args: []string{"--clipboard", "--input"}})
	}
	tools = append(tools, clipTool{name: "pbcopy"})
	return tools
}

func isWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "" {
		return true
	}
	b, err := os.ReadFile("/proc/version")
	return err == nil && strings.Contains(strings.ToLower(string(b)), "microsoft")
}

func encodeUTF16LE(s string) []byte {
	u := utf16.Encode([]rune(s))
	out := make([]byte, 0, len(u)*2)
	for _, c := range u {
		out = append(out, byte(c), byte(c>>8))
	}
	return out
}

// copyExternal writes text with the first clipboard tool that is available.
func copyExternal(text string) error {
	for _, t := range clipTools() {
		path, err := exec.LookPath(t.name)
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second) // a cold PowerShell start is slow
		cmd := exec.CommandContext(ctx, path, t.args...)
		if t.utf16 {
			cmd.Stdin = bytes.NewReader(encodeUTF16LE(text))
		} else {
			cmd.Stdin = strings.NewReader(text)
		}
		err = cmd.Run()
		cancel()
		if err == nil {
			return nil
		}
	}
	return fmt.Errorf("no clipboard tool worked")
}
