package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// runCmd runs a command and returns its message. Timers (toast expiry, ticks)
// would block, so anything slower than a moment is ignored.
func runCmd(c tea.Cmd) tea.Msg {
	if c == nil {
		return nil
	}
	ch := make(chan tea.Msg, 1)
	go func() { ch <- c() }()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(120 * time.Millisecond):
		return nil
	}
}

// settle feeds a command, and every command it produces, back into the model.
func settle(m *model, c tea.Cmd) {
	queue := []tea.Cmd{c}
	for len(queue) > 0 && len(queue) < 200 {
		cur := queue[0]
		queue = queue[1:]
		switch msg := runCmd(cur).(type) {
		case nil:
		case tea.BatchMsg:
			queue = append(queue, msg...)
		default:
			_, next := m.Update(msg)
			queue = append(queue, next)
		}
	}
}

// site writes two small pages that link to each other.
func site(t *testing.T) (a, b string) {
	t.Helper()
	dir := t.TempDir()
	var fill strings.Builder
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&fill, "<p>Filler paragraph number %d with enough words to take a line or two of the screen.</p>", i)
	}
	a, b = filepath.Join(dir, "a.html"), filepath.Join(dir, "b.html")
	_ = os.WriteFile(a, []byte(`<html><head><title>A</title></head><body><article><h1>Page A</h1>`+
		`<p>Intro with a <a href="b.html">link to page b</a> and an <a href="missing.html">unreachable page</a>, `+
		`plus a <a href="#section-three">jump to section three</a> and <a href="mailto:me@x.test">an email</a>.</p>`+
		fill.String()+`<h2>Section Three</h2><p>You made it.</p>`+fill.String()+`</article></body></html>`), 0o644)
	_ = os.WriteFile(b, []byte(`<html><head><title>B</title></head><body><article><h1>Page B</h1>`+
		`<p>Back to <a href="a.html">page a</a>.</p></article></body></html>`), 0o644)
	return a, b
}

func browseModel(t *testing.T, src string) *model {
	t.Helper()
	isolateAll(t)
	cfg := defaultConfig()
	cfg.Animations = false
	m := newModel(src, cfg, false, nil)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	settle(m, m.Init())
	return m
}

func linkByURL(t *testing.T, m *model, href string) int {
	t.Helper()
	for i, u := range m.doc.URLs {
		if u == href {
			return i
		}
	}
	t.Fatalf("no link to %q among %v", href, m.doc.URLs)
	return -1
}

func linkSpan(m *model, id int) LinkSpan {
	for _, l := range m.doc.Links {
		if l.ID == id {
			return l
		}
	}
	return LinkSpan{}
}

func clickLink(m *model, id int) tea.Cmd {
	l := linkSpan(m, id)
	m.y = max(l.Line-3, 0)
	m.clampY()
	m.scroll.snap()
	p := point{l.Line, l.From + 1}
	return tea.Batch(click(m, p), release(m, p))
}

func fileSrc(p string) string { return "file://" + filepath.ToSlash(p) }

func TestOpeningAPageLoadsItAndRecordsHistory(t *testing.T) {
	a, _ := site(t)
	m := browseModel(t, a)
	if m.src != a || m.doc == nil || !strings.Contains(strings.Join(m.doc.Plain, "\n"), "Page A") {
		t.Fatalf("page not loaded: src=%q doc=%v", m.src, m.doc != nil)
	}
	if m.loading || m.pending != nil {
		t.Error("loading state must be cleared")
	}
	if h := historyList(); len(h) != 1 || h[0].URL != a || h[0].Title != "Page A" {
		t.Errorf("history: %+v", h)
	}
	if got := m.View().WindowTitle; !strings.Contains(got, "Page A") {
		t.Errorf("window title: %q", got)
	}
}

