package main

// Style: the one place where ANSI sequences are written, so the vocabulary
// (accent, dim, caution, ...) is shared by the reader, the panels and the help.
// The colors come from theme.go, filled by setAccent from the configuration.

// style wraps s in raw SGR codes.
func style(code, s string) string { return "\x1b[" + code + "m" + s + "\x1b[0m" }

// acc is the accent foreground: keys, links, marks, frames.
func acc(s string) string { return style(accentFG, s) }

// acc2 is the accent's partner: the secondary highlights, where the accent is
// already spoken for (the light end of the footer rule, the search count, the
// focused link).
func acc2(s string) string { return style(accentAlt, s) }

// accBold is the accent in bold: section titles and panel group headers.
func accBold(s string) string { return style("1;"+accentFG, s) }

// accDim is the accent muted, for the quiet end of a gradient and the bars
// that only mark structure.
func accDim(s string) string { return style(accentDim, s) }

// dim is secondary text: hints, captions, metadata.
func dim(s string) string { return style(dimFG, s) }

// bold is emphasis the theme does not color.
func bold(s string) string { return style("1", s) }

// good is "all went well": saved, reloaded, checked.
func good(s string) string { return style(colGood, s) }

// caution is "look at this, but nothing broke": the bookmark star, a newer
// version, a notice that could not do what you asked.
func caution(s string) string { return style(colCaution, s) }

// fail is "something is wrong": the page could not be opened, no results.
func fail(s string) string { return style(colFail, s) }

// link is an address: the footnote list at the end of a page.
func link(s string) string { return style(colLink, s) }

// onAccent is text over the accent background, for selected rows.
func onAccent(s string) string { return style("30;"+accentBG, s) }

// onAccentSpan is the open/close pair for painting a range over the accent
// background without touching the colors the line already has.
func onAccentSpan() (open, close string) { return "\x1b[30;" + accentBG + "m", "\x1b[39;49m" }

// cursor is the block cursor: a reverse-video space.
func cursor() string { return style("7", " ") }

// hint is a key followed by its dim description, as in the footer.
func hint(key, desc string) string { return acc(key) + " " + dim(desc) }
