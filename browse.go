package main

import (
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Browsing: loading pages, back / forward, links (click, Tab, hint labels),
// the address bar, and the history and bookmark panels.

const maxNavStack = 200

type navKind int

const (
	navInitial navKind = iota
	navPush
	navBack
	navForward
	navReplace // reload in place (start page, reload)
)

type navEntry struct {
	src string
	y   int
}

// navReq is a page load in flight. The page you are reading stays on screen
// until the new one arrives; if it fails, nothing changes.
type navReq struct {
	id      int
	src     string
	kind    navKind
	y       int    // scroll position to restore (back / forward)
	frag    string // #section to jump to
	leaving navEntry
}

// ---- loading ----

func loadCmd(id int, src string, fresh bool, cfg Config) tea.Cmd {
	return func() tea.Msg {
		if src == "" {
			md := startPageMD(&cfg, bookmarkList(), historyList())
			return loadedMsg{id: id, src: src, md: md, saved: time.Now()}
		}
		if cfg.Cache && isHTTP(src) && !fresh {
			if md, saved, ok := cacheGet(src); ok {
				return loadedMsg{id: id, src: src, md: sanitize(md), fromCache: true, saved: saved}
			}
		}
		md, err := load(src)
		return loadedMsg{id: id, src: src, md: md, err: err, saved: time.Now()}
	}
}

func fetchCmd(id int, src string, apply bool) tea.Cmd {
	return func() tea.Msg {
		md, err := load(src)
		return fetchedMsg{id: id, src: src, md: md, err: err, apply: apply}
	}
}

func historyCmd(src, title string, limit int) tea.Cmd {
	return func() tea.Msg {
		_ = historyAdd(src, title, limit)
		return nil
	}
}

// startNav begins loading target. kind says how the back/forward stacks change
// once it arrives.
func (m *model) startNav(kind navKind, target string, y int, fresh bool) tea.Cmd {
	page, frag := splitFragment(target)
	m.navID++
	m.pending = &navReq{id: m.navID, src: page, kind: kind, y: y, frag: frag, leaving: navEntry{m.src, m.y}}
	m.loading = true
	m.loadErr = nil
	return loadCmd(m.navID, page, fresh, m.cfg)
}

// navigate opens target, adding the current page to the back stack. A link to
// a section of the page you are on just jumps there.
func (m *model) navigate(target string) tea.Cmd {
	page, frag := splitFragment(target)
	if page == "" || (page == m.src && m.doc != nil) {
		if frag != "" && !m.gotoAnchor(frag) {
			return m.setToast("no section called “"+frag+"” on this page", true)
		}
		return nil
	}
	return m.startNav(navPush, target, 0, false)
}

// navigateInput opens what was typed in the address bar.
func (m *model) navigateInput(in string) tea.Cmd {
	target, search := normalizeInput(in, m.cfg.SearchEngine)
	if target == "" {
		return nil
	}
	if search {
		return tea.Batch(m.startNav(navPush, target, 0, false), m.setToast("searching “"+strings.TrimSpace(in)+"”", false))
	}
	return m.navigate(target)
}

func (m *model) goBack() tea.Cmd {
	if len(m.back) == 0 {
		return m.setToast("no earlier page", true)
	}
	e := m.back[len(m.back)-1]
	return m.startNav(navBack, e.src, e.y, false)
}

func (m *model) goForward() tea.Cmd {
	if len(m.fwd) == 0 {
		return m.setToast("no later page", true)
	}
	e := m.fwd[len(m.fwd)-1]
	return m.startNav(navForward, e.src, e.y, false)
}

func trimStack(s []navEntry) []navEntry {
	if len(s) > maxNavStack {
		return s[len(s)-maxNavStack:]
	}
	return s
}

// commitNav applies a finished load: stacks, document, scroll position, history.
func (m *model) commitNav(msg loadedMsg) tea.Cmd {
	req := m.pending
	m.pending = nil
	m.loading = false
	if req == nil {
		req = &navReq{kind: navReplace}
	}
	switch req.kind {
	case navPush:
		m.back = trimStack(append(m.back, req.leaving))
		m.fwd = nil
	case navBack:
		if n := len(m.back); n > 0 {
			m.back = m.back[:n-1]
		}
		m.fwd = trimStack(append(m.fwd, req.leaving))
	case navForward:
		if n := len(m.fwd); n > 0 {
			m.fwd = m.fwd[:n-1]
		}
		m.back = trimStack(append(m.back, req.leaving))
	}

	newPage := msg.src != m.src || req.kind != navReplace
	m.src = msg.src
	m.md, m.fromCache, m.saved = msg.md, msg.fromCache, msg.saved
	if newPage {
		m.query, m.matches, m.newer = "", nil, ""
		m.mouseSel.clear()
		m.focusID = -1
	}
	m.doc = nil // rebuild from scratch: positions belong to the old page
	m.y = 0
	m.rebuild()
	switch {
	case req.frag != "":
		m.gotoAnchor(req.frag)
	case req.y > 0:
		m.y = req.y
	}
	m.clampY()
	m.scroll.target = float64(m.y)
	m.scroll.snap()
	m.revealStart = m.now
	m.bookmarked = m.src != "" && isBookmarked(m.src)

	cmds := []tea.Cmd{}
	if m.cfg.History && m.src != "" {
		cmds = append(cmds, historyCmd(m.src, m.title(), m.cfg.HistoryLimit))
	}
	if msg.fromCache && isHTTP(m.src) && m.cfg.BackgroundRefresh { // refresh while you read
		m.refreshing = true
		cmds = append(cmds, fetchCmd(m.navID, m.src, false))
	}
	return tea.Batch(cmds...)
}

// gotoAnchor scrolls to the heading a "#fragment" refers to.
func (m *model) gotoAnchor(frag string) bool {
	if m.doc == nil {
		return false
	}
	want := slugify(frag)
	if dec, err := url.PathUnescape(frag); err == nil {
		want = slugify(dec)
	}
	for _, h := range m.doc.Heads {
		if slugify(h.Text) == want {
			m.y = h.Line
			m.clampY()
			return true
		}
	}
	return false
}

// title is the page's own title (its first level-1 heading), else its host.
func (m *model) title() string {
	if m.doc != nil {
		for _, h := range m.doc.Heads {
			if h.Level == 1 {
				return h.Text
			}
		}
		if len(m.doc.Heads) > 0 {
			return m.doc.Heads[0].Text
		}
	}
	return m.host()
}

// ---- links ----

// linksOrder lists each link once, in reading order (its first span).
func (m *model) linksOrder() []LinkSpan {
	if m.doc == nil {
		return nil
	}
	seen := map[int]bool{}
	var out []LinkSpan
	for _, l := range m.doc.Links {
		if !seen[l.ID] {
			seen[l.ID] = true
			out = append(out, l)
		}
	}
	return out
}

func (m *model) linkAt(p point) (int, bool) {
	if m.doc == nil {
		return 0, false
	}
	for _, l := range m.doc.Links {
		if l.Line == p.line && p.col >= l.From && p.col < l.To {
			return l.ID, true
		}
	}
	return 0, false
}

// openLink follows a link by its id.
func (m *model) openLink(id int) tea.Cmd {
	if m.doc == nil || id < 0 || id >= len(m.doc.URLs) {
		return nil
	}
	target, ok := resolveLink(m.src, m.doc.URLs[id])
	if !ok {
		return m.setToast("cannot open this kind of link: "+m.doc.URLs[id], true)
	}
	return m.navigate(target)
}

// moveFocus moves the keyboard focus to the next / previous link, starting from
// what is on screen when nothing is focused yet.
func (m *model) moveFocus(dir int) {
	order := m.linksOrder()
	if len(order) == 0 {
		return
	}
	_, _, bodyH, _ := m.geometry()
	idx := -1
	for i, l := range order {
		if l.ID == m.focusID {
			idx = i
		}
	}
	if idx < 0 {
		y := m.viewY()
		if dir > 0 {
			idx = -1
			for i, l := range order {
				if l.Line >= y {
					idx = i - 1
					break
				}
			}
			if idx == -1 && order[0].Line < y {
				idx = len(order) - 1
			}
		} else {
			idx = len(order)
			for i := len(order) - 1; i >= 0; i-- {
				if order[i].Line < y+bodyH {
					idx = i + 1
					break
				}
			}
		}
	}
	idx = ((idx+dir)%len(order) + len(order)) % len(order)
	m.focusID = order[idx].ID
	if l := order[idx].Line; l < m.y || l >= m.y+bodyH {
		m.y = l - bodyH/3
		m.clampY()
	}
}

// focusedURL is the destination of the focused link, for the footer.
func (m *model) focusedURL() string {
	if m.doc == nil || m.focusID < 0 || m.focusID >= len(m.doc.URLs) {
		return ""
	}
	if t, ok := resolveLink(m.src, m.doc.URLs[m.focusID]); ok {
		return t
	}
	return m.doc.URLs[m.focusID]
}

// ---- hint labels (follow a link by typing its label) ----

type hintLabel struct {
	label string
	id    int
	span  LinkSpan
}

const hintAlphabet = "asdfghjkl"

func hintLabels(n int) []string {
	chars := []rune(hintAlphabet)
	if n <= len(chars) {
		out := make([]string, n)
		for i := range out {
			out[i] = string(chars[i])
		}
		return out
	}
	var out []string
	for _, a := range chars {
		for _, b := range chars {
			out = append(out, string(a)+string(b))
			if len(out) == n {
				return out
			}
		}
	}
	return out // more links than two letters can label: the rest stay unlabeled
}

// enterHints labels every link on screen.
func (m *model) enterHints() tea.Cmd {
	if m.doc == nil {
		return nil
	}
	_, _, bodyH, _ := m.geometry()
	y := m.viewY()
	seen := map[int]bool{}
	var vis []LinkSpan
	for _, l := range m.doc.Links {
		if l.Line >= y && l.Line < y+bodyH && !seen[l.ID] {
			seen[l.ID] = true
			vis = append(vis, l)
		}
	}
	if len(vis) == 0 {
		return m.setToast("no links on this screen", true)
	}
	labels := hintLabels(len(vis))
	m.hints = m.hints[:0]
	for i, l := range vis {
		if i < len(labels) {
			m.hints = append(m.hints, hintLabel{labels[i], l.ID, l})
		}
	}
	m.mode, m.hintInput = modeHints, ""
	return nil
}

func (m *model) keyHints(k string) (tea.Model, tea.Cmd) {
	if k == "esc" || k == "ctrl+c" {
		m.mode = modeRead
		return m, nil
	}
	if k == "backspace" {
		if r := []rune(m.hintInput); len(r) > 0 {
			m.hintInput = string(r[:len(r)-1])
		}
		return m, nil
	}
	if utf8.RuneCountInString(k) != 1 {
		return m, nil
	}
	in := m.hintInput + k
	var match []hintLabel
	for _, h := range m.hints {
		if strings.HasPrefix(h.label, in) {
			match = append(match, h)
		}
	}
	switch {
	case len(match) == 0:
		m.mode = modeRead
		return m, m.setToast("no link labeled “"+in+"”", true)
	case len(match) == 1 && match[0].label == in:
		m.mode = modeRead
		return m, m.openLink(match[0].id)
	}
	m.hintInput = in
	return m, nil
}

// overlayLabel draws a label over a line starting at a rune column.
func overlayLabel(line string, runeCol int, label string, plain string) string {
	r := []rune(plain)
	cell := ansi.StringWidth(string(r[:min(runeCol, len(r))]))
	w := ansi.StringWidth(label)
	total := ansi.StringWidth(line)
	left := ansi.Cut(line, 0, cell)
	right := ""
	if cell+w < total {
		right = ansi.Cut(line, cell+w, total)
	}
	return left + style("30;103", label) + right
}

// ---- bookmarks ----

func (m *model) toggleBookmark() tea.Cmd {
	if m.src == "" {
		return m.setToast("the start page cannot be bookmarked", true)
	}
	added, err := bookmarkToggle(m.src, m.title())
	if err != nil {
		return m.setToast("could not save the bookmark: "+err.Error(), true)
	}
	m.bookmarked = added
	if added {
		return m.setToast("★ bookmarked", false)
	}
	return m.setToast("bookmark removed", false)
}

func (m *model) openExternalNow() tea.Cmd {
	if m.src == "" {
		return m.setToast("nothing to open: this is the start page", true)
	}
	if err := openExternal(m.src); err != nil {
		return m.setToast(err.Error(), true)
	}
	return m.setToast("opened in your browser", false)
}

// ---- address bar ----

type omniItem struct {
	label, hint, target string
	typed               bool // the first row: exactly what was typed
}

type omniState struct {
	input string
	sel   int
}

func matchesAll(hay string, words []string) bool {
	hay = strings.ToLower(hay)
	for _, w := range words {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}

func engineLabel(engine string) string {
	if _, ok := searchPresets[engine]; ok {
		return engine
	}
	return "custom search"
}

// omniItems are the rows of the address bar: what you typed, then bookmarks
// and history that match.
func (m *model) omniItems() []omniItem {
	var items []omniItem
	in := strings.TrimSpace(m.omni.input)
	if in != "" {
		target, search := normalizeInput(in, m.cfg.SearchEngine)
		if search {
			items = append(items, omniItem{"Search the web for “" + in + "”", engineLabel(m.cfg.SearchEngine), target, true})
		} else {
			items = append(items, omniItem{"Open " + in, "address", target, true})
		}
	}
	words := strings.Fields(strings.ToLower(in))
	seen := map[string]bool{}
	add := func(vs []Visit, star bool, limit int) {
		n := 0
		for _, v := range vs {
			if n >= limit || seen[v.URL] || !matchesAll(visitTitle(v)+" "+v.URL, words) {
				continue
			}
			seen[v.URL] = true
			prefix := "  "
			if star {
				prefix = "★ "
			}
			hint := hostOf(v.URL)
			items = append(items, omniItem{label: prefix + visitTitle(v), hint: hint, target: v.URL})
			n++
		}
	}
	add(bookmarkList(), true, 4)
	add(historyList(), false, 8-len(items))
	return items
}

func hostOf(u string) string {
	if p, err := url.Parse(u); err == nil && p.Host != "" {
		return p.Host
	}
	return u
}

func (m *model) openOmni() {
	m.mode = modeOmni
	m.omni = omniState{}
}

func (m *model) omniAppend(s string) {
	s = strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(s)
	m.omni.input += s
	m.omni.sel = 0
}

func (m *model) keyOmni(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	items := m.omniItems()
	switch k := msg.String(); k {
	case "esc":
		m.mode = modeRead
	case "enter":
		m.mode = modeRead
		if len(items) == 0 {
			return m, nil
		}
		it := items[min(m.omni.sel, len(items)-1)]
		return m, m.navigate(it.target)
	case "up", "ctrl+p", "shift+tab":
		if len(items) > 0 {
			m.omni.sel = (m.omni.sel - 1 + len(items)) % len(items)
		}
	case "down", "ctrl+n", "tab":
		if len(items) > 0 {
			m.omni.sel = (m.omni.sel + 1) % len(items)
		}
	case "backspace":
		if r := []rune(m.omni.input); len(r) > 0 {
			m.omni.input = string(r[:len(r)-1])
			m.omni.sel = 0
		}
	case "ctrl+u":
		m.omni.input, m.omni.sel = "", 0
	case "ctrl+w":
		f := strings.Fields(m.omni.input)
		if len(f) > 0 {
			m.omni.input = strings.Join(f[:len(f)-1], " ")
			if len(f) > 1 {
				m.omni.input += " "
			}
		}
		m.omni.sel = 0
	default:
		if msg.Text != "" {
			m.omniAppend(msg.Text)
		}
	}
	return m, nil
}

// ---- history and bookmarks panels ----

type listKind int

const (
	listHistory listKind = iota
	listBookmarks
)

type listState struct {
	kind   listKind
	all    []Visit
	filter string
	sel    int
}

func (l *listState) view() []Visit {
	words := strings.Fields(strings.ToLower(l.filter))
	if len(words) == 0 {
		return l.all
	}
	var out []Visit
	for _, v := range l.all {
		if matchesAll(visitTitle(v)+" "+v.URL, words) {
			out = append(out, v)
		}
	}
	return out
}

func (m *model) openList(kind listKind) tea.Cmd {
	m.list = listState{kind: kind}
	if kind == listHistory {
		if !m.cfg.History {
			return m.setToast("history is turned off (Settings → Browsing)", true)
		}
		m.list.all = historyList()
	} else {
		m.list.all = bookmarkList()
	}
	m.mode = modeList
	return nil
}

func (m *model) keyList(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	view := m.list.view()
	n := len(view)
	switch k := msg.String(); k {
	case "esc":
		m.mode = modeRead
	case "up", "ctrl+p":
		if n > 0 {
			m.list.sel = (m.list.sel - 1 + n) % n
		}
	case "down", "ctrl+n":
		if n > 0 {
			m.list.sel = (m.list.sel + 1) % n
		}
	case "pgup":
		m.list.sel = max(m.list.sel-8, 0)
	case "pgdown":
		m.list.sel = min(m.list.sel+8, max(n-1, 0))
	case "home":
		m.list.sel = 0
	case "end":
		m.list.sel = max(n-1, 0)
	case "enter":
		if n == 0 {
			return m, nil
		}
		m.mode = modeRead
		return m, m.navigate(view[min(m.list.sel, n-1)].URL)
	case "ctrl+d": // remove the selected entry
		if n == 0 {
			return m, nil
		}
		v := view[min(m.list.sel, n-1)]
		if m.list.kind == listHistory {
			_ = historyDelete(v.URL)
			m.list.all = historyList()
		} else {
			_ = bookmarkDelete(v.URL)
			m.list.all = bookmarkList()
			m.bookmarked = m.src != "" && isBookmarked(m.src)
		}
		m.list.sel = min(m.list.sel, max(len(m.list.view())-1, 0))
	case "ctrl+x": // clear everything (history only)
		if m.list.kind == listHistory && len(m.list.all) > 0 {
			m.mode, m.confirm = modeConfirm, confirmHistory
		}
	case "backspace":
		if r := []rune(m.list.filter); len(r) > 0 {
			m.list.filter = string(r[:len(r)-1])
			m.list.sel = 0
		}
	default:
		if msg.Text != "" {
			m.list.filter += msg.Text
			m.list.sel = 0
		}
	}
	return m, nil
}

// listRows renders the visible entries of the panel.
func (m *model) listRows() []boxRow {
	view := m.list.view()
	rows := make([]boxRow, 0, len(view))
	for _, v := range view {
		hint := hostOf(v.URL)
		if m.list.kind == listHistory {
			hint += " · " + humanAge(time.Since(v.At))
		}
		rows = append(rows, boxRow{visitTitle(v), hint})
	}
	return rows
}

func (m *model) confirmText() []boxRow {
	switch m.confirm {
	case confirmHistory:
		return []boxRow{{"Clear all your history?", ""}, {fmt.Sprintf("%d pages", len(m.list.all)), ""}, {"", ""}, {"y / enter  clear", ""}, {"esc        cancel", ""}}
	case confirmSettings:
		return []boxRow{{"Reset every setting to its default?", ""}, {"Your key bindings are kept.", ""}, {"", ""}, {"y / enter  reset", ""}, {"esc        cancel", ""}}
	}
	pages, size := cacheStats()
	return []boxRow{{"Clear all cache?", ""}, {fmt.Sprintf("%d pages · %s on disk", pages, humanSize(size)), ""}, {"", ""}, {"y / enter  clear", ""}, {"esc        cancel", ""}}
}
