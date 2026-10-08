package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func longDoc() string {
	var b strings.Builder
	for i := 1; i <= 40; i++ {
		fmt.Fprintf(&b, "## Section %d\n\n%s\n\n", i, strings.Repeat("filler words ", 12)+fmt.Sprintf("marker%d", i%5))
	}
	return "# Document\n\n" + b.String()
}

// newTestModel builds a model with animations off (deterministic frames).
func newTestModel(t *testing.T, w, h int) *model {
	t.Helper()
	isolateAll(t)
	cfg := defaultConfig()
	cfg.Animations = false
	return loaded(newModel("https://t.test/x", cfg, false, nil), w, h)
}

// isolateAll points every per-user directory at a temp dir so tests never touch
// the real cache, config or history.
func isolateAll(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
}

func loaded(m *model, w, h int) *model {
	m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m.Update(loadedMsg{src: m.src, md: longDoc()})
	return m
}

func press(m *model, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyPressMsg
		switch k {
		case "enter":
			msg = tea.KeyPressMsg{Code: tea.KeyEnter}
		case "esc":
			msg = tea.KeyPressMsg{Code: tea.KeyEscape}
		case "down":
			msg = tea.KeyPressMsg{Code: tea.KeyDown}
		case "up":
			msg = tea.KeyPressMsg{Code: tea.KeyUp}
		case "backspace":
			msg = tea.KeyPressMsg{Code: tea.KeyBackspace}
		case " ":
			msg = tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
		default:
			msg = tea.KeyPressMsg{Code: []rune(k)[0], Text: k}
		}
		m.Update(msg)
	}
}

func lines(m *model) []string { return strings.Split(m.render(), "\n") }

// lastRow is the footer row (shortcuts on the left, progress on the right).
func lastRow(m *model) string {
	ls := lines(m)
	return ansi.Strip(ls[len(ls)-1])
}

func TestViewFitsTerminal(t *testing.T) {
	for _, sz := range [][2]int{{200, 60}, {120, 40}, {80, 24}, {50, 12}, {40, 8}, {30, 6}, {24, 4}} {
		m := newTestModel(t, sz[0], sz[1])
		ls := lines(m)
		if len(ls) != sz[1] {
			t.Errorf("%dx%d: %d lines", sz[0], sz[1], len(ls))
		}
		for i, l := range ls {
			if w := ansi.StringWidth(l); w > sz[0] {
				t.Errorf("%dx%d: line %d is %d wide: %q", sz[0], sz[1], i, w, ansi.Strip(l))
			}
		}
	}
}

func TestTinyTerminalShowsMessage(t *testing.T) {
	m := newTestModel(t, 20, 2)
	if !strings.Contains(ansi.Strip(m.render()), "too small") {
		t.Errorf("got %q", ansi.Strip(m.render()))
	}
}

func TestFooterIsOneRowWithShortcutsAndPercent(t *testing.T) {
	m := newTestModel(t, 100, 30)
	ls := lines(m)
	row := ansi.Strip(ls[len(ls)-1])
	for _, bad := range []string{"Document", "t.test", "cached", "ago"} {
		if strings.Contains(row, bad) {
			t.Errorf("the footer must not show %q: %q", bad, row)
		}
	}
	if !strings.Contains(row, "search") || !strings.Contains(row, "quit") {
		t.Errorf("shortcuts expected on the left: %q", row)
	}
	// the percentage lives on the right of that same row
	i, j := strings.Index(row, "search"), strings.Index(row, "%")
	if j < 0 || j < i || !strings.HasSuffix(strings.TrimRight(row, " "), "%") {
		t.Errorf("the percentage must be at the right end of the shortcuts row: %q", row)
	}
	// above the row there is only the thin rule, not a second status line
	if rule := ansi.Strip(ls[len(ls)-2]); strings.Trim(rule, "⣀") != "" {
		t.Errorf("only a rule above the footer row: %q", rule)
	}
	press(m, "G")
	if !strings.Contains(lastRow(m), "100%") {
		t.Errorf("at the end: %q", lastRow(m))
	}
	press(m, "g", " ", " ")
	if s := lastRow(m); strings.Contains(s, "  0%") || strings.Contains(s, "100%") {
		t.Errorf("in the middle it must be intermediate: %q", s)
	}
}

