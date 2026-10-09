package main

import (
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/PuerkitoBio/goquery"
	nethtml "golang.org/x/net/html"
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

var linkRe = regexp.MustCompile(`!?\[([^\]]*)\]\([^)]*\)`)

// stripLinks keeps only the link text (for --md -L).
func stripLinks(md string) string { return linkRe.ReplaceAllString(md, "$1") }

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
