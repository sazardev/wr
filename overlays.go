package main

import (
	"fmt"
	"math"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Floating panels: the menu, address bar, settings, shortcuts, history and
// bookmarks, section index and confirmations. They use the accent color for
// their frame and selection, so they follow your theme and your choice.

type boxRow struct{ label, hint string }

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// box draws a panel `width` columns wide. sel < 0 means no selection.
func box(title string, rows []boxRow, sel, width int) []string {
	inner := width - 4
	border := "\x1b[" + accentFG + "m"
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
			content = onAccent(ansi.Strip(label) + strings.Repeat(" ", gap) + ansi.Strip(hintStr))
		} else {
			content = label + strings.Repeat(" ", gap) + dim(hintStr)
		}
		pad := max(inner-ansi.StringWidth(label)-gap-hintW, 0)
		out = append(out, border+"│\x1b[0m "+content+strings.Repeat(" ", pad)+" "+border+"│\x1b[0m")
	}
	return append(out, border+"╰"+strings.Repeat("─", width-2)+"╯\x1b[0m")
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

// compose puts the panel centered over the body.
func (m *model) compose(body, panel []string) []string {
	return m.composeAt(body, panel, max((len(body)-len(panel))/2, 0))
}

// composeAt puts the panel at a given row, replacing those rows completely so
// document colors never mix with the panel's. When panel animations are on,
// the panel opens like a shutter from its middle row.
func (m *model) composeAt(body, panel []string, fullTop int) []string {
	w := ansi.StringWidth(panel[0])
	x := max((m.w-w)/2, 0)
	start, count := 0, len(panel)
	if m.anim(m.cfg.AnimPanels) {
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

func (m *model) bodyH() int { _, _, h, _ := m.geometry(); return h }

// helpRows turns a hint into up to two dim rows under a panel's list.
func helpRows(text string, inner int) []boxRow {
	rows := []boxRow{{"", ""}}
	for i, l := range strings.Split(ansi.Wrap(text, max(inner, 8), ""), "\n") {
		if i >= 2 {
			break
		}
		rows = append(rows, boxRow{dim(l), ""})
	}
	return rows
}

// ---- menu ----

type menuItem struct {
	label, hint string
	run         func(m *model) tea.Cmd
}

func (m *model) menuItems() []menuItem {
	pages, size := cacheStats()
	k := func(a action) string { return keyHint(&m.cfg, a) }
	bookmark := "Bookmark this page"
	if m.bookmarked {
		bookmark = "Remove bookmark"
	}
	return []menuItem{
		{"Open address or search…", k(actOpen), func(m *model) tea.Cmd { m.openOmni(); return nil }},
		{"Back", k(actBack), func(m *model) tea.Cmd { return m.goBack() }},
		{"Forward", k(actForward), func(m *model) tea.Cmd { return m.goForward() }},
		{"History…", k(actHistory), func(m *model) tea.Cmd { return m.openList(listHistory) }},
		{"Bookmarks…", k(actBookmarks), func(m *model) tea.Cmd { return m.openList(listBookmarks) }},
		{bookmark, k(actBookmark), func(m *model) tea.Cmd { return m.toggleBookmark() }},
		{"Section index", k(actIndex), func(m *model) tea.Cmd { return m.openTOC() }},
		{"Reload page", k(actReload), func(m *model) tea.Cmd { return m.reload() }},
		{"Settings…", k(actSettings), func(m *model) tea.Cmd { m.mode, m.sel = modeSettings, 0; return nil }},
		{"Keyboard shortcuts…", "", func(m *model) tea.Cmd { m.mode, m.sel = modeKeys, 0; return nil }},
		{"Update wr / what's new…", m.updateMenuHint(), func(m *model) tea.Cmd { return m.openUpdate() }},
		{"Open in your browser", k(actExternal), func(m *model) tea.Cmd { return m.openExternalNow() }},
		{"Clear cache for this page", "", func(m *model) tea.Cmd {
			if !isHTTP(m.src) {
				return m.setToast("nothing is cached for this page", true)
			}
			if err := cacheDelete(m.src); err != nil {
				return m.setToast(err.Error(), true)
			}
			return m.setToast("✓ cache cleared for this page", false)
		}},
		{"Clear all cache", fmt.Sprintf("%d pages · %s", pages, humanSize(size)), func(m *model) tea.Cmd {
			m.mode, m.confirm = modeConfirm, confirmCache
			return nil
		}},
		{"Edit the config file…", "", func(m *model) tea.Cmd { return m.openConfig() }},
		{"Quit", k(actQuit), func(m *model) tea.Cmd { return tea.Quit }},
	}
}

func (m *model) menuBox() []string {
	items := m.menuItems()
	vis := m.visibleEntries()
	m.snapSel(vis)
	rows := make([]boxRow, 0, len(vis))
	selRow := 0
	for p, i := range vis {
		label := "   " + items[i].label
		if p < 10 && !m.filterOpen() { // 1-9 then 0: the number is the shortcut
			label = fmt.Sprintf("%d  %s", (p+1)%10, items[i].label)
		}
		if i == m.sel {
			selRow = len(rows)
		}
		rows = append(rows, boxRow{label, items[i].hint})
	}
	return m.filteredBox("wr "+shortVersion(), rows, selRow, nil, m.panelWidth(58))
}

func (m *model) keyMenu(k string) (tea.Model, tea.Cmd) {
	items := m.menuItems()
	run := func(i int) (tea.Model, tea.Cmd) {
		m.mode = modeRead
		return m, items[i].run(m)
	}
	switch k {
	case "esc", "m", "q", "?":
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
	return box("Confirm", m.confirmText(), -1, m.panelWidth(48))
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
	heads := m.doc.Heads
	vis := m.visibleEntries()
	m.snapSel(vis)
	rows := make([]boxRow, 0, len(vis))
	selRow := 0
	for _, i := range vis {
		if i == m.sel {
			selRow = len(rows)
		}
		rows = append(rows, boxRow{strings.Repeat("  ", max(heads[i].Level-1, 0)) + heads[i].Text, ""})
	}
	title := fmt.Sprintf("Index %d/%d", m.sel+1, len(heads))
	if m.pf.text != "" {
		title = fmt.Sprintf("Index · %d of %d", len(vis), len(heads))
	}
	return m.filteredBox(title, rows, selRow, nil, m.panelWidth(76))
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

// ---- settings ----

func (m *model) settingsBox() []string {
	opts := m.settingsOptions()
	vis := m.visibleEntries()
	m.snapSel(vis)
	m.sel = min(max(m.sel, 0), len(opts)-1)
	width := m.panelWidth(66)
	var rows []boxRow
	selRow, last := 0, ""
	for _, i := range vis {
		o := opts[i]
		if o.group != last {
			rows = append(rows, boxRow{accBold(o.group), ""})
			last = o.group
		}
		val := o.get(&m.cfg)
		if i == m.sel {
			selRow = len(rows)
			if o.kind == optAction {
				val = "↵"
				if o.label == "Start page" {
					val = "↵ " + o.get(&m.cfg)
				}
			} else {
				val = "‹ " + val + " ›"
			}
		} else if o.kind == optAction && o.label != "Start page" {
			val = ""
		}
		rows = append(rows, boxRow{"  " + o.label, val})
	}
	var help []boxRow
	if len(vis) > 0 {
		help = helpRows(opts[m.sel].help, width-4)
	}
	return m.filteredBox("Settings", rows, selRow, help, width)
}

// ---- keyboard shortcuts ----

func (m *model) keysBox() []string {
	width := m.panelWidth(66)
	vis := m.visibleEntries()
	m.snapSel(vis)
	m.sel = min(max(m.sel, 0), len(actionList)-1)
	var rows []boxRow
	selRow, last := 0, ""
	for _, i := range vis {
		a := actionList[i]
		if a.group != last {
			rows = append(rows, boxRow{accBold(a.group), ""})
			last = a.group
		}
		keys := keyList(&m.cfg, a.id)
		if m.capture == string(a.id) {
			keys = "press a key…"
			if m.captureAdd {
				keys = "press another key…"
			}
		}
		if i == m.sel {
			selRow = len(rows)
		}
		rows = append(rows, boxRow{"  " + a.label, keys})
	}
	help := helpRows("enter change · a add a key · x unbind · d default · R reset all", width-4)
	if m.capture != "" {
		help = helpRows("esc cancels", width-4)
	}
	return m.filteredBox("Keyboard shortcuts", rows, selRow, help, width)
}

// filteredBox draws a panel whose entries can be filtered with "/": the prompt
// row on top while a filter is open, the visible entries in a window that keeps
// the selection in view, and optional help rows underneath.
func (m *model) filteredBox(title string, rows []boxRow, selRow int, help []boxRow, width int) []string {
	var head []boxRow
	if m.filterOpen() {
		prompt := acc("/") + " " + m.pf.text
		if m.pf.typing {
			prompt += cursor()
			if m.pf.text == "" {
				prompt += dim(" type to filter")
			}
		}
		head = []boxRow{{prompt, ""}}
	}
	empty := len(rows) == 0
	if empty {
		rows, selRow = []boxRow{{dim("nothing matches"), ""}}, -1
	}
	vis, sel := window(rows, selRow, max(m.bodyH()-2-len(head)-len(help), 3))
	if empty || sel < 0 {
		sel = -1 - len(head) // nothing to highlight
	}
	return box(title, append(append(head, vis...), help...), sel+len(head), width)
}

// ---- address bar ----

func (m *model) omniBox() []string {
	width := m.panelWidth(84)
	items := m.omniItems()
	m.omni.sel = min(max(m.omni.sel, 0), max(len(items)-1, 0))
	prompt := acc("›") + " " + m.omni.input + cursor()
	if m.omni.input == "" {
		prompt = acc("›") + " " + cursor() + dim(" type an address, or words to search the web")
	}
	rows := []boxRow{{prompt, ""}}
	for _, it := range items {
		rows = append(rows, boxRow{it.label, it.hint})
	}
	if len(items) == 0 {
		rows = append(rows, boxRow{dim("no bookmarks or history yet"), ""})
	}
	vis, sel := window(rows[1:], m.omni.sel, max(m.bodyH()-5, 3))
	if len(items) == 0 {
		sel = -1
	}
	return box("Open", append(rows[:1], vis...), sel+1, width)
}

// ---- history and bookmarks ----

func (m *model) listBox() []string {
	width := m.panelWidth(90)
	title := "History"
	help := "enter open · ctrl+d remove · ctrl+x clear all"
	if m.list.kind == listBookmarks {
		title, help = "Bookmarks", "enter open · ctrl+d remove"
	}
	prompt := acc("/") + " " + m.list.filter + cursor()
	if m.list.filter == "" {
		prompt += dim(" type to filter")
	}
	rows := m.listRows()
	n := len(rows)
	m.list.sel = min(max(m.list.sel, 0), max(n-1, 0))
	helpR := helpRows(help, width-4)
	vis, sel := window(rows, m.list.sel, max(m.bodyH()-3-len(helpR), 3))
	if n == 0 {
		msg := "nothing here yet"
		if m.list.filter != "" {
			msg = "nothing matches"
		}
		vis, sel = []boxRow{{dim(msg), ""}}, -1
	}
	all := append([]boxRow{{prompt, ""}}, vis...)
	return box(fmt.Sprintf("%s · %d", title, len(m.list.all)), append(all, helpR...), sel+1, width)
}