func TestFooterIsResponsive(t *testing.T) {
	row := func(w int) string { return lastRow(newTestModel(t, w, 30)) }
	wide, narrow, tiny := row(140), row(50), row(26)
	if !strings.Contains(wide, "sections") || !strings.Contains(wide, "reload") || !strings.Contains(wide, "copy") {
		t.Errorf("wide: %q", wide)
	}
	if strings.Contains(narrow, "sections") || !strings.Contains(narrow, "quit") || !strings.Contains(narrow, "menu") {
		t.Errorf("narrow keeps the essentials only: %q", narrow)
	}
	if !strings.Contains(tiny, "quit") {
		t.Errorf("even a tiny terminal must show how to quit: %q", tiny)
	}
	// the progress bar shrinks with the width; the percentage always stays
	for _, w := range []int{140, 100, 70, 50, 30} {
		if r := row(w); !strings.Contains(r, "%") {
			t.Errorf("w=%d lost the percentage: %q", w, r)
		}
	}
}

func TestFooterHeightAdaptsToTerminalHeight(t *testing.T) {
	for h, want := range map[int]int{30: 2, 10: 2, 8: 2, 7: 1, 5: 1, 3: 0} {
		m := newTestModel(t, 80, h)
		if _, _, _, fh := m.geometry(); fh != want {
			t.Errorf("h=%d: footer is %d rows, want %d", h, fh, want)
		}
	}
}

func TestFooterShowsAttentionChipsOnTheRight(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.newer = "# Other\n"
	press(m, "/", "m", "a", "r", "k", "e", "r", "enter")
	row := lastRow(m)
	if !strings.Contains(row, "new version") || !strings.Contains(row, "«marker»") {
		t.Errorf("a newer version and the search count belong on the right: %q", row)
	}
	if strings.Index(row, "new version") < strings.Index(row, "quit") {
		t.Errorf("chips must sit to the right of the shortcuts: %q", row)
	}
	// too narrow for chips: the toast announces a newer version instead
	n := newTestModel(t, 60, 30)
	n.newer = "# Other\n"
	if strings.Contains(lastRow(n), "new version") {
		t.Error("a narrow footer must not squeeze the chips in")
	}
}

func TestScrollKeysStayInBounds(t *testing.T) {
	m := newTestModel(t, 100, 30)
	press(m, "k", "k", "b")
	if m.y != 0 {
		t.Errorf("must not go below 0: y=%d", m.y)
	}
	press(m, "G", "j", "j", " ")
	if m.y != m.maxY() {
		t.Errorf("must not pass the end: y=%d max=%d", m.y, m.maxY())
	}
}

func TestSearchFlow(t *testing.T) {
	m := newTestModel(t, 100, 30)
	press(m, "/", "m", "a", "r", "k", "e", "r", "1")
	if m.mode != modeSearch || len(m.matches) == 0 {
		t.Fatalf("live search: mode=%v matches=%d", m.mode, len(m.matches))
	}
	n := len(m.matches)
	press(m, "enter")
	if m.mode != modeRead {
		t.Fatal("enter must leave search mode")
	}
	first := m.cur
	press(m, "n")
	if n > 1 && m.cur == first {
		t.Error("n must advance")
	}
	press(m, "N")
	if m.cur != first {
		t.Error("N must go back")
	}
	if v := m.render(); !strings.Contains(v, hlOpen) && !strings.Contains(v, curOpen) {
		t.Error("the view does not highlight matches")
	}
	press(m, "esc")
	if m.query != "" || len(m.matches) != 0 {
		t.Error("esc must clear the search")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}); cmd != nil {
		t.Error("esc without an active search must not quit")
	}
	if _, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"}); cmd == nil {
		t.Error("q must quit")
	}
}

func TestSearchCancelRestoresPosition(t *testing.T) {
	m := newTestModel(t, 100, 30)
	press(m, "d", "d")
	before := m.y
	press(m, "/", "m", "a", "r", "k", "e", "r", "4", "esc")
	if m.y != before || m.query != "" {
		t.Errorf("y=%d (was %d) query=%q", m.y, before, m.query)
	}
}

