package main

import (
	"html"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/x/ansi"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// A custom Markdown-to-ANSI-lines renderer:
//   - the colors come from the terminal's own 16 ANSI palette (or a fixed
//     256-color preset), so the terminal picks the shades and everything
//     follows your theme (gruvbox, nord, ...);
//   - every line is self-contained (no open styles), so lines can be painted
//     one by one in a TUI;
//   - Markdown marks (#, **, `) disappear: you see the formatting, not the syntax.

type Head struct {
	Line  int
	Level int
	Text  string
}

type Doc struct {
	Lines []string   // self-contained ANSI lines
	Plain []string   // the same without escapes (for searching and copying)
	Heads []Head     // top-level headings, with their line
	Meta  []LineMeta // what each line is, so copying can do the right thing
	Links []LinkSpan // every link, in reading order
	URLs  []string   // destinations, indexed by LinkSpan.ID
}

// LinkSpan is where one link sits on screen: a rune range on a line. A link
// that wraps onto several lines becomes several spans with the same ID.
type LinkSpan struct{ Line, From, To, ID int }

type LineKind uint8

const (
	kindText LineKind = iota
	kindCodeTop
	kindCodeBody
	kindCodeBottom
)

// LineMeta describes a rendered line for the benefit of text selection: code
// lines lose their frame when copied, and soft-wrapped lines are re-joined.
type LineMeta struct {
	Kind   LineKind
	Soft   bool // continues the previous line (word wrap), it is not a real break
	Gutter int  // for code lines: rune column where the "│ " gutter starts
}

// Invisible markers (APC sequences: zero width, stripped before display). The
// renderer plants them while building lines and renderDoc turns them into
// LineMeta and removes them.
const (
	mSoft   = "\x1b_s\x1b\\"
	mTop    = "\x1b_t\x1b\\"
	mBottom = "\x1b_b\x1b\\"
	mBody   = "\x1b_c\x1b\\"
	mCont   = "\x1b_k\x1b\\"
)

const mLinkEnd = "\x1b_e\x1b\\"

func mLinkStart(id int) string { return "\x1b_l" + strconv.Itoa(id) + "\x1b\\" }

var allMarkers = []string{mSoft, mTop, mBottom, mBody, mCont, mLinkEnd}

var linkStartRe = regexp.MustCompile("\x1b_l[0-9]+\x1b\\\\")

type Glyphs struct {
	Head    [6]string
	Rule    [2]string // rule under h1 / h2
	HR      string
	Bullets [3]string
	Quote   string
}

var brailleGlyphs = Glyphs{
	Head:    [6]string{"⣿ ", "⣶ ", "⠶ ", "⠤ ", "⠒ ", "⠂ "},
	Rule:    [2]string{"⣀", "⠤"},
	HR:      "⠤",
	Bullets: [3]string{"•", "◦", "▪"},
	Quote:   "▎",
}

var plainGlyphs = Glyphs{
	Head:    [6]string{"█ ", "▌ ", "▎ ", "▏ ", "· ", "· "},
	Rule:    [2]string{"━", "─"},
	HR:      "─",
	Bullets: [3]string{"•", "◦", "▪"},
	Quote:   "▎",
}

var mdParser = goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser()

type renderer struct {
	src     []byte
	cfg     Config
	g       Glyphs
	links   []string
	linkIdx map[string]int
	hl      map[ast.Node][]string // code blocks already highlighted (in parallel)
	urls    []string              // every link destination, for clicking
	urlIdx  map[string]int
}

// linkID registers a destination and returns its id.
func (r *renderer) linkID(dest string) int {
	if i, ok := r.urlIdx[dest]; ok {
		return i
	}
	r.urls = append(r.urls, dest)
	r.urlIdx[dest] = len(r.urls) - 1
	return len(r.urls) - 1
}

// renderDoc converts Markdown into a document `width` columns wide.
func renderDoc(md string, width int, cfg Config) *Doc {
	src := []byte(md)
	root := mdParser.Parse(text.NewReader(src))
	r := &renderer{src: src, cfg: cfg, g: plainGlyphs, linkIdx: map[string]int{}, urlIdx: map[string]int{}}
	if cfg.Braille {
		r.g = brailleGlyphs
	}

	r.prehighlight(root)

	var out []string
	var heads []Head
	first := true
	for c := root.FirstChild(); c != nil; c = c.NextSibling() {
		ls := r.block(c, width)
		if len(ls) == 0 {
			continue
		}
		if !first {
			for i := 0; i < cfg.Spacing; i++ {
				out = append(out, "")
			}
			if h, ok := c.(*ast.Heading); ok && h.Level <= 2 && cfg.Spacing > 0 {
				out = append(out, "") // extra breathing room before h1/h2
			} else if ok && h.Level <= 2 {
				out = append(out, "") // even compact keeps one blank line before a section
			}
		}
		if h, ok := c.(*ast.Heading); ok {
			heads = append(heads, Head{Line: len(out), Level: h.Level, Text: nodeText(h, src)})
		}
		out = append(out, ls...)
		first = false
	}

	if len(r.links) > 0 && cfg.Links == "footnotes" {
		out = append(out, "", "", style("1;"+headColor[2], r.g.Head[2]+"Links"), "")
		for i, u := range r.links {
			line := dim("["+strconv.Itoa(i+1)+"]") + " " + mLinkStart(r.linkID(u)) + link(u) + mLinkEnd
			out = append(out, strings.Split(ansi.Hardwrap(line, width, true), "\n")...)
		}
	}

	out = selfContain(out)
	meta := make([]LineMeta, len(out))
	plain := make([]string, len(out))
	var spans []LinkSpan
	open, start := -1, 0
	for i, l := range out {
		if strings.Contains(l, "\x1b_") {
			meta[i] = metaOf(l)
			spans = scanLinks(l, i, &open, &start, spans)
			for _, mk := range allMarkers {
				l = strings.ReplaceAll(l, mk, "")
			}
			l = linkStartRe.ReplaceAllString(l, "")
			out[i] = l
		} else if open >= 0 { // a link wrapped across a line without markers
			spans = scanLinks(l, i, &open, &start, spans)
		}
		plain[i] = ansi.Strip(l)
	}
	return &Doc{Lines: out, Plain: plain, Heads: heads, Meta: meta, Links: spans, URLs: r.urls}
}

// scanLinks finds the link ranges on one line. A link still open at the end of
// the line continues on the next one.
func scanLinks(l string, line int, open, start *int, out []LinkSpan) []LinkSpan {
	vis, i := 0, 0
	for i < len(l) {
		if l[i] == 0x1b && i+1 < len(l) {
			switch l[i+1] {
			case '[':
				j := i + 2
				for j < len(l) && !(l[j] >= '@' && l[j] <= '~') {
					j++
				}
				i = min(j+1, len(l))
			case '_': // APC marker: l<id> opens a link, e closes it
				end := strings.Index(l[i+2:], "\x1b\\")
				if end < 0 {
					i = len(l)
					break
				}
				payload := l[i+2 : i+2+end]
				switch {
				case strings.HasPrefix(payload, "l"):
					if id, err := strconv.Atoi(payload[1:]); err == nil {
						*open, *start = id, vis
					}
				case payload == "e":
					if *open >= 0 && vis > *start {
						out = append(out, LinkSpan{line, *start, vis, *open})
					}
					*open = -1
				}
				i += 2 + end + 2
			default:
				i++
			}
			continue
		}
		_, size := utf8.DecodeRuneInString(l[i:])
		vis++
		i += size
	}
	if *open >= 0 && vis > *start {
		out = append(out, LinkSpan{line, *start, vis, *open})
	}
	*start = 0 // if the link goes on, it begins at the left edge of the next line
	return out
}

// metaOf reads the markers planted in a line.
func metaOf(l string) LineMeta {
	mt := LineMeta{Soft: strings.Contains(l, mSoft)}
	for _, c := range []struct {
		mk   string
		kind LineKind
	}{{mTop, kindCodeTop}, {mBottom, kindCodeBottom}, {mBody, kindCodeBody}, {mCont, kindCodeBody}} {
		if i := strings.Index(l, c.mk); i >= 0 {
			mt.Kind = c.kind
			mt.Gutter = utf8.RuneCountInString(ansi.Strip(l[:i]))
			if c.mk == mCont {
				mt.Soft = true
			}
			break
		}
	}
	return mt
}

func nodeText(n ast.Node, src []byte) string {
	var b strings.Builder
	var walk func(ast.Node)
	walk = func(n ast.Node) {
		switch v := n.(type) {
		case *ast.Text:
			b.Write(v.Segment.Value(src))
		case *ast.String:
			b.Write(v.Value)
		}
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			walk(c)
		}
	}
	walk(n)
	return strings.TrimSpace(html.UnescapeString(string(util.UnescapePunctuations([]byte(b.String())))))
}

