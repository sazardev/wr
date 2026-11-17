package main

// Pages that draw their content with JavaScript (a Vite/React/Vue SPA) reach wr
// as an empty mount point plus scripts. wr does not run scripts, but such pages
// still say what they are in places the extraction throws away: <noscript>,
// <meta name="description"> and JSON-LD. This file collects those and, when the
// extraction came out empty on a page that looks like a JS shell, shows them
// with a notice that explains why there is so little (and what to do).

import (
	"encoding/json"
	"net/url"
	"path"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const (
	// thinWords is how short an extraction has to be before a JS shell is suspected.
	thinWords = 40
	// shellHead opens the blockquote that tells the reader why the page is bare.
	shellHead = "> **This page draws its content with JavaScript**"
)

// shellNotice is the whole notice: what happened and what the reader can do,
// which depends on whether a browser is available to render the page.
func shellNotice() string {
	if findBrowser() == "" {
		return shellHead + ", which wr does not run, so only its summary is shown. Install Chromium or Chrome and wr renders such pages by itself; meanwhile press `x` to open it in your browser."
	}
	if !jsRender {
		return shellHead + ", and rendering is off, so only its summary is shown. Turn on `render_js` in the settings, or press `x` to open it in your browser."
	}
	return shellHead + " and rendering it failed, so only its summary is shown. Press `x` to open it in your browser."
}

// noscriptArticle returns the markup of a <noscript> fallback that is a whole
// article (a pre-rendered page: headings, code, tables), or "" when there is
// none. Showing it through the normal extraction keeps its structure; the flat
// paragraphs of the hints would lose it.
func noscriptArticle(doc *goquery.Document) string {
	best := ""
	doc.Find("body noscript").Each(func(_ int, ns *goquery.Selection) {
		inner, err := goquery.NewDocumentFromReader(strings.NewReader(ns.Text()))
		if err != nil || len(strings.Fields(inner.Text())) < 4*thinWords {
			return
		}
		if len(ns.Text()) > len(best) {
			best = ns.Text()
		}
	})
	return best
}

// pageHints is what a page says about itself outside of its body.
type pageHints struct {
	blocks []string // summary paragraphs, in order of trust, already de-duplicated
	shell  bool     // the page looks like a script-drawn shell
}

// mountPoints are the elements SPA frameworks render into.
const mountPoints = "#root,#app,#__next,#__nuxt,#svelte,#q-app,#___gatsby,app-root,[data-reactroot]"

// collectHints must run before dropTags removes <noscript> and <script>.
func collectHints(doc *goquery.Document) pageHints {
	var h pageHints
	add := func(s string) {
		s = strings.TrimSpace(sanitize(s))
		if s == "" || strings.Contains(strings.ToLower(s), "javascript") && len(strings.Fields(s)) < 25 {
			return // "you need to enable JavaScript" says nothing the notice does not
		}
		key := strings.ToLower(strings.Join(strings.Fields(s), " "))
		for _, b := range h.blocks {
			bk := strings.ToLower(strings.Join(strings.Fields(b), " "))
			if strings.Contains(bk, key) { // already said, possibly in more words
				return
			}
		}
		h.blocks = append(h.blocks, s)
	}

	// <noscript> outside <head> holds real fallback content on shells (the parser
	// keeps it as one text node, so it is parsed again).
	doc.Find("body noscript").Each(func(_ int, ns *goquery.Selection) {
		inner, err := goquery.NewDocumentFromReader(strings.NewReader(ns.Text()))
		if err != nil {
			return
		}
		if t := strings.ToLower(ns.Text()); strings.Contains(t, "javascript") {
			h.shell = true
		}
		inner.Find("h1,h2,h3,p,li").Each(func(_ int, el *goquery.Selection) {
			add(strings.Join(strings.Fields(el.Text()), " "))
		})
	})

	// JSON-LD: the body of the article when the site ships it, else its summary.
	var bodies, summaries []string
	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		var v any
		if json.Unmarshal([]byte(s.Text()), &v) == nil {
			walkLD(v, &bodies, &summaries)
		}
	})
	for _, b := range bodies {
		add(b)
	}
	if desc := metaContent(doc, `meta[name="description"]`, `meta[property="og:description"]`); desc != "" {
		summaries = append([]string{desc}, summaries...)
	}
	for _, s := range summaries {
		add(s)
	}

	// A mount point that holds next to no text ("Loading…" included), a module
	// script on a bare page, or a nearly textless body that loads scripts.
	doc.Find(mountPoints).EachWithBreak(func(_ int, m *goquery.Selection) bool {
		if len(strings.Fields(m.Text())) < 15 {
			h.shell = true
			return false
		}
		return true
	})
	if doc.Find(`script[type="module"][src]`).Length() > 0 {
		h.shell = true
	}
	if body := doc.Find("body").First(); body.Length() > 0 && textWeight(body.Nodes[0]) < 15 && doc.Find("script[src]").Length() > 0 {
		h.shell = true
	}
	return h
}