func TestSearchNoResultsToast(t *testing.T) {
	m := newTestModel(t, 100, 30)
	press(m, "/", "z", "z", "z", "enter")
	if !strings.Contains(m.toast, "no results") {
		t.Errorf("toast=%q", m.toast)
	}
}

func TestHeadingJumps(t *testing.T) {
	m := newTestModel(t, 100, 30)
	press(m, "]")
	if m.y != m.doc.Heads[1].Line {
		t.Errorf("] must jump to the next heading: y=%d want=%d", m.y, m.doc.Heads[1].Line)
	}
	press(m, "[")
	if m.y >= m.doc.Heads[1].Line {
		t.Errorf("[ must go back: y=%d", m.y)
	}
}

func TestTOCOverlayAndJump(t *testing.T) {
	m := newTestModel(t, 100, 30)
	press(m, "t")
	if m.mode != modeTOC || !strings.Contains(ansi.Strip(m.render()), "Index") {
		t.Fatalf("the index did not open (mode %v)", m.mode)
	}
	press(m, "down", "down", "down", "enter")
	if m.mode != modeRead || m.y != min(m.doc.Heads[3].Line, m.maxY()) {
		t.Errorf("wrong jump: y=%d expected=%d", m.y, m.doc.Heads[3].Line)
	}
}

// pickMenu opens the menu and chooses the entry with that label.
func pickMenu(t *testing.T, m *model, label string) {
	t.Helper()
	m.mode, m.sel = modeMenu, -1
	for i, it := range m.menuItems() {
		if strings.Contains(it.label, label) {
			m.sel = i
		}
	}
	if m.sel < 0 {
		t.Fatalf("no menu entry %q", label)
	}
	press(m, "enter")
}

func TestMenuOpensAndCloses(t *testing.T) {
	m := newTestModel(t, 100, 40)
	press(m, "m")
	v := ansi.Strip(m.render())
	for _, want := range []string{"Open address or search", "Back", "Forward", "History", "Bookmarks", "Bookmark this page",
		"Section index", "Reload page", "Settings", "Keyboard shortcuts", "Open in your browser",
		"Clear cache for this page", "Clear all cache", "Edit the config file", "Quit"} {
		if !strings.Contains(v, want) {
			t.Errorf("the menu does not show %q", want)
		}
	}
	press(m, "esc")
	if m.mode != modeRead {
		t.Error("esc must close the menu")
	}
}

func TestMenuClearAllCacheWithConfirmation(t *testing.T) {
	m := newTestModel(t, 100, 30)
	_ = cachePut("https://t.test/a", "one")
	_ = cachePut("https://t.test/b", "two")
	pickMenu(t, m, "Clear all cache")
	if m.mode != modeConfirm {
		t.Fatalf("mode=%v", m.mode)
	}
	press(m, "esc") // cancelling keeps the cache
	if n, _ := cacheStats(); n != 2 {
		t.Fatalf("cancelling cleared the cache (n=%d)", n)
	}
	pickMenu(t, m, "Clear all cache")
	press(m, "y")
	if n, _ := cacheStats(); n != 0 {
		t.Errorf("confirming must clear (n=%d)", n)
	}
	if !strings.Contains(m.toast, "cache cleared") {
		t.Errorf("toast=%q", m.toast)
	}
}

func TestMenuClearPageCache(t *testing.T) {
	m := newTestModel(t, 100, 30)
	_ = cachePut("https://t.test/x", "content")
	_ = cachePut("https://t.test/other", "other")
	pickMenu(t, m, "Clear cache for this page")
	if _, _, ok := cacheGet("https://t.test/x"); ok {
		t.Error("did not clear this page's cache")
	}
	if _, _, ok := cacheGet("https://t.test/other"); !ok {
		t.Error("cleared another page's cache")
	}
}

func TestMenuNumbersAreShortcuts(t *testing.T) {
	m := newTestModel(t, 100, 40)
	press(m, "m")
	v := ansi.Strip(m.render())
	for _, want := range []string{"1  Open address", "2  Back", "5  Bookmarks", "9  Settings", "0  Keyboard shortcuts"} {
		if !strings.Contains(v, want) {
			t.Errorf("missing %q", want)
		}
	}
	press(m, "9")
	if m.mode != modeSettings {
		t.Errorf("9 must open Settings (mode %v)", m.mode)
	}
}

