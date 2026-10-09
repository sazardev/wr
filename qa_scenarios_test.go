package main

// Scenario tests for the QA rounds documented in docs/QA.md, tracked one by one
// in docs/QA-FINDINGS.md and ordered in docs/QA-PLAN.md.
//
// Each subtest names the finding it covers:
//
//   - QA-F-001 … QA-F-013 plus the working-behavior regressions.
//
// Open findings are skipped in the normal run and asserted for real when
// WR_QA_STRICT=1 is set, so "still red" is never a surprise:
//
//	go test ./...                                # pending findings skip
//	WR_QA_STRICT=1 go test -run TestQA -v ./...  # open findings must fail
//
// TestQAFindingsCovered keeps this file and the register in sync: every ID in
// the register must appear in qaScenarioCoverage and vice versa.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
)

// pendingFindings lists findings that are still open (docs/QA-FINDINGS.md). A
// fix removes the ID here and flips the state there; that is what makes the
// scenario run for real and turns CI green on it.
var pendingFindings = map[string]bool{
	"QA-F-001": true,
	"QA-F-002": true,
	"QA-F-003": true,
	"QA-F-004": true,
	"QA-F-005": true,
	"QA-F-006": true,
	"QA-F-007": true,
	"QA-F-008": true,
	"QA-F-009": true,
	"QA-F-010": true,
	"QA-F-013": true,
}

// qaScenarioCoverage maps every finding ID to the subtest that covers it.
var qaScenarioCoverage = map[string]string{
	"QA-F-001": "TestQAExtraction/QA-F-001_has_toc_token",
	"QA-F-002": "TestQAExtraction/QA-F-002_first_article_teaser",
	"QA-F-003": "TestQAExtraction/QA-F-003_form_wrapped_content",
	"QA-F-004": "TestQAExtraction/QA-F-004_header_h1",
	"QA-F-005": "TestQAExtraction/QA-F-005_title_fallback_fence",
	"QA-F-006": "TestQACodeFidelity",
	"QA-F-007": "TestQACodeFidelity/QA-F-007_escape_bytes_in_code",
	"QA-F-008": "TestQALanguageDetection",
	"QA-F-009": "TestQATables",
	"QA-F-010": "TestQACharset/QA-F-010_utf8_past_sniff_window",
	"QA-F-011": "TestQAClipboard/code_tabs_become_four_spaces",
	"QA-F-012": "TestQALinkModes",
	"QA-F-013": "TestQAExtraction/QA-F-013_non_semantic_nav",
}

// qaPending skips an open finding's assertion in the normal run.
func qaPending(t *testing.T, id string) {
	t.Helper()
	if pendingFindings[id] && os.Getenv("WR_QA_STRICT") == "" {
		t.Skipf("finding %s is open (docs/QA-FINDINGS.md); WR_QA_STRICT=1 asserts it anyway", id)
	}
}

// qaHTML builds a page whose reading content sits inside <article>.
func qaHTML(body string) string {
	return "<!doctype html><html><head><title>The Title</title></head><body><article>" + body + "</article></body></html>"
}

// qaMarkdown runs wr's extraction without the network.
func qaMarkdown(t *testing.T, html string) string {
	t.Helper()
	md, err := toMarkdown([]byte(html), "https://t.test/page")
	if err != nil {
		t.Fatalf("toMarkdown: %v", err)
	}
	return md
}

var qaFenceRe = regexp.MustCompile("(?m)^(`{3,}|~{3,})([^\\n`]*)$")

// qaFenceLang returns the language of the first fenced block.
func qaFenceLang(md string) string {
	m := qaFenceRe.FindStringSubmatchIndex(md)
	if m == nil {
		return "<no fence>"
	}
	return strings.TrimSpace(md[m[4]:m[5]])
}

