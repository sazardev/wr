package main

// "/" in the menu, Settings, Keyboard shortcuts and the section index: type a
// few words and only the entries that mention all of them stay. m.sel always
// holds the *real* index of the selected entry, so clearing the filter leaves
// you on the same entry; moving the selection steps over the hidden ones.

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

type panelFilter struct {
	typing bool   // keys go into the filter
	text   string // what was typed (kept after enter, until esc clears it)
}

// filterable reports the panels that have the filter.
func (m *model) filterable() bool {
	switch m.mode {
	case modeMenu, modeSettings, modeTOC:
		return true
	case modeKeys:
		return m.capture == "" // while capturing, "/" is a key you may want to bind
	}
	return false
}

func (m *model) filterOpen() bool { return m.pf.typing || m.pf.text != "" }

// matchesFilter is true when every word of query appears in hay, ignoring case.
func matchesFilter(hay, query string) bool {
	hay = strings.ToLower(hay)
	for _, w := range strings.Fields(strings.ToLower(query)) {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}

// panelCount and panelHay describe the entries of the open panel to the filter:
// what each one is searched by.
func (m *model) panelCount() int {
	switch m.mode {
	case modeMenu:
		return len(m.menuItems())
	case modeSettings:
		return len(m.settingsOptions())
	case modeKeys:
		return len(actionList)
	case modeTOC:
		if m.doc != nil {
			return len(m.doc.Heads)
		}
	}
	return 0
}

func (m *model) panelHay(i int) string {
	switch m.mode {
	case modeMenu:
		it := m.menuItems()[i]
		return it.label + " " + it.hint
	case modeSettings:
		o := m.settingsOptions()[i]
		return o.label + " " + o.group + " " + o.help
	case modeKeys:
		a := actionList[i]
		return a.label + " " + a.group + " " + keyList(&m.cfg, a.id)
	case modeTOC:
		return m.doc.Heads[i].Text
	}
	return ""
}

// visibleEntries returns the real indices that pass the filter, in order.
func (m *model) visibleEntries() []int {
	n := m.panelCount()
	vis := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if m.pf.text == "" || matchesFilter(m.panelHay(i), m.pf.text) {
			vis = append(vis, i)
		}
	}
	return vis
}

// posOf returns where sel sits in vis, or -1.
func posOf(vis []int, sel int) int {
	for p, i := range vis {
		if i == sel {
			return p
		}
	}
	return -1
}

// snapSel puts the selection on a visible entry (the first one if it is hidden).
func (m *model) snapSel(vis []int) {
	if len(vis) > 0 && posOf(vis, m.sel) < 0 {
		m.sel = vis[0]
	}
}

// moveVisible moves the selection among the visible entries.
func (m *model) moveVisible(vis []int, k string) {
	if len(vis) == 0 {
		return
	}
	p := max(posOf(vis, m.sel), 0)
	switch k {
	case "up":
		p = (p - 1 + len(vis)) % len(vis)
	case "down":
		p = (p + 1) % len(vis)
	case "pgup":
		p = max(p-6, 0)
	case "pgdown":
		p = min(p+6, len(vis)-1)
	case "home":
		p = 0
	case "end":
		p = len(vis) - 1
	}
	m.sel = vis[p]
}

// filterKey handles a key for the filter of the open panel. handled is true
// when the key was consumed; a false answer lets the panel act on it as usual
// (with m.sel on a visible entry).
func (m *model) filterKey(msg tea.KeyPressMsg) (handled bool, cmd tea.Cmd, model tea.Model) {
	k := msg.String()
	model = m
	if !m.filterable() {
		return false, nil, m
	}
	vis := m.visibleEntries()
	if !m.pf.typing {
		switch {
		case k == "/":
			m.pf.typing = true
			return true, nil, m
		case !m.filterOpen():
			return false, nil, m
		case k == "esc": // first esc only clears the filter
			m.pf = panelFilter{}
			return true, nil, m
		}
		m.snapSel(vis)
		switch k {
		case "up", "k":
			m.moveVisible(vis, "up")
			return true, nil, m
		case "down", "j":
			m.moveVisible(vis, "down")
			return true, nil, m
		case "pgup", "pgdown", "home", "end":
			m.moveVisible(vis, k)
			return true, nil, m
		case "g":
			m.moveVisible(vis, "home")
			return true, nil, m
		case "G":
			m.moveVisible(vis, "end")
			return true, nil, m
		}
		if len(vis) == 0 { // nothing to act on
			return k != "q", nil, m
		}
		return false, nil, m
	}

	// typing
	switch k {
	case "esc":
		m.pf = panelFilter{}
		return true, nil, m
	case "enter":
		m.pf.typing = false
		if len(vis) == 0 {
			return true, nil, m
		}
		m.snapSel(vis)
		// the menu and the index act on the entry; Settings and shortcuts keep
		// the results so you can change the value you found
		return m.mode != modeMenu && m.mode != modeTOC, nil, m
	case "backspace":
		if r := []rune(m.pf.text); len(r) > 0 {
			m.pf.text = string(r[:len(r)-1])
		} else {
			m.pf.typing = false
		}
	case "up", "down", "pgup", "pgdown", "home", "end":
		m.moveVisible(vis, k)
		return true, nil, m
	case "left", "right", "tab", "shift+tab", "ctrl+c":
		return k != "ctrl+c", nil, m
	default:
		if msg.Text == "" {
			return true, nil, m
		}
		m.pf.text += strings.NewReplacer("\n", " ", "\r", " ").Replace(msg.Text)
	}
	m.snapSel(m.visibleEntries())
	return true, nil, m
}