// ---- blocks ----

func (r *renderer) blocks(parent ast.Node, w int, tight bool) []string {
	var out []string
	first := true
	for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
		ls := r.block(c, w)
		if len(ls) == 0 {
			continue
		}
		if !first && !tight {
			for i := 0; i < r.cfg.Spacing; i++ {
				out = append(out, "")
			}
		}
		out = append(out, ls...)
		first = false
	}
	return out
}

// wrap word-wraps s. Lines produced by wrapping (not by an explicit line break)
// are marked as soft continuations, so a copied paragraph comes out as one line.
func wrap(s string, w int) []string {
	var out []string
	for _, part := range strings.Split(s, "\n") {
		for i, l := range strings.Split(ansi.Wrap(part, max(w, 1), ""), "\n") {
			if i > 0 {
				l = mSoft + l
			}
			out = append(out, l)
		}
	}
	return out
}

// wrapRaw wraps without markers (table cells: their lines are not a paragraph).
func wrapRaw(s string, w int) []string {
	return strings.Split(ansi.Wrap(s, max(w, 1), ""), "\n")
}

// codeOf returns the language and text of a code block.
func (r *renderer) codeOf(n ast.Node) (lang, code string, ok bool) {
	var lines *text.Segments
	switch v := n.(type) {
	case *ast.FencedCodeBlock:
		lang, lines = strings.ToLower(strings.TrimSpace(string(v.Language(r.src)))), v.Lines()
	case *ast.CodeBlock:
		lines = v.Lines()
	default:
		return "", "", false
	}
	var b strings.Builder
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.Write(seg.Value(r.src))
	}
	return lang, strings.TrimRight(b.String(), "\n"), true
}