func TestQAExtraction(t *testing.T) {
	t.Run("QA-F-001_has_toc_token", func(t *testing.T) {
		html := qaHTML(`<div class="page-html has-toc"><h1>Real Title</h1><p>THE ENTIRE ARTICLE BODY must survive.</p></div>`)
		md := qaMarkdown(t, html)
		qaPending(t, "QA-F-001")
		if !strings.Contains(md, "THE ENTIRE ARTICLE BODY") {
			t.Errorf("junkClass removed the article:\n%s", md)
		}
	})

	t.Run("QA-F-002_first_article_teaser", func(t *testing.T) {
		// body-level articles: a teaser first, the real article inside <main>
		html := `<!doctype html><html><head><title>The Title</title></head><body>` +
			`<article><p>Teaser: one short line.</p></article>` +
			`<main><article>` + strings.Repeat(`<p>The full body has many paragraphs. </p>`, 40) + `</article></main>` +
			`</body></html>`
		md := qaMarkdown(t, html)
		qaPending(t, "QA-F-002")
		if !strings.Contains(md, "The full body has many paragraphs") {
			t.Errorf("root selection kept the teaser instead of the article:\n%.300s", md)
		}
	})

	// Working behavior: with a single article the root is that article.
	t.Run("single_article_is_the_root", func(t *testing.T) {
		if md := qaMarkdown(t, qaHTML(`<h1>H</h1><p>BODY WORDS HERE</p>`)); !strings.Contains(md, "BODY WORDS HERE") {
			t.Errorf("a plain article page must be extracted:\n%s", md)
		}
	})

	t.Run("QA-F-003_form_wrapped_content", func(t *testing.T) {
		html := qaHTML(`<form class="js-live-search"><p>THE NEWS LISTING BODY must survive.</p></form>`)
		md := qaMarkdown(t, html)
		qaPending(t, "QA-F-003")
		if !strings.Contains(md, "THE NEWS LISTING BODY") {
			t.Errorf("dropTags removed the whole form:\n%s", md)
		}
	})

	t.Run("QA-F-004_header_h1", func(t *testing.T) {
		html := `<!doctype html><html><head><title>The Title</title></head><body><main>` +
			`<header class="mw-body-header"><h1>The Page Heading</h1></header><p>BODY WORDS</p></main></body></html>`
		md := qaMarkdown(t, html)
		qaPending(t, "QA-F-004")
		if !strings.Contains(md, "# The Page Heading") {
			t.Errorf("the heading inside <header> was removed:\n%.200s", md)
		}
	})

	t.Run("QA-F-005_title_fallback_fence", func(t *testing.T) {
		html := qaHTML(`<pre class="language-bash"><code># this is a shell comment, not a Markdown heading
echo hi</code></pre><p>body</p>`)
		md := qaMarkdown(t, html)
		qaPending(t, "QA-F-005")
		if !strings.Contains(md, "# The Title") {
			t.Errorf("a # line inside a fence blocked the title fallback:\n%.200s", md)
		}
	})

	t.Run("QA-F-013_non_semantic_nav", func(t *testing.T) {
		html := qaHTML(`<div role="navigation">NAVIGATION JUNK</div><p>KEEP WORDS</p>`)
		md := qaMarkdown(t, html)
		qaPending(t, "QA-F-013")
		if strings.Contains(md, "NAVIGATION JUNK") {
			t.Errorf("non-semantic navigation was kept:\n%s", md)
		}
		if !strings.Contains(md, "KEEP WORDS") {
			t.Errorf("content lost while dropping chrome:\n%s", md)
		}
	})

	// Working behavior: the junk-class drop still has to work.
	t.Run("sidebar_junk_removed", func(t *testing.T) {
		html := qaHTML(`<aside class="sidebar">SIDEBAR JUNK</aside><p>KEEP WORDS</p>`)
		if md := qaMarkdown(t, html); strings.Contains(md, "SIDEBAR JUNK") || !strings.Contains(md, "KEEP WORDS") {
			t.Errorf("sidebar handling regressed:\n%s", md)
		}
	})

	t.Run("table_of_contents_removed", func(t *testing.T) {
		html := qaHTML(`<nav class="table-of-contents">TOC JUNK</nav><p>KEEP WORDS</p>`)
		if md := qaMarkdown(t, html); strings.Contains(md, "TOC JUNK") || !strings.Contains(md, "KEEP WORDS") {
			t.Errorf("table of contents handling regressed:\n%s", md)
		}
	})
}

