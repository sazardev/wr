package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestFindMatchesSmartCase(t *testing.T) {
	plain := []string{"Hello world", "hello again", "nothing"}
	if got := findMatches(plain, "hello"); len(got) != 2 { // lowercase: case-insensitive
		t.Errorf("hello: %v", got)
	}
	if got := findMatches(plain, "Hello"); len(got) != 1 || got[0].line != 0 { // uppercase: exact
		t.Errorf("Hello: %v", got)
	}
	if findMatches(plain, "") != nil || findMatches(plain, "zzz") != nil {
		t.Error("an empty query or no results must give nil")
	}
	if got := findMatches([]string{"a.b a+b"}, "a.b"); len(got) != 1 { // literal, not a regexp
		t.Errorf("literal: %v", got)
	}
}

func TestFindMatchesRuneIndexes(t *testing.T) {
	got := findMatches([]string{"αβγδ αβγδ"}, "γδ")
	if len(got) != 2 || got[0].sp != (span{2, 4}) || got[1].sp != (span{7, 9}) {
		t.Fatalf("wrong rune indexes: %v", got)
	}
}

func TestHighlightLinePreservesTextAndColors(t *testing.T) {
	line := "\x1b[1mhell\x1b[0mo \x1b[94mworld\x1b[0m"
	out := highlightLine(line, []span{{2, 8}}, -1) // "llo wo"
	if ansi.Strip(out) != "hello world" {
		t.Fatalf("the text changed: %q", ansi.Strip(out))
	}
	if !strings.Contains(out, hlOpen) || !strings.Contains(out, hlClose) {
		t.Errorf("no highlight: %q", out)
	}
	if !strings.Contains(out, "\x1b[94m") || !strings.Contains(out, "\x1b[1m") {
		t.Errorf("the original colors were lost: %q", out)
	}
	if got := highlightLine(line, []span{{0, 4}}, 0); !strings.Contains(got, curOpen) {
		t.Errorf("the current match must use its own style: %q", got)
	}
	if highlightLine(line, nil, -1) != line {
		t.Error("without spans the line must come back untouched")
	}
}

func TestHighlightLineMultipleAndUnicode(t *testing.T) {
	out := highlightLine("αβγδ and αβγδ", []span{{2, 4}, {9, 11}}, 1)
	if ansi.Strip(out) != "αβγδ and αβγδ" {
		t.Fatalf("text: %q", ansi.Strip(out))
	}
	if strings.Count(out, hlOpen) != 1 || strings.Count(out, curOpen) != 1 {
		t.Errorf("expected 1 normal + 1 current: %q", out)
	}
}
