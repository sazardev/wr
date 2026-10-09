package main

// Helpers for choosing and cleaning the extraction root (see toMarkdown). The
// fixes in here are tracked in docs/QA-FINDINGS.md (QA-F-001 ... QA-F-013).

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	nethtml "golang.org/x/net/html"
)

var dropSet = func() map[string]bool {
	m := map[string]bool{}
	for _, t := range strings.Split(dropTags, ",") {
		m[t] = true
	}
	return m
}()

// textWeight counts the words that would survive dropTags under n.
func textWeight(n *nethtml.Node) int {
	switch n.Type {
	case nethtml.TextNode:
		return len(strings.Fields(n.Data))
	case nethtml.ElementNode:
		if dropSet[n.Data] {
			return 0
		}
	}
	w := 0
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		w += textWeight(c)
	}
	return w
}

// pickRoot chooses the element the reading content lives in. A page with a
// single <article> uses it; with several (listings, tag pages) the one holding
// most of the text wins, and when none dominates the whole <main>/<body> is
// used so no teaser is silently picked over the rest. An <article> that is only
// a small fragment of the page (a banner, a lone card) is not trusted either.
// It reports whether the root is an <article>.
func pickRoot(doc *goquery.Document) (*goquery.Selection, bool) {
	container := doc.Find("main").First()
	if container.Length() == 0 {
		container = doc.Find("body").First()
	}
	arts := doc.Find("article").FilterFunction(func(_ int, a *goquery.Selection) bool {
		return a.ParentsFiltered("article").Length() == 0 // outermost only
	})
	if arts.Length() == 0 || container.Length() == 0 {
		if arts.Length() > 0 {
			return arts.First(), true
		}
		return container, false
	}
	best, bestW, total := 0, -1, 0
	arts.Each(func(i int, a *goquery.Selection) {
		w := textWeight(a.Nodes[0])
		total += w
		if w > bestW {
			best, bestW = i, w
		}
	})
	dominant := bestW*100 >= total*60 // the biggest article holds most of the article text
	fragment := textWeight(container.Nodes[0]) >= 5*bestW
	if dominant && !fragment {
		return arts.Eq(best), true
	}
	return container, false
}

// dropHeaders removes <header> elements (site chrome) but keeps the page heading
// when the first <h1> lives inside one, as on Wikipedia.
func dropHeaders(root *goquery.Selection) {
	var heading string
	if h1 := root.Find("h1").First(); h1.Length() > 0 && h1.ParentsUntilSelection(root).Filter("header").Length() > 0 {
		heading = strings.Join(strings.Fields(h1.Text()), " ")
	}
	root.Find("header").Remove()
	if heading == "" {
		return
	}
	h := &nethtml.Node{Type: nethtml.ElementNode, Data: "h1"}
	h.AppendChild(&nethtml.Node{Type: nethtml.TextNode, Data: heading})
	r := root.Nodes[0]
	r.InsertBefore(h, r.FirstChild)
}

// dropChrome removes page chrome that is not a semantic element: ARIA landmarks
// and elements whose class names one of junkWords. Class matching is by whole
// token, and an element holding most of the page text is never removed, so a
// layout flag such as class="has-toc" cannot take the article with it.
func dropChrome(root *goquery.Selection) {
	total := textWeight(root.Nodes[0])
	root.Find(junkRoles).Remove()
	root.Find("[class]").Each(func(_ int, s *goquery.Selection) {
		if cls, _ := s.Attr("class"); hasJunkClass(cls) && textWeight(s.Nodes[0])*2 <= total {
			s.Remove()
		}
	})
}

// hasJunkClass reports whether a class attribute has a token that is a junk word
// or starts with one followed by - or _ (toc-wrapper, sidebar__left). A word at
// the end of a token (has-toc, no-sidebar) does not count: those are flags.
func hasJunkClass(cls string) bool {
	for _, tok := range strings.Fields(strings.ToLower(cls)) {
		if strings.HasPrefix(tok, "cookie") {
			return true
		}
		for _, w := range junkWords {
			if tok == w || strings.HasPrefix(tok, w+"-") || strings.HasPrefix(tok, w+"_") {
				return true
			}
		}
	}
	return false
}