func TestClickingALinkNavigates(t *testing.T) {
	a, b := site(t)
	m := browseModel(t, a)
	id := linkByURL(t, m, "b.html")
	settle(m, clickLink(m, id))
	if m.src != fileSrc(b) {
		t.Fatalf("a click must open the link: src=%q", m.src)
	}
	if len(m.back) != 1 || len(m.fwd) != 0 {
		t.Errorf("stacks: back=%d fwd=%d", len(m.back), len(m.fwd))
	}
	if !strings.Contains(strings.Join(m.doc.Plain, "\n"), "Page B") {
		t.Error("page B not shown")
	}
}

func TestBackAndForwardWalkTheHistory(t *testing.T) {
	a, b := site(t)
	m := browseModel(t, a)
	m.y = 12
	m.clampY()
	m.scroll.snap()
	settle(m, m.openLink(linkByURL(t, m, "b.html")))
	if m.src != fileSrc(b) {
		t.Fatalf("src=%q", m.src)
	}

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'H', Text: "H"})
	settle(m, cmd)
	if m.src != a || m.y != 12 {
		t.Errorf("back must return to A at the same position: src=%q y=%d", m.src, m.y)
	}
	if len(m.back) != 0 || len(m.fwd) != 1 {
		t.Errorf("stacks after back: back=%d fwd=%d", len(m.back), len(m.fwd))
	}

	_, cmd = m.Update(tea.KeyPressMsg{Code: 'L', Text: "L"})
	settle(m, cmd)
	if m.src != fileSrc(b) || len(m.back) != 1 || len(m.fwd) != 0 {
		t.Errorf("forward: src=%q back=%d fwd=%d", m.src, len(m.back), len(m.fwd))
	}

	// nothing earlier than the start: a notice, not a crash
	m.back, m.fwd = nil, nil
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'H', Text: "H"})
	settle(m, cmd)
	if !strings.Contains(m.toast, "no earlier page") {
		t.Errorf("toast=%q", m.toast)
	}
}

func TestANewVisitClearsTheForwardStack(t *testing.T) {
	a, b := site(t)
	m := browseModel(t, a)
	settle(m, m.openLink(linkByURL(t, m, "b.html")))
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'H', Text: "H"})
	settle(m, cmd)
	if len(m.fwd) != 1 {
		t.Fatal("expected a forward entry")
	}
	settle(m, m.navigate(fileSrc(b)))
	if len(m.fwd) != 0 {
		t.Error("opening something new must clear the forward stack")
	}
}

func TestAFailedNavigationChangesNothing(t *testing.T) {
	a, _ := site(t)
	m := browseModel(t, a)
	before, y := len(m.back), m.y
	settle(m, m.openLink(linkByURL(t, m, "missing.html")))
	if m.src != a || len(m.back) != before || m.y != y {
		t.Errorf("a failed load must leave the page and the history alone: src=%q back=%d", m.src, len(m.back))
	}
	if !strings.Contains(m.toast, "could not open") {
		t.Errorf("toast=%q", m.toast)
	}
	if m.loading || m.pending != nil {
		t.Error("the loading state must be cleared after a failure")
	}
	if m.doc == nil {
		t.Error("the page must stay on screen")
	}
}

func TestAnOvertakenLoadIsIgnored(t *testing.T) {
	a, b := site(t)
	m := browseModel(t, a)
	first := m.startNav(navPush, fileSrc(b), 0, false)
	second := m.startNav(navPush, a, 0, false)
	stale := runCmd(first)
	m.Update(stale)
	if m.src != a || m.pending == nil || m.pending.src != a {
		t.Errorf("the older response must not apply: src=%q", m.src)
	}
	m.Update(runCmd(second))
	if m.src != a || len(m.back) != 1 {
		t.Errorf("the newer one applies: src=%q back=%d", m.src, len(m.back))
	}
}

func TestUnsupportedLinksOnlyWarn(t *testing.T) {
	a, _ := site(t)
	m := browseModel(t, a)
	settle(m, m.openLink(linkByURL(t, m, "mailto:me@x.test")))
	if m.src != a || !strings.Contains(m.toast, "cannot open") {
		t.Errorf("src=%q toast=%q", m.src, m.toast)
	}
}

