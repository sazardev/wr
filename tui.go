package main

import (
	"fmt"
	"math"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type mode int

const (
	modeRead   mode = iota
	modeSearch      // typing a search in the page
	modeHints       // typing a link label
	modeOmni        // from here on the mode is a floating panel: the address bar
	modeMenu
	modeConfirm
	modeTOC
	modeSettings
	modeKeys
	modeList // history or bookmarks
)

func (m mode) isPanel() bool { return m >= modeOmni }

type confirmKind int

const (
	confirmCache confirmKind = iota
	confirmHistory
	confirmSettings
)

type (
	loadedMsg struct {
		id        int
		src       string
		md        string
		fromCache bool
		saved     time.Time
		err       error
	}
	fetchedMsg struct {
		id    int
		src   string
		md    string
		err   error
		apply bool // apply when done (explicit reload) or just announce it
	}
	animTickMsg    time.Time
	toastExpireMsg struct{ id int }
	editorDoneMsg  struct{ err error }
)

type model struct {
	src    string // the page being read ("" = the start page)
	cfg    Config
	fresh  bool
	keymap map[string]action

	// settings that came from a flag and must not be saved
	linksFromFlag bool
	fileLinks     string

	w, h int
	md   string
	doc  *Doc
	docW int

	y      int    // logical scroll position (the target)
	scroll spring // displayed position: follows y with spring physics

	mode, prevMode mode
	sel            int // selection in menus and panels
	input          string

	// browsing
	back, fwd  []navEntry
	navID      int
	pending    *navReq
	focusID    int // keyboard-focused link (-1 = none)
	hints      []hintLabel
	hintInput  string
	omni       omniState
	list       listState
	confirm    confirmKind
	capture    string // action waiting for a key to bind
	captureAdd bool
	bookmarked bool

	fromCache  bool
	saved      time.Time
	loading    bool
	refreshing bool
	loadErr    error
	newer      string // a newer version already downloaded, not applied yet

	now, t0, lastTick time.Time
	animRunning       bool
	revealStart       time.Time
	panelStart        time.Time

	toast      string
	toastWarn  bool
	toastID    int
	toastStart time.Time

	query   string
	matches []match
	cur     int
	searchY int // position when search typing began (to cancel)

	mouseSel selection // mouse selection (copied to the clipboard on release)
}

func newModel(src string, cfg Config, fresh bool, cfgErr error) *model {
	now := time.Now()
	m := &model{src: src, cfg: cfg, fresh: fresh, loading: true, now: now, t0: now, focusID: -1}
	m.keymap = buildKeymap(&m.cfg)
	if file, err := loadConfig(); err == nil && file.Links != cfg.Links {
		m.linksFromFlag, m.fileLinks = true, file.Links
	}
	if cfgErr != nil {
		m.toast, m.toastWarn = cfgErr.Error()+" (using defaults)", true
	}
	return m
}

func runTUI(src string, cfg Config, fresh bool, cfgErr error) error {
	_, err := tea.NewProgram(newModel(src, cfg, fresh, cfgErr)).Run()
	return err
}

// ---- commands ----

func animTick(fast bool) tea.Cmd {
	d := frameSlow
	if fast {
		d = frameFast
	}
	return tea.Tick(d, func(t time.Time) tea.Msg { return animTickMsg(t) })
}

func (m *model) setToast(s string, warn bool) tea.Cmd {
	m.toastID++
	m.toast, m.toastWarn, m.toastStart = s, warn, m.now
	id := m.toastID
	linger := toastLinger
	if warn {
		linger = toastWarnLinger
	}
	return tea.Tick(linger, func(time.Time) tea.Msg { return toastExpireMsg{id} })
}

func (m *model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.startNav(navInitial, m.src, 0, m.fresh)}
	if m.toast != "" {
		cmds = append(cmds, m.setToast(m.toast, m.toastWarn))
	}
	return m.afterUpdate(tea.Batch(cmds...))
}

// ---- animation scheduling ----

// anim reports whether one kind of animation is on (the master switch and its own).
func (m *model) anim(flag bool) bool { return m.cfg.Animations && flag }

