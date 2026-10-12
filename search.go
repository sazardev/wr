package main

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// span is a [start, end) range of rune indexes inside the escape-free line.
type span [2]int

type match struct {
	line int
	sp   span
}

// compileQuery builds the search expression: literal, and case-insensitive
// unless the query contains an uppercase letter (smart-case, like vim).
func compileQuery(q string) *regexp.Regexp {
	if q == "" {
		return nil
	}
	pat := regexp.QuoteMeta(q)
	if strings.IndexFunc(q, unicode.IsUpper) < 0 {
		pat = "(?i)" + pat
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return nil
	}
	return re
}

// findMatches searches where q appears in every line, in order of appearance.
// The query is always a literal (it is quoted for the engine), so the common
// case is matched by hand over a small window of runes: the regexp engine cost
// nine tenths of a search over a long document. Smart-case applies, like vim:
// case-insensitive unless the query has an uppercase rune.
func findMatches(plain []string, q string) []match {
	if q == "" {
		return nil
	}
	fold := strings.IndexFunc(q, unicode.IsUpper) < 0
	qr := []rune(q)
	if len(qr) > maxLiteralQuery {
		return findMatchesRegexp(plain, q) // absurdly long query: keep the engine
	}
	var out []match
	var win [maxLiteralQuery]rune
	for li, line := range plain {
		n := 0    // runes in the window
		seen := 0 // runes consumed on this line
		for i := 0; i < len(line); {
			r, size := utf8.DecodeRuneInString(line[i:])
			if n == len(qr) { // the window is full: slide it
				copy(win[:], win[1:len(qr)])
			} else {
				n++
			}
			win[n-1] = r
			i += size
			seen++
			if n == len(qr) && equalFoldRunes(win[:n], qr, fold) {
				out = append(out, match{li, span{seen - n, seen}})
				n = 0 // matches do not overlap, like FindAllStringIndex
			}
		}
	}
	return out
}

const maxLiteralQuery = 64

// findMatchesRegexp is the engine path, kept for queries too long for the
// window (rare in practice: it is a search string typed by a person).
func findMatchesRegexp(plain []string, q string) []match {
	re := compileQuery(q)
	if re == nil {
		return nil
	}
	var out []match
	for i, l := range plain {
		for _, loc := range re.FindAllStringIndex(l, -1) {
			if loc[0] == loc[1] {
				continue
			}
			out = append(out, match{i, span{
				utf8.RuneCountInString(l[:loc[0]]),
				utf8.RuneCountInString(l[:loc[1]]),
			}})
		}
	}
	return out
}

// equalFoldRunes compares two equal-length rune slices, case-insensitively
// when fold is set.
func equalFoldRunes(a, b []rune, fold bool) bool {
	for i := range b {
		if fold {
			if unicode.ToLower(a[i]) != unicode.ToLower(b[i]) {
				return false
			}
		} else if a[i] != b[i] {
			return false
		}
	}
	return true
}

const (
	hlOpen   = "\x1b[7m" // reverse video: keeps the syntax colors
	hlClose  = "\x1b[27m"
	curOpen  = "\x1b[30;103m" // current match: black on yellow
	curClose = "\x1b[39;49m"
)

// markSpans wraps ranges (in visible rune indexes) of an ANSI line with the
// open/close sequences chosen per range, without altering the line's own colors.
func markSpans(line string, spans []span, styleOf func(i int) (open, closeSeq string)) string {
	if len(spans) == 0 {
		return line
	}
	var b strings.Builder
	vis, si := 0, 0
	inSpan := false
	var closeSeq string
	i := 0
	for i < len(line) {
		if line[i] == 0x1b && i+1 < len(line) && line[i+1] == '[' {
			j := i + 2
			for j < len(line) && !(line[j] >= '@' && line[j] <= '~') {
				j++
			}
			j = min(j+1, len(line))
			b.WriteString(line[i:j])
			i = j
			continue
		}
		_, size := utf8.DecodeRuneInString(line[i:])
		for !inSpan && si < len(spans) && vis == spans[si][0] && spans[si][1] <= spans[si][0] {
			si++ // empty range: nothing to mark
		}
		if !inSpan && si < len(spans) && vis == spans[si][0] {
			var open string
			open, closeSeq = styleOf(si)
			b.WriteString(open)
			inSpan = true
		}
		b.WriteString(line[i : i+size])
		vis++
		i += size
		if inSpan && vis == spans[si][1] {
			b.WriteString(closeSeq)
			inSpan = false
			si++
		}
	}
	if inSpan {
		b.WriteString(closeSeq)
	}
	return b.String()
}

// highlightLine highlights search matches. cur is the index of the "current"
// range (or -1).
func highlightLine(line string, spans []span, cur int) string {
	return markSpans(line, spans, func(i int) (string, string) {
		if i == cur {
			return curOpen, curClose
		}
		return hlOpen, hlClose
	})
}

// Selection style: black on bright cyan, distinct from search matches.
const (
	selOpen  = "\x1b[30;106m"
	selClose = "\x1b[39;49m"
)

// selectLine paints one selected range.
func selectLine(line string, sp span) string {
	return markSpans(line, []span{sp}, func(int) (string, string) { return selOpen, selClose })
}
