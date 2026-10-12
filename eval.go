package main

// wr --eval: deterministic scenarios over the reader's own code, offline, with
// a JSON report. They are the contract an agent (or CI) can check after a
// change: input handling, extraction, rendering, links, the headless session,
// search, the store and the cache. Reports exit non-zero when anything fails.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type evalCheck struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type evalScenario struct {
	Name   string      `json:"name"`
	OK     bool        `json:"ok"`
	MS     int64       `json:"ms"`
	Checks []evalCheck `json:"checks"`
}

type evalReport struct {
	Version   string         `json:"version"`
	Passed    int            `json:"passed"`
	Failed    int            `json:"failed"`
	Scenarios []evalScenario `json:"scenarios"`
}

type evalRun struct {
	checks []evalCheck
}

func (r *evalRun) check(name string, ok bool, format string, args ...any) {
	err := ""
	if !ok {
		err = fmt.Sprintf(format, args...)
	}
	r.checks = append(r.checks, evalCheck{Name: name, OK: ok, Error: err})
}

func (r *evalRun) eq(name string, got, want any) {
	r.check(name, fmt.Sprint(got) == fmt.Sprint(want), "got %v, want %v", got, want)
}

func (r *evalRun) has(name, haystack, needle string) {
	r.check(name, strings.Contains(haystack, needle), "%q does not contain %q", haystack, needle)
}

func (r *evalRun) misses(name, haystack, needle string) {
	r.check(name, !strings.Contains(haystack, needle), "%q unexpectedly contains %q", haystack, needle)
}

// runEval runs every scenario and prints the JSON report.
func runEval() error {
	isolateForEval()
	report := evalReport{Version: versionLine()}
	scenarios := []struct {
		name string
		run  func(*evalRun)
	}{
		{"resolve_input", evalResolveInput},
		{"extract", evalExtract},
		{"render_links", evalRenderLinks},
		{"session_navigate", evalSessionNavigate},
		{"session_search", evalSessionSearch},
		{"search", evalSearch},
		{"store", evalStore},
		{"cache", evalCache},
		{"config", evalConfig},
	}
	for _, sc := range scenarios {
		start := time.Now()
		r := &evalRun{}
		sc.run(r)
		s := evalScenario{Name: sc.name, Checks: r.checks, MS: time.Since(start).Milliseconds()}
		for _, c := range r.checks {
			if c.OK {
				report.Passed++
			} else {
				report.Failed++
				s.OK = false
			}
		}
		s.OK = s.OK || len(r.checks) == 0
		report.Scenarios = append(report.Scenarios, s)
	}
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	if report.Failed > 0 {
		os.Exit(1)
	}
	return nil
}

// isolateForEval points every per-user directory at a throwaway one so the
// report never touches the real config, cache or history.
func isolateForEval() {
	dir, err := os.MkdirTemp("", "wr-eval-")
	if err != nil {
		return
	}
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	os.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	os.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
}

func evalResolveInput(r *evalRun) {
	for _, tc := range []struct {
		in, url string
		search  bool
	}{
		{"https://a.test/x", "https://a.test/x", false},
		{"http://a.test", "http://a.test", false},
		{"a.test", "https://a.test", false},
		{"localhost:8080/x", "https://localhost:8080/x", false},
		{"go concurrency book", "", true},
		{"", "", false},
	} {
		url, search := normalizeInput(tc.in, "duckduckgo")
		r.eq(tc.in+" search", search, tc.search)
		if !tc.search && tc.in != "" {
			r.eq(tc.in+" url", url, tc.url)
		}
	}
	// search goes through the configured engine
	url, _ := normalizeInput("go book", "wikipedia")
	r.has("engine wikipedia", url, "wikipedia.org")
	url, _ = normalizeInput("go book", "https://s.test/?q=%s")
	r.has("engine custom", url, "s.test")
	// a local file becomes a file:// URL
	dir, _ := os.MkdirTemp("", "wr-eval-")
	defer os.RemoveAll(dir)
	p := filepath.Join(dir, "doc.md")
	os.WriteFile(p, []byte("# hi\n"), 0o644)
	url, search := normalizeInput(p, "duckduckgo")
	r.check("file url", strings.HasPrefix(url, "file://") && !search, "got %q (search=%v)", url, search)
}