func TestQACodeFidelity(t *testing.T) {
	t.Run("QA-F-006_blank_lines_in_code", func(t *testing.T) {
		md := qaMarkdown(t, qaHTML(`<pre class="language-python"><code>a = 1


b = 2
</code></pre>`))
		qaPending(t, "QA-F-006")
		if !strings.Contains(md, "a = 1\n\n\nb = 2") {
			t.Errorf("consecutive blank lines inside code collapsed:\n%q", md)
		}
	})

	t.Run("QA-F-006_trailing_spaces_in_code", func(t *testing.T) {
		md := qaMarkdown(t, qaHTML(`<pre class="language-python"><code>x = 1   
y = 2
</code></pre>`))
		qaPending(t, "QA-F-006")
		if !strings.Contains(md, "x = 1   \n") {
			t.Errorf("trailing spaces inside code were stripped:\n%q", md)
		}
	})

	t.Run("QA-F-006_link_line_in_code", func(t *testing.T) {
		md := qaMarkdown(t, qaHTML(`<pre class="language-text"><code>[a](u1) [b](u2) [c](u3) [d](u4)
keep me
</code></pre>`))
		qaPending(t, "QA-F-006")
		if !strings.Contains(md, "[a](u1) [b](u2) [c](u3) [d](u4)") {
			t.Errorf("a code line made of links was deleted:\n%q", md)
		}
	})

	t.Run("QA-F-007_escape_bytes_in_code", func(t *testing.T) {
		md := qaMarkdown(t, qaHTML(`<pre class="language-bash"><code>echo -e "\x1b[31mred\x1b[0m"
</code></pre>`))
		qaPending(t, "QA-F-007")
		if !strings.Contains(md, "\x1b[31mred\x1b[0m") {
			t.Errorf("escape bytes were stripped from code:\n%q", md)
		}
	})

	// Working behavior: the Markdown text keeps the tab; only the view expands it.
	t.Run("tabs_kept_in_markdown", func(t *testing.T) {
		md := qaMarkdown(t, qaHTML(`<pre class="language-go"><code>func main() {
	println("hi")
}
</code></pre>`))
		if !strings.Contains(md, "\tprintln") {
			t.Errorf("the tab was lost before rendering:\n%q", md)
		}
	})

	t.Run("fence_inside_code_is_escaped", func(t *testing.T) {
		md := qaMarkdown(t, qaHTML(`<pre class="language-markdown"><code>Example:

`+"```go"+`
fmt.Println("hi")
`+"```"+`

End.
</code></pre>`))
		if !strings.Contains(md, "fmt.Println") {
			t.Fatalf("inner code lost:\n%s", md)
		}
		d := render(t, md, 80, nil)
		all := strings.Join(d.Plain, "\n")
		// the inner fence is content: it must be drawn inside a single frame
		if !strings.Contains(all, "fmt.Println") || !strings.Contains(all, "```go") {
			t.Errorf("nested fence not rendered as content:\n%s", all)
		}
		if n := strings.Count(all, "╭─"); n != 1 {
			t.Errorf("the outer block was split into %d frames:\n%s", n, all)
		}
	})

	t.Run("unicode_in_code_is_preserved", func(t *testing.T) {
		md := qaMarkdown(t, qaHTML(`<pre class="language-python"><code># ünïcödé ñ 日本語
s = "héllo 世界 🚀"
</code></pre>`))
		if !strings.Contains(md, `# ünïcödé ñ 日本語`) || !strings.Contains(md, `"héllo 世界 🚀"`) {
			t.Errorf("unicode in code was mangled:\n%q", md)
		}
	})

	t.Run("render_keeps_indentation", func(t *testing.T) {
		md := "```python\ndef f():\n\tif x:\n\t\treturn 1\n\treturn 0\n```\n"
		d := render(t, md, 80, nil)
		body := strings.Join(d.Plain, "\n")
		// a tab is four spaces on screen, and two of them are eight
		for _, want := range []string{"│     if x:", "│         return 1", "│     return 0"} {
			if !strings.Contains(body, want) {
				t.Errorf("indentation lost in the view (want %q):\n%s", want, body)
			}
		}
		if strings.Contains(body, "\t") {
			t.Errorf("a tab reached the screen:\n%q", body)
		}
	})
}

