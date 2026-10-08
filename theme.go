package main

// The accent color is used for the UI chrome: panel frames and selections,
// shortcut keys, marks, the scrollbar thumb and the progress bar. It is one of
// the 16 ANSI colors, so it still follows the terminal theme.

type accentColor struct{ fg, bg, dim string }

var accents = map[string]accentColor{
	"blue":    {"94", "104", "34"},
	"cyan":    {"96", "106", "36"},
	"green":   {"92", "102", "32"},
	"magenta": {"95", "105", "35"},
	"yellow":  {"93", "103", "33"},
	"red":     {"91", "101", "31"},
	"white":   {"97", "107", "37"},
}

var accentNames = []string{"blue", "cyan", "green", "magenta", "yellow", "red", "white"}

// The active accent (SGR codes). Set once from the configuration.
var accentFG, accentBG, accentDim = "94", "104", "34"

func setAccent(name string) {
	a, ok := accents[name]
	if !ok {
		a = accents["blue"]
	}
	accentFG, accentBG, accentDim = a.fg, a.bg, a.dim
}

// acc wraps s in the accent foreground.
func acc(s string) string { return "\x1b[" + accentFG + "m" + s + "\x1b[0m" }
