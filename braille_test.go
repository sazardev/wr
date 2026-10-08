package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestScrollbarNilWhenEverythingFits(t *testing.T) {
	if scrollbar(20, 10, 20, 0) != nil || scrollbar(20, 20, 20, 0) != nil {
		t.Fatal("no bar when everything fits")
	}
}

func TestScrollbarThumbMovesAndStaysInside(t *testing.T) {
	const h, total, view = 20, 400, 20
	thumbCells := func(top float64) (first, last, n int) {
		first, last = -1, -1
		for i, c := range scrollbar(h, total, view, top) {
			if strings.Contains(c, "\x1b[94m") { // the thumb is bright blue
				if first < 0 {
					first = i
				}
				last = i
				n++
			}
		}
		return
	}
	f0, _, n0 := thumbCells(0)
	_, l1, _ := thumbCells(total - view)
	if f0 != 0 || n0 == 0 {
		t.Errorf("at the top the thumb must touch row 0 (first=%d n=%d)", f0, n0)
	}
	if l1 != h-1 {
		t.Errorf("at the bottom the thumb must touch the last row (last=%d)", l1)
	}
	prev := -1
	for top := 0; top <= total-view; top += 7 {
		f, _, _ := thumbCells(float64(top))
		if f < prev {
			t.Fatalf("thumb moved backwards at top=%d (%d < %d)", top, f, prev)
		}
		prev = f
	}
	for _, c := range scrollbar(h, total, view, 123) {
		if ansi.StringWidth(c) != 1 {
			t.Fatalf("every cell must be 1 column wide: %q", c)
		}
	}
}

func TestScrollbarSubCellPrecision(t *testing.T) {
	// h=10 rows, 1000 lines: each sub-row (1/4 cell) is ~25 lines. Moving 50
	// lines moves the thumb 2 sub-rows: the first cell ends up half full.
	a := scrollbar(10, 1000, 100, 0)
	b := scrollbar(10, 1000, 100, 50)
	if strings.Join(a, "") == strings.Join(b, "") {
		t.Fatal("scrolling 50 lines should be visible in the bar")
	}
	first := []rune(ansi.Strip(b[0]))[0]
	if first == 0x28FF || first == 0x2847 {
		t.Fatalf("the first cell should be partial, got %q", first)
	}
}

func TestScrollbarFollowsFractionalPosition(t *testing.T) {
	// during an animation the position is fractional: the bar must follow it
	a := strings.Join(scrollbar(10, 1000, 100, 100), "")
	b := strings.Join(scrollbar(10, 1000, 100, 112.5), "")
	if a == b {
		t.Fatal("a fractional move of half a sub-row or more should change the bar")
	}
}

func TestProgressBar(t *testing.T) {
	plain := func(p float64) string { return ansi.Strip(progressBar(8, p)) }
	if got := plain(0); got != strings.Repeat("⣀", 8) {
		t.Errorf("0%%: %q", got)
	}
	if got := plain(100); got != strings.Repeat("⣿", 8) {
		t.Errorf("100%%: %q", got)
	}
	if got := plain(50); !strings.HasPrefix(got, "⣿⣿⣿⣿") || !strings.HasSuffix(got, "⣀⣀⣀⣀") {
		t.Errorf("50%%: %q", got)
	}
	for p := 0.0; p <= 100; p += 0.5 {
		if w := ansi.StringWidth(progressBar(8, p)); w != 8 {
			t.Fatalf("%v%%: width %d", p, w)
		}
	}
	if progressBar(0, 50) != "" {
		t.Error("0 cells must give an empty string")
	}
}

func TestProgressBarIsMonotonic(t *testing.T) {
	fill := func(p float64) int {
		n := 0
		for _, r := range ansi.Strip(progressBar(10, p)) {
			n += strings.IndexRune(string(brailleFill), r)
		}
		return n
	}
	prev := -1
	for p := 0.0; p <= 100; p += 1 {
		got := fill(p)
		if got < prev {
			t.Fatalf("fill went down at %v%%", p)
		}
		prev = got
	}
}

func TestSpinnerCycles(t *testing.T) {
	if spinner(0) != spinner(len(spinnerFrames)) || spinner(0) == spinner(1) {
		t.Fatal("the spinner must cycle")
	}
}

func TestWaveWidthAndMotion(t *testing.T) {
	for _, cells := range []int{1, 8, 36} {
		if w := ansi.StringWidth(wave(cells, 1.234)); w != cells {
			t.Errorf("wave(%d) is %d wide", cells, w)
		}
	}
	if wave(20, 0) == wave(20, 0.2) {
		t.Error("the wave must move with time")
	}
	if wave(20, 0.5) != wave(20, 0.5) {
		t.Error("the wave must be deterministic for a given time")
	}
}