// animating reports whether anything on screen is still moving, and whether it
// needs smooth (60 fps) frames or just the slow spinner tick.
func (m *model) animating() (active, fast bool) {
	spinner := m.loading || m.refreshing
	_, toastDone := typed(m.toast, m.toastStart, m.now, typeSpeed)
	switch {
	case m.anim(m.cfg.AnimScroll) && !m.scroll.settled(),
		m.anim(m.cfg.AnimReveal) && progressSince(m.revealStart, m.now, revealDur) < 1,
		m.anim(m.cfg.AnimPanels) && m.mode.isPanel() && progressSince(m.panelStart, m.now, panelDur) < 1,
		m.anim(m.cfg.AnimNotices) && !toastDone,
		m.anim(m.cfg.AnimLoading) && m.loading && m.doc == nil:
		return true, true
	}
	return spinner, false
}

// afterUpdate runs after every message: it keeps the scroll spring pointed at
// the logical position and schedules the next animation frame if needed.
func (m *model) afterUpdate(cmd tea.Cmd) tea.Cmd {
	if m.mode != m.prevMode {
		if m.mode.isPanel() {
			m.panelStart = m.now
		}
		m.prevMode = m.mode
	}
	m.scroll.target = float64(m.y)
	if !m.anim(m.cfg.AnimScroll) {
		m.scroll.snap()
	}
	if active, fast := m.animating(); active && !m.animRunning {
		m.animRunning, m.lastTick = true, m.now
		return tea.Batch(cmd, animTick(fast))
	}
	return cmd
}

// ---- geometry ----

func (m *model) tooSmall() bool { return m.w < 24 || m.h < 3 }

func (m *model) geometry() (contentW, left, bodyH, footerH int) {
	sb := 0
	if m.cfg.Scrollbar {
		sb = 1
	}
	usable := m.w - sb
	margin := 2
	if m.w < 60 {
		margin = 1 // narrow terminals: don't waste columns
	}
	maxW := m.cfg.Width
	if maxW == 0 {
		maxW = usable // "full width"
	}
	contentW = max(min(maxW, usable-2*margin), 20)
	left = margin
	if m.cfg.Center {
		left = max((usable-contentW)/2, margin)
	}
	switch {
	case !m.cfg.Footer || m.h < 4:
		footerH = 0
	case m.cfg.FooterRule && m.h >= 8:
		footerH = 2 // rule + row
	default:
		footerH = 1 // just the row
	}
	return contentW, left, max(m.h-footerH, 1), footerH
}

func (m *model) maxY() int {
	if m.doc == nil {
		return 0
	}
	_, _, bodyH, _ := m.geometry()
	return max(len(m.doc.Lines)-bodyH, 0)
}

func (m *model) clampY() { m.y = min(max(m.y, 0), m.maxY()) }

// viewPos is the (fractional) displayed scroll position.
func (m *model) viewPos() float64 {
	if !m.anim(m.cfg.AnimScroll) {
		return float64(m.y)
	}
	return math.Min(math.Max(m.scroll.pos, 0), float64(m.maxY()))
}

func (m *model) viewY() int { return int(math.Round(m.viewPos())) }

// rebuild draws the document again (width, style or link change) and keeps the
// approximate reading position. The spring snaps: no fly-by after a resize.
func (m *model) rebuild() {
	if m.md == "" || m.w == 0 {
		return
	}
	cw, _, _, _ := m.geometry()
	frac := 0.0
	if m.doc != nil && len(m.doc.Lines) > 0 {
		frac = float64(m.y) / float64(len(m.doc.Lines))
	}
	m.doc = renderDoc(m.md, cw, m.cfg)
	m.docW = cw
	m.y = int(frac * float64(len(m.doc.Lines)))
	m.clampY()
	m.scroll.target = float64(m.y)
	m.scroll.snap()
	m.mouseSel.clear()
	if m.focusID >= len(m.doc.URLs) {
		m.focusID = -1
	}
	m.refreshMatches()
}

func (m *model) refreshMatches() {
	m.matches = nil
	if m.doc != nil && m.query != "" {
		m.matches = findMatches(m.doc.Plain, m.query)
	}
	m.cur = min(m.cur, max(len(m.matches)-1, 0))
}

