package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

const sample = "# Title **one**\n\nA paragraph with **bold**, *italic*, `code` and a [link](https://ex.com/a) and [another](https://ex.com/b).\n\n" +
	"## List\n\n- one\n  - nested\n- two with **force**\n\n1. first\n2. second\n\n> a quote\n\n" +
	"```go\npackage main\n\nfunc main() { println(\"hi\") }\n```\n\n| a | b |\n|---|---|\n| 1 | two |\n\n---\n\n- [x] done\n- [ ] pending\n"

func render(t *testing.T, md string, w int, mut func(*Config)) *Doc {
	t.Helper()
	cfg := defaultConfig()
	if mut != nil {
		mut(&cfg)
	}
	return renderDoc(md, w, cfg)
}

func TestMarkdownSyntaxDisappears(t *testing.T) {
	d := render(t, sample, 80, nil)
	all := strings.Join(d.Plain, "\n")
	for _, bad := range []string{"# ", "**", "`", "](", "- [x]", "|---|"} {
		if strings.Contains(all, bad) {
			t.Errorf("Markdown syntax %q left over in:\n%s", bad, all)
		}
	}
	for _, want := range []string{"Title one", "bold", "code", "link[1]", "another[2]", "◦ nested",
		"1. first", "▎ a quote", "╭─ go", "│ package main", "☑", "☐", "Links", "[1] https://ex.com/a"} {
		if !strings.Contains(all, want) {
			t.Errorf("missing %q in:\n%s", want, all)
		}
	}
}

func TestNoLineExceedsWidth(t *testing.T) {
	long := "# " + strings.Repeat("word ", 30) + "\n\n" + strings.Repeat("long text ", 60) + "\n\n" +
		"```\n" + strings.Repeat("x", 300) + "\n```\n\n| " + strings.Repeat("c", 80) + " | " + strings.Repeat("d", 80) + " |\n|---|---|\n| 1 | 2 |\n"
	for _, w := range []int{20, 40, 60, 100} {
		d := render(t, long, w, nil)
		for i, l := range d.Lines {
			if got := ansi.StringWidth(l); got > w {
				t.Fatalf("w=%d line %d is %d wide: %q", w, i, got, ansi.Strip(l))
			}
		}
	}
}

func TestLinesAreSelfContained(t *testing.T) {
	// a long link that wraps must not leave the underline open
	md := "[" + strings.Repeat("word ", 40) + "](https://ex.com)\n"
	d := render(t, md, 40, nil)
	if len(d.Lines) < 3 {
		t.Fatalf("expected several lines, got %d", len(d.Lines))
	}
	for i, l := range d.Lines {
		if strings.Contains(l, "\x1b[") && !strings.HasSuffix(l, "\x1b[0m") {
			t.Errorf("line %d leaves styles open: %q", i, l)
		}
	}
	if !strings.HasPrefix(d.Lines[1], "\x1b[") {
		t.Errorf("the continuation does not reopen the style: %q", d.Lines[1])
	}
}

func TestHeadsRecordLineAndLevel(t *testing.T) {
	d := render(t, "# A\n\ntext\n\n## B\n\ntext\n\n### C\n", 60, nil)
	if len(d.Heads) != 3 {
		t.Fatalf("heads=%v", d.Heads)
	}
	for i, want := range []struct {
		lvl  int
		text string
	}{{1, "A"}, {2, "B"}, {3, "C"}} {
		h := d.Heads[i]
		if h.Level != want.lvl || h.Text != want.text {
			t.Errorf("head %d: %+v", i, h)
		}
		if !strings.Contains(d.Plain[h.Line], want.text) {
			t.Errorf("head %d points at line %q", i, d.Plain[h.Line])
		}
	}
}

func TestLinksModes(t *testing.T) {
	md := "see [here](https://x.test/p) and [anchor](#sec)\n"
	foot := strings.Join(render(t, md, 80, nil).Plain, "\n")
	if !strings.Contains(foot, "here[1]") || !strings.Contains(foot, "[1] https://x.test/p") || strings.Contains(foot, "anchor[") {
		t.Errorf("footnotes:\n%s", foot)
	}
	inl := strings.Join(render(t, md, 80, func(c *Config) { c.Links = "inline" }).Plain, "\n")
	if !strings.Contains(inl, "here (https://x.test/p)") || strings.Contains(inl, "Links") {
		t.Errorf("inline:\n%s", inl)
	}
	hid := strings.Join(render(t, md, 80, func(c *Config) { c.Links = "hidden" }).Plain, "\n")
	if strings.Contains(hid, "x.test") || !strings.Contains(hid, "here and anchor") {
		t.Errorf("hidden:\n%s", hid)
	}
}

