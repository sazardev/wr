package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// options is a parsed command line.
type options struct {
	words   []string // address, file or search words
	noLinks bool
	rawMD   bool
	fresh   bool
	action  string // a command that prints and exits (actHelp, ...); "" = open the reader
}

// Command flags (options.action). The first one on the line wins.
const (
	actHelp    = "help"
	actVersion = "version"
	actClear   = "clear-cache"
	actConfig  = "config"
)

// flagDef is one option: how it is written, what it does and how it is
// documented. Parsing and the help are both driven by this table, so adding a
// flag is one row and the two can never disagree.
type flagDef struct {
	short string // alias, e.g. "-L" ("" if there is none)
	long  string // e.g. "--no-links"
	desc  string
	apply func(*options)
}

var flagDefs = []flagDef{
	{"-L", "--no-links", "no links (text only)", func(o *options) { o.noLinks = true }},
	{"", "--md", "print the Markdown uncolored (always fetches fresh)", func(o *options) { o.rawMD = true }},
	{"", "--fresh", "ignore the cache and download again", func(o *options) { o.fresh = true }},
	{"", "--clear-cache", "delete the cache (~/.cache/wr)", func(o *options) { o.action = actClear }},
	{"", "--config", "create (if missing) and print the config file path", func(o *options) { o.action = actConfig }},
	{"-h", "--help", "show this help", func(o *options) { o.action = actHelp }},
	{"-V", "--version", "print the version", func(o *options) { o.action = actVersion }},
}

// parseArgs reads the command line. Flags come before the first word; after
// that every argument is a word. A command flag stops the scan there: it prints
// and exits, so what follows cannot matter.
func parseArgs(args []string) (options, error) {
	var o options
	for _, a := range args {
		if len(o.words) == 0 {
			if f := findFlag(a); f != nil {
				f.apply(&o)
				if o.action != "" {
					break
				}
				continue
			}
			if strings.HasPrefix(a, "-") && len(a) > 1 {
				return o, fmt.Errorf("unknown option: %s", a)
			}
		}
		o.words = append(o.words, a)
	}
	return o, nil
}

func findFlag(arg string) *flagDef {
	for i := range flagDefs {
		if f := &flagDefs[i]; arg == f.long || (f.short != "" && arg == f.short) {
			return f
		}
	}
	return nil
}

// usage prints the help: with the reader's look on a terminal, plain in a pipe.
func usage(w io.Writer) {
	tty := isTerminalWriter(w)
	if tty {
		cfg, _ := loadConfig()
		setAccent(cfg.Accent)
	}
	fmt.Fprint(w, helpText(tty))
}

// helpText builds the help. The flag rows and their column width come from
// flagDefs, so a new flag documents itself.
func helpText(tty bool) string {
	st := func(code, s string) string {
		if !tty {
			return s
		}
		return "\x1b[" + code + "m" + s + "\x1b[0m"
	}
	head := func(s string) string { return st("1;"+accentFG, s) }
	key := func(s string) string { return st(accentFG, s) }
	bold := func(s string) string { return st("1", s) }
	dim := func(s string) string { return st("90", s) }

	var b strings.Builder
	line := func(s string) { b.WriteString(s); b.WriteByte('\n') }
	blank := func() { line("") }

	brand := "wr " + dim("— a terminal reader")
	if tty {
		brand = key("⣿") + " " + bold("wr") + " " + dim("— a terminal reader")
		blank()
	}
	line("  " + brand)
	if tty {
		line("  " + key(strings.Repeat("⣀", utf8.RuneCountInString("⣿ wr — a terminal reader"))))
	}

	blank()
	line("  " + head("usage"))
	line("    " + bold("wr") + " [flags] [URL | file | search words…]")

	blank()
	line("  " + head("flags"))
	keyW := 0
	for _, f := range flagDefs {
		keyW = max(keyW, len(f.long)+4)
	}
	for _, f := range flagDefs {
		short := "    "
		if f.short != "" {
			short = f.short + ", "
		}
		line("    " + key(short+f.long) + strings.Repeat(" ", keyW-len(short)-len(f.long)+2) + dim(f.desc))
	}

	blank()
	line("  " + head("examples"))
	examples := []struct{ cmd, desc string }{
		{"wr example.com", "open the reader"},
		{"wr rust async book", "search the web"},
		{"wr notes.md", "a local file works too"},
	}
	cmdW := 0
	for _, e := range examples {
		cmdW = max(cmdW, len(e.cmd))
	}
	for _, e := range examples {
		line("    " + bold(e.cmd) + strings.Repeat(" ", cmdW-len(e.cmd)+2) + dim(e.desc))
	}

	blank()
	line("  With no argument wr opens its start page. Words that are not an address")
	line("  or a file are searched on the web. In the reader:")
	var keys []string
	for _, h := range [][2]string{{"o", "open"}, {"/", "search"}, {"m", "menu"}, {"q", "quit"}} {
		keys = append(keys, key(h[0])+" "+dim(h[1]))
	}
	line("    " + strings.Join(keys, dim(" · ")))

	return b.String()
}

// isTerminalWriter reports whether w is a terminal, for the help styling.
func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && isTerminal(f)
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