// prehighlight highlights every code block at once, one goroutine each:
// tokenizing with chroma is the most expensive part of rendering a document.
func (r *renderer) prehighlight(root ast.Node) {
	type job struct {
		node       ast.Node
		lang, code string
		out        []string
	}
	var jobs []*job
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if lang, code, ok := r.codeOf(n); ok {
				jobs = append(jobs, &job{node: n, lang: lang, code: code})
			}
		}
		return ast.WalkContinue, nil
	})
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			j.out = highlightLines(j.lang, j.code)
		}()
	}
	wg.Wait()
	r.hl = make(map[ast.Node][]string, len(jobs))
	for _, j := range jobs {
		r.hl[j.node] = j.out
	}
}

func (r *renderer) block(n ast.Node, w int) []string {
	switch v := n.(type) {
	case *ast.Heading:
		lvl := min(max(v.Level, 1), 6)
		col := headColor[lvl-1]
		body := r.inline(n, "1;"+col)
		ls := wrap(style("1;"+col, r.g.Head[lvl-1])+body, w)
		if lvl <= 2 && r.cfg.HeadingRules {
			rule := strings.Repeat(r.g.Rule[lvl-1], max(w, 1))
			rcol := accDim
			if lvl == 2 {
				rcol = dim
			}
			ls = append(ls, rcol(rule))
		}
		return ls
	case *ast.Paragraph, *ast.TextBlock:
		return wrap(r.inline(n, ""), w)
	case *ast.Blockquote:
		inner := r.blocks(n, w-2, false)
		out := make([]string, len(inner))
		for i, l := range inner {
			out[i] = accDim(r.g.Quote) + " " + l
		}
		return out
	case *ast.List:
		return r.list(v, w, 0)
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		lang, code, _ := r.codeOf(n)
		hl, ok := r.hl[n]
		if !ok {
			hl = highlightLines(lang, code)
		}
		return r.code(lang, hl, w)
	case *ast.ThematicBreak:
		return []string{dim(strings.Repeat(r.g.HR, max(w, 1)))}
	case *east.Table:
		return r.table(v, w)
	case *ast.HTMLBlock:
		return nil
	}
	if n.HasChildren() {
		return r.blocks(n, w, false)
	}
	return nil
}

