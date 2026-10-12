package main

// The update window and its plumbing in the reader: a footer note when a newer
// wr exists, the U key and menu entry, and a small "what's new" window (off by
// default, whats_new) that shows the release notes and lets you update from it.

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type updPhase int

const (
	updIdle     updPhase = iota
	updChecking          // asking GitHub
	updBusy              // downloading and installing
	updDone              // installed: restart to use it
	updFailed            // the last action failed (msg says why)
)

// updateUI is the reader's view of the update state.
type updateUI struct {
	rel     *releaseInfo // the newest release known
	newer   bool         // rel is newer than the running version
	just    *releaseInfo // set on the first run after an update: show its notes
	showing bool         // the window is showing the notes of `just`
	phase   updPhase
	msg     string
	scroll  int
	updated string // the version just installed, until restart
}

type (
	updateCheckedMsg struct {
		res    checkResult
		manual bool
	}
	updateDoneMsg struct {
		tag string
		err error
	}
)

func checkUpdateCmd(force, manual bool) tea.Cmd {
	return func() tea.Msg { return updateCheckedMsg{checkForUpdate(force), manual} }
}

func applyUpdateCmd(rel *releaseInfo) tea.Cmd {
	return func() tea.Msg {
		return updateDoneMsg{rel.Tag, applyUpdate(rel, func(string) {})}
	}
}

// startUpdateCheck is the background check at launch: nothing happens when
// updates are off or this is not a release build.
func (m *model) startUpdateCheck() tea.Cmd {
	if !m.cfg.UpdateCheck || !updatesPossible() {
		return nil
	}
	return checkUpdateCmd(false, false)
}

func (m *model) openUpdate() tea.Cmd {
	m.mode, m.sel = modeUpdate, 0
	m.upd.scroll = 0
	if m.upd.phase == updFailed {
		m.upd.phase, m.upd.msg = updIdle, ""
	}
	if m.upd.rel == nil && m.upd.phase == updIdle && updatesPossible() {
		m.upd.phase = updChecking
		return checkUpdateCmd(false, true)
	}
	return nil
}

func (m *model) onUpdateChecked(msg updateCheckedMsg) (tea.Model, tea.Cmd) {
	res := msg.res
	if m.upd.phase == updChecking {
		m.upd.phase = updIdle
	}
	if res.Latest != nil {
		m.upd.rel, m.upd.newer = res.Latest, res.Newer
	}
	if msg.manual {
		if res.Err != nil && res.Latest == nil {
			m.upd.phase, m.upd.msg = updFailed, "could not check: "+shortErr(res.Err)
		}
		return m, nil
	}
	if !m.cfg.UpdateCheck {
		return m, nil
	}
	// first run after an update: the notes of what you now have
	if res.JustUpdate != nil && m.cfg.WhatsNew && m.mode == modeRead {
		m.upd.just, m.upd.showing = res.JustUpdate, true
		m.mode, m.upd.scroll = modeUpdate, 0
		return m, nil
	}
	if res.Announce {
		markAnnounced(res.Latest.Tag)
		if m.cfg.WhatsNew && m.mode == modeRead {
			m.mode, m.upd.scroll = modeUpdate, 0
			return m, nil
		}
		return m, m.setToast("● wr "+res.Latest.Tag+" is available: press "+keyHint(&m.cfg, actUpdateKey)+" to update", false)
	}
	return m, nil
}

func (m *model) onUpdateDone(msg updateDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.upd.phase, m.upd.msg = updFailed, msg.err.Error()
		return m, m.setToast("✗ update failed: "+shortErr(msg.err), true)
	}
	m.upd.phase, m.upd.msg, m.upd.updated, m.upd.newer = updDone, "", msg.tag, false
	return m, m.setToast("✓ wr "+msg.tag+" installed: restart wr to use it", false)
}

// startApply begins downloading the newer release.
func (m *model) startApply() tea.Cmd {
	if m.upd.rel == nil || !m.upd.newer || m.upd.phase == updBusy {
		return nil
	}
	m.upd.phase, m.upd.msg = updBusy, ""
	return applyUpdateCmd(m.upd.rel)
}

func (m *model) keyUpdate(k string) (tea.Model, tea.Cmd) {
	switch k {
	case "esc", "q", "U", "left", "h":
		m.mode, m.upd.showing = modeRead, false
		return m, nil
	case "enter":
		if m.upd.newer && m.upd.phase != updBusy && m.upd.phase != updDone {
			return m, m.startApply()
		}
		m.mode, m.upd.showing = modeRead, false
	case "up", "k":
		m.upd.scroll = max(m.upd.scroll-1, 0)
	case "down", "j":
		m.upd.scroll++
	case "pgup", "b":
		m.upd.scroll = max(m.upd.scroll-6, 0)
	case "pgdown", "space", " ", "f":
		m.upd.scroll += 6
	case "home", "g":
		m.upd.scroll = 0
	case "c":
		if m.upd.phase != updBusy && updatesPossible() {
			m.upd.phase, m.upd.msg, m.upd.showing = updChecking, "", false
			return m, checkUpdateCmd(true, true)
		}
	case "o":
		if m.upd.rel != nil && m.upd.rel.URL != "" {
			if err := openExternal(m.upd.rel.URL); err != nil {
				return m, m.setToast(err.Error(), true)
			}
			return m, m.setToast("opened the release page in your browser", false)
		}
	}
	return m, nil
}