func TestNewerVersionIsNotAppliedUntilAsked(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.fromCache = true
	m.Update(fetchedMsg{src: m.src, md: "# Other\n\nnew content\n"})
	if m.newer == "" || strings.Contains(strings.Join(m.doc.Plain, ""), "new content") {
		t.Fatal("a newer version must not replace what you are reading")
	}
	if !strings.Contains(ansi.Strip(m.render()), "new version") {
		t.Error("the footer must announce the newer version")
	}
	press(m, "r")
	if m.newer != "" || !strings.Contains(strings.Join(m.doc.Plain, ""), "new content") {
		t.Error("r must apply the newer version")
	}
}

func TestLoadingAndErrorScreens(t *testing.T) {
	isolateAll(t)
	cfg := defaultConfig()
	cfg.Animations = false
	m := newModel("https://t.test/x", cfg, false, nil)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	if v := ansi.Strip(m.render()); !strings.Contains(v, "fetching") || !strings.Contains(v, "t.test") {
		t.Errorf("loading screen:\n%s", v)
	}
	m.Update(loadedMsg{err: fmt.Errorf("HTTP 404")})
	if v := ansi.Strip(m.render()); !strings.Contains(v, "HTTP 404") {
		t.Errorf("error screen:\n%s", v)
	}
}

func TestResizeRebuildsKeepingPosition(t *testing.T) {
	m := newTestModel(t, 120, 30)
	press(m, "G")
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 30})
	if m.docW > 58 {
		t.Errorf("the width was not adjusted: docW=%d", m.docW)
	}
	if m.y != m.maxY() {
		t.Errorf("must stay at the end: y=%d max=%d", m.y, m.maxY())
	}
}

func TestConfigTogglesFooterAndScrollbar(t *testing.T) {
	isolateAll(t)
	cfg := defaultConfig()
	cfg.Footer, cfg.Scrollbar, cfg.Animations = false, false, false
	m := loaded(newModel("https://t.test/x", cfg, false, nil), 80, 20)
	ls := lines(m)
	if len(ls) != 20 {
		t.Fatalf("lines=%d", len(ls))
	}
	if strings.Contains(ansi.Strip(strings.Join(ls, "")), "%") {
		t.Error("without a footer there must be no percentage")
	}
}

func TestPanelsAreRectangular(t *testing.T) {
	m := newTestModel(t, 100, 30)
	for name, box := range map[string][]string{
		"menu": m.menuBox(), "toc": m.tocBox(), "settings": m.settingsBox(), "keys": m.keysBox(),
		"omni": m.omniBox(), "list": m.listBox(), "confirm": m.confirmBox(),
	} {
		w := ansi.StringWidth(box[0])
		for i, l := range box {
			if got := ansi.StringWidth(l); got != w {
				t.Errorf("%s: row %d is %d wide and the border %d: %q", name, i, got, w, ansi.Strip(l))
			}
		}
	}
}

func TestPanelsFitSmallTerminals(t *testing.T) {
	for _, sz := range [][2]int{{40, 10}, {30, 8}, {50, 12}} {
		m := newTestModel(t, sz[0], sz[1])
		for _, key := range []string{"m", "t"} {
			press(m, key)
			ls := lines(m)
			if len(ls) != sz[1] {
				t.Errorf("%dx%d key %s: %d lines", sz[0], sz[1], key, len(ls))
			}
			for i, l := range ls {
				if ansi.StringWidth(l) > sz[0] {
					t.Errorf("%dx%d key %s: line %d overflows", sz[0], sz[1], key, i)
				}
			}
			press(m, "esc")
		}
	}
}

func TestMenuWindowKeepsSelectionVisible(t *testing.T) {
	m := newTestModel(t, 60, 9) // body is only 6 rows: the long menu must scroll
	press(m, "m")
	n := len(m.menuItems())
	for i := 0; i < n-1; i++ {
		press(m, "down")
	}
	if v := ansi.Strip(m.render()); !strings.Contains(v, "Quit") {
		t.Errorf("the selected last item must be visible:\n%s", v)
	}
}

// ---- animations ----