func TestBrailleOffUsesPlainGlyphs(t *testing.T) {
	d := render(t, "# T\n\ntext\n", 40, func(c *Config) { c.Braille = false })
	all := strings.Join(d.Plain, "\n")
	if strings.ContainsAny(all, "⣿⣀⠤⠶") {
		t.Errorf("braille present with braille=false:\n%s", all)
	}
	if !strings.Contains(all, "█ T") {
		t.Errorf("missing the plain mark:\n%s", all)
	}
}

func TestCodeBlockLabelAndHighlight(t *testing.T) {
	d := render(t, "```go\nfunc main() {}\n```\n\n```\nno language\n```\n", 60, nil)
	if !strings.Contains(d.Plain[0], "go") || !strings.Contains(strings.Join(d.Plain, "\n"), "╭─ text") {
		t.Errorf("labels:\n%s", strings.Join(d.Plain, "\n"))
	}
	if !strings.Contains(d.Lines[1], "\x1b[94m") { // `func` is a keyword: bright blue
		t.Errorf("syntax was not colored: %q", d.Lines[1])
	}
}

func TestHighlightIsMemoizedAndStable(t *testing.T) {
	a := highlightLines("go", "package main\nfunc main() {}")
	b := highlightLines("go", "package main\nfunc main() {}")
	if strings.Join(a, "\n") != strings.Join(b, "\n") {
		t.Fatal("highlighting must be deterministic")
	}
	if got := highlightLines("mermaid", "graph LR\n  A-->B"); len(got) != 2 || strings.Contains(got[0], "\x1b[") {
		t.Errorf("an unknown language must stay plain: %q", got)
	}
}

func TestEscapesAndEntities(t *testing.T) {
	d := render(t, "a \\* b &amp; c &lt;d&gt;\n", 60, nil)
	if got := d.Plain[0]; got != "a * b & c <d>" {
		t.Errorf("got %q", got)
	}
}

func TestSelfContainRestoresState(t *testing.T) {
	out := selfContain([]string{"\x1b[1;94mhello", "world\x1b[0m", "free"})
	if out[0] != "\x1b[1;94mhello\x1b[0m" {
		t.Errorf("l0 %q", out[0])
	}
	if out[1] != "\x1b[1;94mworld\x1b[0m" {
		t.Errorf("l1 %q", out[1])
	}
	if out[2] != "free" {
		t.Errorf("l2 %q", out[2])
	}
}

func TestRendererMetadataForSelection(t *testing.T) {
	md := "intro paragraph " + strings.Repeat("word ", 30) + "\n\n```go\nfunc main() {}\n" + strings.Repeat("x", 120) + "\n```\n"
	d := render(t, md, 40, nil)
	if len(d.Meta) != len(d.Lines) {
		t.Fatalf("meta has %d entries for %d lines", len(d.Meta), len(d.Lines))
	}
	for i, l := range d.Lines {
		if strings.Contains(l, "\x1b_") {
			t.Fatalf("an invisible marker leaked into the displayed line %d: %q", i, l)
		}
	}
	var kinds []LineKind
	soft := 0
	for _, m := range d.Meta {
		kinds = append(kinds, m.Kind)
		if m.Soft {
			soft++
		}
	}
	has := func(k LineKind) bool {
		for _, x := range kinds {
			if x == k {
				return true
			}
		}
		return false
	}
	if !has(kindCodeTop) || !has(kindCodeBody) || !has(kindCodeBottom) {
		t.Errorf("code frame lines not recorded: %v", kinds)
	}
	if soft < 3 {
		t.Errorf("wrapped lines must be marked soft (got %d)", soft)
	}
}

func TestMarkersAreZeroWidth(t *testing.T) {
	for _, mk := range allMarkers {
		if w := ansi.StringWidth("a" + mk + "b"); w != 2 {
			t.Errorf("marker %q counts as width %d", mk, w-2)
		}
		if got := ansi.Strip("a" + mk + "b"); got != "ab" {
			t.Errorf("marker %q not stripped: %q", mk, got)
		}
	}
}

