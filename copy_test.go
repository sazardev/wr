package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestCopyCodeBlockShortcut(t *testing.T) {
	m, copied := selModel(t)
	top := lineOf(t, m, "╭─ go")
	m.y = top
	settle(m, m.copyCodeBlock())

	want := "func main() {\n    println(\"hello\")\n}"
	if len(*copied) == 0 || (*copied)[0] != want {
		t.Fatalf("copied %q, want %q", *copied, want)
	}
	if !strings.Contains(m.toast, "code block") || !strings.Contains(m.toast, "3 lines") {
		t.Errorf("toast=%q", m.toast)
	}
}

func TestCopyCodeBlockFindsABlockOnScreen(t *testing.T) {
	m, copied := selModel(t)
	m.y = 0 // the reading position is on the title; the block is visible below
	settle(m, m.copyCodeBlock())
	if len(*copied) == 0 || !strings.Contains((*copied)[0], "func main") {
		t.Fatalf("copied %q", *copied)
	}
}

func TestCopyCodeBlockWithoutOneExplains(t *testing.T) {
	m := newTestModel(t, 100, 30) // the fixture has no code
	settle(m, m.copyCodeBlock())
	if !m.toastWarn || !strings.Contains(m.toast, "no code block") {
		t.Errorf("toast=%q warn=%v", m.toast, m.toastWarn)
	}
}

func TestCopyCodeBlockRejoinsScreenWraps(t *testing.T) {
	m, _ := selModel(t)
	long := strings.Repeat("x", 120)
	m.md = "# T\n\n```go\n" + long + "\n```\n"
	m.rebuild()
	for i, mt := range m.doc.Meta {
		if mt.Kind != kindCodeTop {
			continue
		}
		from, to, ok := m.codeBlockAt(i)
		if !ok {
			t.Fatal("the block was not found")
		}
		if got, lines := m.codeText(from, to); got != long || lines != 1 {
			t.Fatalf("wrapped code copied as %q (%d lines)", got, lines)
		}
		return
	}
	t.Fatal("the code frame was not rendered")
}

func TestCopyPageAsMarkdown(t *testing.T) {
	m, copied := selModel(t)
	settle(m, m.copyPageMarkdown())
	if len(*copied) == 0 || (*copied)[0] != m.md {
		t.Errorf("copied %q, want the extracted page", *copied)
	}
	if !strings.Contains(m.toast, "page as Markdown") {
		t.Errorf("toast=%q", m.toast)
	}
}

func TestMenuShowsGroupsAndContextHelp(t *testing.T) {
	m := newTestModel(t, 110, 42)
	press(m, "m")
	v := ansi.Strip(m.render())
	for _, want := range []string{"Navigate", "This page", "wr", "Tools", "Type an address", "t.test", "Copy code block", "Copy page as Markdown"} {
		if !strings.Contains(v, want) {
			t.Errorf("the menu does not show %q:\n%s", want, v)
		}
	}
}