func animModel(t *testing.T, w, h int) *model {
	t.Helper()
	isolateAll(t)
	cfg := defaultConfig() // animations on
	return loaded(newModel("https://t.test/x", cfg, false, nil), w, h)
}

// run feeds animation ticks until the model stops asking for more.
func run(m *model, from time.Time, maxFrames int) (frames int) {
	now := from
	for i := 0; i < maxFrames; i++ {
		now = now.Add(frameFast)
		_, cmd := m.Update(animTickMsg(now))
		frames++
		if cmd == nil {
			break
		}
	}
	return frames
}

func TestScrollAnimatesWithSpring(t *testing.T) {
	m := animModel(t, 100, 30)
	m.revealStart = time.Time{}
	m.scroll.snap()
	press(m, "G")
	if m.y != m.maxY() {
		t.Fatalf("the logical target must jump at once: y=%d", m.y)
	}
	if m.viewY() == m.maxY() {
		t.Fatal("the displayed position must not teleport")
	}
	mid := 0
	prev := m.scroll.pos
	now := time.Now()
	for i := 0; i < 400; i++ {
		now = now.Add(frameFast)
		m.Update(animTickMsg(now))
		if m.scroll.pos < prev-1e-9 {
			t.Fatalf("the scroll went backwards at frame %d", i)
		}
		prev = m.scroll.pos
		if m.scroll.pos > 1 && m.scroll.pos < float64(m.maxY())-1 {
			mid++
		}
	}
	if mid == 0 {
		t.Error("expected intermediate frames between start and target")
	}
	if m.viewY() != m.maxY() {
		t.Errorf("must arrive: viewY=%d target=%d", m.viewY(), m.maxY())
	}
}

func TestAnimationStopsWhenSettled(t *testing.T) {
	m := animModel(t, 100, 30)
	m.revealStart = time.Time{}
	m.loading = false
	press(m, "G")
	frames := run(m, time.Now(), 1000)
	if frames >= 1000 {
		t.Fatal("the animation never stopped asking for ticks")
	}
	// at rest: no tick is requested, so the program idles at 0% CPU
	if active, _ := m.animating(); active {
		t.Error("something still reports animating at rest")
	}
	if _, cmd := m.Update(animTickMsg(time.Now())); cmd != nil {
		t.Error("a settled model must not schedule another frame")
	}
}

func TestNoAnimationsMeansInstantScroll(t *testing.T) {
	m := newTestModel(t, 100, 30)
	press(m, "G")
	if m.viewY() != m.maxY() {
		t.Errorf("with animations off the view must follow immediately: %d vs %d", m.viewY(), m.maxY())
	}
	if active, _ := m.animating(); active {
		t.Error("nothing should animate with animations off")
	}
}

func TestRevealWipesInFromTheTop(t *testing.T) {
	m := animModel(t, 100, 30)
	m.revealStart = time.Now()
	m.now = m.revealStart
	first := strings.Count(strings.Join(lines(m)[:20], ""), "filler")
	m.now = m.revealStart.Add(revealDur / 2)
	half := strings.Count(strings.Join(lines(m)[:20], ""), "filler")
	m.now = m.revealStart.Add(revealDur * 2)
	full := strings.Count(strings.Join(lines(m)[:20], ""), "filler")
	if !(first < half && half <= full && full > 0) {
		t.Errorf("rows must appear progressively: start=%d half=%d full=%d", first, half, full)
	}
	m.now = m.revealStart.Add(revealDur / 3)
	if !strings.Contains(strings.Join(lines(m), "\n"), "⣿⣿⣿⣿") {
		t.Error("the reveal must draw a scan line at its edge")
	}
}

func TestPanelOpensLikeAShutter(t *testing.T) {
	m := animModel(t, 100, 30)
	m.revealStart = time.Time{}
	press(m, "m")
	labels := []string{"Reload", "Section index", "Clear cache", "Clear all", "Links:", "Style:", "Animations:", "Settings", "Keyboard", "Quit"}
	count := func(at time.Duration) int {
		m.now = m.panelStart.Add(at)
		v, n := ansi.Strip(m.render()), 0
		for _, l := range labels {
			if strings.Contains(v, l) {
				n++
			}
		}
		return n
	}
	// ease-out: most of the motion happens early, so sample early on
	start, early, end := count(0), count(panelDur/8), count(panelDur*2)
	if !(start < early && early < end) {
		t.Errorf("the panel must grow: %d -> %d -> %d", start, early, end)
	}
	if !strings.Contains(ansi.Strip(m.render()), "Quit") {
		t.Error("fully open, the panel must show every row")
	}
}

