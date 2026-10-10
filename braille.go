package main

import (
	"math"
	"strings"
)

// UI elements drawn with Braille characters (U+2800..U+28FF). A cell has
// 2 columns x 4 rows of dots, which gives 4x the vertical resolution of a
// normal block: the scrollbar moves smoothly even in a long document.
//
// Bit map of a braille character (row: left / right):
//
//	row 0: 0x01 / 0x08
//	row 1: 0x02 / 0x10
//	row 2: 0x04 / 0x20
//	row 3: 0x40 / 0x80

var brailleRow = [4]rune{0x01 | 0x08, 0x02 | 0x10, 0x04 | 0x20, 0x40 | 0x80}

// scrollbar returns the bar character for each of the h rows.
// total = document lines, view = visible lines, top = first visible line
// (fractional while a scroll animation is running).
// If everything fits it returns nil (no bar is drawn).
func scrollbar(h, total, view int, top float64) []string {
	if h <= 0 || total <= view {
		return nil
	}
	sub := h * 4 // available sub-rows
	thumb := max(4, sub*view/total)
	maxTop := float64(total - view)
	start := 0
	if maxTop > 0 {
		start = int(math.Round(float64(sub-thumb) * top / maxTop))
	}
	start = min(max(start, 0), sub-thumb)
	end := start + thumb

	out := make([]string, h)
	for r := range out {
		var bits rune
		for k := 0; k < 4; k++ {
			if s := r*4 + k; s >= start && s < end {
				bits |= brailleRow[k]
			}
		}
		if bits == 0 {
			out[r] = dim(string(rune(0x2800 | 0x47))) // faint track (left column: ⡇)
		} else {
			out[r] = acc(string(0x2800 + bits))
		}
	}
	return out
}

// Fill levels of a cell, from empty to full (bottom to top).
var brailleFill = []rune{'⣀', '⣄', '⣤', '⣦', '⣶', '⣷', '⣿'}

// progressBar draws a horizontal bar of `cells` cells filled to `pct` percent
// (0..100), with a partial fill in the last cell.
func progressBar(cells int, pct float64) string {
	if cells <= 0 {
		return ""
	}
	pct = math.Min(math.Max(pct, 0), 100)
	steps := len(brailleFill) - 1                              // 6 steps between empty and full
	units := int(math.Round(pct * float64(cells*steps) / 100)) // total fill units
	var b strings.Builder
	b.WriteString("\x1b[" + accentFG + "m")
	closed := false
	for i := 0; i < cells; i++ {
		u := units - i*steps
		switch {
		case u >= steps:
			b.WriteRune(brailleFill[steps])
		case u <= 0:
			if !closed {
				b.WriteString("\x1b[" + dimFG + "m")
				closed = true
			}
			b.WriteRune(brailleFill[0])
		default:
			b.WriteRune(brailleFill[u])
		}
	}
	b.WriteString("\x1b[0m")
	return b.String()
}

// Classic braille spinner (10 frames).
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func spinner(i int) string { return spinnerFrames[i%len(spinnerFrames)] }

// Colors (SGR) the loading wave travels through; all from the 16-color palette.
var wavePalette = []string{"34", "94", "96", "92", "96", "94"}

// wave draws the loading animation: a ripple of braille fill levels with a
// color gradient that travels along it. t is time in seconds.
func wave(cells int, t float64) string {
	var b strings.Builder
	for i := 0; i < cells; i++ {
		level := (math.Sin(t*7-float64(i)*0.5) + 1) / 2 // 0..1
		ch := brailleFill[int(math.Round(level*float64(len(brailleFill)-1)))]
		col := wavePalette[(i+int(t*9))%len(wavePalette)]
		b.WriteString("\x1b[" + col + "m")
		b.WriteRune(ch)
	}
	b.WriteString("\x1b[0m")
	return b.String()
}
