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
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/PuerkitoBio/goquery"
	nethtml "golang.org/x/net/html"
	"golang.org/x/net/html/charset"
)

const (
	userAgent = "Mozilla/5.0 (X11; Linux x86_64) wr/2.0"
	maxBytes  = 8_000_000
)

var (
	// Elements that are chrome or controls, never reading content. A <form> is not
	// here on purpose: some sites wrap the whole page in one, so only its controls go.
	dropTags = "script,style,noscript,nav,aside,footer,svg,button,iframe,template,dialog,select,input,textarea,label"
	// Class conventions that name the language outright.
	langRes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:^|\s)(?:language|lang|highlight-source|brush)[-:_ ]+([\w+#.-]+)`),
		regexp.MustCompile(`(?i)(?:^|\s)sourceCode\s+([\w+#-]+)`),
	}
	// Looser conventions (Sphinx/Pygments highlight-python, MediaWiki
	// mw-highlight-lang-python): the match is only trusted when chroma has a
	// lexer for it, because a wrong label is worse than none.
	weakLangRes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:^|\s)mw-highlight-lang-([\w+#-]+)`),
		regexp.MustCompile(`(?i)(?:^|\s)highlight-([\w+#-]+)`),
	}
	codeJunk = regexp.MustCompile(`(?i)line-?no|linenum|gutter|copy|clipboard`)
	// name used on the page -> lexer name chroma understands
	langMap = map[string]string{
		"shell": "bash", "sh": "bash", "zsh": "bash", "console": "bash", "shellsession": "bash",
		"golang": "go", "js": "javascript", "ts": "typescript", "yml": "yaml", "py": "python",
		"docker": "dockerfile", "text": "", "plaintext": "", "txt": "",
		"ipython": "python", "ipython3": "python", "python3": "python", "py3": "python", "pycon": "python",
		"cs": "csharp", "nodejs": "javascript", "none": "", "default": "",
	}
	// Class words that mark page chrome. A class token is junk when it is one of
	// these or starts with one followed by - or _ (toc-wrapper, sidebar-left);
	// never when the word only ends it ("has-toc" is a layout flag, not a TOC).
	junkWords = []string{
		"toc", "table-of-contents", "breadcrumb", "breadcrumbs", "sidebar", "cookie", "cookies",
		"related", "newsletter", "social", "share", "sharing", "advert", "advertisement",
	}
	// ARIA landmarks that are chrome even when the element is a plain <div>.
	junkRoles = "[role=navigation],[role=banner],[role=contentinfo]"
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

var linkRe = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)

// stripLinks keeps only the link text (for --md -L).
func stripLinks(md string) string { return linkRe.ReplaceAllString(md, "$1") }

// load downloads, converts and (for a URL) stores the result in the cache.
func load(src string) (string, error) {
	body, ctype, err := fetch(src)
	if err != nil {
		return "", err
	}
	md, err := convert(body, src, ctype)
	if err != nil {
		return "", err
	}
	if cacheOn && isHTTP(src) {
		_ = cachePut(src, md) // the cache is optional: if it fails, nothing happens
	}
	return md, nil
}

// mediaType is the Content-Type without parameters, lowercased.
func mediaType(ctype string) string {
	mt, _, _ := strings.Cut(ctype, ";")
	return strings.ToLower(strings.TrimSpace(mt))
}

// unsupportedType reports content wr cannot show as text (images, PDFs, ...).
func unsupportedType(mt string) bool {
	switch {
	case mt == "", strings.HasPrefix(mt, "text/"), strings.Contains(mt, "json"), strings.Contains(mt, "xml"):
		return false
	}
	return true
}

// fence wraps text in a code fence long enough not to be closed by its content.
func fence(lang, text string) string {
	n := 3
	for _, run := range regexp.MustCompile("`+").FindAllString(text, -1) {
		n = max(n, len(run)+1)
	}
	f := strings.Repeat("`", n)
	return f + lang + "\n" + strings.TrimRight(text, "\n") + "\n" + f + "\n"
}

