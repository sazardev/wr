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
	dropTags = "script,style,noscript,nav,aside,footer,form,svg,button,iframe,template,dialog,select,input"
	langRes  = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:^|\s)(?:language|lang|highlight-source|brush)[-:_ ]+([\w+#.-]+)`),
		regexp.MustCompile(`(?i)(?:^|\s)sourceCode\s+([\w+#-]+)`),
	}
	junkClass = regexp.MustCompile(`(?i)(?:^|[\s_-])(?:toc|table-of-contents|breadcrumbs?|sidebar|cookie\w*)(?:$|[\s_-])`)
	codeJunk  = regexp.MustCompile(`(?i)line-?no|linenum|gutter|copy|clipboard`)
	// name used on the page -> lexer name chroma understands
	langMap = map[string]string{
		"shell": "bash", "sh": "bash", "zsh": "bash", "console": "bash", "shellsession": "bash",
		"golang": "go", "js": "javascript", "ts": "typescript", "yml": "yaml", "py": "python",
		"docker": "dockerfile", "text": "", "plaintext": "", "txt": "",
	}
)

func main() {
	var words []string
	noLinks, rawMD, fresh := false, false, false
	for _, a := range os.Args[1:] {
		switch a {
		case "-L", "--no-links":
			noLinks = true
		case "--md":
			rawMD = true
		case "--fresh":
			fresh = true
		case "--clear-cache":
			if err := cacheClear(); err != nil {
				fmt.Fprintf(os.Stderr, "wr: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("cache cleared")
			return
		case "--config":
			path, err := ensureConfigFile()
			if err != nil {
				fmt.Fprintf(os.Stderr, "wr: %v\n", err)
				os.Exit(1)
			}
			fmt.Println(path)
			return
		case "-h", "--help":
			usage(os.Stdout)
			return
		case "-V", "--version":
			fmt.Println(versionLine())
			return
		default:
			if strings.HasPrefix(a, "-") && len(a) > 1 && len(words) == 0 {
				fmt.Fprintf(os.Stderr, "wr: unknown option: %s\n", a)
				usage(os.Stderr)
				os.Exit(2)
			}
			words = append(words, a)
		}
	}

	cfg, cfgErr := loadConfig()
	if noLinks {
		cfg.Links = "hidden"
	}
	applyRuntime(cfg)

	// What to open: an address, a file, search words, or (nothing) the homepage.
	input := strings.Join(words, " ")
	src, _ := normalizeInput(input, cfg.SearchEngine)
	if input == "" && cfg.Homepage != "" {
		src, _ = normalizeInput(cfg.Homepage, cfg.SearchEngine)
	}

	interactive := !rawMD && isTerminal(os.Stdout) && isTerminal(os.Stdin)
	if src == "" && !interactive {
		usage(os.Stderr)
		os.Exit(2)
	}

	// The interactive UI only runs on a terminal. With --md or in a pipe it
	// prints and exits (always downloading fresh).
	if interactive {
		if err := runTUI(src, cfg, fresh, cfgErr); err != nil {
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
	if rawMD {
		if noLinks {
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

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: wr [-L] [--md] [--fresh] [URL | file | search words...]")
	fmt.Fprintln(w, "  -L, --no-links  no links (text only)")
	fmt.Fprintln(w, "  --md            print the Markdown uncolored (always fetches fresh)")
	fmt.Fprintln(w, "  --fresh         ignore the cache and download again")
	fmt.Fprintln(w, "  --clear-cache   delete the cache (~/.cache/wr)")
	fmt.Fprintln(w, "  --config        create (if missing) and print the config file path")
	fmt.Fprintln(w, "  -V, --version   print the version")
	fmt.Fprintln(w, "With no argument wr opens its start page. Words that are not an address or a file")
	fmt.Fprintln(w, "are searched on the web. In the reader: o = open, m = menu, q = quit.")
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
	// convert to UTF-8 according to the header / <meta charset>
	utf8r, err := charset.NewReader(r, ctype)
	if err != nil {
		return nil, ctype, err
	}
	body, err := io.ReadAll(utf8r)
	return body, ctype, err
}

// ---- HTML -> Markdown ----

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
		if lang == "" {
			cls, _ := el.Attr("class")
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
	}
	return ""
}

func toMarkdown(page []byte, src string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(page))
	if err != nil {
		return "", err
	}
	title := strings.TrimSpace(doc.Find("title").First().Text())

	root := doc.Find("article").First()
	isArticle := root.Length() > 0
	if !isArticle {
		if root = doc.Find("main").First(); root.Length() == 0 {
			root = doc.Find("body").First()
		}
	}
	if root.Length() == 0 {
		return "", fmt.Errorf("the page has no content")
	}

	root.Find(dropTags).Remove()
	if !isArticle {
		root.Find("header").Remove()
	}
	root.Find("[class]").Each(func(_ int, s *goquery.Selection) {
		if cls, _ := s.Attr("class"); junkClass.MatchString(cls) {
			s.Remove()
		}
	})
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
	})
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
	text := string(md)
	text = regexp.MustCompile(`(?m)^(?:\[[^\]]*\]\([^)]*\)[ \t]*){4,}$`).ReplaceAllString(text, "")
	text = regexp.MustCompile(`[ \t]+\n`).ReplaceAllString(text, "\n")
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")
	// page content is untrusted: strip control characters (ESC, etc.) so a page
	// cannot manipulate your terminal.
	text = sanitize(text)
	text = strings.TrimSpace(text) + "\n"
	if title != "" && !regexp.MustCompile(`(?m)^#\s`).MatchString(text) {
		text = "# " + title + "\n\n" + text
	}
	return text, nil
}

var controlChars = regexp.MustCompile(`[\x00-\x08\x0b-\x1f\x7f-\x9f]`)

// sanitize strips control characters (ESC, etc.). It is also applied to what
// comes from the cache, in case the file was tampered with.
func sanitize(s string) string { return controlChars.ReplaceAllString(s, "") }

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