func TestAnchorLinkScrollsWithoutLeavingThePage(t *testing.T) {
	a, _ := site(t)
	m := browseModel(t, a)
	settle(m, m.openLink(linkByURL(t, m, "#section-three")))
	if m.src != a || len(m.back) != 0 {
		t.Errorf("an anchor stays on the page: src=%q back=%d", m.src, len(m.back))
	}
	var want int
	for _, h := range m.doc.Heads {
		if h.Text == "Section Three" {
			want = min(h.Line, m.maxY())
		}
	}
	if m.y != want || m.y == 0 {
		t.Errorf("y=%d want %d", m.y, want)
	}
	if cmd := m.navigate("#nope"); cmd == nil || m.toast == "" {
		t.Error("an unknown anchor must say so")
	}
}

func TestTabFocusesLinksAndEnterOpens(t *testing.T) {
	a, b := site(t)
	m := browseModel(t, a)
	press(m, "tab")
	if m.focusID < 0 {
		t.Fatal("tab must focus a link")
	}
	first := m.focusID
	if !strings.Contains(ansi.Strip(m.render()), "→ ") {
		t.Error("the footer must show where the focused link goes")
	}
	press(m, "tab")
	if m.focusID == first {
		t.Error("tab again must move on")
	}
	m.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}) // shift+tab goes back
	if m.focusID != first {
		t.Errorf("shift+tab must return: %d vs %d", m.focusID, first)
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	settle(m, cmd)
	if m.src != fileSrc(b) {
		t.Errorf("enter on a focused link must open it: src=%q", m.src)
	}
	// esc clears the focus
	press(m, "tab")
	press(m, "esc")
	if m.focusID != -1 {
		t.Error("esc must clear the link focus")
	}
}

func TestHintLabelsFollowALink(t *testing.T) {
	a, b := site(t)
	m := browseModel(t, a)
	press(m, "f")
	if m.mode != modeHints || len(m.hints) < 2 {
		t.Fatalf("hint mode: mode=%v hints=%d", m.mode, len(m.hints))
	}
	scr := ansi.Strip(m.render())
	for _, h := range m.hints {
		if !strings.Contains(scr, h.label) {
			t.Errorf("label %q is not drawn", h.label)
		}
	}
	target := m.hints[0]
	url := m.doc.URLs[target.id]
	_, cmd := m.Update(tea.KeyPressMsg{Code: []rune(target.label)[0], Text: target.label})
	settle(m, cmd)
	if m.mode != modeRead {
		t.Error("choosing a label returns to reading")
	}
	if url == "b.html" && m.src != fileSrc(b) {
		t.Errorf("src=%q", m.src)
	}

	m2 := browseModel(t, a)
	press(m2, "f", "esc")
	if m2.mode != modeRead {
		t.Error("esc leaves hint mode")
	}
	press(m2, "f", "z")
	if m2.mode != modeRead || !strings.Contains(m2.toast, "no link labeled") {
		t.Errorf("an unknown label must say so: mode=%v toast=%q", m2.mode, m2.toast)
	}
}

func TestMouseBackAndForwardButtons(t *testing.T) {
	a, b := site(t)
	m := browseModel(t, a)
	settle(m, m.openLink(linkByURL(t, m, "b.html")))
	_, cmd := m.Update(tea.MouseClickMsg{Button: tea.MouseBackward})
	settle(m, cmd)
	if m.src != a {
		t.Errorf("the mouse back button must go back: %q", m.src)
	}
	_, cmd = m.Update(tea.MouseClickMsg{Button: tea.MouseForward})
	settle(m, cmd)
	if m.src != fileSrc(b) {
		t.Errorf("the mouse forward button must go forward: %q", m.src)
	}
}

func TestReloadRereadsALocalFile(t *testing.T) {
	a, _ := site(t)
	m := browseModel(t, a)
	_ = os.WriteFile(a, []byte("<article><h1>Page A edited</h1></article>"), 0o644)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	settle(m, cmd)
	if !strings.Contains(strings.Join(m.doc.Plain, "\n"), "Page A edited") {
		t.Error("reload must read the file again")
	}
	if len(m.back) != 0 {
		t.Error("a reload must not add a history entry")
	}
}