func evalExtract(r *evalRun) {
	page := `<html><head><title>Page Title</title></head><body><article>
<h1>Heading One</h1>
<p>Alpha paragraph, long enough to survive extraction with words to spare.</p>
<p>Beta paragraph, with <a href="https://x.test/b">a link</a> inside.</p>
<pre><code class="language-go">func main() { println(1) }</code></pre>
</article></body></html>`
	md, err := toMarkdown([]byte(page), "https://a.test/")
	r.check("no error", err == nil, "%v", err)
	r.has("keeps paragraph", md, "Alpha paragraph")
	r.has("keeps link", md, "https://x.test/b")
	r.has("keeps code fence", md, "```go")
	r.has("keeps code body", md, "println(1)")
	r.misses("drops script", md, "<script")
	// the page has an h1 of its own, so it is the title
	r.has("keeps h1 as title", md, "# Heading One")
	stripped := stripLinks(md)
	r.check("stripLinks keeps text", strings.Contains(stripped, "a link") && !strings.Contains(stripped, "https://x.test/b"),
		"stripped markdown: %q", stripped)
	// without one, the <title> is prepended
	md, err = toMarkdown([]byte(`<html><head><title>Page Title</title></head><body><article>`+
		`<p>Only a paragraph here, long enough to survive extraction.</p></article></body></html>`), "https://a.test/")
	r.check("no error (2)", err == nil, "%v", err)
	r.has("prepends page title", md, "# Page Title")
}

func evalRenderLinks(r *evalRun) {
	cfg := defaultConfig()
	md := "# Doc\n\nSee [one](a.html) and [two](https://x.test) plus [mail](mailto:a@b.test).\n"
	doc := renderDoc(md, 100, cfg)
	r.eq("link spans", len(doc.Links), 6) // each link renders twice: inline and in the footnotes
	r.eq("distinct links", len(uniqueLinks(doc)), 3)
	r.eq("distinct urls", len(doc.URLs), 3)
	if len(doc.URLs) == 3 {
		r.eq("url a.html", doc.URLs[0], "a.html")
		r.eq("url https", doc.URLs[1], "https://x.test")
	}
	abs, ok := resolveLink("file:///base/page.html", doc.URLs[0])
	r.check("relative resolved", ok && abs == "file:///base/a.html", "got %q ok=%v", abs, ok)
	abs, ok = resolveLink("https://a.test/x", doc.URLs[1])
	r.check("absolute kept", ok && abs == "https://x.test", "got %q ok=%v", abs, ok)
	_, ok = resolveLink("https://a.test/x", doc.URLs[2])
	r.check("mailto refused", !ok, "mailto should not resolve")
	_, ok = resolveLink("https://a.test/x", "javascript:void(0)")
	r.check("javascript refused", !ok, "javascript: should not resolve")
	abs, ok = resolveLink("https://a.test/x", "#section")
	r.check("anchor kept", ok && abs == "#section", "got %q ok=%v", abs, ok)
}

