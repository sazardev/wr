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

// fenceOpens reports the fence a line opens (the run of backticks or tildes),
// by hand: the regexp was the single most expensive line of the extraction on
// a big article, running once per line of prose. An info string cannot hold
// backticks after a backtick fence.
func fenceOpens(body string) (string, bool) {
	i := 0
	for i < len(body) && (body[i] == ' ' || body[i] == '\t') {
		i++
	}
	if i >= len(body) || (body[i] != '`' && body[i] != '~') {
		return "", false
	}
	start, c := i, body[i]
	for i < len(body) && body[i] == c {
		i++
	}
	if i-start < 3 {
		return "", false
	}
	if c == '`' && strings.Contains(body[i:], "`") {
		return "", false
	}
	return body[start:i], true
}

// fenceClosed reports whether a line closes the open fence: nothing but the
// fence character, at least as long as the opening run.
func fenceClosed(line, mark string) bool {
	t := strings.TrimSpace(line)
	if len(t) < len(mark) {
		return false
	}
	return strings.Trim(t, mark[:1]) == ""
}

// isLinkOnly reports whether a line is nothing but markdown links (four or
// more), the shape a page of link lists takes: those rows carry no reading.
// The rule is the old linkOnlyLine one, matched by hand.
func isLinkOnly(body string) bool {
	count := 0
	for i := 0; i < len(body); {
		for i < len(body) && (body[i] == ' ' || body[i] == '\t') {
			i++
		}
		if i >= len(body) {
			break
		}
		if body[i] != '[' {
			return false
		}
		j := i + 1
		for j < len(body) && body[j] != ']' {
			j++
		}
		if j >= len(body) || j+1 >= len(body) || body[j+1] != '(' {
			return false
		}
		k := j + 2
		for k < len(body) && body[k] != ')' {
			k++
		}
		if k >= len(body) {
			return false
		}
		count++
		i = k + 1
	}
	return count >= 4
}

// listSeparator is what the converter writes between two adjacent lists to keep
// them apart; it is markup for Markdown parsers, not something to read.
const listSeparator = "<!--THE END-->"

// tidyMarkdown cleans extracted Markdown in one pass. Outside code fences it
// drops link-only lines, trims trailing whitespace and collapses runs of blank
// lines to at most one, which is what tidyProse did with three regexp
// replacements per stretch; fenced blocks keep their bytes. The result is
// identical: the trailing newline is settled later by the caller.
func tidyMarkdown(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	mark := ""  // the fence currently open
	blanks := 0 // newlines pending, so runs collapse across line boundaries
	for _, line := range strings.SplitAfter(text, "\n") {
		body := strings.TrimSuffix(line, "\n")
		cr := strings.HasSuffix(body, "\r")
		if cr {
			body = strings.TrimSuffix(body, "\r")
		}
		if mark != "" { // inside a fence: copy the line, watch for the close
			b.WriteString(line)
			if fenceClosed(line, mark) {
				mark = ""
			}
			continue
		}
		if f, ok := fenceOpens(body); ok {
			if n := min(blanks, 2); n > 0 {
				b.WriteString(strings.Repeat("\n", n))
			}
			blanks = 0
			b.WriteString(line)
			mark = f
			continue
		}
		if isLinkOnly(body) || body == listSeparator {
			body = ""
		}
		body = strings.TrimRight(body, " \t")
		nl := strings.HasSuffix(line, "\n")
		if body == "" {
			if nl {
				blanks++
			}
			continue
		}
		if n := min(blanks, 2); n > 0 {
			b.WriteString(strings.Repeat("\n", n))
		}
		blanks = 0
		b.WriteString(body)
		if cr {
			b.WriteByte('\r')
		}
		if nl {
			blanks = 1
		}
	}
	if n := min(blanks, 2); n > 0 {
		b.WriteString(strings.Repeat("\n", n))
	}
	return b.String()
}