// convert turns a downloaded document into Markdown according to its type:
// HTML is extracted, Markdown is used as is, and plain text or JSON are shown
// in a code block.
func convert(body []byte, src, ctype string) (string, error) {
	mt := mediaType(ctype)
	ext := strings.ToLower(filepath.Ext(strings.TrimSuffix(strings.SplitN(src, "?", 2)[0], "/")))
	switch {
	case unsupportedType(mt):
		return "", fmt.Errorf("cannot show %s here (press x to open it in your browser)", mt)
	case mt == "text/markdown" || mt == "text/x-markdown" || ((mt == "text/plain" || mt == "") && (ext == ".md" || ext == ".markdown")):
		return sanitize(string(body)), nil
	case strings.Contains(mt, "json"):
		return sanitize(fence("json", string(body))), nil
	case mt == "text/plain" || (mt == "" && ext == ".txt"):
		return sanitize(fence("", string(body))), nil
	case strings.Contains(mt, "xml") && !strings.Contains(mt, "xhtml"):
		return sanitize(fence("xml", string(body))), nil
	}
	return toMarkdown(body, src)
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

// ---- download ----

func fetch(src string) ([]byte, string, error) {
	var r io.Reader
	var ctype string
	if isHTTP(src) {
		req, _ := http.NewRequest("GET", src, nil)
		req.Header.Set("User-Agent", userAgent)
		resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
		if err != nil {
			return nil, "", fmt.Errorf("could not download: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, "", fmt.Errorf("could not download: HTTP %d", resp.StatusCode)
		}
		ctype = resp.Header.Get("Content-Type")
		if mt := mediaType(ctype); unsupportedType(mt) {
			return nil, ctype, fmt.Errorf("cannot show %s here (press x to open it in your browser)", mt)
		}
		r = io.LimitReader(resp.Body, maxBytes)
	} else {
		f, err := os.Open(localPath(src))
		if err != nil {
			return nil, "", err
		}
		defer f.Close()
		r = io.LimitReader(f, maxBytes)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, ctype, err
	}
	return toUTF8(raw, ctype), ctype, nil
}

// toUTF8 converts a document to UTF-8 according to the header, the BOM or
// <meta charset>. When nothing declares the encoding, a document that is valid
// UTF-8 as a whole is taken as UTF-8: the detector only looks at the first KB,
// and falling back to windows-1252 for an accent further down produces mojibake.
func toUTF8(raw []byte, ctype string) []byte {
	enc, _, certain := charset.DetermineEncoding(raw[:min(len(raw), 1024)], ctype)
	if !certain && validUTF8(raw) {
		return raw
	}
	if out, err := enc.NewDecoder().Bytes(raw); err == nil {
		return out
	}
	return raw
}

// validUTF8 reports whether b is valid UTF-8, ignoring a multibyte character cut
// in half at the end (the download is capped at maxBytes).
func validUTF8(b []byte) bool {
	for cut := 0; cut < 4 && cut <= len(b); cut++ {
		if utf8.Valid(b[:len(b)-cut]) {
			return true
		}
	}
	return false
}

// ---- HTML -> Markdown ----

// langOf reads the language of a <pre> from its own attributes/classes, its
// <code> child or the three closest ancestors (Sphinx puts it on a wrapper div).
func langOf(pre *goquery.Selection) string {
	cands := []*goquery.Selection{pre, pre.Find("code").First()}
	for par, i := pre.Parent(), 0; i < 3 && par.Length() > 0; par, i = par.Parent(), i+1 {
		cands = append(cands, par)
	}
	for _, el := range cands {
		if el.Length() == 0 {
			continue
		}
		lang, _ := el.Attr("data-language")
		if lang == "" {
			lang, _ = el.Attr("data-lang")
		}
		cls, _ := el.Attr("class")
		if lang == "" {
			for _, rx := range langRes {
				if m := rx.FindStringSubmatch(cls); m != nil {
					lang = m[1]
					break
				}
			}
		}
		if lang != "" {
			lang = strings.ToLower(lang)
			if v, ok := langMap[lang]; ok {
				return v
			}
			return lang
		}
		for _, rx := range weakLangRes {
			if m := rx.FindStringSubmatch(cls); m != nil {
				name := strings.ToLower(m[1])
				if v, ok := langMap[name]; ok {
					if v != "" {
						return v
					}
					return "" // the page says "no language" (highlight-none, -text)
				}
				if lexerFor(name) != nil {
					return name
				}
			}
		}
	}
	return ""
}

func toMarkdown(page []byte, src string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(page))
	if err != nil {
		return "", err
	}
	title := strings.TrimSpace(doc.Find("title").First().Text())

	root, isArticle := pickRoot(doc)
	if root.Length() == 0 {
		return "", fmt.Errorf("the page has no content")
	}

	root.Find(dropTags).Remove()
	if !isArticle {
		dropHeaders(root)
	}
	dropChrome(root)
	root.Find("pre *").Each(func(_ int, s *goquery.Selection) {
		if cls, _ := s.Attr("class"); codeJunk.MatchString(cls) {
			s.Remove()
		}
	})
	// the language travels as class="language-xx", which is what the converter reads
	root.Find("pre").Each(func(_ int, pre *goquery.Selection) {
		if lang := langOf(pre); lang != "" {
			pre.SetAttr("class", "language-"+lang)
			pre.Find("code").First().SetAttr("class", "language-"+lang)
		}
		visibleEscapes(pre)
	})
	flattenTables(root)
	root.Find("img").Each(func(_ int, img *goquery.Selection) {
		alt := strings.TrimSpace(img.AttrOr("alt", ""))
		dup := alt != "" && title != "" && strings.Contains(strings.ToLower(title), strings.ToLower(alt))
		if alt == "" || dup {
			img.Remove()
			return
		}
		img.ReplaceWithNodes(&nethtml.Node{Type: nethtml.TextNode, Data: "[image: " + alt + "]"})
	})
	baseURL, _ := url.Parse(src)
	root.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		if baseURL != nil && baseURL.Scheme != "" {
			if ref, err := url.Parse(a.AttrOr("href", "")); err == nil {
				a.SetAttr("href", baseURL.ResolveReference(ref).String())
			}
		}
	})

	conv := converter.NewConverter(converter.WithPlugins(
		base.NewBasePlugin(),
		commonmark.NewCommonmarkPlugin(),
		strikethrough.NewStrikethroughPlugin(),
		table.NewTablePlugin(),
	))
	md, err := conv.ConvertNode(root.Nodes[0])
	if err != nil {
		return "", err
	}
	// the prose cleanup never touches code: fenced blocks are quoted data
	text := mapOutsideFences(string(md), tidyProse)
	// page content is untrusted: strip control characters (ESC, etc.) so a page
	// cannot manipulate your terminal.
	text = sanitize(text)
	text = strings.TrimSpace(text) + "\n"
	if title != "" && !hasTitleHeading(text) {
		text = "# " + title + "\n\n" + text
	}
	return text, nil
}

var (
	linkOnlyLine = regexp.MustCompile(`(?m)^(?:\[[^\]]*\]\([^)]*\)[ \t]*){4,}$`)
	trailingWS   = regexp.MustCompile(`[ \t]+\n`)
	blankRuns    = regexp.MustCompile(`\n{3,}`)
)

// tidyProse normalizes a stretch of Markdown that is not code.
func tidyProse(text string) string {
	text = linkOnlyLine.ReplaceAllString(text, "")
	text = trailingWS.ReplaceAllString(text, "\n")
	return blankRuns.ReplaceAllString(text, "\n\n")
}

var controlChars = regexp.MustCompile(`[\x00-\x08\x0b-\x1f\x7f-\x9f]`)

// sanitize strips control characters (ESC, etc.). It is also applied to what
// comes from the cache, in case the file was tampered with.
func sanitize(s string) string { return controlChars.ReplaceAllString(s, "") }

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