// walkLD collects articleBody and description strings from decoded JSON-LD,
// following arrays and @graph.
func walkLD(v any, bodies, summaries *[]string) {
	switch t := v.(type) {
	case []any:
		for _, e := range t {
			walkLD(e, bodies, summaries)
		}
	case map[string]any:
		if s, ok := t["articleBody"].(string); ok {
			*bodies = append(*bodies, s)
		}
		if s, ok := t["description"].(string); ok && t["@type"] != nil {
			*summaries = append(*summaries, s)
		}
		if g, ok := t["@graph"]; ok {
			walkLD(g, bodies, summaries)
		}
	}
}

func metaContent(doc *goquery.Document, selectors ...string) string {
	for _, sel := range selectors {
		if c := strings.TrimSpace(doc.Find(sel).First().AttrOr("content", "")); c != "" {
			return c
		}
	}
	return ""
}

// bodyWords counts the words of an extraction without its title heading.
func bodyWords(md string) int {
	n := 0
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "# ") {
			continue
		}
		n += len(strings.Fields(line))
	}
	return n
}

// withShellFallback prepends the notice and the page's own summary to a thin
// extraction of a page that looks drawn by scripts. Other pages come back as is.
func withShellFallback(text, title string, h pageHints) string {
	if !h.shell || bodyWords(text) >= thinWords {
		return text
	}
	var b strings.Builder
	b.WriteString(shellNotice())
	b.WriteString("\n\n")
	for _, blk := range h.blocks {
		if !strings.Contains(text, blk) && !strings.Contains(strings.ToLower(title), strings.ToLower(blk)) {
			b.WriteString(blk)
			b.WriteString("\n\n")
		}
	}
	return b.String() + strings.TrimSpace(text) + "\n"
}

// isShellPage reports whether an extraction carries the JS-shell notice.
func isShellPage(md string) bool { return strings.Contains(md, shellHead) }

// alternateLink finds a text version a page advertises about itself
// (<link rel="alternate" type="text/markdown">), resolved against src.
func alternateLink(page []byte, src string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(page)))
	if err != nil {
		return ""
	}
	base, err := url.Parse(src)
	if err != nil {
		return ""
	}
	var found string
	doc.Find(`link[rel~="alternate"][href]`).EachWithBreak(func(_ int, l *goquery.Selection) bool {
		switch mediaType(l.AttrOr("type", "")) {
		case "text/markdown", "text/x-markdown", "text/plain":
			if strings.HasPrefix(path.Base(l.AttrOr("href", "")), "llms") {
				return true // llms.txt indexes the whole site; it is not this page
			}
			if ref, err := url.Parse(l.AttrOr("href", "")); err == nil {
				found = base.ResolveReference(ref).String()
				return false
			}
		}
		return true
	})
	return found
}