func (r *renderer) list(l *ast.List, w, depth int) []string {
	var out []string
	num := l.Start
	if num == 0 {
		num = 1
	}
	for item := l.FirstChild(); item != nil; item = item.NextSibling() {
		marker := r.g.Bullets[min(depth, 2)]
		if l.IsOrdered() {
			marker = strconv.Itoa(num) + "."
			num++
		}
		indent := ansi.StringWidth(marker) + 1
		var inner []string
		first, prevPara := true, false
		for c := item.FirstChild(); c != nil; c = c.NextSibling() {
			var ls []string
			if sub, ok := c.(*ast.List); ok {
				ls = r.list(sub, w-indent, depth+1)
			} else {
				ls = r.block(c, w-indent)
			}
			if len(ls) == 0 {
				continue
			}
			// compact: only separate two real paragraphs with a blank line
			if _, isPara := c.(*ast.Paragraph); !first && isPara && prevPara {
				inner = append(inner, "")
			}
			_, prevPara = c.(*ast.Paragraph)
			inner = append(inner, ls...)
			first = false
		}
		if len(inner) == 0 {
			inner = []string{""}
		}
		pad := strings.Repeat(" ", indent)
		for i, line := range inner {
			switch {
			case i == 0:
				out = append(out, acc(marker)+" "+line)
			case line == "":
				out = append(out, "")
			default:
				out = append(out, pad+line)
			}
		}
	}
	return out
}

// ---- code ----

func (r *renderer) code(lang string, hl []string, w int) []string {
	label := lang
	if label == "" {
		label = "text"
	}
	var out []string
	if r.cfg.CodeFrame {
		out = append(out, mTop+dim("╭─ ")+bold(label))
	}
	inner := max(w-2, 8)
	for _, ln := range hl {
		for i, seg := range strings.Split(ansi.Hardwrap(ln, inner, true), "\n") {
			gutter, mk := "│", mBody // continuations of a long line use a different stroke
			if i > 0 {
				gutter, mk = "┆", mCont
			}
			out = append(out, mk+dim(gutter)+" "+seg)
		}
	}
	if r.cfg.CodeFrame {
		out = append(out, mBottom+dim("╰─"))
	}
	return out
}

var (
	hlMu     sync.Mutex
	hlMemo   = map[string][]string{} // (lang, code) -> highlighted lines
	lexerMu  sync.Mutex
	lexerMap = map[string]chroma.Lexer{} // nil = does not exist (remembered: looking it up is expensive)
)