func TestCodeGutterColumnIsRecorded(t *testing.T) {
	d := render(t, "> ```go\n> package main\n> ```\n", 60, nil)
	found := false
	for i, m := range d.Meta {
		if m.Kind == kindCodeBody {
			found = true
			if r := []rune(d.Plain[i]); string(r[m.Gutter:m.Gutter+1]) != "│" {
				t.Errorf("gutter column %d does not point at the gutter in %q", m.Gutter, d.Plain[i])
			}
		}
	}
	if !found {
		t.Fatal("no code body line recorded")
	}
}

func TestDocLinksRecordPositionAndDestination(t *testing.T) {
	d := render(t, "see [the docs](https://a.test/d) and [more](https://a.test/m) now\n", 80, nil)
	if len(d.Links) < 2 {
		t.Fatalf("links=%v urls=%v", d.Links, d.URLs)
	}
	l := d.Links[0]
	got := string([]rune(d.Plain[l.Line])[l.From:l.To])
	if got != "the docs" || d.URLs[l.ID] != "https://a.test/d" {
		t.Errorf("first link: %q -> %q", got, d.URLs[l.ID])
	}
	l2 := d.Links[1]
	if got := string([]rune(d.Plain[l2.Line])[l2.From:l2.To]); got != "more" || d.URLs[l2.ID] != "https://a.test/m" {
		t.Errorf("second link: %q -> %q", got, d.URLs[l2.ID])
	}
}

func TestWrappedLinkBecomesSeveralSpansOfOneLink(t *testing.T) {
	md := "[" + strings.Repeat("word ", 30) + "](https://a.test/long)\n"
	d := render(t, md, 30, func(c *Config) { c.Links = "hidden" })
	var spans []LinkSpan
	for _, l := range d.Links {
		if d.URLs[l.ID] == "https://a.test/long" {
			spans = append(spans, l)
		}
	}
	if len(spans) < 3 {
		t.Fatalf("a link wrapped over several lines needs a span per line: %v", spans)
	}
	for _, s := range spans {
		if s.ID != spans[0].ID || s.To <= s.From {
			t.Errorf("bad span %+v", s)
		}
	}
	// consecutive lines
	for i := 1; i < len(spans); i++ {
		if spans[i].Line != spans[i-1].Line+1 {
			t.Errorf("spans must be on consecutive lines: %v", spans)
		}
	}
}

func TestFootnoteListUrlsAreClickableToo(t *testing.T) {
	d := render(t, "go [here](https://a.test/p)\n", 80, nil) // footnotes mode
	n := 0
	for _, l := range d.Links {
		if d.URLs[l.ID] == "https://a.test/p" {
			n++
			if strings.HasPrefix(d.Plain[l.Line], "[1]") && string([]rune(d.Plain[l.Line])[l.From:l.To]) != "https://a.test/p" {
				t.Errorf("the footnote link must cover the URL: %q", d.Plain[l.Line])
			}
		}
	}
	if n != 2 {
		t.Errorf("expected the inline link and the footnote URL (2 spans), got %d", n)
	}
}

func TestLinksAreClickableInEveryLinkMode(t *testing.T) {
	for _, mode := range linkModes {
		d := render(t, "see [x](https://a.test/x)\n", 80, func(c *Config) { c.Links = mode })
		found := false
		for _, l := range d.Links {
			if d.URLs[l.ID] == "https://a.test/x" {
				found = true
			}
		}
		if !found {
			t.Errorf("mode %s: the link must stay clickable", mode)
		}
	}
}

func TestAnchorAndAutoLinksAreRecorded(t *testing.T) {
	d := render(t, "jump [there](#sec) or visit <https://auto.test/page>\n", 80, nil)
	var urls []string
	for _, l := range d.Links {
		urls = append(urls, d.URLs[l.ID])
	}
	joined := strings.Join(urls, " ")
	if !strings.Contains(joined, "#sec") || !strings.Contains(joined, "https://auto.test/page") {
		t.Errorf("urls: %v", urls)
	}
	if strings.Contains(strings.Join(d.Plain, "\n"), "there[") {
		t.Error("an in-page anchor gets no footnote number")
	}
}

func TestLinkMarkersNeverReachTheScreen(t *testing.T) {
	d := render(t, sample, 60, nil)
	for i, l := range d.Lines {
		if strings.Contains(l, "\x1b_") {
			t.Fatalf("an APC marker leaked into line %d: %q", i, l)
		}
	}
	// and the visible text is exactly what it was without link tracking
	if got := strings.Join(d.Plain, "\n"); !strings.Contains(got, "link[1]") {
		t.Errorf("text changed:\n%s", got)
	}
}