func TestQALanguageDetection(t *testing.T) {
	cases := []struct {
		name string
		html string
		want string
	}{
		{"language_class", qaHTML(`<pre class="language-go"><code>package main</code></pre>`), "go"},
		{"data_language_attr", qaHTML(`<pre data-language="python"><code>x = 1</code></pre>`), "python"},
		{"highlight_source_class", qaHTML(`<div class="highlight-source-rust"><pre><code>fn main() {}</code></pre></div>`), "rust"},
		{"alias_map_sh", qaHTML(`<pre class="language-sh"><code>echo hi</code></pre>`), "bash"},
		{"QA-F-008_sphinx_highlight_class", qaHTML(`<div class="highlight-python notranslate"><pre><code>x = 1</code></pre></div>`), "python"},
		{"QA-F-008_mediawiki_lang_class", qaHTML(`<div class="mw-highlight mw-highlight-lang-python"><pre><code>x = 1</code></pre></div>`), "python"},
		{"no_language_is_text", qaHTML(`<pre><code>plain code</code></pre>`), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if strings.HasPrefix(tc.name, "QA-F-008_") {
				qaPending(t, "QA-F-008")
			}
			md := qaMarkdown(t, tc.html)
			if got := qaFenceLang(md); got != tc.want {
				t.Errorf("fence language = %q, want %q\n%s", got, tc.want, md)
			}
		})
	}
}

func TestQATables(t *testing.T) {
	t.Run("QA-F-009_table_with_list_is_a_table", func(t *testing.T) {
		md := qaMarkdown(t, qaHTML(`<table><tr><th>K</th><td><ul><li>a</li><li>b</li></ul></td></tr></table>`))
		qaPending(t, "QA-F-009")
		if !regexp.MustCompile(`(?m)^\|`).MatchString(md) {
			t.Errorf("table destroyed by a list in a cell:\n%s", md)
		}
	})

	// Working behavior: the cell contents themselves do not disappear.
	t.Run("table_with_list_keeps_cells", func(t *testing.T) {
		md := qaMarkdown(t, qaHTML(`<table><tr><th>K</th><td><ul><li>aaa</li><li>bbb</li></ul></td></tr></table>`))
		if !strings.Contains(md, "aaa") || !strings.Contains(md, "bbb") || !strings.Contains(md, "K") {
			t.Errorf("cell contents lost:\n%s", md)
		}
	})

	t.Run("rowspan_keeps_cells", func(t *testing.T) {
		md := qaMarkdown(t, qaHTML(`<table><tr><th rowspan="2">R</th><td>one</td></tr><tr><td>two</td></tr></table>`))
		if !strings.Contains(md, "one") || !strings.Contains(md, "two") {
			t.Errorf("spanned cells lost:\n%s", md)
		}
	})
}

func TestQACharset(t *testing.T) {
	serve := func(t *testing.T, html string) string {
		t.Helper()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html") // no charset declared
			_, _ = w.Write([]byte(html))
		}))
		t.Cleanup(srv.Close)
		body, _, err := fetch(srv.URL)
		if err != nil {
			t.Fatalf("fetch: %v", err)
		}
		md, err := toMarkdown(body, srv.URL)
		if err != nil {
			t.Fatalf("toMarkdown: %v", err)
		}
		return md
	}

	t.Run("control_accents_in_first_bytes", func(t *testing.T) {
		html := `<!doctype html><html><head><title>t</title></head><body><article><h1>T</h1><p>Mañana española: ¿qué tal?</p></article></body></html>`
		if md := serve(t, html); !strings.Contains(md, "Mañana española") {
			t.Errorf("UTF-8 near the top is broken:\n%s", md)
		}
	})

	t.Run("QA-F-010_utf8_past_sniff_window", func(t *testing.T) {
		html := `<!doctype html><html><head><title>t</title></head><body><article><h1>T</h1><p>` +
			strings.Repeat("a", 2500) + `</p><p>Mañana española</p></article></body></html>`
		md := serve(t, html)
		qaPending(t, "QA-F-010")
		if !strings.Contains(md, "Mañana española") {
			t.Errorf("UTF-8 past the sniff window became mojibake:\n%.200s", md)
		}
	})
}

