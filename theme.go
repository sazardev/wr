package main

// The palette: the one place where every color wr draws with is named.
// style.go writes the sequences; the configuration names the accent, and
// setAccent resolves it into the codes below.
//
// Two families, both usable as the accent:
//
//   - the terminal's own 16 ANSI colors (blue, cyan, green, magenta, yellow,
//     red, gray, white). Each one is stored twice: the bright half of the
//     palette (94) on a dark terminal, the plain half (34) on a light one,
//     which is the darker of the two and reads better on a light background.
//     The terminal supplies the actual RGB, so these follow your colorscheme.
//   - the 256-color presets (orange, violet, teal, pink, lime, slate): fixed
//     indexes, for a color your theme may not have. They look the same on
//     every background.
//
// The accent paints the UI chrome: panel frames and selections, shortcut keys,
// marks, the scrollbar thumb and the progress bar. accentAlt is its partner,
// for the secondary highlights where the accent is already spoken for: the
// light end of the footer rule, the page-reveal scan, the search count and the
// focused link. good, caution and fail are semantic: they keep their meaning
// whatever the accent is.

import "strconv"

// shade is one resolved set of SGR codes for a color: fg is the color itself,
// bg the background it takes when something is selected, dim its muted end of
// a gradient, and alt the partner color that sits beside it.
type shade struct{ fg, bg, dim, alt string }

// accentColor is one accent as the configuration names it. idx and altIdx are
// positions in the terminal's 16-color palette (-1 for a 256-color preset);
// they walk the document's heading colors.
type accentColor struct {
	idx, altIdx int
	dark, light shade
}

// hueOrder is the palette in the order the document's heading levels walk it:
// the accent, its partner, then the hues that are left.
var hueOrder = []int{4, 6, 2, 3, 5, 7, 1, 0} // blue, cyan, green, yellow, magenta, white, red, black

// sgr16 is one of the terminal's 16 colors: the bright half (90-97) on a dark
// background, the plain half (30-37) on a light one.
func sgr16(i int, bright bool) string {
	if bright {
		return strconv.Itoa(90 + i)
	}
	return strconv.Itoa(30 + i)
}

// ansi16 builds the two shades of one of the terminal's 16 colors. The
// selection background stays bright on both: black text over it reads well
// either way, and the gradient keeps a dark and a light end to run between.
// Black has no darker half to start that gradient from, so a gray accent
// starts from itself.
func ansi16(idx, altIdx int) accentColor {
	bg := strconv.Itoa(100 + idx)
	darkEnd := sgr16(idx, false)
	if idx == 0 {
		darkEnd = sgr16(idx, true)
	}
	return accentColor{idx, altIdx,
		shade{sgr16(idx, true), bg, darkEnd, sgr16(altIdx, true)},
		shade{sgr16(idx, false), bg, sgr16(idx, true), sgr16(altIdx, false)}}
}

// preset is a color from the 256-color palette: it does not follow the
// terminal's theme, so both of its shades are the same one.
func preset(fg, dim, alt int) accentColor {
	f, d, a := "38;5;"+strconv.Itoa(fg), "38;5;"+strconv.Itoa(dim), "38;5;"+strconv.Itoa(alt)
	sh := shade{f, "48;5;" + strconv.Itoa(fg), d, a}
	return accentColor{-1, -1, sh, sh}
}

var accents = map[string]accentColor{
	"blue":    ansi16(4, 6),
	"cyan":    ansi16(6, 4),
	"green":   ansi16(2, 3),
	"magenta": ansi16(5, 6),
	"yellow":  ansi16(3, 2),
	"red":     ansi16(1, 3),
	"gray":    ansi16(0, 7),
	"white":   ansi16(7, 6),

	"orange": preset(208, 130, 214),
	"violet": preset(135, 97, 141),
	"teal":   preset(37, 30, 44),
	"pink":   preset(205, 89, 211),
	"lime":   preset(118, 64, 154),
	"slate":  preset(246, 240, 250),
}

// accentNames is the order the Settings panel cycles them: the terminal's own
// colors first, then the presets that do not follow it.
var accentNames = []string{"blue", "cyan", "green", "magenta", "yellow", "red", "gray", "white",
	"orange", "violet", "teal", "pink", "lime", "slate"}

// The active palette (SGR codes). Set once from the configuration by setAccent,
// and again if the terminal turns out to have a light background.
var (
	accentFG, accentBG, accentDim = "94", "104", "34"
	accentAlt                     = "96"
	terminalDark                  = true // until the terminal says otherwise
)

// accentName is the accent the configuration asked for, kept so the palette can
// be resolved again when the terminal reports its background.
var accentName = "blue"

// dimFG is the fixed gray of secondary text: hints, captions, metadata.
const dimFG = "90"

// The semantic colors. They keep their meaning whatever the accent is.
const (
	colGood    = "92" // ✓ saved, reloaded, checked
	colCaution = "93" // ★ bookmark, a newer version, careful
	colFail    = "91" // ✗ could not open, no results
	colLink    = "36" // addresses: the footnote list at the end of a page
)

// headColor is the color of each heading level. Recomputed by setAccent, so
// the document follows the accent instead of a fixed rainbow.
var headColor = [6]string{"94", "96", "92", "93", "95", "97"}

func setAccent(name string) {
	a, ok := accents[name]
	if !ok {
		name, a = "blue", accents["blue"]
	}
	accentName = name
	sh := a.dark
	if !terminalDark {
		sh = a.light
	}
	accentFG, accentBG, accentDim, accentAlt = sh.fg, sh.bg, sh.dim, sh.alt
	headColor = headColors(a, sh)
}

// headColors walks the accent, its partner and then the hues that are left,
// skipping the two already used. A preset brings its own two colors and then
// borrows the terminal's.
func headColors(a accentColor, sh shade) [6]string {
	var out [6]string
	out[0], out[1] = sh.fg, sh.alt
	for n, i := 2, 0; n < len(out) && i < len(hueOrder); i++ {
		if a.idx >= 0 && (hueOrder[i] == a.idx || hueOrder[i] == a.altIdx) {
			continue
		}
		out[n] = sgr16(hueOrder[i], terminalDark)
		n++
	}
	return out
}

// setTerminalDark records what the terminal reported about its background and
// resolves the palette again: the same accent, on the half of the 16 colors
// that reads on that background. It says whether anything moved.
func setTerminalDark(dark bool) bool {
	if dark == terminalDark {
		return false
	}
	terminalDark = dark
	setAccent(accentName)
	return true
}
