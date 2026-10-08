package main

import (
	"fmt"
	"math"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Floating panels: menu, index, help and confirmation. They all use the
// 16-color palette (bright blue for the frame and the selection), so they
// follow the terminal theme.

type boxRow struct{ label, hint string }

// box draws a panel `width` columns wide. sel < 0 means no selection.
func box(title string, rows []boxRow, sel, width int) []string {
	inner := width - 4
	border := "\x1b[94m"
	titleStr := " " + title + " "
	top := border + "╭─\x1b[1m" + titleStr + "\x1b[22m" +
		strings.Repeat("─", max(width-3-ansi.StringWidth(titleStr), 0)) + "╮\x1b[0m"
	out := []string{top}
	for i, r := range rows {
		hintStr := r.hint
		if ansi.StringWidth(r.label)+1+ansi.StringWidth(hintStr) > inner {
			// narrow panel: the label gives way first, and the hint goes if it still does not fit
			if inner-ansi.StringWidth(hintStr)-1 < 8 {
				hintStr = ""
			}
		}
		label := ansi.Truncate(r.label, max(inner-ansi.StringWidth(hintStr)-boolInt(hintStr != ""), 1), "…")
		hintW := ansi.StringWidth(hintStr)
		gap := inner - ansi.StringWidth(label) - hintW
		if hintStr != "" {
			gap = max(gap, 1) // keep the label and its hint apart
		}
		gap = max(gap, 0)
		var content string
		if i == sel {
			content = "\x1b[30;104m" + ansi.Strip(label) + strings.Repeat(" ", gap) + ansi.Strip(hintStr) + "\x1b[0m"
		} else {
			content = label + strings.Repeat(" ", gap) + "\x1b[90m" + hintStr + "\x1b[0m"
		}
		pad := max(inner-ansi.StringWidth(label)-gap-hintW, 0)
		out = append(out, border+"│\x1b[0m "+content+strings.Repeat(" ", pad)+" "+border+"│\x1b[0m")
	}
	return append(out, border+"╰"+strings.Repeat("─", width-2)+"╯\x1b[0m")
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// window keeps the selected row visible when there is not enough height.
func window(rows []boxRow, sel, maxRows int) ([]boxRow, int) {
	maxRows = max(maxRows, 1)
	if len(rows) <= maxRows {
		return rows, sel
	}
	start := 0
	if sel >= 0 {
		start = min(max(sel-maxRows/2, 0), len(rows)-maxRows)
	}
	if sel >= 0 {
		sel -= start
	}
	return rows[start : start+maxRows], sel
}

// panelWidth adapts the panel to the terminal width.
func (m *model) panelWidth(want int) int { return min(max(m.w-2, 24), want) }

// compose puts the panel centered over the body, replacing those rows
// completely so document colors never mix with the panel's. When animations
// are on, the panel opens like a shutter from its middle row.
func (m *model) compose(body, panel []string) []string {
	w := ansi.StringWidth(panel[0])
	x := max((m.w-w)/2, 0)
	fullTop := max((len(body)-len(panel))/2, 0)
	start, count := 0, len(panel)
	if m.cfg.Animations {
		p := easeOutCubic(progressSince(m.panelStart, m.now, panelDur))
		count = min(max(int(math.Ceil(p*float64(len(panel)))), 1), len(panel))
		start = (len(panel) - count) / 2
	}
	for i := 0; i < count; i++ {
		row := fullTop + start + i
		if row >= len(body) {
			break
		}
		body[row] = strings.Repeat(" ", x) + panel[start+i] + strings.Repeat(" ", max(m.w-x-w, 0))
	}
	return body
}

// ---- menu ----

type menuItem struct {
	label, hint string
	run         func(m *model) tea.Cmd
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func (m *model) menuItems() []menuItem {
	pages, size := cacheStats()
	nextLinks := map[string]string{"footnotes": "inline", "inline": "hidden", "hidden": "footnotes"}
	style := "braille"
	if !m.cfg.Braille {
		style = "simple"
	}
	return []menuItem{
		{"Reload page", "r", func(m *model) tea.Cmd { return m.reload() }},
		{"Section index", "t", func(m *model) tea.Cmd { return m.openTOC() }},
		{"Clear cache for this page", "", func(m *model) tea.Cmd {
			if !isHTTP(m.src) {
				return m.setToast("local file: nothing is cached for it", true)
			}
			if err := cacheDelete(m.src); err != nil {
				return m.setToast(err.Error(), true)
			}
			return m.setToast("✓ cache cleared for this page", false)
		}},
		{"Clear all cache", fmt.Sprintf("%d pages · %s", pages, humanSize(size)), func(m *model) tea.Cmd {
			m.mode = modeConfirm
			return nil
		}},
		{"Links: " + m.cfg.Links, "change", func(m *model) tea.Cmd {
			m.cfg.Links = nextLinks[m.cfg.Links]
			m.rebuild()
			return m.setToast("links: "+m.cfg.Links, false)
		}},
		{"Style: " + style, "change", func(m *model) tea.Cmd {
			m.cfg.Braille = !m.cfg.Braille
			m.rebuild()
			return nil
		}},
		{"Animations: " + onOff(m.cfg.Animations), "change", func(m *model) tea.Cmd {
			m.cfg.Animations = !m.cfg.Animations
			return m.setToast("animations: "+onOff(m.cfg.Animations), false)
		}},
		{"Settings", "opens your editor", func(m *model) tea.Cmd { return m.openConfig() }},
		{"Keyboard shortcuts", "", func(m *model) tea.Cmd { m.mode = modeHelp; return nil }},
		{"Quit", "q", func(m *model) tea.Cmd { return tea.Quit }},
	}
}

func (m *model) menuBox() []string {
	_, _, bodyH, _ := m.geometry()
	items := m.menuItems()
	rows := make([]boxRow, len(items))
	for i, it := range items {
		n := (i + 1) % 10 // 1-9, then 0: the number is the shortcut
		rows[i] = boxRow{fmt.Sprintf("%d  %s", n, it.label), it.hint}
	}
	vis, sel := window(rows, m.sel, bodyH-2)
	return box("wr", vis, sel, m.panelWidth(54))
}

func (m *model) keyMenu(k string) (tea.Model, tea.Cmd) {
	items := m.menuItems()
	run := func(i int) (tea.Model, tea.Cmd) {
		m.mode = modeRead
		return m, items[i].run(m)
	}
	switch k {
	case "esc", "m", "q", "?", "tab":
		m.mode = modeRead
	case "up", "k":
		m.sel = (m.sel - 1 + len(items)) % len(items)
	case "down", "j":
		m.sel = (m.sel + 1) % len(items)
	case "enter", "space", " ":
		return run(m.sel)
	default:
		if len(k) == 1 && k[0] >= '0' && k[0] <= '9' {
			i := int(k[0] - '1') // '1'..'9' -> 0..8
			if k[0] == '0' {
				i = 9
			}
			if i < len(items) {
				return run(i)
			}
		}
	}
	return m, nil
}

// ---- confirmation ----

func (m *model) confirmBox() []string {
	pages, size := cacheStats()
	rows := []boxRow{
		{"Clear all cache?", ""},
		{fmt.Sprintf("%d pages · %s on disk", pages, humanSize(size)), ""},
		{"", ""},
		{"y / enter  clear", ""},
		{"esc        cancel", ""},
	}
	return box("Confirm", rows, -1, m.panelWidth(44))
}

// ---- section index ----

func (m *model) openTOC() tea.Cmd {
	if m.doc == nil || len(m.doc.Heads) == 0 {
		return m.setToast("this page has no sections", true)
	}
	m.mode, m.sel = modeTOC, 0
	for i, h := range m.doc.Heads {
		if h.Line <= m.y {
			m.sel = i
		}
	}
	return nil
}

func (m *model) tocBox() []string {
	_, _, bodyH, _ := m.geometry()
	heads := m.doc.Heads
	rows := make([]boxRow, len(heads))
	for i, h := range heads {
		rows[i] = boxRow{strings.Repeat("  ", max(h.Level-1, 0)) + h.Text, ""}
	}
	vis, sel := window(rows, m.sel, min(bodyH-2, 16))
	return box(fmt.Sprintf("Index %d/%d", m.sel+1, len(heads)), vis, sel, m.panelWidth(76))
}

func (m *model) keyTOC(k string) (tea.Model, tea.Cmd) {
	n := len(m.doc.Heads)
	switch k {
	case "esc", "t", "q":
		m.mode = modeRead
	case "up", "k":
		m.sel = max(m.sel-1, 0)
	case "down", "j":
		m.sel = min(m.sel+1, n-1)
	case "pgup":
		m.sel = max(m.sel-8, 0)
	case "pgdown":
		m.sel = min(m.sel+8, n-1)
	case "g", "home":
		m.sel = 0
	case "G", "end":
		m.sel = n - 1
	case "enter":
		m.y = m.doc.Heads[m.sel].Line
		m.clampY()
		m.mode = modeRead
	}
	return m, nil
}

// ---- help ----

func (m *model) helpBox() []string {
	_, _, bodyH, _ := m.geometry()
	rows := []boxRow{
		{"j k ↓ ↑", "one line"},
		{"space  b", "page down / up"},
		{"d  u", "half page"},
		{"g  G", "top / bottom"},
		{"]  [", "next / previous section"},
		{"t", "section index"},
		{"/", "search (n / N: next / previous)"},
		{"r", "reload the page"},
		{"m", "menu (cache, style, links, ...)"},
		{"q", "quit"},
	}
	vis, _ := window(rows, -1, bodyH-2)
	return box("Shortcuts", vis, -1, m.panelWidth(58))
}