// hasTitleHeading reports whether the Markdown has a level-1 heading of its own.
// A "# comment" line inside a code block is not one, so the scan tracks fences
// as it goes.
func hasTitleHeading(md string) bool {
	mark := ""
	for _, line := range strings.SplitAfter(md, "\n") {
		body := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if mark != "" {
			if fenceClosed(line, mark) {
				mark = ""
			}
			continue
		}
		if f, ok := fenceOpens(body); ok {
			mark = f
			continue
		}
		if len(body) > 1 && body[0] == '#' && (body[1] == ' ' || body[1] == '\t') {
			return true
		}
	}
	return false
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

// ---- headings ----

// permalinkGlyphs are the texts of "link to this heading" anchors.
var permalinkGlyphs = map[string]bool{
	"¶": true, "#": true, "§": true, "¤": true, "🔗": true, "∞": true, "⚓": true,
	"permalink": true, "link": true, "anchor": true, "copy link": true,
}

func isPermalinkText(t string) bool {
	t = strings.ToLower(strings.TrimSpace(t))
	return t == "" || permalinkGlyphs[t] ||
		strings.HasPrefix(t, "link to this") || strings.HasPrefix(t, "direct link to") ||
		strings.HasPrefix(t, "permalink to")
}

// cleanHeadings removes the permalink anchors generators put in headings
// ("Title¶", "Title #") and unwraps headings that are one big link to
// themselves (MDN), so a heading reads as a heading and not as a link.
func cleanHeadings(root *goquery.Selection) {
	root.Find("h1,h2,h3,h4,h5,h6").Each(func(_ int, h *goquery.Selection) {
		h.Find("a").Each(func(_ int, a *goquery.Selection) {
			txt := a.Text()
			if isPermalinkText(txt) {
				a.Remove()
				return
			}
			if strings.HasPrefix(strings.TrimSpace(a.AttrOr("href", "")), "#") {
				a.ReplaceWithSelection(a.Contents())
			}
		})
	})
}

// ---- small leaks ----

// gutterClass matches the class of a line-number gutter cell.
var gutterClass = regexp.MustCompile(`(?i)(?:^|[\s_-])(?:line-?nos?|line-?numbers?|linenos|gutter)(?:$|[\s_-])`)

// dropGutters removes the line-number cells of code tables (a <td class="line-no">
// next to the <td> that holds the <pre>): outside the <pre> they would read as
// stray numbers.
func dropGutters(root *goquery.Selection) {
	root.Find("td[class],th[class]").Each(func(_ int, c *goquery.Selection) {
		cls, _ := c.Attr("class")
		if gutterClass.MatchString(cls) && c.Closest("table").Find("pre").Length() > 0 {
			c.Remove()
		}
	})
}

// stripComments removes HTML comments, which the converter would otherwise
// keep between block elements.
func stripComments(root *goquery.Selection) {
	var walk func(n *nethtml.Node)
	walk = func(n *nethtml.Node) {
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			if c.Type == nethtml.CommentNode {
				n.RemoveChild(c)
			} else {
				walk(c)
			}
			c = next
		}
	}
	for _, n := range root.Nodes {
		walk(n)
	}
}

// crumbSeparators are what sits between the links of a breadcrumb strip.
const crumbSeparators = " \t/\\>»›|:·•–—-→"

var (
	loneLinkRe   = regexp.MustCompile(`^\[[^\]\n]{1,40}\]\([^)\s]*(?: "[^"]*")?\)$`)
	prevNextRe   = regexp.MustCompile(`(?i)^(next|previous|prev|up|back|home)\s*:\s*\[`)
	listItemRe   = regexp.MustCompile(`^\s*(?:\d+[.)]|[-*+])\s+(.*)$`)
	crumbItemMax = 40
)