// ---- Update ----

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if t, ok := msg.(animTickMsg); ok {
		m.now = time.Time(t)
	} else {
		m.now = time.Now()
	}
	mdl, cmd := m.update(msg)
	return mdl, m.afterUpdate(cmd)
}

func (m *model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		atEnd := m.doc != nil && m.y > 0 && m.y >= m.maxY() // at the end: stay at the end
		m.w, m.h = msg.Width, msg.Height
		if cw, _, _, _ := m.geometry(); m.docW != cw || (m.doc == nil && m.md != "") {
			m.rebuild()
		}
		if atEnd {
			m.y = m.maxY()
		}
		m.clampY()
		m.scroll.target = float64(m.y)
		m.scroll.snap() // a resize repositions: it must not animate a fly-by
		return m, nil

	case animTickMsg:
		m.animRunning = false
		n := 1
		if !m.lastTick.IsZero() {
			n = min(max(int(math.Round(float64(m.now.Sub(m.lastTick))/float64(frameFast))), 1), 6)
		}
		m.lastTick = m.now
		m.scroll.step(n)
		if m.scroll.settled() {
			m.scroll.snap() // arrive exactly and stop asking for frames
		}
		return m, nil

	case toastExpireMsg:
		if msg.id == m.toastID {
			m.toast = ""
		}
		return m, nil

	case loadedMsg:
		if msg.id != m.navID { // an older request that was overtaken
			return m, nil
		}
		if msg.err != nil {
			m.pending, m.loading = nil, false
			if m.doc == nil {
				m.loadErr = msg.err
				return m, nil
			}
			return m, m.setToast("could not open "+hostOf(msg.src)+": "+msg.err.Error(), true)
		}
		return m, m.commitNav(msg)

	case fetchedMsg:
		if msg.src != m.src {
			return m, nil // you have moved on
		}
		m.refreshing = false
		switch {
		case msg.err != nil && msg.apply:
			return m, m.setToast("could not reload: "+msg.err.Error(), true)
		case msg.err != nil:
			return m, m.setToast("offline: showing the saved version", true)
		case msg.apply || m.doc == nil:
			m.md, m.fromCache, m.saved, m.newer = msg.md, false, time.Now(), ""
			m.rebuild()
			m.revealStart = m.now
			return m, m.setToast("✓ page reloaded", false)
		case msg.md != m.md:
			m.newer = msg.md
			return m, m.setToast("● a newer version is ready: press "+keyHint(&m.cfg, actReload)+" to view it", false)
		}
		return m, nil // already up to date: no need to announce it

	case editorDoneMsg:
		cfg, err := loadConfig()
		if err != nil {
			return m, m.setToast(err.Error(), true)
		}
		m.cfg, m.keymap = cfg, buildKeymap(&cfg)
		applyRuntime(cfg)
		m.rebuild()
		return m, m.setToast("✓ settings reloaded", false)

	case tea.MouseWheelMsg:
		if m.mode == modeRead && m.doc != nil {
			switch msg.Mouse().Button {
			case tea.MouseWheelUp:
				m.y -= m.cfg.WheelLines
			case tea.MouseWheelDown:
				m.y += m.cfg.WheelLines
			}
			m.clampY()
		}
		return m, nil

	case tea.MouseClickMsg:
		m.toast = "" // a click dismisses a notice too
		mo := tea.Mouse(msg)
		switch mo.Button {
		case tea.MouseBackward:
			return m, m.goBack()
		case tea.MouseForward:
			return m, m.goForward()
		case tea.MouseLeft:
			if m.mode == modeRead && m.doc != nil {
				return m, m.mouseDown(mo.X, mo.Y)
			}
		}
		return m, nil

	case tea.MouseMotionMsg:
		if m.mouseSel.dragging {
			mo := tea.Mouse(msg)
			m.mouseDrag(mo.X, mo.Y)
		}
		return m, nil

	case tea.MouseReleaseMsg:
		if m.mouseSel.dragging {
			return m, m.mouseUp()
		}
		return m, nil

	case tea.PasteMsg:
		switch m.mode {
		case modeOmni:
			m.omniAppend(msg.Content)
		case modeSearch:
			m.input += strings.NewReplacer("\n", " ", "\r", " ").Replace(msg.Content)
			m.liveSearch()
		case modeList:
			m.list.filter += strings.NewReplacer("\n", " ", "\r", " ").Replace(msg.Content)
			m.list.sel = 0
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m *model) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if m.capture == "" && k == "ctrl+c" {
		return m, tea.Quit
	}
	m.toast = "" // any key dismisses a notice at once
	switch m.mode {
	case modeSearch:
		return m.keySearch(msg)
	case modeHints:
		return m.keyHints(k)
	case modeOmni:
		return m.keyOmni(msg)
	case modeMenu:
		return m.keyMenu(k)
	case modeConfirm:
		return m.keyConfirm(k)
	case modeTOC:
		return m.keyTOC(k)
	case modeSettings:
		return m.keySettings(k)
	case modeKeys:
		return m.keyKeys(msg)
	case modeList:
		return m.keyList(msg)
	}
	return m.keyRead(k)
}

