// wr: a terminal reader mode. It fetches a page, extracts the article,
// converts it to Markdown and shows it in an interactive reader using the
// terminal's own 16-color ANSI palette (so it follows your theme), with code
// blocks highlighted per language.
//
//	wr URL           open the reader
//	wr -L URL        no links (text only)
//	wr --md URL      print the Markdown as is (for pipes)
//	wr --fresh URL   ignore the cache and download again
//	wr --clear-cache delete the cache
//	wr file.html     works with a local file too
//
// A page you already read opens instantly from the cache (~/.cache/wr) and is
// refreshed in the background for next time.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	o, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "wr: %v\n", err)
		usage(os.Stderr)
		os.Exit(2)
	}

	switch o.action {
	case actHelp:
		usage(os.Stdout)
		return
	case actVersion:
		fmt.Println(versionLine())
		return
	case actClear:
		if err := cacheClear(); err != nil {
			fmt.Fprintf(os.Stderr, "wr: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("cache cleared")
		return
	case actConfig:
		path, err := ensureConfigFile()
		if err != nil {
			fmt.Fprintf(os.Stderr, "wr: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(path)
		return
	}

	cfg, cfgErr := loadConfig()
	if o.noLinks {
		cfg.Links = "hidden"
	}
	applyRuntime(cfg)

	// What to open: an address, a file, search words, or (nothing) the homepage.
	input := strings.Join(o.words, " ")
	src, _ := normalizeInput(input, cfg.SearchEngine)
	if input == "" && cfg.Homepage != "" {
		src, _ = normalizeInput(cfg.Homepage, cfg.SearchEngine)
	}

	interactive := !o.rawMD && isTerminal(os.Stdout) && isTerminal(os.Stdin)
	if src == "" && !interactive {
		usage(os.Stderr)
		os.Exit(2)
	}

	// The interactive UI only runs on a terminal. With --md or in a pipe it
	// prints and exits (always downloading fresh).
	if interactive {
		if err := runTUI(src, cfg, o.fresh, cfgErr); err != nil {
			fmt.Fprintf(os.Stderr, "wr: %v\n", err)
			os.Exit(1)
		}
		return
	}

	md, err := load(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wr: %v\n", err)
		os.Exit(1)
	}
	if o.rawMD {
		if o.noLinks {
			md = stripLinks(md)
		}
		fmt.Print(md)
		return
	}
	width := cfg.Width
	if cols, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && cols > 20 && (width == 0 || cols < width) {
		width = cols
	}
	if width == 0 {
		width = 100
	}
	fmt.Println(strings.Join(renderDoc(md, width, cfg).Lines, "\n"))
}

// applyRuntime pushes the settings that live in package-level state.
func applyRuntime(cfg Config) {
	cacheMaxAge = time.Duration(cfg.CacheDays) * 24 * time.Hour
	cacheOn = cfg.Cache
	setAccent(cfg.Accent)
	setScrollSpeed(cfg.ScrollSpeed)
}
