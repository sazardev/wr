package main

// Style: the one place where ANSI sequences are written, so the vocabulary
// (accent, dim, bold, ...) is shared by the reader, the panels and the help.
// The colors come from theme.go, filled by setAccent from the configuration.

// style wraps s in raw SGR codes.
func style(code, s string) string { return "\x1b[" + code + "m" + s + "\x1b[0m" }

// acc is the accent foreground: keys, links, marks, frames.
func acc(s string) string { return style(accentFG, s) }

// accBold is the accent in bold: section titles and panel group headers.
func accBold(s string) string { return style("1;"+accentFG, s) }

// dim is secondary text: hints, captions, metadata.
func dim(s string) string { return style("90", s) }

// bold is emphasis the theme does not color.
func bold(s string) string { return style("1", s) }

// onAccent is text over the accent background, for selected rows.
func onAccent(s string) string { return style("30;"+accentBG, s) }

// cursor is the block cursor: a reverse-video space.
func cursor() string { return style("7", " ") }

// hint is a key followed by its dim description, as in the footer.
func hint(key, desc string) string { return acc(key) + " " + dim(desc) }