func (m *model) keyConfirm(k string) (tea.Model, tea.Cmd) {
	switch k {
	case "y", "Y", "enter":
		m.mode = modeRead
		switch m.confirm {
		case confirmHistory:
			_ = historyClear()
			m.list.all = nil
			return m, m.setToast("✓ history cleared", false)
		case confirmSettings:
			keys := m.cfg.Keys
			m.cfg = defaultConfig()
			m.cfg.Keys = keys
			m.linksFromFlag = false
			return m, tea.Batch(m.applyConfigRebuild(), m.setToast("✓ settings reset", false))
		}
		pages, _ := cacheStats()
		_ = cacheClear()
		return m, m.setToast(fmt.Sprintf("✓ cache cleared (%d pages)", pages), false)
	}
	if m.confirm == confirmHistory { // came from the history panel: go back to it
		m.mode = modeList
	} else {
		m.mode = modeRead
	}
	return m, nil
}

func (m *model) applyConfigRebuild() tea.Cmd { return m.applyConfig(true) }

func (m *model) keyRead(k string) (tea.Model, tea.Cmd) {
	_, _, bodyH, _ := m.geometry()
	if k == "esc" { // clears the search, the selection and the link focus; quit with q
		m.query, m.matches, m.focusID = "", nil, -1
		m.mouseSel.clear()
		return m, nil
	}
	switch m.keymap[k] {
	case actQuit:
		return m, tea.Quit
	case actDown:
		m.y++
	case actUp:
		m.y--
	case actPageDown:
		m.y += max(bodyH-1, 1)
	case actPageUp:
		m.y -= max(bodyH-1, 1)
	case actHalfDown:
		m.y += bodyH / 2
	case actHalfUp:
		m.y -= bodyH / 2
	case actTop:
		m.y = 0
	case actBottom:
		m.y = m.maxY()
	case actNextSec:
		if m.doc != nil {
			for _, h := range m.doc.Heads {
				if h.Line > m.y {
					m.y = h.Line
					break
				}
			}
		}
	case actPrevSec:
		if m.doc != nil {
			for i := len(m.doc.Heads) - 1; i >= 0; i-- {
				if m.doc.Heads[i].Line < m.y {
					m.y = m.doc.Heads[i].Line
					break
				}
			}
		}
	case actSearch:
		m.mode, m.input, m.searchY = modeSearch, "", m.y
	case actNextMatch:
		m.step(1)
	case actPrevMatch:
		m.step(-1)
	case actIndex:
		return m, m.openTOC()
	case actReload:
		return m, m.reload()
	case actMenu:
		m.mode, m.sel = modeMenu, 0
	case actSettings:
		m.mode, m.sel = modeSettings, 0
	case actYank:
		return m, m.copySelection()
	case actOpen:
		m.openOmni()
	case actBack:
		return m, m.goBack()
	case actForward:
		return m, m.goForward()
	case actHome:
		return m, m.startNav(navPush, "", 0, false)
	case actHistory:
		return m, m.openList(listHistory)
	case actBookmarks:
		return m, m.openList(listBookmarks)
	case actBookmark:
		return m, m.toggleBookmark()
	case actExternal:
		return m, m.openExternalNow()
	case actFollow:
		return m, m.enterHints()
	case actNextLink:
		m.moveFocus(1)
	case actPrevLink:
		m.moveFocus(-1)
	case actActivate:
		if m.focusID >= 0 {
			return m, m.openLink(m.focusID)
		}
		m.y++
	}
	m.clampY()
	return m, nil
}

