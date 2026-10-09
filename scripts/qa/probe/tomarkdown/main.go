// Probe: instrumented replica of toMarkdown to attribute content loss to each
// removal rule. It replicates the v0.2.1 (fcb54cf) rules, before the QA fixes, so
// it keeps explaining the loss on the original corpus; it does not follow the
// current extraction in extract.go. Usage: go run ./scripts/qa/probe/tomarkdown page.html
package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	dropTags  = "script,style,noscript,nav,aside,footer,form,svg,button,iframe,template,dialog,select,input"
	junkClass = regexp.MustCompile(`(?i)(?:^|[\s_-])(?:toc|table-of-contents|breadcrumbs?|sidebar|cookie\w*)(?:$|[\s_-])`)
	codeJunk  = regexp.MustCompile(`(?i)line-?no|linenum|gutter|copy|clipboard`)
	wordRe    = regexp.MustCompile(`[^\W_]+`)
)

func words(s string) int { return len(wordRe.FindAllString(s, -1)) }

func main() {
	for _, path := range os.Args[1:] {
		b, err := os.ReadFile(path)
		if err != nil {
			fmt.Println(err)
			continue
		}
		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(b))
		if err != nil {
			fmt.Println(err)
			continue
		}
		fmt.Printf("\n########## %s\n", path)
		root := doc.Find("article").First()
		isArticle := root.Length() > 0
		name := "article"
		if !isArticle {
			if root = doc.Find("main").First(); root.Length() == 0 {
				root = doc.Find("body").First()
				name = "body"
			} else {
				name = "main"
			}
		}
		fmt.Printf("root=%s  words_before=%d\n", name, words(root.Text()))

		root.Find(dropTags).Remove()
		fmt.Printf("  after dropTags: %d words\n", words(root.Text()))
		if !isArticle {
			root.Find("header").Remove()
			fmt.Printf("  after header removal: %d words\n", words(root.Text()))
		}

		type hit struct {
			words int
			tag   string
			cls   string
			text  string
		}
		var hits []hit
		root.Find("[class]").Each(func(_ int, s *goquery.Selection) {
			if cls, _ := s.Attr("class"); junkClass.MatchString(cls) {
				tag := goquery.NodeName(s)
				text := strings.TrimSpace(s.Text())
				hits = append(hits, hit{words(text), tag, cls, text})
			}
		})
		sort.Slice(hits, func(i, j int) bool { return hits[i].words > hits[j].words })
		sum := 0
		for _, h := range hits {
			sum += h.words
		}
		fmt.Printf("  junkClass: %d elements, %d words inside\n", len(hits), sum)
		for i, h := range hits {
			if i >= 6 {
				break
			}
			t := h.text
			if len(t) > 90 {
				t = t[:90]
			}
			fmt.Printf("      <%s class=%q> %d words: %s\n", h.tag, h.cls, h.words, strings.ReplaceAll(t, "\n", " "))
		}
		root.Find("[class]").Each(func(_ int, s *goquery.Selection) {
			if cls, _ := s.Attr("class"); junkClass.MatchString(cls) {
				s.Remove()
			}
		})
		fmt.Printf("  after junkClass: %d words\n", words(root.Text()))

		var codeHits []hit
		root.Find("pre *").Each(func(_ int, s *goquery.Selection) {
			if cls, _ := s.Attr("class"); codeJunk.MatchString(cls) {
				tag := goquery.NodeName(s)
				codeHits = append(codeHits, hit{words(s.Text()), tag, cls, ""})
			}
		})
		for _, h := range codeHits {
			fmt.Printf("      codeJunk <%s class=%q> %d words\n", h.tag, h.cls, h.words)
		}
		root.Find("pre *").Each(func(_ int, s *goquery.Selection) {
			if cls, _ := s.Attr("class"); codeJunk.MatchString(cls) {
				s.Remove()
			}
		})
		fmt.Printf("  after codeJunk: %d words\n", words(root.Text()))
		h, _ := goquery.OuterHtml(root)
		fmt.Printf("  final HTML: %d bytes\n", len(h))
	}
}
