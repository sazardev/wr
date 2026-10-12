package main

// Performance benchmarks. The hot paths of wr are three: extract (fetch ->
// Markdown), render (Markdown -> ANSI lines) and model interaction (keys,
// scroll, resize, navigate). Everything here is local and deterministic: no
// real network (a loopback httptest server stands in for one), no wall-clock
// dependencies, animation off.
//
// Run with:
//
//	go test -run '^$' -bench . -benchmem
//	go test -run '^$' -bench BenchmarkRender -benchmem -cpuprofile cpu.prof
//	sh scripts/bench.sh    # everything + cpu/mem profiles into bench-out/

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/PuerkitoBio/goquery"
)

// ---- fixtures ----

// benchIsolate points the per-user directories at a temp dir, like isolateAll
// does for tests.
func benchIsolate(b *testing.B) {
	b.Helper()
	b.Setenv("XDG_CONFIG_HOME", b.TempDir())
	b.Setenv("XDG_CACHE_HOME", b.TempDir())
	b.Setenv("XDG_STATE_HOME", b.TempDir())
}

func benchProse(paras, words int) string {
	var s strings.Builder
	s.WriteString("# The Benchmark Document\n\n")
	for i := range paras {
		fmt.Fprintf(&s, "## Section %d\n\n", i)
		fmt.Fprintf(&s, "%s\n\n", strings.Repeat(fmt.Sprintf("word%d ", i%97), words))
		if i%10 == 0 {
			fmt.Fprintf(&s, "- item one\n- item two with **bold** and `code`\n  - nested\n\n")
		}
		if i%17 == 0 {
			s.WriteString("| a | b | c |\n|---|---|---|\n| 1 | 2 | 3 |\n\n")
		}
		if i%23 == 0 {
			fmt.Fprintf(&s, "A [link number %d](https://ex.test/%d) inside.\n\n", i, i)
		}
	}
	return s.String()
}

func benchCodeDoc(blocks int, lines int) string {
	var s strings.Builder
	s.WriteString("# Code Heavy\n\n")
	for i := range blocks {
		lang := []string{"go", "python", "javascript", "rust", "bash"}[i%5]
		fmt.Fprintf(&s, "```%s\n", lang)
		for j := range lines {
			fmt.Fprintf(&s, "// line %d of block %d\n", j, i)
			fmt.Fprintf(&s, "value_%d := compute(%d, %d)\n", j, i, j)
		}
		s.WriteString("```\n\n")
	}
	return s.String()
}

func benchArticle(paras, codeBlocks int) string {
	var s strings.Builder
	s.WriteString(`<!doctype html><html><head><title>The Benchmark Article</title>` +
		`<script>var tracking = 1;</script><style>.a{color:red}</style></head><body>` +
		`<nav class="menu"><a href="/">home</a></nav><article>`)
	for i := range paras {
		fmt.Fprintf(&s, `<h2>Section %d</h2><p>%s A <a href="https://ex.test/%d">link</a> `+
			`and an <img src="x.png" alt="image">.</p>`, i, strings.Repeat("Filler prose that wraps across the reading column. ", 12), i)
		if i%5 == 0 {
			s.WriteString(`<pre><code class="language-go">package main` + "\n" +
				strings.Repeat("// comment line\nx := f(1)\n", 8) + "</code></pre>")
		}
	}
	s.WriteString(`</article><footer>legal boilerplate</footer></body></html>`)
	return s.String()
}

// benchPage writes a markdown file and returns its path.
func benchPage(b *testing.B, md string) string {
	b.Helper()
	p := filepath.Join(b.TempDir(), "page.md")
	if err := os.WriteFile(p, []byte(md), 0o644); err != nil {
		b.Fatal(err)
	}
	return p
}