// updateTitle and the lead line say where you stand.
func (m *model) updateTitle() (title, lead string) {
	cur := shortVersion()
	switch {
	case m.upd.showing && m.upd.just != nil:
		return "What's new in wr " + m.upd.just.Tag, "You are now on " + cur + "."
	case m.upd.phase == updDone:
		return "wr " + m.upd.updated + " installed", "Restart wr to start using it."
	case m.upd.newer && m.upd.rel != nil:
		return "wr " + m.upd.rel.Tag + " is available", "You have " + cur + "."
	case m.upd.phase == updChecking:
		return "wr " + cur, "Checking for updates…"
	case m.upd.phase == updFailed:
		return "wr " + cur, ""
	}
	if !updatesPossible() && m.upd.rel == nil {
		return "wr " + cur, "Update checks are off for this build or session."
	}
	return "wr " + cur, "You are up to date."
}

func (m *model) updateBox() []string {
	width := m.panelWidth(70)
	inner := width - 4
	title, lead := m.updateTitle()
	var rows []boxRow
	if lead != "" {
		rows = append(rows, boxRow{lead, ""}, boxRow{"", ""})
	}

	// what to show underneath
	rel := m.upd.rel
	if m.upd.showing && m.upd.just != nil {
		rel = m.upd.just
	}
	switch {
	case m.upd.phase == updBusy:
		rows = append(rows, boxRow{acc(m.spin()) + " Downloading and installing " + m.upd.rel.Tag + "…", ""})
	case m.upd.phase == updFailed && m.upd.msg != "":
		for _, l := range strings.Split(ansi.Wrap(m.upd.msg, inner, ""), "\n") {
			rows = append(rows, boxRow{fail(l), ""})
		}
		rows = append(rows, boxRow{"", ""})
	}
	notesFor := rel != nil && (m.upd.newer || (m.upd.showing && m.upd.just != nil) || m.upd.phase == updDone || sameVersion(rel.Tag, versionString()))
	if notesFor && m.upd.phase != updBusy {
		var all []boxRow
		for _, n := range releaseNotes(rel.Body) {
			switch n.kind {
			case noteTitle:
				if n.text != "" {
					all = append(all, boxRow{dim("Released " + n.text), ""})
				}
			case noteHead:
				if len(all) > 0 {
					all = append(all, boxRow{"", ""})
				}
				all = append(all, boxRow{accBold(n.text), ""})
			case noteItem:
				for i, l := range strings.Split(ansi.Wrap(n.text, max(inner-2, 8), ""), "\n") {
					prefix := "  "
					if i == 0 {
						prefix = acc("•") + " "
					}
					all = append(all, boxRow{prefix + l, ""})
				}
			default:
				for _, l := range strings.Split(ansi.Wrap(n.text, inner, ""), "\n") {
					all = append(all, boxRow{dim(l), ""})
				}
			}
		}
		if len(all) == 0 {
			all = append(all, boxRow{dim("No release notes were published for this version."), ""})
		}
		room := max(m.bodyH()-len(rows)-6, 3)
		maxScroll := max(len(all)-room, 0)
		m.upd.scroll = min(m.upd.scroll, maxScroll)
		rows = append(rows, all[m.upd.scroll:min(m.upd.scroll+room, len(all))]...)
		if len(all) > room {
			rows = append(rows, boxRow{dim(fmt.Sprintf("↑↓ more (%d/%d)", m.upd.scroll+room, len(all))), ""})
		}
	}

	rows = append(rows, boxRow{"", ""}, boxRow{m.updateHints(), ""})
	return box(title, rows, -1, width)
}

func (m *model) updateHints() string {
	switch {
	case m.upd.phase == updBusy:
		return hint("esc", "keep reading, it continues")
	case m.upd.phase == updDone:
		return hint("esc", "close")
	case m.upd.newer:
		return hint("enter", "update now") + "  " + hint("esc", "later") + "  " + hint("o", "release page")
	}
	return hint("c", "check now") + "  " + hint("o", "release page") + "  " + hint("esc", "close")
}

// updateChip is the footer note: a pending update, or a finished one.
func (m *model) updateChip() string {
	switch {
	case m.upd.phase == updBusy:
		return caution("↻ updating") + "  "
	case m.upd.phase == updDone:
		return good("✓ "+m.upd.updated+" · restart") + "  "
	case m.upd.newer && m.upd.rel != nil:
		return caution("↑ "+m.upd.rel.Tag+" ("+keyHint(&m.cfg, actUpdateKey)+")") + "  "
	}
	return ""
}

// updateMenuHint is what the menu entry says on its right.
func (m *model) updateMenuHint() string {
	switch {
	case m.upd.phase == updDone:
		return m.upd.updated + " installed"
	case m.upd.newer && m.upd.rel != nil:
		return m.upd.rel.Tag + " available"
	case m.upd.rel != nil:
		return "up to date"
	}
	return shortVersion()
}