// visibleEscapes turns real ESC bytes inside a code block into the visible
// symbol ␛. The control byte itself must never reach the terminal (sanitize
// strips it), but the code should still show that it was there.
func visibleEscapes(pre *goquery.Selection) {
	var walk func(n *nethtml.Node)
	walk = func(n *nethtml.Node) {
		if n.Type == nethtml.TextNode {
			n.Data = strings.ReplaceAll(n.Data, "\x1b", "␛")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	for _, n := range pre.Nodes {
		walk(n)
	}
}

// ---- Markdown text outside of code ----

var fenceOpenRe = regexp.MustCompile("^[ \\t]*(`{3,}|~{3,})(.*)$")

// mapOutsideFences applies f to every stretch of text that is not inside a
// fenced code block and leaves the fenced blocks (fences included) untouched.
func mapOutsideFences(text string, f func(string) string) string {
	var out, prose strings.Builder
	flush := func() {
		if prose.Len() > 0 {
			out.WriteString(f(prose.String()))
			prose.Reset()
		}
	}
	mark := "" // the fence that is open, "" when in prose
	for _, line := range strings.SplitAfter(text, "\n") {
		if mark == "" {
			m := fenceOpenRe.FindStringSubmatch(strings.TrimRight(line, "\r\n"))
			// an info string cannot hold backticks after a backtick fence
			if m != nil && !(m[1][0] == '`' && strings.Contains(m[2], "`")) {
				flush()
				mark = m[1]
				out.WriteString(line)
				continue
			}
			prose.WriteString(line)
			continue
		}
		out.WriteString(line)
		if t := strings.TrimSpace(line); len(t) >= len(mark) && strings.Trim(t, mark[:1]) == "" {
			mark = ""
		}
	}
	flush()
	return out.String()
}

var titleHeadingRe = regexp.MustCompile(`(?m)^#\s`)

// hasTitleHeading reports whether the Markdown has a level-1 heading of its own.
// A "# comment" line inside a code block is not one.
func hasTitleHeading(md string) bool {
	found := false
	mapOutsideFences(md, func(p string) string {
		found = found || titleHeadingRe.MatchString(p)
		return p
	})
	return found
}

// ---- tables ----

// flattenTables prepares tables for the converter, which gives up on a table as
// soon as a cell is not plain inline text: lists inside cells become one line
// separated by " · ", paragraphs and line breaks become spaces, colspan/rowspan
// become real (empty) cells and a table with no header row promotes its first
// row (Markdown tables need one; the converter would add an empty fake row).
func flattenTables(root *goquery.Selection) {
	root.Find("td ul, td ol, th ul, th ol").Each(func(_ int, list *goquery.Selection) {
		if list.Closest("td, th").Length() == 0 {
			return
		}
		var items []string
		list.Children().Each(func(_ int, li *goquery.Selection) {
			if t := strings.Join(strings.Fields(li.Text()), " "); t != "" {
				items = append(items, t)
			}
		})
		list.ReplaceWithNodes(&nethtml.Node{Type: nethtml.TextNode, Data: strings.Join(items, " · ")})
	})
	root.Find("td br, th br").ReplaceWithNodes(&nethtml.Node{Type: nethtml.TextNode, Data: " "})
	root.Find("td p, td div, th p, th div").Each(func(_ int, blk *goquery.Selection) {
		blk.BeforeNodes(&nethtml.Node{Type: nethtml.TextNode, Data: " "})
		blk.ReplaceWithSelection(blk.Contents())
	})
	root.Find("table").Each(func(_ int, t *goquery.Selection) {
		expandSpans(t)
		if t.Find("table").Length() == 0 && t.Find("th").Length() == 0 {
			t.Find("tr").First().Children().Filter("td").Each(func(_ int, c *goquery.Selection) {
				c.Get(0).Data = "th"
			})
		}
	})
}

// spanOf reads a colspan/rowspan attribute (at least 1, capped to stay sane).
func spanOf(c *goquery.Selection, attr string) int {
	n, err := strconv.Atoi(strings.TrimSpace(c.AttrOr(attr, "1")))
	if err != nil || n < 1 {
		return 1
	}
	return min(n, 50)
}

// expandSpans rewrites a table so that no cell spans: the cell keeps its text
// and the covered positions become empty cells.
func expandSpans(table *goquery.Selection) {
	pending := map[int]int{} // column -> rows still covered by a rowspan above
	empty := func(tag string) *nethtml.Node { return &nethtml.Node{Type: nethtml.ElementNode, Data: tag} }
	table.Find("tr").Each(func(_ int, tr *goquery.Selection) {
		if tr.ParentsFiltered("table").First().Get(0) != table.Get(0) {
			return // a row of a nested table
		}
		row := tr.Get(0)
		col := 0
		// fill takes the columns still covered from above before the next cell
		fill := func(before *nethtml.Node) {
			for pending[col] > 0 {
				pending[col]--
				row.InsertBefore(empty("td"), before)
				col++
			}
		}
		for c := row.FirstChild; c != nil; {
			next := c.NextSibling
			if c.Type != nethtml.ElementNode || (c.Data != "td" && c.Data != "th") {
				c = next
				continue
			}
			fill(c)
			cell := goquery.NewDocumentFromNode(c).Selection
			cs, rs := spanOf(cell, "colspan"), spanOf(cell, "rowspan")
			for i := 1; i < cs; i++ {
				row.InsertBefore(empty(c.Data), next)
			}
			for i := 0; i < cs && rs > 1; i++ {
				pending[col+i] = rs - 1
			}
			col += cs
			c = next
		}
		fill(nil)
	})
	table.Find("[colspan],[rowspan]").RemoveAttr("colspan").RemoveAttr("rowspan")
}
