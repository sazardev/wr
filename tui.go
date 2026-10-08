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
	modeRead mode = iota
	modeSearch
	modeMenu // from here on the mode is a floating panel
	modeConfirm
	modeTOC
	modeHelp
)

func (m mode) isPanel() bool { return m >= modeMenu }

type (
	loadedMsg struct {
		md        string
		fromCache bool
		saved     time.Time
		err       error
	}
	fetchedMsg struct {
		md    string
		err   error
		apply bool // apply when done (explicit reload) or just announce it
	}
	animTickMsg    time.Time
	toastExpireMsg struct{ id int }
	editorDoneMsg  struct{ err error }
)

type model struct {
	src   string
	cfg   Config
	fresh bool

	w, h int
	md   string
	doc  *Doc
	docW int

	y      int    // logical scroll position (the target)
	scroll spring // displayed position: follows y with spring physics

	mode, prevMode mode
	sel            int // selection in menu / index
	input          string

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
	m := &model{src: src, cfg: cfg, fresh: fresh, loading: true, now: now, t0: now}
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

func loadCmd(src string, fresh bool) tea.Cmd {
	return func() tea.Msg {
		if isHTTP(src) && !fresh {
			if md, saved, ok := cacheGet(src); ok {
				return loadedMsg{md: sanitize(md), fromCache: true, saved: saved}
			}
		}
		md, err := load(src)
		return loadedMsg{md: md, err: err, saved: time.Now()}
	}
}

func fetchCmd(src string, apply bool) tea.Cmd {
	return func() tea.Msg {
		md, err := load(src)
		return fetchedMsg{md: md, err: err, apply: apply}
	}
}

func (m *model) setToast(s string, warn bool) tea.Cmd {
	m.toastID++
	m.toast, m.toastWarn, m.toastStart = s, warn, m.now
	id := m.toastID
	return tea.Tick(4*time.Second, func(time.Time) tea.Msg { return toastExpireMsg{id} })
}

func (m *model) Init() tea.Cmd {
	cmds := []tea.Cmd{loadCmd(m.src, m.fresh)}
	if m.toast != "" {
		cmds = append(cmds, m.setToast(m.toast, m.toastWarn))
	}
	return m.afterUpdate(tea.Batch(cmds...))
}

// ---- animation scheduling ----

// animating reports whether anything on screen is still moving, and whether it
// needs smooth (60 fps) frames or just the slow spinner tick.
func (m *model) animating() (active, fast bool) {
	spinner := m.loading || m.refreshing
	if !m.cfg.Animations {
		return spinner, false
	}
	_, toastDone := typed(m.toast, m.toastStart, m.now, typeSpeed)
	switch {
	case !m.scroll.settled(),
		progressSince(m.revealStart, m.now, revealDur) < 1,
		m.mode.isPanel() && progressSince(m.panelStart, m.now, panelDur) < 1,
		!toastDone,
		m.loading && m.doc == nil:
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
	if !m.cfg.Animations {
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
	contentW = max(min(m.cfg.Width, usable-2*margin), 20)
	left = margin
	if m.cfg.Center {
		left = max((usable-contentW)/2, margin)
	}
	switch {
	case !m.cfg.Footer || m.h < 4:
		footerH = 0
	case m.h >= 8:
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
	if !m.cfg.Animations {
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
		m.loading = false
		if msg.err != nil {
			m.loadErr = msg.err
			return m, nil
		}
		m.md, m.fromCache, m.saved = msg.md, msg.fromCache, msg.saved
		m.rebuild()
		m.revealStart = m.now
		if msg.fromCache && isHTTP(m.src) { // background refresh
			m.refreshing = true
			return m, fetchCmd(m.src, false)
		}
		return m, nil

	case fetchedMsg:
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
			return m, m.setToast("● a newer version is ready: press r to view it", false)
		}
		return m, nil // already up to date: no need to announce it

	case editorDoneMsg:
		cfg, err := loadConfig()
		if err != nil {
			return m, m.setToast(err.Error(), true)
		}
		m.cfg = cfg
		m.rebuild()
		return m, m.setToast("✓ settings reloaded", false)

	case tea.MouseWheelMsg:
		if m.mode == modeRead && m.doc != nil {
			switch msg.Mouse().Button {
			case tea.MouseWheelUp:
				m.y -= 3
			case tea.MouseWheelDown:
				m.y += 3
			}
			m.clampY()
		}
		return m, nil

	case tea.MouseClickMsg:
		if mo := tea.Mouse(msg); mo.Button == tea.MouseLeft && m.mode == modeRead && m.doc != nil {
			return m, m.mouseDown(mo.X, mo.Y)
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

	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m *model) key(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.mode {
	case modeSearch:
		return m.keySearch(msg)
	case modeMenu:
		return m.keyMenu(k)
	case modeConfirm:
		switch k {
		case "y", "Y", "enter":
			m.mode = modeRead
			pages, _ := cacheStats()
			_ = cacheClear()
			return m, m.setToast(fmt.Sprintf("✓ cache cleared (%d pages)", pages), false)
		default:
			m.mode = modeRead
		}
		return m, nil
	case modeTOC:
		return m.keyTOC(k)
	case modeHelp:
		m.mode = modeRead
		return m, nil
	}
	return m.keyRead(k)
}

func (m *model) keyRead(k string) (tea.Model, tea.Cmd) {
	_, _, bodyH, _ := m.geometry()
	switch k {
	case "q":
		return m, tea.Quit
	case "esc":
		m.query, m.matches = "", nil // only clears the search and selection: quit with q
		m.mouseSel.clear()
	case "y":
		return m, m.copySelection()
	case "j", "down", "enter":
		m.y++
	case "k", "up":
		m.y--
	case "space", " ", "f", "pgdown", "ctrl+f":
		m.y += max(bodyH-1, 1)
	case "b", "pgup", "ctrl+b":
		m.y -= max(bodyH-1, 1)
	case "d", "ctrl+d":
		m.y += bodyH / 2
	case "u", "ctrl+u":
		m.y -= bodyH / 2
	case "g", "home":
		m.y = 0
	case "G", "end":
		m.y = m.maxY()
	case "]":
		if m.doc != nil {
			for _, h := range m.doc.Heads {
				if h.Line > m.y {
					m.y = h.Line
					break
				}
			}
		}
	case "[":
		if m.doc != nil {
			for i := len(m.doc.Heads) - 1; i >= 0; i-- {
				if m.doc.Heads[i].Line < m.y {
					m.y = m.doc.Heads[i].Line
					break
				}
			}
		}
	case "/":
		m.mode, m.input, m.searchY = modeSearch, "", m.y
	case "n":
		m.step(1)
	case "N":
		m.step(-1)
	case "t":
		return m, m.openTOC()
	case "r":
		return m, m.reload()
	case "m", "?", "tab":
		m.mode, m.sel = modeMenu, 0
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
	if !isHTTP(m.src) {
		return m.setToast("only a URL can be reloaded", true)
	}
	if m.newer != "" {
		m.md, m.fromCache, m.saved, m.newer = m.newer, false, time.Now(), ""
		m.rebuild()
		m.revealStart = m.now
		return m.setToast("✓ newer version applied", false)
	}
	m.refreshing = true
	return fetchCmd(m.src, true)
}

// ---- View ----

func (m *model) host() string {
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
	v.WindowTitle = "wr · " + m.host()
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
		return ansi.Truncate("\x1b[90mwr: too small\x1b[0m", max(m.w, 1), "")
	}
	body := m.body()
	switch m.mode {
	case modeMenu:
		body = m.compose(body, m.menuBox())
	case modeConfirm:
		body = m.compose(body, m.confirmBox())
	case modeTOC:
		body = m.compose(body, m.tocBox())
	case modeHelp:
		body = m.compose(body, m.helpBox())
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
	if m.cfg.Animations && m.cfg.Braille {
		rows[row] = m.center(wave(min(max(m.w/3, 8), 36), m.elapsed()))
	} else {
		rows[row] = m.center("\x1b[94m" + m.spin() + "\x1b[0m")
	}
	if row+2 < len(rows) {
		dots := strings.Repeat(".", int(m.elapsed()*3)%4)
		rows[row+2] = m.center("\x1b[90mfetching \x1b[0m\x1b[1m" + m.host() + "\x1b[0m\x1b[90m" + dots + "\x1b[0m")
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
			rows[bodyH/2] = m.center("\x1b[91m✗ " + m.loadErr.Error() + "\x1b[0m  \x1b[90m(q to quit)\x1b[0m")
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
	if m.cfg.Animations {
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
			line = "\x1b[96m" + strings.Repeat(glyph, cw) + "\x1b[0m"
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
			if ss, ok := m.mouseSel.spanOn(idx, len([]rune(m.doc.Plain[idx]))); ok {
				line = selectLine(line, ss)
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

// rule is a thin line with a blue-to-cyan gradient (16-color palette).
func (m *model) rule() string {
	glyph := "─"
	if m.cfg.Braille {
		glyph = "⣀"
	}
	third := m.w / 3
	return "\x1b[34m" + strings.Repeat(glyph, third) +
		"\x1b[94m" + strings.Repeat(glyph, third) +
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
// (a newer version, search results), then the progress bar and percentage.
func (m *model) progressText() string {
	chips := ""
	if m.w >= 70 {
		if m.newer != "" {
			chips += "\x1b[93m● new version (r)\x1b[0m  "
		}
		if len(m.matches) > 0 {
			q := []rune(m.query)
			if len(q) > 12 {
				q = append(q[:11], '…')
			}
			chips += fmt.Sprintf("\x1b[96m«%s» %d/%d\x1b[0m  ", string(q), m.cur+1, len(m.matches))
		}
	}
	pct, scrollable := m.percent()
	if !scrollable {
		if m.doc != nil && m.w >= 40 {
			return chips + "\x1b[90mall visible\x1b[0m "
		}
		return chips
	}
	label := fmt.Sprintf("\x1b[1m%3d%%\x1b[0m ", int(math.Round(pct)))
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
	switch {
	case cells == 0:
		return chips + label
	case m.cfg.Braille:
		return chips + progressBar(cells, pct) + " " + label
	}
	return chips + plainBar(cells, pct) + " " + label
}

func hint(key, desc string) string { return "\x1b[94m" + key + "\x1b[0m \x1b[90m" + desc + "\x1b[0m" }

// leftText is the left side of the footer.
func (m *model) leftText(room int) string {
	lead := " "
	if m.loading || m.refreshing {
		lead = " \x1b[94m" + m.spin() + "\x1b[0m "
	}
	switch {
	case m.mode == modeSearch:
		count := "\x1b[90mtype to search\x1b[0m"
		if m.input != "" {
			if len(m.matches) == 0 {
				count = "\x1b[91mno results\x1b[0m"
			} else {
				count = fmt.Sprintf("\x1b[96m%d results\x1b[0m", len(m.matches))
			}
		}
		return lead + "\x1b[94m/\x1b[0m" + m.input + "\x1b[7m \x1b[0m  " + count + "  " + hint("enter", "accept") + "  " + hint("esc", "cancel")
	case m.mode.isPanel():
		return lead + hint("↑↓", "move") + "  " + hint("enter", "select") + "  " + hint("esc", "close")
	case m.toast != "":
		col := "92"
		if m.toastWarn {
			col = "93"
		}
		shown := m.toast
		if m.cfg.Animations {
			shown, _ = typed(m.toast, m.toastStart, m.now, typeSpeed)
		}
		return lead + "\x1b[" + col + "m" + shown + "\x1b[0m"
	}
	return lead + fitHints(room-ansi.StringWidth(lead), m.cfg.Mouse)
}

// fitHints shows as many shortcuts as the width allows, most important first,
// in their natural order.
func fitHints(room int, mouse bool) string {
	type h struct {
		prio      int
		key, desc string
	}
	all := []h{{2, "/", "search"}, {5, "n N", "next"}, {6, "[ ]", "sections"},
		{3, "t", "index"}, {4, "r", "reload"}, {1, "m", "menu"}, {0, "q", "quit"}}
	if mouse {
		all = append(all, h{7, "drag", "copy"})
	}
	keep := map[int]bool{}
	used := 0
	for p := 0; p < len(all); p++ {
		for i, it := range all {
			if it.prio != p {
				continue
			}
			w := len(it.key) + 1 + len(it.desc)
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
			out[i] = "\x1b[94m┃\x1b[0m"
		} else {
			out[i] = "\x1b[90m│\x1b[0m"
		}
	}
	return out
}

func plainBar(cells int, pct float64) string {
	filled := min(max(int(float64(cells)*pct/100), 0), cells)
	return "\x1b[94m" + strings.Repeat("█", filled) + "\x1b[90m" + strings.Repeat("░", cells-filled) + "\x1b[0m"
}

// ---- external editor (settings) ----

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