// ---- the address bar ----

func typeInto(m *model, s string) {
	for _, r := range s {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

func TestAddressBarOpensAFileAndSuggestsFromHistory(t *testing.T) {
	a, b := site(t)
	m := browseModel(t, a)
	press(m, "o")
	if m.mode != modeOmni {
		t.Fatalf("o opens the address bar (mode %v)", m.mode)
	}
	if !strings.Contains(ansi.Strip(m.render()), "A") { // empty bar lists recent pages
		t.Error("an empty address bar must suggest recent pages")
	}
	typeInto(m, b)
	items := m.omniItems()
	if len(items) == 0 || !items[0].typed || items[0].target != fileSrc(b) {
		t.Fatalf("the first row is what you typed: %+v", items)
	}
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	settle(m, cmd)
	if m.src != fileSrc(b) {
		t.Errorf("enter opens it: %q", m.src)
	}
	if m.mode != modeRead {
		t.Error("the bar closes after opening")
	}
}

func TestAddressBarSearchesTheWebForWords(t *testing.T) {
	a, _ := site(t)
	m := browseModel(t, a)
	press(m, "o")
	typeInto(m, "rust async book")
	it := m.omniItems()[0]
	if !it.typed || !strings.Contains(it.target, "duckduckgo.com") || !strings.Contains(it.target, "rust+async+book") || !strings.Contains(it.label, "Search the web") {
		t.Errorf("plain words become a search: %+v", it)
	}
	// pressing enter starts loading the results page
	m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.pending == nil || !strings.Contains(m.pending.src, "duckduckgo.com") {
		t.Errorf("a search must start a load: %+v", m.pending)
	}
}

func TestAddressBarMatchesHistoryByTitleAndURL(t *testing.T) {
	a, b := site(t)
	m := browseModel(t, a)
	settle(m, m.openLink(linkByURL(t, m, "b.html")))
	press(m, "o")
	typeInto(m, "page b")
	var found bool
	for _, it := range m.omniItems() {
		if !it.typed && it.target == fileSrc(b) {
			found = true
		}
	}
	if !found {
		t.Errorf("history must be searched by title: %+v", m.omniItems())
	}
	press(m, "ctrl+u")
	if m.omni.input != "" {
		t.Error("ctrl+u clears the line")
	}
	typeInto(m, "ab")
	press(m, "backspace")
	if m.omni.input != "a" {
		t.Errorf("backspace: %q", m.omni.input)
	}
	m.Update(tea.PasteMsg{Content: "pasted\ntext"})
	if m.omni.input != "apasted text" {
		t.Errorf("a paste is one line: %q", m.omni.input)
	}
	press(m, "esc")
	if m.mode != modeRead {
		t.Error("esc closes the bar")
	}
}

// ---- history and bookmarks panels ----

func TestHistoryPanelListsFiltersOpensAndDeletes(t *testing.T) {
	a, b := site(t)
	m := browseModel(t, a)
	settle(m, m.openLink(linkByURL(t, m, "b.html")))
	press(m, "v")
	if m.mode != modeList {
		t.Fatalf("v opens the history (mode %v)", m.mode)
	}
	v := ansi.Strip(m.render())
	if !strings.Contains(v, "Page A") || !strings.Contains(v, "Page B") {
		t.Errorf("both pages must be listed:\n%s", v)
	}
	typeInto(m, "a.html")
	if rows := m.listRows(); len(rows) != 1 || rows[0].label != "Page A" {
		t.Errorf("filter: %+v", rows)
	}
	press(m, "ctrl+u")
	m.list.filter = ""
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	settle(m, cmd)
	if m.mode != modeRead {
		t.Error("enter opens the entry and closes the panel")
	}

	press(m, "v")
	m.list.sel = 0
	first := m.list.view()[0].URL
	m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	for _, v := range historyList() {
		if v.URL == first {
			t.Error("ctrl+d must delete the selected entry")
		}
	}
	m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	if m.mode != modeConfirm || m.confirm != confirmHistory {
		t.Fatalf("ctrl+x asks before clearing: mode=%v", m.mode)
	}
	press(m, "esc")
	if m.mode != modeList || len(historyList()) == 0 {
		t.Error("cancelling returns to the list and keeps the history")
	}
	m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	press(m, "y")
	if len(historyList()) != 0 {
		t.Error("confirming clears the history")
	}
	_ = b
}

func TestHistoryCanBeTurnedOff(t *testing.T) {
	a, _ := site(t)
	isolateAll(t)
	cfg := defaultConfig()
	cfg.Animations, cfg.History = false, false
	m := newModel(a, cfg, false, nil)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	settle(m, m.Init())
	if len(historyList()) != 0 {
		t.Error("nothing may be recorded with history off")
	}
	press(m, "v")
	if m.mode == modeList || !strings.Contains(m.toast, "turned off") {
		t.Errorf("the panel explains why it is empty: mode=%v toast=%q", m.mode, m.toast)
	}
}

func TestBookmarksToggleStarAndPanel(t *testing.T) {
	a, _ := site(t)
	m := browseModel(t, a)
	press(m, "*")
	if !m.bookmarked || !isBookmarked(a) || !strings.Contains(m.toast, "bookmarked") {
		t.Errorf("bookmarked=%v toast=%q", m.bookmarked, m.toast)
	}
	m.toast = ""
	if !strings.Contains(lastRow(m), "★") {
		t.Errorf("the footer shows a star for a bookmarked page: %q", lastRow(m))
	}
	press(m, "'")
	if m.mode != modeList || !strings.Contains(ansi.Strip(m.render()), "Page A") {
		t.Error("the bookmarks panel lists it")
	}
	m.Update(tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})
	if isBookmarked(a) || m.bookmarked {
		t.Error("ctrl+d removes the bookmark")
	}
	press(m, "esc")
	press(m, "*", "*")
	if m.bookmarked {
		t.Error("toggling twice removes it")
	}
}

func TestStartPageShowsRecentAndBookmarksAsClickableLinks(t *testing.T) {
	a, _ := site(t)
	isolateAll(t)
	_ = historyAdd(a, "Page A", 100)
	_, _ = bookmarkToggle(a, "Page A")
	cfg := defaultConfig()
	cfg.Animations = false
	m := newModel("", cfg, false, nil)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	settle(m, m.Init())
	if m.src != "" || m.doc == nil {
		t.Fatalf("the start page loads: src=%q", m.src)
	}
	plain := strings.Join(m.doc.Plain, "\n")
	if !strings.Contains(plain, "Bookmarks") || !strings.Contains(plain, "Recent") || !strings.Contains(plain, "Page A") {
		t.Errorf("start page:\n%s", plain)
	}
	if h := historyList(); len(h) != 1 {
		t.Errorf("the start page itself is not recorded: %+v", h)
	}
	settle(m, m.openLink(linkByURL(t, m, a))) // either entry opens the page
	if m.src != fileSrc(a) {
		t.Errorf("src=%q", m.src)
	}
	press(m, "~")
	settle(m, m.startNav(navPush, "", 0, false))
	if m.src != "" {
		t.Errorf("~ goes to the start page: %q", m.src)
	}
	if err := m.openExternalNow(); err == nil || !strings.Contains(m.toast, "start page") {
		t.Error("the start page cannot be opened in a browser")
	}
}

func TestSelectionStillWorksNextToLinks(t *testing.T) {
	a, _ := site(t)
	m := browseModel(t, a)
	l := lineOf(t, m, "Intro")
	drag(m, point{l, 0}, point{l, 4})
	if !m.mouseSel.active {
		t.Error("dragging selects")
	}
	if m.src != a {
		t.Error("a drag must not follow a link")
	}
}