// benchSite writes two interlinked html pages and returns the first.
func benchSite(b *testing.B) string {
	b.Helper()
	dir := b.TempDir()
	a := filepath.Join(dir, "a.html")
	os.WriteFile(a, []byte(`<html><head><title>A</title></head><body><article><h1>Page A</h1>`+
		`<p>Intro with a <a href="b.html">link to page b</a>.</p>`+
		strings.Repeat("<p>Filler paragraph with plenty of words to fill the screen.</p>", 60)+
		`</article></body></html>`), 0o644)
	os.WriteFile(filepath.Join(dir, "b.html"), []byte(`<html><head><title>B</title></head><body><article><h1>Page B</h1>`+
		`<p>Back to <a href="a.html">page a</a>.</p></article></body></html>`), 0o644)
	return a
}

func benchConfig() Config {
	cfg := defaultConfig()
	cfg.Animations = false
	return cfg
}

// ---- extraction: fetch -> markdown ----

func benchLoad(b *testing.B, page string) string {
	b.Helper()
	get := func(src string) ([]byte, string, error) { return []byte(page), "text/html; charset=utf-8", nil }
	var md string
	b.ReportAllocs()
	b.SetBytes(int64(len(page)))
	b.ResetTimer()
	for b.Loop() {
		var err error
		md, err = loadWith("https://t.test/x", get)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	return md
}

func BenchmarkExtractArticleSmall(b *testing.B)  { benchLoad(b, benchArticle(20, 2)) }
func BenchmarkExtractArticleMedium(b *testing.B) { benchLoad(b, benchArticle(120, 12)) }
func BenchmarkExtractArticleLarge(b *testing.B)  { benchLoad(b, benchArticle(400, 40)) }

func BenchmarkExtractMarkdownPassthrough(b *testing.B) {
	page := []byte(benchProse(200, 60))
	get := func(src string) ([]byte, string, error) { return page, "text/markdown", nil }
	b.ReportAllocs()
	b.SetBytes(int64(len(page)))
	for b.Loop() {
		if _, err := loadWith("https://t.test/x.md", get); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkToMarkdownChromeHeavy(b *testing.B) {
	// the pathological page for extraction: menus, scripts, forms, share rows
	page := []byte(`<!doctype html><html><head><title>T</title></head><body>` +
		strings.Repeat(`<div class="nav menu sidebar" role="navigation"><a href="/a">a</a></div>`, 40) +
		`<article>` + strings.Repeat(`<p class="share social newsletter">Share this</p>`, 20) +
		`<h1>Real Head</h1>` + strings.Repeat(`<p>The actual article body, long enough to matter.</p>`, 60) +
		`</article></body></html>`)
	b.ReportAllocs()
	b.SetBytes(int64(len(page)))
	for b.Loop() {
		if _, err := toMarkdown(page, "https://t.test/"); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkExtractPhases splits the extraction: the HTML parse alone, and the
// whole conversion. The difference is the Markdown writer and the cleanups.
func BenchmarkExtractPhases(b *testing.B) {
	page := benchArticle(400, 40)
	b.Run("parse", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(page)))
		for b.Loop() {
			if _, err := goquery.NewDocumentFromReader(strings.NewReader(page)); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("full", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(page)))
		for b.Loop() {
			if _, err := toMarkdown([]byte(page), "https://t.test/"); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// ---- render: markdown -> ANSI ----

func benchRender(b *testing.B, md string, width int) *Doc {
	b.Helper()
	cfg := benchConfig()
	b.ReportAllocs()
	b.SetBytes(int64(len(md)))
	b.ResetTimer()
	var doc *Doc
	for b.Loop() {
		doc = renderDoc(md, width, cfg)
	}
	b.StopTimer()
	return doc
}

func BenchmarkRenderDoc(b *testing.B) {
	small := sample
	medium := benchProse(60, 60)
	large := benchProse(400, 80)
	cases := []struct {
		name  string
		md    string
		width int
	}{
		{"sample-80", small, 80},
		{"medium-100", medium, 100},
		{"medium-200", medium, 200},
		{"large-100", large, 100},
		{"large-200", large, 200},
		{"prose-40cols", large, 40},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) { benchRender(b, c.md, c.width) })
	}
}

func BenchmarkRenderCodeHeavy(b *testing.B) {
	for _, blocks := range []int{5, 20, 80} {
		md := benchCodeDoc(blocks, 30)
		b.Run(fmt.Sprintf("blocks%d", blocks), func(b *testing.B) {
			benchRender(b, md, 100)
		})
	}
}

// BenchmarkRenderReRender measures a re-render of the same document (a resize):
// the syntax highlighting is memoized, so this is the cheap path.
func BenchmarkRenderReRender(b *testing.B) {
	md := benchProse(120, 60)
	cfg := benchConfig()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		renderDoc(md, 100, cfg)
	}
}

// BenchmarkRenderLinksFootnotes vs inline: the link mode changes the work.
func BenchmarkRenderLinkModes(b *testing.B) {
	md := "# L\n\n" + strings.Repeat("A [link](https://ex.test/x) here.\n\n", 300)
	for _, mode := range []string{"footnotes", "inline", "hidden"} {
		b.Run(mode, func(b *testing.B) {
			cfg := benchConfig()
			cfg.Links = mode
			b.ReportAllocs()
			for b.Loop() {
				renderDoc(md, 100, cfg)
			}
		})
	}
}

// ---- the whole load path: network, cache, disk ----

func benchServer(b *testing.B) *httptest.Server {
	b.Helper()
	page := benchArticle(120, 12)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(page))
	}))
	b.Cleanup(srv.Close)
	return srv
}

