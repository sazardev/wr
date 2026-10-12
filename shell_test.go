package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const spaShell = `<!doctype html><html><head><title>SOLID principles · Site</title>
<meta name="description" content="The five SOLID principles explained with concrete examples.">
<script type="module" src="/assets/index.js"></script>
<script type="application/ld+json">{"@context":"https://schema.org","@type":"TechArticle","description":"Longer summary from the structured data of the article."}</script>
</head><body><div id="root"></div>
<noscript><h1>SOLID principles</h1><p>Plain fallback paragraph for readers without scripts.</p></noscript></body></html>`

func TestJSShellShowsSummaryAndNotice(t *testing.T) {
	md, err := toMarkdown([]byte(spaShell), "https://t.test/solid")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# SOLID principles · Site",
		shellHead,
		"The five SOLID principles explained with concrete examples.",
		"Plain fallback paragraph for readers without scripts.",
		"Longer summary from the structured data",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
	if strings.Contains(md, "\nSOLID principles\n") {
		t.Errorf("the noscript heading repeats the title:\n%s", md)
	}
	if !isShellPage(md) {
		t.Error("isShellPage must recognise the notice")
	}
}

func TestShortPlainPageGetsNoShellNotice(t *testing.T) {
	md, err := toMarkdown([]byte(`<html><head><title>T</title></head><body><p>hi there</p></body></html>`), "https://t.test/")
	if err != nil {
		t.Fatal(err)
	}
	if isShellPage(md) {
		t.Errorf("a short page that is not a shell must not get the notice:\n%s", md)
	}
}

func TestRealArticleOnSPAMountIsLeftAlone(t *testing.T) {
	// server-side rendered into #root: plenty of text, so no notice
	words := strings.Repeat("word ", 80)
	md, err := toMarkdown([]byte(`<html><head><title>T</title><script type="module" src="/a.js"></script></head><body><div id="root"><article><p>`+words+`</p></article></div></body></html>`), "https://t.test/")
	if err != nil {
		t.Fatal(err)
	}
	if isShellPage(md) {
		t.Errorf("a hydrated page must not be flagged:\n%s", md)
	}
}

func TestJSONLDArticleBodyIsUsed(t *testing.T) {
	page := `<html><head><title>T</title><script type="application/ld+json">{"@graph":[{"@type":"Article","articleBody":"Body text shipped in the structured data."}]}</script></head><body><div id="root"></div></body></html>`
	md, err := toMarkdown([]byte(page), "https://t.test/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "Body text shipped in the structured data.") {
		t.Errorf("articleBody lost:\n%s", md)
	}
}

func TestShellFollowsAdvertisedMarkdown(t *testing.T) {
	long := strings.Repeat("real content ", 40)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/page":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><head><title>T</title><link rel="alternate" type="text/markdown" href="/page.md"></head><body><div id="root"></div></body></html>`))
		case "/page.md":
			w.Header().Set("Content-Type", "text/markdown")
			w.Write([]byte("# T\n\n" + long))
		}
	}))
	defer srv.Close()
	md, err := loadFrom(srv.URL+"/page", fetchURL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "real content") || isShellPage(md) {
		t.Errorf("the advertised markdown was not followed:\n%s", md)
	}
}

func TestRelativeLinksFollowTheRedirect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old" {
			http.Redirect(w, r, "/docs/guide/", http.StatusMovedPermanently)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>G</title></head><body><article><p>See <a href="next">the next page</a>.</p></article></body></html>`))
	}))
	defer srv.Close()
	md, err := loadFrom(srv.URL+"/old", fetchURL)
	if err != nil {
		t.Fatal(err)
	}
	if want := "(" + srv.URL + "/docs/guide/next)"; !strings.Contains(md, want) {
		t.Errorf("link not resolved against the final address, want %s in:\n%s", want, md)
	}
}

func TestHTTPErrorsExplainThemselves(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusForbidden)
	}))
	defer srv.Close()
	_, err := load(srv.URL)
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") || !strings.Contains(err.Error(), "press x") {
		t.Errorf("a 403 must say what to do, got %v", err)
	}
}

func TestHeadingPermalinksAreCleaned(t *testing.T) {
	page := `<html><head><title>T</title></head><body><article>