// lexerFor looks a lexer up once per name: the ones that do not exist
// (mermaid, text...) force a walk over every lexer with glob matching.
func lexerFor(lang string) chroma.Lexer {
	lexerMu.Lock()
	defer lexerMu.Unlock()
	if lx, ok := lexerMap[lang]; ok {
		return lx
	}
	lx := lexers.Get(lang)
	lexerMap[lang] = lx
	return lx
}

// highlightLines returns the highlighted code, one string per source line.
// The result is remembered: changing the width or the style does not tokenize again.
func highlightLines(lang, code string) []string {
	key := lang + "\x00" + code
	hlMu.Lock()
	if v, ok := hlMemo[key]; ok {
		hlMu.Unlock()
		return v
	}
	hlMu.Unlock()
	out := highlightUncached(lang, code)
	hlMu.Lock()
	hlMemo[key] = out
	hlMu.Unlock()
	return out
}

func highlightUncached(lang, code string) []string {
	code = strings.ReplaceAll(code, "\t", "    ")
	var lexer chroma.Lexer
	if lang != "" {
		lexer = lexerFor(lang)
	}
	if lexer == nil {
		return strings.Split(code, "\n")
	}
	// nil = chroma default options (State "root"); custom ones leave it empty
	it, err := lexer.Tokenise(nil, code)
	if err != nil {
		return strings.Split(code, "\n")
	}
	var lines []string
	var cur strings.Builder
	for _, t := range it.Tokens() {
		sgr := sgrFor(t.Type)
		for i, part := range strings.Split(t.Value, "\n") {
			if i > 0 {
				lines = append(lines, cur.String())
				cur.Reset()
			}
			if part == "" {
				continue
			}
			if sgr == "" {
				cur.WriteString(part)
			} else {
				cur.WriteString(style(sgr, part))
			}
		}
	}
	lines = append(lines, cur.String())
	for len(lines) > 1 && lines[len(lines)-1] == "" { // chroma adds a trailing newline
		lines = lines[:len(lines)-1]
	}
	return lines
}

func sgrFor(t chroma.TokenType) string {
	switch {
	case t.InCategory(chroma.Comment):
		return "90"
	case t == chroma.KeywordType:
		return "96"
	case t == chroma.KeywordConstant || t.InSubCategory(chroma.LiteralNumber):
		return "95"
	case t.InCategory(chroma.Keyword) || t == chroma.OperatorWord:
		return "94"
	case t == chroma.LiteralStringEscape || t == chroma.LiteralStringInterpol:
		return "93"
	case t.InCategory(chroma.LiteralString):
		return "33"
	case t == chroma.NameFunction || t == chroma.NameFunctionMagic || t == chroma.NameAttribute:
		return "32"
	case t == chroma.NameClass || t == chroma.NameBuiltin || t == chroma.NameBuiltinPseudo || t == chroma.NameNamespace:
		return "96"
	case t == chroma.NameConstant:
		return "95"
	case t == chroma.NameDecorator || t == chroma.NameTag:
		return "94"
	case t == chroma.GenericInserted:
		return "32"
	case t == chroma.GenericDeleted || t == chroma.Error:
		return "91"
	case t == chroma.GenericHeading || t == chroma.GenericSubheading || t == chroma.GenericStrong:
		return "1"
	case t == chroma.GenericPrompt || t == chroma.GenericOutput:
		return "90"
	}
	return ""
}

// ---- tablas ----