func BenchmarkLoadFile(b *testing.B) {
	p := benchPage(b, benchProse(200, 60))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := load(p); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoadLoopback(b *testing.B) {
	srv := benchServer(b)
	url := srv.URL + "/page"
	benchIsolate(b)
	cacheOn = false
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := load(url); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLoadCacheMiss: every iteration is a fresh URL (fetch + write), which
// is the worst case of the disk.
func BenchmarkLoadCacheMiss(b *testing.B) {
	srv := benchServer(b)
	benchIsolate(b)
	cacheOn, cacheMaxAge = true, 24*time.Hour
	b.Cleanup(func() { cacheOn = true })
	cfg := benchConfig()
	cfg.Cache = true
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		url := fmt.Sprintf("%s/page-%d", srv.URL, i)
		msg := loadCmd(1, url, false, cfg)()
		if lm, ok := msg.(loadedMsg); !ok || lm.err != nil {
			b.Fatalf("load failed: %#v", msg)
		}
	}
}

// BenchmarkLoadCacheHit: the page is already on disk: no network, one read and
// the sanitize pass. This is the "open instantly" path of the reader.
func BenchmarkLoadCacheHit(b *testing.B) {
	srv := benchServer(b)
	benchIsolate(b)
	cacheOn, cacheMaxAge = true, 24*time.Hour
	b.Cleanup(func() { cacheOn = true })
	url := srv.URL + "/page"
	if _, err := load(url); err != nil {
		b.Fatal(err)
	}
	cfg := benchConfig()
	cfg.Cache = true
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		msg := loadCmd(1, url, false, cfg)()
		if lm, ok := msg.(loadedMsg); !ok || lm.err != nil {
			b.Fatalf("load failed: %#v", msg)
		}
	}
}

func BenchmarkCachePutGet(b *testing.B) {
	benchIsolate(b)
	md := benchProse(120, 60)
	b.ReportAllocs()
	b.SetBytes(int64(len(md)))
	b.ResetTimer()
	for b.Loop() {
		if err := cachePut("https://t.test/p", md); err != nil {
			b.Fatal(err)
		}
		if _, _, ok := cacheGet("https://t.test/p"); !ok {
			b.Fatal("cacheGet failed")
		}
	}
}

// ---- model interaction: what a keypress costs ----

func benchModel(b *testing.B, src string) *model {
	b.Helper()
	cfg := benchConfig()
	m := newModel(src, cfg, false, nil)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	settle(m, m.Init())
	return m
}

// BenchmarkModelKey: the per-keypress cost with a page on screen (scroll keys,
// which re-render nothing but the scroll position).
func BenchmarkModelKey(b *testing.B) {
	for _, key := range []string{"j", "k", "d", "u", "G", "g", "tab"} {
		b.Run(key, func(b *testing.B) {
			m := benchModel(b, benchSite(b))
			msg, _ := keyPressMsg(key)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				m.Update(msg)
			}
		})
	}
}