<h1>Module<a class="headerlink" href="#module" title="Link to this heading">¶</a></h1>
<h2 id="usage"><a href="#usage">Concepts and usage</a></h2>
<h2>Other <a href="https://example.com/real">real link</a></h2>
<p>text</p></article></body></html>`
	md, err := toMarkdown([]byte(page), "https://t.test/")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(md, "¶") || strings.Contains(md, "#usage") {
		t.Errorf("permalinks left in headings:\n%s", md)
	}
	if !strings.Contains(md, "## Concepts and usage") || !strings.Contains(md, "(https://example.com/real)") {
		t.Errorf("headings or real links damaged:\n%s", md)
	}
}

func TestFooterClassAndLeadingRuleAreDropped(t *testing.T) {
	page := `<html><head><title>T</title></head><body><div><h1>Doc</h1><hr><p>Body paragraph with enough words to be the main text of this page, more than the footer.</p><div class="footer">Copyright 2001 Somebody</div></div></body></html>`
	md, err := toMarkdown([]byte(page), "https://t.test/")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(md, "Copyright") || strings.Contains(md, "* * *") {
		t.Errorf("footer or leading rule kept:\n%s", md)
	}
}

func TestAcceptLanguage(t *testing.T) {
	for _, c := range []struct{ lang, want string }{
		{"en_US.UTF-8", "en"},
		{"es_MX.UTF-8", "es-MX, es;q=0.9, en;q=0.6"},
		{"es", "es, en;q=0.6"},
		{"C", "en"},
	} {
		t.Setenv("LC_ALL", "")
		t.Setenv("LC_MESSAGES", "")
		t.Setenv("LANG", c.lang)
		if got := acceptLanguage(); got != c.want {
			t.Errorf("LANG=%s -> %q, want %q", c.lang, got, c.want)
		}
	}
}

func withRenderer(t *testing.T, fn func(string) ([]byte, error)) {
	t.Helper()
	oldR, oldJS := renderDOM, jsRender
	renderDOM, jsRender = fn, true
	t.Cleanup(func() { renderDOM, jsRender = oldR, oldJS })
}

func serveShell(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(spaShell))
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/solid"
}

func TestShellIsRenderedWhenABrowserCanDoIt(t *testing.T) {
	var asked string
	withRenderer(t, func(src string) ([]byte, error) {
		asked = src
		return []byte(`<html><head><title>SOLID</title></head><body><div id="root"><article><h1>SOLID</h1><p>` +
			strings.Repeat("rendered paragraph ", 30) + `</p></article></div></body></html>`), nil
	})
	url := serveShell(t)
	md, err := loadFrom(url, fetchURL)
	if err != nil {
		t.Fatal(err)
	}
	if asked != url {
		t.Errorf("the renderer was asked for %q, want %q", asked, url)
	}
	if !strings.Contains(md, "rendered paragraph") || isShellPage(md) {
		t.Errorf("the rendered DOM was not used:\n%s", md)
	}
}

func TestShellFallsBackWhenRenderingFails(t *testing.T) {
	isolate(t)
	oldCache := cacheOn
	cacheOn = true
	t.Cleanup(func() { cacheOn = oldCache })
	withRenderer(t, func(string) ([]byte, error) { return nil, errTest })
	url := serveShell(t)
	md, err := loadFrom(url, fetchURL)
	if err != nil {
		t.Fatal(err)
	}
	if !isShellPage(md) || !strings.Contains(md, "The five SOLID principles") {
		t.Errorf("the summary and notice must remain:\n%s", md)
	}
	if _, _, ok := cacheGet(url); ok {
		t.Error("a bare shell must not be cached")
	}
}

func TestRenderingIsSkippedForNormalPagesAndWhenOff(t *testing.T) {
	calls := 0
	withRenderer(t, func(string) ([]byte, error) { calls++; return nil, errTest })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>T</title></head><body><article><p>` + strings.Repeat("plain words ", 50) + `</p></article></body></html>`))
	}))
	defer srv.Close()
	if _, err := loadFrom(srv.URL, fetchURL); err != nil || calls != 0 {
		t.Errorf("a normal page must not start a browser (calls=%d, err=%v)", calls, err)
	}
	jsRender = false
	if _, err := loadFrom(serveShell(t), fetchURL); err != nil || calls != 0 {
		t.Errorf("render_js=false must not start a browser (calls=%d, err=%v)", calls, err)
	}
}