func (m *model) step(d int) {
	if len(m.matches) == 0 {
		return
	}
	m.cur = (m.cur + d + len(m.matches)) % len(m.matches)
	m.showMatch()
}

func (m *model) showMatch() {
	if len(m.matches) == 0 {
		return
	}
	_, _, bodyH, _ := m.geometry()
	if l := m.matches[m.cur].line; l < m.y || l >= m.y+bodyH {
		m.y = l - bodyH/3
	}
	m.clampY()
}

func (m *model) keySearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch k := msg.String(); k {
	case "esc":
		m.mode, m.query, m.matches, m.y = modeRead, "", nil, m.searchY
	case "enter":
		m.mode = modeRead
		if m.query != "" && len(m.matches) == 0 {
			return m, m.setToast("no results for «"+m.query+"»", true)
		}
	case "backspace":
		if r := []rune(m.input); len(r) > 0 {
			m.input = string(r[:len(r)-1])
		}
		m.liveSearch()
	case "ctrl+u":
		m.input = ""
		m.liveSearch()
	default:
		if msg.Text != "" { // printable characters (space included)
			m.input += msg.Text
			m.liveSearch()
		}
	}
	return m, nil
}

// liveSearch searches while you type: it highlights and jumps to the first
// match from where the search began.
func (m *model) liveSearch() {
	m.query = m.input
	m.refreshMatches()
	m.cur = 0
	for i, mt := range m.matches {
		if mt.line >= m.searchY {
			m.cur = i
			break
		}
	}
	m.showMatch()
}

func (m *model) reload() tea.Cmd {
	if m.src == "" || !isHTTP(m.src) { // the start page and local files are read again in place
		return m.startNav(navReplace, m.src, m.y, true)
	}
	if m.newer != "" {
		m.md, m.fromCache, m.saved, m.newer = m.newer, false, time.Now(), ""
		m.rebuild()
		m.revealStart = m.now
		return m.setToast("✓ newer version applied", false)
	}
	m.refreshing = true
	return fetchCmd(m.navID, m.src, true)
}

// ---- View ----

func (m *model) host() string {
	if m.src == "" {
		return "wr"
	}
	if u, err := url.Parse(m.src); err == nil && u.Host != "" {
		return u.Host
	}
	return filepath.Base(m.src)
}

// View delivers the content and asks for the alternate screen (and the mouse,
// if enabled).
func (m *model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "wr · " + m.title()
	if m.cfg.Mouse {
		v.MouseMode = tea.MouseModeCellMotion
	}
	return v
}

func (m *model) render() string {
	if m.w == 0 || m.h == 0 {
		return ""
	}
	if m.tooSmall() {
		return ansi.Truncate(dim("wr: too small"), max(m.w, 1), "")
	}
	body := m.body()
	switch m.mode {
	case modeOmni:
		body = m.composeAt(body, m.omniBox(), 1)
	case modeMenu:
		body = m.compose(body, m.menuBox())
	case modeConfirm:
		body = m.compose(body, m.confirmBox())
	case modeTOC:
		body = m.compose(body, m.tocBox())
	case modeSettings:
		body = m.compose(body, m.settingsBox())
	case modeKeys:
		body = m.compose(body, m.keysBox())
	case modeList:
		body = m.compose(body, m.listBox())
	}
	return strings.Join(append(body, m.footer()...), "\n")
}

func (m *model) pad(s string) string {
	return s + strings.Repeat(" ", max(m.w-ansi.StringWidth(s), 0))
}

func (m *model) center(s string) string {
	return m.pad(strings.Repeat(" ", max((m.w-ansi.StringWidth(s))/2, 0)) + s)
}

func (m *model) elapsed() float64 { return m.now.Sub(m.t0).Seconds() }

func (m *model) spin() string {
	frame := int(m.now.Sub(m.t0) / frameSlow)
	if m.cfg.Braille {
		return spinner(frame)
	}
	return []string{"|", "/", "-", "\\"}[frame%4]
}