// BenchmarkModelScreen: drawing the whole screen (the View), plain path.
func BenchmarkModelScreen(b *testing.B) {
	m := benchModel(b, benchSite(b))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = m.render()
	}
}

// BenchmarkModelResize: a resize re-renders the document at the new width.
func BenchmarkModelResize(b *testing.B) {
	m := benchModel(b, benchPage(b, benchProse(300, 80)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		w := 80 + i%100
		m.Update(tea.WindowSizeMsg{Width: w, Height: 30})
	}
}

// BenchmarkModelSearch: typing a query and finding every match.
func BenchmarkModelSearch(b *testing.B) {
	m := benchModel(b, benchPage(b, benchProse(300, 80)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m.Update(tea.KeyPressMsg{Code: '/'})
		for _, r := range "section 4" {
			m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		}
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	}
}

// BenchmarkModelNavigate: following a link (load the next page, render it).
func BenchmarkModelNavigate(b *testing.B) {
	src := benchSite(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m := benchModel(b, src)
		m.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // focus the first link
		_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		settle(m, cmd)
		if !strings.HasSuffix(m.src, "b.html") {
			b.Fatalf("did not navigate: %q", m.src)
		}
	}
}

// BenchmarkModelLongDoc: a very long page (section index, jumps, end).
func BenchmarkModelLongDoc(b *testing.B) {
	src := benchPage(b, benchProse(1500, 120))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		m := benchModel(b, src)
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnd}) // G
		m.Update(tea.KeyPressMsg{Code: ']'})        // next section x3
		m.Update(tea.KeyPressMsg{Code: ']'})
		m.Update(tea.KeyPressMsg{Code: ']'})
		m.Update(tea.KeyPressMsg{Code: tea.KeyHome}) // g
	}
}

// ---- the headless session layer (what the agent harness drives) ----

func BenchmarkSessionLifecycle(b *testing.B) {
	src := benchSite(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		sess, err := newAgentSession(src, 100, 30, false)
		if err != nil {
			b.Fatal(err)
		}
		sess.close()
	}
}

func BenchmarkSessionKeys(b *testing.B) {
	src := benchSite(b)
	sess, err := newAgentSession(src, 100, 30, false)
	if err != nil {
		b.Fatal(err)
	}
	defer sess.close()
	keys := []string{"j", "j", "j", "d", "G", "g", "tab", "esc"}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for _, k := range keys {
			if err := sess.key(k); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// ---- store ----

func benchSeedHistory(b *testing.B, n int) {
	b.Helper()
	var vs []Visit
	for i := range n {
		vs = append(vs, Visit{URL: fmt.Sprintf("https://ex.test/%d", i), Title: fmt.Sprintf("Page %d", i), At: time.Now()})
	}
	if err := writeHistory(vs); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkHistoryList(b *testing.B) {
	benchIsolate(b)
	for _, n := range []int{100, 1000, 5000} {
		b.Run(fmt.Sprintf("entries%d", n), func(b *testing.B) {
			benchSeedHistory(b, n)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				historyList()
			}
		})
	}
}

func BenchmarkHistoryAddAppend(b *testing.B) {
	benchIsolate(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := historyAdd(fmt.Sprintf("https://ex.test/%d", b.N), "t", 1000); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBookmarkList(b *testing.B) {
	benchIsolate(b)
	for range 500 {
		if _, err := bookmarkToggle(fmt.Sprintf("https://ex.test/%d", b.N), "t"); err != nil {
			b.Fatal(err)
		}
	}
	var vs []Visit
	for i := range 500 {
		vs = append(vs, Visit{URL: fmt.Sprintf("https://ex.test/%d", i), Title: "t"})
	}
	writeBookmarks(vs)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		bookmarkList()
	}
}

// ---- in-page search ----

func BenchmarkFindMatches(b *testing.B) {
	md := benchProse(600, 100)
	cfg := benchConfig()
	doc := renderDoc(md, 100, cfg)
	b.ReportAllocs()
	b.SetBytes(int64(len(md)))
	b.ResetTimer()
	for b.Loop() {
		findMatches(doc.Plain, "word42")
	}
}