type testErr string

func (e testErr) Error() string { return string(e) }

const errTest = testErr("render failed")

func TestErrorPageTitleNamesWhoBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`<html><head><TITLE> Acceso   web restringido </TITLE></head><body>x</body></html>`))
	}))
	defer srv.Close()
	_, err := load(srv.URL)
	if err == nil || !strings.Contains(err.Error(), "“Acceso web restringido”") || !strings.Contains(err.Error(), "your network") {
		t.Errorf("a 403 page must say who answered: %v", err)
	}
}

func TestTransientErrorsAreRetriedOnce(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			conn.Close() // reset before any answer
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><head><title>T</title></head><body><article><p>second try works</p></article></body></html>`))
	}))
	defer srv.Close()
	md, err := load(srv.URL)
	if err != nil || !strings.Contains(md, "second try works") || calls != 2 {
		t.Errorf("calls=%d err=%v md=%q", calls, err, md)
	}
}

func TestNetworkErrorsAreReadable(t *testing.T) {
	_, err := load("http://127.0.0.1:1/")
	if err == nil || !strings.Contains(err.Error(), "connection refused (is the server running?)") || strings.Contains(err.Error(), "dial tcp") {
		t.Errorf("a refused connection must read plainly: %v", err)
	}
}

func TestCardsKeepTheTitleAsTheLink(t *testing.T) {
	page := `<html><head><title>T</title></head><body><article>
<a href="/essentials"><div><h3>Essentials</h3><p>An overview of it.</p></div></a>
<a href="/skip">Skip to content</a>
<a href="#x"></a><a id="old"></a>
<p>real text</p></article></body></html>`
	md, err := toMarkdown([]byte(page), "https://t.test/")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(md, "\\\n") || strings.Contains(md, "[](") || strings.Contains(strings.ToLower(md), "skip to") {
		t.Errorf("anchor artifacts left:\n%s", md)
	}
	if !strings.Contains(md, "[Essentials](https://t.test/essentials)") || !strings.Contains(md, "An overview of it.") {
		t.Errorf("card not split into link title + text:\n%s", md)
	}
}

func TestDuplicateLinkLinesCollapse(t *testing.T) {
	in := "[Ana](https://x/ana)\n\n[Ana](https://x/ana)\n\nplain\n\nplain\n\n```\n[Ana](https://x/ana)\n\n[Ana](https://x/ana)\n```\n"
	got := dedupeLinkLines(in)
	if strings.Count(got, "[Ana](https://x/ana)") != 3 { // one outside, two untouched in the fence
		t.Errorf("dedupe wrong:\n%s", got)
	}
	if strings.Count(got, "plain") != 2 {
		t.Errorf("plain repeated text must stay:\n%s", got)
	}
}

func TestShellDetectionShapes(t *testing.T) {
	long := strings.Repeat("real words here ", 30)
	for name, c := range map[string]struct {
		page string
		want bool
	}{
		"loading placeholder":  {`<html><head><title>T</title></head><body><div id="app"><div class="spinner">Loading…</div></div><script src="/bundle.js"></script></body></html>`, true},
		"angular root":         {`<html><head><title>T</title></head><body><app-root></app-root><script src="main.js"></script></body></html>`, true},
		"textless with script": {`<html><head><title>T</title></head><body><div class="x"></div><script src="/a.js"></script></body></html>`, true},
		"hydrated app":         {`<html><head><title>T</title></head><body><div id="root"><p>` + long + `</p></div><script src="/a.js"></script></body></html>`, false},
		"short static page":    {`<html><head><title>T</title></head><body><p>hello there</p></body></html>`, false},
		"short with analytics": {`<html><head><title>T</title></head><body><p>` + strings.Repeat("some static words ", 10) + `</p><script src="/analytics.js"></script></body></html>`, false},
	} {
		md, err := toMarkdown([]byte(c.page), "https://t.test/")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := isShellPage(md); got != c.want {
			t.Errorf("%s: shell = %v, want %v\n%s", name, got, c.want, md)
		}
	}
}

func TestBlockLineCodeIsNotDoubleSpaced(t *testing.T) {
	page := `<html><head><title>T</title></head><body><article><div class="expressive-code"><figure class="frame"><figcaption class="header"><span class="title"></span><span class="sr-only">Terminal window</span></figcaption><pre data-language="bash"><code><div class="ec-line"><div class="code"><span>mkdir</span><span> myapp</span></div></div><div class="ec-line"><div class="code"><span>cd</span><span> myapp</span></div></div></code></pre><div class="copy"><button data-code="x"><div></div></button></div></figure></div><p>after</p></article></body></html>`
	md, err := toMarkdown([]byte(page), "https://t.test/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "```bash\nmkdir myapp\ncd myapp\n```") {
		t.Errorf("block lines not flattened (or language lost):\n%q", md)
	}
	if strings.Contains(md, "Terminal window") {
		t.Errorf("the hidden frame label leaked:\n%s", md)
	}
}

func TestLeadingNavigationIsDroppedButContentIsNot(t *testing.T) {
	for name, c := range map[string]struct{ in, want, gone string }{
		"ordered breadcrumb": {"1. [Documentation](https://x/d)\n2. [Effective Go](https://x/e)\n\n# Effective Go\n\nBody.", "# Effective Go", "Documentation"},
		"section label":      {"[Learn React](https://x/learn)\n\n# Quick Start\n\nBody.", "# Quick Start", "Learn React"},
		"gnu next strip":     {"Next: [Intro](https://x/i), Previous: [Top](https://x/t), Up: [Top](https://x/t)\n\n# Bash\n\nBody.", "# Bash", "Next:"},
		"link strip":         {"[std](/a)::[vec](/b)\n\n# Vec\n\nBody.", "# Vec", "std"},
	} {
		got := dropLeadingCrumbs(c.in)
		if !strings.HasPrefix(got, c.want) || strings.Contains(got, c.gone) {
			t.Errorf("%s: got %q", name, got)
		}
	}
	for name, in := range map[string]string{
		"real toc of many items":   "- [One](/1)\n- [Two](/2)\n- [Three](/3)\n- [Four](/4)\n- [Five](/5)\n- [Six](/6)\n\n# Title",
		"lone link, no heading":    "[Read the docs](https://x/docs)\n\nSome paragraph of text here.",
		"intro paragraph":          "This guide covers [Go](https://go.dev) and [Rust](https://rust-lang.org) basics for you.\n\n# Title",
		"short list, not followed": "- [One](/1)\n- [Two](/2)\n\nSome text follows.",
	} {
		if got := dropLeadingCrumbs(in); got != in {
			t.Errorf("%s: content was dropped: %q", name, got)
		}
	}
}

func TestHiddenElementsAreDropped(t *testing.T) {
	page := `<html><head><title>T</title></head><body><main><div style="display:none;" hidden="true">Are you an LLM? Read the markdown.</div><h1>Doc</h1><p>` +
		strings.Repeat("visible words ", 30) + `</p><div hidden="until-found">collapsed but real</div></main></body></html>`
	md, err := toMarkdown([]byte(page), "https://t.test/")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(md, "LLM") || !strings.Contains(md, "collapsed but real") || !strings.Contains(md, "visible words") {
		t.Errorf("hidden handling wrong:\n%s", md)
	}
	// a page that hides its whole article until a script shows it must keep it
	whole := `<html><head><title>T</title></head><body><main style="display:none"><h1>Doc</h1><p>` + strings.Repeat("the real article ", 40) + `</p></main></body></html>`
	md, err = toMarkdown([]byte(whole), "https://t.test/")
	if err != nil || !strings.Contains(md, "the real article") {
		t.Errorf("the article hidden until JS must stay: %v\n%s", err, md)
	}
}

func TestLightAndDarkImageVariantsCollapse(t *testing.T) {
	in := "\\[image: A diagram\\]\n\n\\[image: A diagram\\]\n\ntext\n\n\\[image: Other\\]\n"
	got := dedupeLinkLines(in)
	if strings.Count(got, "A diagram") != 1 || !strings.Contains(got, "Other") {
		t.Errorf("repeated image placeholders must collapse:\n%s", got)
	}
}