func TestQAClipboard(t *testing.T) {
	t.Run("code_tabs_become_four_spaces", func(t *testing.T) {
		m, _ := selModel(t)
		l := lineOf(t, m, "println")
		for i := 0; i < 3; i++ { // triple click: the whole line
			click(m, point{l, 4})
			release(m, point{l, 4})
		}
		got := m.selectedText()
		if !strings.Contains(got, "    println(\"hello\")") {
			t.Errorf("code line copied without its indentation:\n%q", got)
		}
		if strings.Contains(got, "\t") {
			t.Errorf("a tab reached the clipboard:\n%q", got)
		}
	})

	t.Run("copy_across_two_blocks_keeps_both", func(t *testing.T) {
		m, _ := selModel(t)
		from := lineOf(t, m, "func main()")
		to := endOf(m, lineOf(t, m, "first item"))
		if cmd := drag(m, point{from, 0}, to); cmd == nil {
			t.Error("a drag must copy on release")
		}
		got := m.selectedText()
		for _, want := range []string{"func main() {", "first item"} {
			if !strings.Contains(got, want) {
				t.Errorf("missing %q in the copied text:\n%q", want, got)
			}
		}
		if strings.ContainsAny(got, "│╭╰") {
			t.Errorf("frame characters leaked:\n%q", got)
		}
	})
}

func TestQALinkModes(t *testing.T) {
	t.Run("footnotes_default", func(t *testing.T) {
		d := render(t, sample, 80, nil)
		all := strings.Join(d.Plain, "\n")
		if !strings.Contains(all, "[1] https://ex.com/a") || !strings.Contains(all, "link[1]") {
			t.Errorf("footnotes mode broken:\n%s", all)
		}
	})

	t.Run("inline", func(t *testing.T) {
		d := render(t, sample, 80, func(c *Config) { c.Links = "inline" })
		all := strings.Join(d.Plain, "\n")
		if !strings.Contains(all, "(https://ex.com/a)") || strings.Contains(all, "[1] https://ex.com/a") {
			t.Errorf("inline mode broken:\n%s", all)
		}
	})

	t.Run("hidden", func(t *testing.T) {
		d := render(t, sample, 80, func(c *Config) { c.Links = "hidden" })
		all := strings.Join(d.Plain, "\n")
		if strings.Contains(all, "https://ex.com/a") || !strings.Contains(all, "link") {
			t.Errorf("hidden mode leaked the URL or lost the text:\n%s", all)
		}
	})
}

// TestQAFindingsCovered keeps the register, the plan and the test file in sync:
// every finding in docs/QA-FINDINGS.md needs a scenario, and every scenario ID
// must exist in the register.
func TestQAFindingsCovered(t *testing.T) {
	b, err := os.ReadFile("docs/QA-FINDINGS.md")
	if err != nil {
		t.Fatalf("read register: %v", err)
	}
	doc := string(b)
	re := regexp.MustCompile(`QA-F-\d{3}`)
	inDoc := map[string]bool{}
	for _, m := range re.FindAllString(doc, -1) {
		inDoc[m] = true
	}
	if len(inDoc) == 0 {
		t.Fatal("no finding IDs found in the register")
	}
	for id := range qaScenarioCoverage {
		if !inDoc[id] {
			t.Errorf("scenario covers %s but the register does not list it", id)
		}
	}
	for id := range inDoc {
		if _, ok := qaScenarioCoverage[id]; !ok {
			t.Errorf("finding %s has no scenario in qa_scenarios_test.go", id)
		}
	}
	for id := range pendingFindings {
		if !inDoc[id] {
			t.Errorf("pendingFindings lists %s, absent from the register", id)
		}
	}
}