// loadingScreen: the braille wave and what is being fetched.
func (m *model) loadingScreen(rows []string, bodyH int) {
	row := max(bodyH/2-1, 0)
	host := m.host()
	if m.pending != nil {
		host = hostOf(m.pending.src)
		if m.pending.src == "" {
			host = "wr"
		}
	}
	if m.anim(m.cfg.AnimLoading) && m.cfg.Braille {
		rows[row] = m.center(wave(min(max(m.w/3, 8), 36), m.elapsed()))
	} else {
		rows[row] = m.center(acc(m.spin()))
	}
	if row+2 < len(rows) {
		dots := strings.Repeat(".", int(m.elapsed()*3)%4)
		rows[row+2] = m.center(dim("fetching ") + bold(host) + dim(dots))
	}
}

func (m *model) body() []string {
	cw, left, bodyH, _ := m.geometry()
	rows := make([]string, bodyH)
	blank := strings.Repeat(" ", m.w)
	for i := range rows {
		rows[i] = blank
	}
	if m.doc == nil {
		switch {
		case m.loadErr != nil:
			rows[bodyH/2] = m.center(style("91", "✗ "+m.loadErr.Error()))
			if bodyH/2+2 < bodyH {
				rows[bodyH/2+2] = m.center(dim(keyHint(&m.cfg, actOpen) + " open another page · " + keyHint(&m.cfg, actHome) + " start page · " + keyHint(&m.cfg, actQuit) + " quit"))
			}
		case m.loading:
			m.loadingScreen(rows, bodyH)
		}
		return rows
	}

	y := m.viewY()

	// matches per visible line, for highlighting
	var byLine map[int][]span
	var curSpan map[int]int
	if len(m.matches) > 0 {
		byLine, curSpan = map[int][]span{}, map[int]int{}
		for i, mt := range m.matches {
			if mt.line >= y && mt.line < y+bodyH {
				if i == m.cur {
					curSpan[mt.line] = len(byLine[mt.line])
				}
				byLine[mt.line] = append(byLine[mt.line], mt.sp)
			}
		}
	}

	// the focused link, and the labels of hint mode
	focus := map[int][]span{}
	if m.focusID >= 0 {
		for _, l := range m.doc.Links {
			if l.ID == m.focusID && l.Line >= y && l.Line < y+bodyH {
				focus[l.Line] = append(focus[l.Line], span{l.From, l.To})
			}
		}
	}
	labels := map[int][]hintLabel{}
	if m.mode == modeHints {
		for _, h := range m.hints {
			if strings.HasPrefix(h.label, m.hintInput) && h.span.Line >= y && h.span.Line < y+bodyH {
				labels[h.span.Line] = append(labels[h.span.Line], h)
			}
		}
	}

	var sb []string
	if m.cfg.Scrollbar {
		if m.cfg.Braille {
			sb = scrollbar(bodyH, len(m.doc.Lines), bodyH, m.viewPos())
		} else {
			sb = plainScrollbar(bodyH, len(m.doc.Lines), bodyH, m.viewPos())
		}
	}

	// reveal animation: the page wipes in from the top with a bright scan line
	visible, scanRow := bodyH, -1
	if m.anim(m.cfg.AnimReveal) {
		if p := progressSince(m.revealStart, m.now, revealDur); p < 1 {
			visible = int(math.Ceil(easeOutCubic(p) * float64(bodyH)))
			scanRow = visible
		}
	}

	pad := strings.Repeat(" ", left)
	for i := 0; i < bodyH; i++ {
		line := ""
		switch {
		case i == scanRow:
			glyph := "─"
			if m.cfg.Braille {
				glyph = "⣿"
			}
			line = style("96", strings.Repeat(glyph, cw))
		case i < visible && y+i < len(m.doc.Lines):
			idx := y + i
			line = m.doc.Lines[idx]
			if sp, ok := byLine[idx]; ok {
				c := -1
				if v, ok := curSpan[idx]; ok {
					c = v
				}
				line = highlightLine(line, sp, c)
			}
			if sp, ok := focus[idx]; ok {
				line = markSpans(line, sp, func(int) (string, string) { return "\x1b[30;" + accentBG + "m", "\x1b[39;49m" })
			}
			if ss, ok := m.mouseSel.spanOn(idx, len([]rune(m.doc.Plain[idx]))); ok {
				line = selectLine(line, ss)
			}
			for _, h := range labels[idx] {
				line = overlayLabel(line, h.span.From, h.label, m.doc.Plain[idx])
			}
			line = ansi.Truncate(line, cw, "")
		}
		row := pad + line
		if m.cfg.Scrollbar {
			row += strings.Repeat(" ", max(m.w-1-left-ansi.StringWidth(line), 0))
			if sb != nil {
				row += sb[i]
			} else {
				row += " "
			}
		}
		rows[i] = row
	}
	return rows
}