// isCrumbBlock reports whether a block (paragraph or list, up to the first
// blank line) is navigation that precedes the page's own heading: a strip of
// links and separators ([std](/a)::[vec](/b)), a lone link above the title
// (a section label), a GNU-style "Next: …, Up: …" line, or a short list of
// links or names (a breadcrumb written as <ol>). headingAhead says the page's
// heading follows (directly, or after a one-line label); afterCrumb says a
// crumb was already dropped, which is what allows a bare label to go too.
func isCrumbBlock(block string, headingAhead, afterCrumb bool) bool {
	block = strings.TrimSpace(block)
	if block == "" || strings.HasPrefix(block, "#") || strings.HasPrefix(block, "```") {
		return false
	}
	lines := strings.Split(block, "\n")
	if len(lines) == 1 {
		line := lines[0]
		switch {
		case prevNextRe.MatchString(line):
			return true
		case loneLinkRe.MatchString(line):
			return headingAhead
		case strings.Count(line, "](") >= 2 && strings.Trim(linkRe.ReplaceAllString(line, ""), crumbSeparators) == "":
			return true
		case afterCrumb && headingAhead && isBareLabel(line):
			return true
		}
	}
	// a short list whose items are all short: a breadcrumb trail
	if !headingAhead || len(lines) < 2 || len(lines) > 5 {
		return false
	}
	for _, l := range lines {
		m := listItemRe.FindStringSubmatch(l)
		if m == nil || len([]rune(linkRe.ReplaceAllString(m[1], "$1"))) > crumbItemMax {
			return false
		}
	}
	return true
}

// isBareLabel is a short line with no sentence in it ("Installation").
func isBareLabel(line string) bool {
	return len([]rune(line)) <= 30 && !strings.ContainsAny(line, ".!?:;[]()`*_<>|") && len(strings.Fields(line)) <= 4
}

// dropLeadingCrumbs removes the navigation blocks a page opens with, before
// its first heading (at most three), so the reading starts at the title.
func dropLeadingCrumbs(text string) string {
	blocks := strings.Split(text, "\n\n")
	startsHeading := func(i int) bool { return i < len(blocks) && strings.HasPrefix(strings.TrimSpace(blocks[i]), "#") }
	dropped := 0
	for dropped < 3 && dropped+1 < len(blocks) {
		i := dropped
		ahead := startsHeading(i+1) || (i+2 < len(blocks) && isBareLabel(strings.TrimSpace(blocks[i+1])) && startsHeading(i+2))
		if !isCrumbBlock(blocks[i], ahead, dropped > 0) {
			break
		}
		dropped++
	}
	if dropped == 0 {
		return text
	}
	return strings.TrimLeft(strings.Join(blocks[dropped:], "\n\n"), "\n")
}

// dropHidden removes what the page itself hides from sight: the hidden
// attribute and inline display:none / visibility:hidden. An element holding
// most of the page's text stays, because some sites hide the whole article
// until their script shows it.
func dropHidden(root *goquery.Selection) {
	total := textWeight(root.Nodes[0])
	root.Find(`[hidden],[style*="display:none"],[style*="display: none"],[style*="visibility:hidden"],[style*="visibility: hidden"]`).Each(func(_ int, el *goquery.Selection) {
		if v, ok := el.Attr("hidden"); ok && strings.EqualFold(v, "until-found") {
			return // collapsed but real content, the browser finds it on search
		}
		if textWeight(el.Nodes[0])*2 <= total {
			el.Remove()
		}
	})
}

// ---- anchors ----

const blockTags = "div,p,h1,h2,h3,h4,h5,h6,li,ul,ol,section,article,header,footer,dl,dt,dd,figure,figcaption,table,blockquote,pre"

var skipLinkRe = regexp.MustCompile(`(?i)^\s*(skip|jump) to (the )?(main |primary )?(content|navigation|menu|search|footer)`)