func evalSessionNavigate(r *evalRun) {
	dir, _ := os.MkdirTemp("", "wr-eval-")
	defer os.RemoveAll(dir)
	a := filepath.Join(dir, "a.html")
	b := filepath.Join(dir, "b.html")
	os.WriteFile(a, []byte(`<html><head><title>A</title></head><body><article><h1>Page A</h1>`+
		`<p>Intro with a <a href="b.html">link to page b</a>.</p>`+
		strings.Repeat("<p>Filler paragraph with plenty of words to fill the screen.</p>", 30)+
		`</article></body></html>`), 0o644)
	os.WriteFile(b, []byte(`<html><head><title>B</title></head><body><article><h1>Page B</h1>`+
		`<p>Back to <a href="a.html">page a</a>.</p></article></body></html>`), 0o644)

	sess, err := newAgentSession(a, 80, 24, false)
	r.check("session opens", err == nil, "%v", err)
	if err != nil {
		return
	}
	defer sess.close()
	st := sess.state()
	r.eq("starts on a.html", strings.HasSuffix(st.URL, "a.html"), true)
	r.check("not loading", !st.Loading, "still loading: %v", st.Error)
	r.has("renders Page A", sess.screen(false), "Page A")
	r.eq("screen height", strings.Count(sess.screen(false), "\n")+1, 24)

	// scrolling moves the viewport over the long page
	r.check("key g (top)", sess.key("g") == nil, "g failed")
	r.eq("at top", sess.state().Scroll.Y, 0)
	r.check("key G (bottom)", sess.key("G") == nil, "G failed")
	r.check("at bottom", sess.state().Scroll.Y > 0, "y did not move (total=%d visible=%d)",
		sess.state().Scroll.Total, sess.state().Scroll.Visible)
	r.check("key j scrolls up", sess.key("j") == nil, "j failed")
	r.check("moved up", sess.state().Scroll.Y < sess.state().Scroll.Total, "y is %d", sess.state().Scroll.Y)
	r.check("key g again", sess.key("g") == nil, "g failed")

	// focus the first link and open it
	r.check("key tab", sess.key("tab") == nil, "tab failed")
	st2 := sess.state()
	r.check("has focused link", st2.FocusedLink != "", "no focused link after tab")
	r.check("key enter", sess.key("enter") == nil, "enter failed")
	sess.waitSettled(2 * time.Second)
	st = sess.state()
	r.check("followed the link", strings.HasSuffix(st.URL, "b.html"), "url is %q", st.URL)
	r.has("renders Page B", sess.screen(false), "Page B")
	r.eq("back stack", len(st.Back), 1)

	r.check("key H (back)", sess.key("H") == nil, "H failed")
	sess.waitSettled(2 * time.Second)
	st = sess.state()
	r.check("went back", strings.HasSuffix(st.URL, "a.html"), "url is %q", st.URL)
	r.has("renders Page A again", sess.screen(false), "Page A")
	r.eq("forward stack", len(st.Forward), 1)

	r.check("key L (forward)", sess.key("L") == nil, "L failed")
	sess.waitSettled(2 * time.Second)
	st = sess.state()
	r.check("went forward", strings.HasSuffix(st.URL, "b.html"), "url is %q", st.URL)
}

func evalSessionSearch(r *evalRun) {
	dir, _ := os.MkdirTemp("", "wr-eval-")
	defer os.RemoveAll(dir)
	p := filepath.Join(dir, "doc.md")
	os.WriteFile(p, []byte("# Doc\n\n"+strings.Repeat("filler line about nothing\n\n", 60)+
		"a rare marker word appears here\n"), 0o644)
	sess, err := newAgentSession(p, 80, 24, false)
	r.check("session opens", err == nil, "%v", err)
	if err != nil {
		return
	}
	defer sess.close()
	r.check("key /", sess.key("/") == nil, "/ failed")
	r.eq("mode search", sess.state().Mode, "search")
	sess.typeText("rare marker")
	r.check("key enter", sess.key("enter") == nil, "enter failed")
	st := sess.state()
	r.check("found matches", st.Matches > 0, "no matches for the rare marker")
	r.check("key n (next)", sess.key("n") == nil, "n failed")
	r.check("key esc (cancel)", sess.key("esc") == nil, "esc failed")
	r.eq("mode back to read", sess.state().Mode, "read")

	// the address bar resolves words to a search URL without fetching it here:
	// press o, type, and the omni panel shows the resolved candidate.
	r.check("key o", sess.key("o") == nil, "o failed")
	r.eq("mode omni", sess.state().Mode, "omni")
	sess.typeText("a.test")
	sess.key("esc")
	r.eq("mode read again", sess.state().Mode, "read")
}

func evalSearch(r *evalRun) {
	plain := []string{"alpha beta gamma", "Beta again", "nothing here"}
	ms := findMatches(plain, "beta")
	r.eq("smart case matches", len(ms), 2)
	ms = findMatches(plain, "Beta")
	r.eq("case sensitive matches", len(ms), 1)
	r.eq("no query no matches", len(findMatches(plain, "")), 0)
}