func (m *model) percent() (float64, bool) {
	if m.doc == nil || m.maxY() == 0 {
		return 100, false
	}
	return 100 * m.viewPos() / float64(m.maxY()), true
}

// ---- footer ----

func (m *model) footer() []string {
	_, _, _, fh := m.geometry()
	switch fh {
	case 0:
		return nil
	case 1:
		return []string{m.footerRow()}
	}
	return []string{m.rule(), m.footerRow()}
}

// rule is a thin line with a gradient in the accent color.
func (m *model) rule() string {
	glyph := "─"
	if m.cfg.Braille {
		glyph = "⣀"
	}
	third := m.w / 3
	return "\x1b[" + accentDim + "m" + strings.Repeat(glyph, third) +
		"\x1b[" + accentFG + "m" + strings.Repeat(glyph, third) +
		"\x1b[96m" + strings.Repeat(glyph, m.w-2*third) + "\x1b[0m"
}

// footerRow is the whole footer: shortcuts (or the current prompt) on the
// left, reading progress on the right.
func (m *model) footerRow() string {
	right := m.progressText()
	room := max(m.w-ansi.StringWidth(right)-1, 4)
	left := ansi.Truncate(m.leftText(room), room, "…")
	gap := max(m.w-ansi.StringWidth(left)-ansi.StringWidth(right), 0)
	return left + strings.Repeat(" ", gap) + right
}

// progressText is the right side of the footer: what needs your attention
// (a bookmark star, a newer version, search results), then the progress.
func (m *model) progressText() string {
	chips := ""
	if m.cfg.FooterChips && m.w >= 70 {
		if m.bookmarked {
			chips += style("93", "★") + "  "
		}
		if m.newer != "" {
			chips += style("93", "● new version ("+keyHint(&m.cfg, actReload)+")") + "  "
		}
		if len(m.matches) > 0 {
			q := []rune(m.query)
			if len(q) > 12 {
				q = append(q[:11], '…')
			}
			chips += style("96", fmt.Sprintf("«%s» %d/%d", string(q), m.cur+1, len(m.matches))) + "  "
		}
	}
	mode := m.cfg.FooterProgress
	if mode == "off" {
		return chips
	}
	pct, scrollable := m.percent()
	if !scrollable {
		if m.doc != nil && m.w >= 40 {
			return chips + dim("all visible") + " "
		}
		return chips
	}
	label := bold(fmt.Sprintf("%3d%%", int(math.Round(pct)))) + " "
	cells := 0
	switch {
	case m.w >= 110:
		cells = 16
	case m.w >= 80:
		cells = 12
	case m.w >= 60:
		cells = 8
	case m.w >= 45:
		cells = 5
	}
	bar := ""
	if cells > 0 {
		if m.cfg.Braille {
			bar = progressBar(cells, pct)
		} else {
			bar = plainBar(cells, pct)
		}
	}
	switch mode {
	case "percent":
		return chips + label
	case "bar":
		if bar == "" {
			return chips + label
		}
		return chips + bar + " "
	}
	if bar == "" {
		return chips + label
	}
	return chips + bar + " " + label
}