func (r *renderer) table(t *east.Table, w int) []string {
	var rows [][]string
	var header []bool
	for c := t.FirstChild(); c != nil; c = c.NextSibling() {
		var cells []string
		for cell := c.FirstChild(); cell != nil; cell = cell.NextSibling() {
			cells = append(cells, r.inline(cell, ""))
		}
		_, isHead := c.(*east.TableHeader)
		rows = append(rows, cells)
		header = append(header, isHead)
	}
	cols := 0
	for _, row := range rows {
		cols = max(cols, len(row))
	}
	if cols == 0 {
		return nil
	}
	nat := make([]int, cols)
	for _, row := range rows {
		for i, c := range row {
			nat[i] = max(nat[i], ansi.StringWidth(c))
		}
	}
	avail := w - (3*cols + 1) // bordes y relleno
	sum := 0
	for _, n := range nat {
		sum += n
	}
	widths := append([]int(nil), nat...)
	if sum > avail && avail > cols*4 {
		for i := range widths {
			widths[i] = max(4, nat[i]*avail/sum)
		}
	}
	line := func(l, m, r2 string) string {
		parts := make([]string, cols)
		for i, wd := range widths {
			parts[i] = strings.Repeat("─", wd+2)
		}
		return dim(l + strings.Join(parts, m) + r2)
	}
	bar := dim("│")
	out := []string{line("┌", "┬", "┐")}
	for ri, row := range rows {
		wrapped := make([][]string, cols)
		height := 1
		for i := 0; i < cols; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			if header[ri] {
				cell = accBold(ansi.Strip(cell))
			}
			wrapped[i] = wrapRaw(cell, widths[i])
			height = max(height, len(wrapped[i]))
		}
		for k := 0; k < height; k++ {
			var b strings.Builder
			b.WriteString(bar)
			for i := 0; i < cols; i++ {
				seg := ""
				if k < len(wrapped[i]) {
					seg = wrapped[i][k]
				}
				b.WriteString(" " + seg + strings.Repeat(" ", max(widths[i]-ansi.StringWidth(seg), 0)) + " " + bar)
			}
			out = append(out, b.String())
		}
		if header[ri] {
			out = append(out, line("├", "┼", "┤"))
		}
	}
	return append(out, line("└", "┴", "┘"))
}

// ---- inline text ----

type inl struct {
	b     strings.Builder
	stack []string
}

func (x *inl) push(code string) {
	x.stack = append(x.stack, code)
	x.b.WriteString("\x1b[" + code + "m")
}

// pop closes the current style and reopens the ones still active (so inline
// code inside a link does not leave the rest of the link uncolored).
func (x *inl) pop() {
	x.stack = x.stack[:len(x.stack)-1]
	x.b.WriteString("\x1b[0m")
	for _, c := range x.stack {
		x.b.WriteString("\x1b[" + c + "m")
	}
}

func (r *renderer) inline(n ast.Node, base string) string {
	x := &inl{}
	if base != "" {
		x.push(base)
	}
	r.walkInline(x, n)
	return x.b.String()
}

func (r *renderer) textOf(b []byte) string {
	return html.UnescapeString(string(util.UnescapePunctuations(b)))
}

func (r *renderer) walkInline(x *inl, n ast.Node) {
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *ast.Text:
			x.b.WriteString(r.textOf(v.Segment.Value(r.src)))
			if v.HardLineBreak() {
				x.b.WriteString("\n")
			} else if v.SoftLineBreak() {
				x.b.WriteString(" ")
			}
		case *ast.String:
			x.b.WriteString(r.textOf(v.Value))
		case *ast.CodeSpan:
			x.push("33")
			x.b.WriteString(r.textOf([]byte(nodeRaw(v, r.src))))
			x.pop()
		case *ast.Emphasis:
			if v.Level >= 2 {
				x.push("1")
			} else {
				x.push("3")
			}
			r.walkInline(x, v)
			x.pop()
		case *east.Strikethrough:
			x.push("9")
			r.walkInline(x, v)
			x.pop()
		case *east.TaskCheckBox:
			if v.IsChecked {
				x.b.WriteString(good("☑") + " ")
			} else {
				x.b.WriteString(dim("☐") + " ")
			}
		case *ast.Link:
			dest := string(v.Destination)
			id := -1
			if dest != "" {
				id = r.linkID(dest)
				x.b.WriteString(mLinkStart(id))
			}
			x.push("36;4")
			r.walkInline(x, v)
			x.pop()
			if id >= 0 {
				x.b.WriteString(mLinkEnd)
			}
			if dest == "" || strings.HasPrefix(dest, "#") {
				break
			}
			switch r.cfg.Links {
			case "footnotes":
				i, ok := r.linkIdx[dest]
				if !ok {
					r.links = append(r.links, dest)
					i = len(r.links) - 1
					r.linkIdx[dest] = i
				}
				x.b.WriteString(dim("[" + strconv.Itoa(i+1) + "]"))
			case "inline":
				x.b.WriteString(dim(" (" + dest + ")"))
			}
		case *ast.AutoLink:
			u := string(v.URL(r.src))
			x.b.WriteString(mLinkStart(r.linkID(u)))
			x.push("36;4")
			x.b.WriteString(u)
			x.pop()
			x.b.WriteString(mLinkEnd)
		case *ast.Image:
			x.b.WriteString(dim("[image: ") + nodeText(v, r.src) + dim("]"))
		case *ast.RawHTML:
			// stray HTML is ignored
		default:
			r.walkInline(x, c)
		}
	}
}