func evalStore(r *evalRun) {
	historyClear() // earlier scenarios navigated too
	r.check("history add", historyAdd("https://a.test", "A", 1000) == nil, "historyAdd failed")
	historyAdd("https://b.test", "B", 1000)
	list := historyList()
	r.eq("history newest first", len(list), 2)
	if len(list) == 2 {
		r.eq("newest is b", list[0].URL, "https://b.test")
	}
	r.check("dedupe last", historyAdd("https://b.test", "B again", 1000) == nil, "second add failed")
	r.eq("history still 2", len(historyList()), 2)

	added, err := bookmarkToggle("https://a.test", "A")
	r.check("bookmark added", err == nil && added, "err=%v added=%v", err, added)
	r.check("is bookmarked", isBookmarked("https://a.test"), "not bookmarked")
	r.eq("bookmark list", len(bookmarkList()), 1)
	added, err = bookmarkToggle("https://a.test", "A")
	r.check("bookmark removed", err == nil && !added, "err=%v added=%v", err, added)
	r.eq("bookmarks empty", len(bookmarkList()), 0)

	r.check("history delete", historyDelete("https://a.test") == nil, "delete failed")
	r.eq("one entry left", len(historyList()), 1)
	r.check("history clear", historyClear() == nil, "clear failed")
	r.eq("history empty", len(historyList()), 0)
}

func evalCache(r *evalRun) {
	src := "https://a.test/page"
	md := "# Cached\n\nbody\n"
	r.check("cache put", cachePut(src, md) == nil, "cachePut failed")
	got, _, ok := cacheGet(src)
	r.check("cache roundtrip", ok && got == md, "ok=%v got=%q", ok, got)
	r.check("cache miss", func() bool { _, _, ok := cacheGet("https://a.test/other"); return !ok }(), "unknown url was cached")
	pages, _ := cacheStats()
	r.check("one page cached", pages >= 1, "pages=%d", pages)
	r.check("cache clear", cacheClear() == nil, "cacheClear failed")
	_, _, ok = cacheGet(src)
	r.check("cache empty", !ok, "still readable after clear")
}

func evalConfig(r *evalRun) {
	cfg := Config{Width: 10, Center: true, Spacing: 5, WheelLines: 99, CacheDays: 0,
		Links: "bogus", FooterProgress: "sideways", ScrollSpeed: "instant", Accent: "nope",
		SearchEngine: "no-template-here", HistoryLimit: 1}
	cfg.normalize()
	r.eq("width clamped", cfg.Width, 40)
	r.eq("spacing clamped", cfg.Spacing, 2)
	r.eq("wheel clamped", cfg.WheelLines, 20)
	r.eq("cache days min", cfg.CacheDays, 1)
	r.eq("links fallback", cfg.Links, "footnotes")
	r.eq("progress fallback", cfg.FooterProgress, "both")
	r.eq("speed fallback", cfg.ScrollSpeed, "normal")
	r.eq("accent fallback", cfg.Accent, "blue")
	r.eq("engine fallback", cfg.SearchEngine, "duckduckgo")
	r.eq("history limit min", cfg.HistoryLimit, 50)

	// settings apply by name and round-trip through a map
	cfg = defaultConfig()
	if err := applyConfigSettings(&cfg, map[string]any{"width": 120.0, "animations": false, "links": "inline"}); err != nil {
		r.check("apply settings", false, "%v", err)
	} else {
		r.check("apply settings", true, "")
		r.eq("width set", cfg.Width, 120)
		r.eq("animations off", cfg.Animations, false)
		r.eq("links inline", cfg.Links, "inline")
	}
	m := configMap(&cfg)
	r.eq("map has width", m["width"], 120)
	if err := applyConfigSettings(&cfg, map[string]any{"nope": 1}); err == nil {
		r.check("unknown setting rejected", false, "no error for an unknown key")
	} else {
		r.check("unknown setting rejected", true, "")
	}
}