func TestToastTypesOut(t *testing.T) {
	m := animModel(t, 100, 30)
	m.revealStart = time.Time{}
	m.setToast("✓ page reloaded", false)
	m.now = m.toastStart.Add(3 * typeSpeed)
	partial := ansi.Strip(lines(m)[len(lines(m))-1])
	m.now = m.toastStart.Add(time.Second)
	full := ansi.Strip(lines(m)[len(lines(m))-1])
	if strings.Contains(partial, "reloaded") || !strings.Contains(full, "page reloaded") {
		t.Errorf("typewriter: partial=%q full=%q", strings.TrimSpace(partial), strings.TrimSpace(full))
	}
}

func TestLoadingWaveAnimates(t *testing.T) {
	isolateAll(t)
	m := newModel("https://t.test/x", defaultConfig(), false, nil)
	m.Update(tea.WindowSizeMsg{Width: 90, Height: 24})
	m.now = m.t0.Add(100 * time.Millisecond)
	a := m.render()
	m.now = m.t0.Add(300 * time.Millisecond)
	b := m.render()
	if a == b {
		t.Error("the loading screen must move")
	}
	if !strings.Contains(ansi.Strip(a), "fetching") {
		t.Error("the loading screen must say what it is fetching")
	}
	if active, fast := m.animating(); !active || !fast {
		t.Error("loading with no document needs smooth frames")
	}
}

func TestResizeDoesNotFlyBy(t *testing.T) {
	m := animModel(t, 120, 30)
	m.revealStart = time.Time{}
	press(m, "G")
	run(m, time.Now(), 1000)
	m.Update(tea.WindowSizeMsg{Width: 70, Height: 30})
	if m.scroll.pos != float64(m.y) {
		t.Errorf("a resize must snap the spring: pos=%v y=%d", m.scroll.pos, m.y)
	}
}

func TestPanelRowsAreBlankAroundThePanel(t *testing.T) {
	m := newTestModel(t, 100, 30)
	press(m, "m")
	panel := m.menuBox()
	w := ansi.StringWidth(panel[0])
	x := (m.w - w) / 2
	found := 0
	for _, l := range lines(m) {
		p := ansi.Strip(l)
		if strings.Contains(p, "╭─ wr") || strings.Contains(p, "Reload") || strings.Contains(p, "Quit") {
			found++
			if strings.TrimSpace(ansi.Cut(p, 0, x)) != "" || strings.TrimSpace(ansi.Cut(p, x+w, m.w)) != "" {
				t.Errorf("document text next to the panel: %q", p)
			}
		}
	}
	if found < 3 {
		t.Fatalf("panel rows not found (%d)", found)
	}
}

func TestToastIsShortAndDismissedByInput(t *testing.T) {
	m := newTestModel(t, 100, 30)
	m.now = time.Now()
	m.setToast("✓ copied 5 characters", false)
	if m.toast == "" {
		t.Fatal("toast not set")
	}
	// the typewriter must finish quickly: a 25-character notice in about 100 ms
	if d := time.Duration(len([]rune("✓ copied 5 characters"))) * typeSpeed; d > 150*time.Millisecond {
		t.Errorf("typing a notice takes %v", d)
	}
	press(m, "j") // any key dismisses it
	if m.toast != "" {
		t.Error("a key press must dismiss the notice immediately")
	}
	m.setToast("✓ again", false)
	m.Update(tea.MouseClickMsg{X: 5, Y: 5, Button: tea.MouseLeft})
	if m.toast != "" {
		t.Error("a click must dismiss the notice immediately")
	}
}

func TestToastLingerDurations(t *testing.T) {
	if toastLinger > 1500*time.Millisecond {
		t.Errorf("a confirmation should be gone in about a second: %v", toastLinger)
	}
	if toastWarnLinger <= toastLinger || toastWarnLinger > 3*time.Second {
		t.Errorf("a warning lingers a little longer, but not long: %v", toastWarnLinger)
	}
}