func nodeRaw(n ast.Node, src []byte) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			b.Write(t.Segment.Value(src))
		}
	}
	return b.String()
}

// ---- self-contained lines ----

var sgrRe = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

type sgrState struct {
	flags  map[int]bool // 1 bold, 2 faint, 3 italic, 4 underline, 7 reverse, 9 strikethrough
	fg, bg string
}

func (s *sgrState) apply(params string) {
	if params == "" {
		params = "0"
	}
	ps := strings.Split(params, ";")
	for i := 0; i < len(ps); i++ {
		n, _ := strconv.Atoi(ps[i])
		switch {
		case n == 0:
			s.flags, s.fg, s.bg = nil, "", ""
		case n == 1 || n == 2 || n == 3 || n == 4 || n == 7 || n == 9:
			if s.flags == nil {
				s.flags = map[int]bool{}
			}
			s.flags[n] = true
		case n == 22:
			delete(s.flags, 1)
			delete(s.flags, 2)
		case n == 23 || n == 24 || n == 27 || n == 29:
			delete(s.flags, n-20)
		case (n >= 30 && n <= 37) || (n >= 90 && n <= 97):
			s.fg = strconv.Itoa(n)
		case n == 39:
			s.fg = ""
		case (n >= 40 && n <= 47) || (n >= 100 && n <= 107):
			s.bg = strconv.Itoa(n)
		case n == 49:
			s.bg = ""
		case n == 38 || n == 48:
			take := 2 // 38;5;N
			if i+1 < len(ps) && ps[i+1] == "2" {
				take = 4 // 38;2;R;G;B
			}
			end := min(i+1+take, len(ps))
			seq := strings.Join(ps[i:end], ";")
			if n == 38 {
				s.fg = seq
			} else {
				s.bg = seq
			}
			i = end - 1
		}
	}
}

func (s *sgrState) open() string {
	var parts []string
	for _, f := range []int{1, 2, 3, 4, 7, 9} {
		if s.flags[f] {
			parts = append(parts, strconv.Itoa(f))
		}
	}
	if s.fg != "" {
		parts = append(parts, s.fg)
	}
	if s.bg != "" {
		parts = append(parts, s.bg)
	}
	if len(parts) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(parts, ";") + "m"
}

// selfContain makes every line reopen the style the previous one ended with
// and close itself, so each line can be painted on its own.
func selfContain(lines []string) []string {
	var st sgrState
	out := make([]string, len(lines))
	for i, l := range lines {
		prefix := st.open()
		for _, m := range sgrRe.FindAllStringSubmatch(l, -1) {
			st.apply(m[1])
		}
		suffix := ""
		if st.open() != "" {
			suffix = "\x1b[0m"
		}
		out[i] = prefix + l + suffix
	}
	return out
}