// tidyAnchors fixes the three shapes of link that read badly once converted:
//   - "skip to content" links: invisible in a browser, noise here;
//   - anchors with no text at all (heading anchors, icon links, <a id=...>), which
//     would be written as "[](url)";
//   - cards: an anchor wrapping blocks (title + description). The converter joins
//     them with hard breaks inside the link text; here the first block becomes the
//     link and the rest stays as plain text after it.
func tidyAnchors(root *goquery.Selection) {
	root.Find("a").Each(func(_ int, a *goquery.Selection) {
		text := strings.Join(strings.Fields(a.Text()), " ")
		switch {
		case text == "" && a.Find("img").Length() == 0:
			a.Remove()
			return
		case skipLinkRe.MatchString(text) && len(text) < 40:
			a.Remove()
			return
		}
		if a.AttrOr("href", "") == "" {
			a.ReplaceWithSelection(a.Contents()) // a named anchor with text: just the text
			return
		}
		a.Find("br").ReplaceWithNodes(&nethtml.Node{Type: nethtml.TextNode, Data: " "})
		if a.Find(blockTags).Length() == 0 {
			return
		}
		// leaf blocks: those holding text and no block of their own; a heading
		// is the best title for the link, else the first one
		var first, firstHeading *goquery.Selection
		a.Find(blockTags).Each(func(_ int, b *goquery.Selection) {
			if b.Find(blockTags).Length() > 0 || strings.TrimSpace(b.Text()) == "" {
				return
			}
			if first == nil {
				first = b
			}
			if firstHeading == nil && len(goquery.NodeName(b)) == 2 && goquery.NodeName(b)[0] == 'h' {
				firstHeading = b
			}
		})
		if firstHeading != nil {
			first = firstHeading
		}
		if first == nil {
			return
		}
		link := &nethtml.Node{Type: nethtml.ElementNode, Data: "a", Attr: append([]nethtml.Attribute(nil), a.Get(0).Attr...)}
		for c := first.Get(0).FirstChild; c != nil; {
			next := c.NextSibling
			first.Get(0).RemoveChild(c)
			link.AppendChild(c)
			c = next
		}
		first.Get(0).AppendChild(link)
		a.ReplaceWithSelection(a.Contents()) // the card's other blocks become plain text
	})
}

// dedupeLinkLines drops a link-bearing line that repeats the previous
// non-blank one (an avatar link followed by the same name link, an image link
// followed by its caption link), outside code fences.
func dedupeLinkLines(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	mark, prev := "", ""
	for _, line := range strings.SplitAfter(text, "\n") {
		body := strings.TrimSuffix(line, "\n")
		if mark != "" {
			b.WriteString(line)
			if fenceClosed(line, mark) {
				mark = ""
			}
			continue
		}
		if f, ok := fenceOpens(body); ok {
			mark, prev = f, ""
			b.WriteString(line)
			continue
		}
		if t := strings.TrimSpace(body); t != "" {
			if t == prev && strings.Contains(t, "](") {
				continue
			}
			prev = t
		}
		b.WriteString(line)
	}
	return b.String()
}

// ---- code lines ----

// flattenCodeLines rewrites a <pre> whose lines are block elements (Expressive
// Code's div.ec-line, Torchlight's div.line, tables of lines) into plain text,
// one newline per line. Left alone, the converter separates the blocks with
// blank lines and the code comes out double spaced. Pre blocks whose lines are
// inline spans with real newlines between them (Shiki, Pygments) are untouched.
func flattenCodeLines(root *goquery.Selection) {
	root.Find("pre").Each(func(_ int, pre *goquery.Selection) {
		if pre.Find("div,p,li,tr").Length() == 0 {
			return
		}
		var b strings.Builder
		var walk func(n *nethtml.Node)
		walk = func(n *nethtml.Node) {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				switch {
				case c.Type == nethtml.TextNode:
					b.WriteString(c.Data)
				case c.Type == nethtml.ElementNode && c.Data == "br":
					b.WriteByte('\n')
				case c.Type == nethtml.ElementNode:
					walk(c)
					switch c.Data {
					case "div", "p", "li", "tr":
						if s := b.String(); s != "" && !strings.HasSuffix(s, "\n") {
							b.WriteByte('\n')
						}
					}
				}
			}
		}
		walk(pre.Get(0))
		holder := pre.Find("code").First()
		if holder.Length() == 0 {
			holder = pre
		}
		text := &nethtml.Node{Type: nethtml.TextNode, Data: strings.TrimRight(b.String(), "\n")}
		for c := holder.Get(0).FirstChild; c != nil; {
			next := c.NextSibling
			holder.Get(0).RemoveChild(c)
			c = next
		}
		holder.Get(0).AppendChild(text)
		if holder.Get(0) != pre.Get(0) { // anything beside the <code> (copy buttons, labels) goes
			pre.Children().Not("code").Remove()
		}
	})
}

// dropFrameLabels removes the visually hidden label of a code frame
// ("Terminal window"), which is chrome for screen readers.
func dropFrameLabels(root *goquery.Selection) {
	root.Find("figure").Each(func(_ int, f *goquery.Selection) {
		if f.Find("pre").Length() > 0 {
			f.Find("figcaption .sr-only, figcaption .visually-hidden").Remove()
		}
	})
}