// leftText is the left side of the footer.
func (m *model) leftText(room int) string {
	lead := " "
	if m.loading || m.refreshing {
		lead = " " + acc(m.spin()) + " "
	}
	switch {
	case m.mode == modeSearch:
		count := dim("type to search")
		if m.input != "" {
			if len(m.matches) == 0 {
				count = style("91", "no results")
			} else {
				count = style("96", fmt.Sprintf("%d results", len(m.matches)))
			}
		}
		return lead + acc("/") + m.input + cursor() + "  " + count + "  " + hint("enter", "accept") + "  " + hint("esc", "cancel")
	case m.mode == modeHints:
		return lead + hint("type a label", "to follow a link") + "  " + m.hintInput + "  " + hint("esc", "cancel")
	case m.mode == modeOmni:
		return lead + hint("↑↓", "choose") + "  " + hint("enter", "go") + "  " + hint("esc", "cancel")
	case m.mode.isPanel():
		return lead + hint("↑↓", "move") + "  " + hint("enter", "select") + "  " + hint("esc", "close")
	case m.toast != "":
		col := "92"
		if m.toastWarn {
			col = "93"
		}
		shown := m.toast
		if m.anim(m.cfg.AnimNotices) {
			shown, _ = typed(m.toast, m.toastStart, m.now, typeSpeed)
		}
		return lead + "\x1b[" + col + "m" + shown + "\x1b[0m"
	case m.focusID >= 0:
		return lead + acc("→") + " " + style("96", m.focusedURL())
	case !m.cfg.FooterHints:
		return lead
	}
	return lead + m.hintsText(room-ansi.StringWidth(lead))
}

// hintsText shows as many shortcuts as the width allows, most important first,
// in their natural order, with the keys you actually have bound.
func (m *model) hintsText(room int) string {
	type h struct {
		prio      int
		key, desc string
	}
	pair := func(a, b action) string {
		x, y := keyHint(&m.cfg, a), keyHint(&m.cfg, b)
		switch {
		case x == "":
			return y
		case y == "":
			return x
		}
		return x + " " + y
	}
	all := []h{
		{2, keyHint(&m.cfg, actOpen), "open"},
		{4, keyHint(&m.cfg, actSearch), "search"},
		{3, keyHint(&m.cfg, actBack), "back"},
		{7, keyHint(&m.cfg, actForward), "fwd"},
		{6, keyHint(&m.cfg, actFollow), "links"},
		{8, pair(actNextMatch, actPrevMatch), "next"},
		{9, pair(actNextSec, actPrevSec), "sections"},
		{5, keyHint(&m.cfg, actIndex), "index"},
		{10, keyHint(&m.cfg, actReload), "reload"},
		{1, keyHint(&m.cfg, actMenu), "menu"},
		{0, keyHint(&m.cfg, actQuit), "quit"},
	}
	if m.cfg.Mouse {
		all = append(all, h{11, "drag", "copy"})
	}
	keep := map[int]bool{}
	used := 0
	for p := 0; p <= 11; p++ {
		for i, it := range all {
			if it.prio != p || it.key == "" {
				continue
			}
			w := ansi.StringWidth(it.key) + 1 + len(it.desc)
			if len(keep) > 0 {
				w += 2
			}
			if used+w <= room {
				keep[i] = true
				used += w
			}
		}
	}
	var parts []string
	for i, it := range all {
		if keep[i] {
			parts = append(parts, hint(it.key, it.desc))
		}
	}
	return strings.Join(parts, "  ")
}

// ---- variants without braille ----

func plainScrollbar(h, total, view int, top float64) []string {
	if h <= 0 || total <= view {
		return nil
	}
	thumb := max(1, h*view/total)
	start := 0
	if total > view {
		start = int(math.Round(float64(h-thumb) * top / float64(total-view)))
	}
	out := make([]string, h)
	for i := range out {
		if i >= start && i < start+thumb {
			out[i] = acc("┃")
		} else {
			out[i] = dim("│")
		}
	}
	return out
}

func plainBar(cells int, pct float64) string {
	filled := min(max(int(float64(cells)*pct/100), 0), cells)
	return "\x1b[" + accentFG + "m" + strings.Repeat("█", filled) + "\x1b[90m" + strings.Repeat("░", cells-filled) + "\x1b[0m"
}

// ---- external editor (config file) ----

func (m *model) openConfig() tea.Cmd {
	path, err := ensureConfigFile()
	if err != nil {
		return m.setToast(err.Error(), true)
	}
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	return tea.ExecProcess(exec.Command(parts[0], append(parts[1:], path)...), func(err error) tea.Msg {
		return editorDoneMsg{err}
	})
}
