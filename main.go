// wr: modo lector para terminal. Descarga una pagina, se queda con el articulo,
// lo convierte a Markdown y lo muestra con los 16 colores ANSI de la terminal
// (siguen tu tema), con los bloques de codigo coloreados por lenguaje.
//
//	wr URL           lee el articulo en un pager (q para salir)
//	wr -L URL        sin las URL de los enlaces (solo el texto)
//	wr --md URL      imprime el Markdown tal cual (para pipes)
//	wr --fresh URL   ignora la cache y descarga de nuevo
//	wr --clear-cache borra la cache
//	wr archivo.html  tambien funciona con un archivo local
//
// Al leer en la terminal, una pagina ya vista se abre al instante desde la
// cache (~/.cache/wr) y se refresca en segundo plano para la proxima vez.
package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/JohannesKaufmann/html-to-markdown/v2/converter"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/base"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/commonmark"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/strikethrough"
	"github.com/JohannesKaufmann/html-to-markdown/v2/plugin/table"
	"github.com/PuerkitoBio/goquery"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
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
	// nombre en la pagina -> nombre de lexer que chroma entiende
	langMap = map[string]string{
		"shell": "bash", "sh": "bash", "zsh": "bash", "console": "bash", "shellsession": "bash",
		"golang": "go", "js": "javascript", "ts": "typescript", "yml": "yaml", "py": "python",
		"docker": "dockerfile", "text": "", "plaintext": "", "txt": "",
	}
)

func main() {
	var src string
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
			fmt.Println("cache borrada")
			return
		case "-h", "--help":
			usage(os.Stdout)
			return
		default:
			if strings.HasPrefix(a, "-") && src == "" {
				fmt.Fprintf(os.Stderr, "wr: opcion desconocida: %s\n", a)
				usage(os.Stderr)
				os.Exit(2)
			}
			src = a
		}
	}
	if src == "" {
		usage(os.Stderr)
		os.Exit(2)
	}

	// La cache solo se lee en la vista interactiva: ahi el refresco en segundo
	// plano tiene tiempo de terminar mientras el pager sigue abierto. Con
	// --md o en un pipe siempre se descarga fresco (y se actualiza la cache).
	interactive := !rawMD && isTerminal(os.Stdout)
	if isHTTP(src) && interactive && !fresh {
		if md, saved, ok := cacheGet(src, noLinks); ok {
			go refresh(src, noLinks)
			show(cacheBanner(saved) + render(sanitize(md)))
			return
		}
	}

	md, err := load(src, noLinks)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wr: %v\n", err)
		os.Exit(1)
	}
	if rawMD {
		fmt.Print(md)
		return
	}
	show(render(md))
}

// load descarga, convierte y (si es una URL) guarda en la cache.
func load(src string, noLinks bool) (string, error) {
	body, err := fetch(src)
	if err != nil {
		return "", err
	}
	md, err := toMarkdown(body, src, noLinks)
	if err != nil {
		return "", err
	}
	if isHTTP(src) {
		_ = cachePut(src, noLinks, md) // la cache es optativa: si falla, no pasa nada
	}
	return md, nil
}

// refresh actualiza la cache en segundo plano; los errores se ignoran (si el
// proceso termina antes, el rename atomico evita dejar una entrada a medias).
func refresh(src string, noLinks bool) {
	_, _ = load(src, noLinks)
}

func cacheBanner(saved time.Time) string {
	return dim + "↺ desde cache (" + humanAge(time.Since(saved)) + ") · wr --fresh para recargar" + off + "\n\n"
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "uso: wr [-L] [--md] [--fresh] URL|archivo.html")
	fmt.Fprintln(w, "  -L, --no-links  sin las URL de los enlaces")
	fmt.Fprintln(w, "  --md            imprime el Markdown sin colorear (siempre descarga fresco)")
	fmt.Fprintln(w, "  --fresh         ignora la cache y descarga de nuevo")
	fmt.Fprintln(w, "  --clear-cache   borra la cache (~/.cache/wr)")
}

// ---- descarga ----

func fetch(src string) ([]byte, error) {
	var r io.Reader
	var ctype string
	if isHTTP(src) {
		req, _ := http.NewRequest("GET", src, nil)
		req.Header.Set("User-Agent", userAgent)
		resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
		if err != nil {
			return nil, fmt.Errorf("no se pudo descargar: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("no se pudo descargar: HTTP %d", resp.StatusCode)
		}
		r, ctype = io.LimitReader(resp.Body, maxBytes), resp.Header.Get("Content-Type")
	} else {
		f, err := os.Open(strings.TrimPrefix(src, "file://"))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = io.LimitReader(f, maxBytes)
	}
	// convierte a UTF-8 segun la cabecera / <meta charset>
	utf8r, err := charset.NewReader(r, ctype)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(utf8r)
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

func toMarkdown(page []byte, src string, noLinks bool) (string, error) {
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
		return "", fmt.Errorf("la pagina no tiene contenido")
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
	// el lenguaje viaja como class="language-xx", que es lo que lee el conversor
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
		img.ReplaceWithNodes(&nethtml.Node{Type: nethtml.TextNode, Data: "[imagen: " + alt + "]"})
	})
	baseURL, _ := url.Parse(src)
	root.Find("a[href]").Each(func(_ int, a *goquery.Selection) {
		if noLinks {
			a.ReplaceWithNodes(&nethtml.Node{Type: nethtml.TextNode, Data: a.Text()})
			return
		}
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
	// el contenido de la pagina es no confiable: fuera caracteres de control
	// (ESC, etc.) para que no puedan manipular tu terminal.
	text = sanitize(text)
	text = strings.TrimSpace(text) + "\n"
	if title != "" && !regexp.MustCompile(`(?m)^#\s`).MatchString(text) {
		text = "# " + title + "\n\n" + text
	}
	return text, nil
}

var controlChars = regexp.MustCompile(`[\x00-\x08\x0b-\x1f\x7f-\x9f]`)

// sanitize quita caracteres de control (ESC, etc.). Tambien se aplica a lo que
// viene de la cache, por si el archivo fue alterado.
func sanitize(s string) string { return controlChars.ReplaceAllString(s, "") }

// ---- Markdown -> ANSI ----

const (
	dim, bold, off = "\033[90m", "\033[1m", "\033[0m"
)

var (
	reFence  = regexp.MustCompile("^(`{3,}|~{3,})\\s*([\\w+#.-]*)")
	reHead   = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	reHR     = regexp.MustCompile(`^(?:(?:-\s*){3,}|(?:\*\s*){3,}|(?:_\s*){3,})$`)
	reList   = regexp.MustCompile(`^(\s*)([-*+]|\d+\.)(\s)`)
	reCode   = regexp.MustCompile("`+[^`]+`+")
	reBold   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reItal   = regexp.MustCompile(`(^|[^*\w])\*([^*\s][^*]*)\*`)
	reLink   = regexp.MustCompile(`\[([^\]]*)\]\(([^)\s]*)\)`)
	reUnesc  = regexp.MustCompile("\\\\([\\\\`*_{}\\[\\]()#+\\-.!|<>~\"])")
	reQuote  = regexp.MustCompile(`^>\s?`)
	headCols = []string{"1;96", "1;96", "1;94", "1;94", "1;92", "1;92"}
)

func render(md string) string {
	var out strings.Builder
	lines := strings.Split(strings.TrimRight(md, "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if m := reFence.FindStringSubmatch(line); m != nil {
			fence, lang := m[1], strings.ToLower(m[2])
			var code []string
			for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), fence); i++ {
				code = append(code, lines[i])
			}
			out.WriteString(codeBlock(lang, strings.Join(code, "\n")))
			out.WriteString("\n")
			continue
		}
		out.WriteString(prose(line))
		out.WriteString("\n")
	}
	return out.String()
}

func prose(line string) string {
	switch {
	case reHR.MatchString(line):
		return dim + strings.Repeat("─", 40) + off
	case reHead.MatchString(line):
		m := reHead.FindStringSubmatch(line)
		return "\033[" + headCols[len(m[1])-1] + "m" + m[1] + " " + inline(m[2], true) + off
	case reQuote.MatchString(line):
		return dim + "▎ " + off + inline(reQuote.ReplaceAllString(line, ""), false)
	case strings.HasPrefix(strings.TrimSpace(line), "|"):
		return strings.ReplaceAll(inline(line, false), "|", dim+"│"+off)
	case reList.MatchString(line):
		m := reList.FindStringSubmatch(line)
		return m[1] + "\033[33m" + m[2] + off + m[3] + inline(line[len(m[0]):], false)
	}
	return inline(line, false)
}

// inline aplica negritas, cursivas, codigo y enlaces; plain=true solo quita
// las marcas (para titulos, que ya van en negrita y color).
func inline(s string, plain bool) string {
	pick := func(styled, bare string) string {
		if plain {
			return bare
		}
		return styled
	}
	codeLocs := reCode.FindAllStringIndex(s, -1)
	var b strings.Builder
	last := 0
	flush := func(seg string) {
		seg = reLink.ReplaceAllString(seg, pick("\033[36;4m$1\033[39;24m"+dim+" ($2)\033[39m", "$1"))
		seg = reBold.ReplaceAllString(seg, pick("\033[1m$1\033[22m", "$1"))
		seg = reItal.ReplaceAllString(seg, pick("$1\033[3m$2\033[23m", "$1$2"))
		b.WriteString(reUnesc.ReplaceAllString(seg, "$1"))
	}
	for _, loc := range codeLocs {
		flush(s[last:loc[0]])
		code := strings.Trim(s[loc[0]:loc[1]], "`")
		b.WriteString(pick("\033[33m"+code+"\033[39m", code))
		last = loc[1]
	}
	flush(s[last:])
	return b.String()
}

// codeBlock colorea con chroma usando solo los 16 colores ANSI, asi que la
// terminal decide los tonos (gruvbox, nord, ...).
func codeBlock(lang, code string) string {
	gutter := dim + "│" + off + " "
	label := lang
	if label == "" {
		label = "texto"
	}
	var b strings.Builder
	b.WriteString(dim + "╭─ " + off + bold + label + off + "\n")
	b.WriteString(gutter)

	var lexer chroma.Lexer
	if lang != "" {
		lexer = lexers.Get(lang)
	}
	var toks []chroma.Token
	// nil = opciones por defecto de chroma (State "root"); unas propias lo dejan vacio
	if lexer != nil {
		if it, err := lexer.Tokenise(nil, code); err == nil {
			toks = it.Tokens()
		}
	}
	if toks == nil {
		toks = []chroma.Token{{Type: chroma.Text, Value: code}}
	}
	for _, t := range toks {
		sgr := sgrFor(t.Type)
		for i, part := range strings.Split(strings.TrimSuffix(t.Value, "\n"), "\n") {
			if i > 0 {
				b.WriteString("\n" + gutter)
			}
			if part == "" {
				continue
			}
			if sgr == "" {
				b.WriteString(part)
			} else {
				b.WriteString("\033[" + sgr + "m" + part + off)
			}
		}
		if strings.HasSuffix(t.Value, "\n") && t.Value != "" {
			b.WriteString("\n" + gutter)
		}
	}
	return strings.TrimSuffix(b.String(), "\n"+gutter) + "\n" + dim + "╰─" + off
}

func sgrFor(t chroma.TokenType) string {
	switch {
	case t.InCategory(chroma.Comment):
		return "90"
	case t == chroma.KeywordType:
		return "96"
	case t == chroma.KeywordConstant || t.InSubCategory(chroma.LiteralNumber):
		return "95"
	case t.InCategory(chroma.Keyword) || t == chroma.OperatorWord:
		return "94"
	case t == chroma.LiteralStringEscape || t == chroma.LiteralStringInterpol:
		return "93"
	case t.InCategory(chroma.LiteralString):
		return "33"
	case t == chroma.NameFunction || t == chroma.NameFunctionMagic || t == chroma.NameAttribute:
		return "32"
	case t == chroma.NameClass || t == chroma.NameBuiltin || t == chroma.NameBuiltinPseudo || t == chroma.NameNamespace:
		return "96"
	case t == chroma.NameConstant:
		return "95"
	case t == chroma.NameDecorator || t == chroma.NameTag:
		return "94"
	case t == chroma.GenericInserted:
		return "32"
	case t == chroma.GenericDeleted || t == chroma.Error:
		return "91"
	case t == chroma.GenericHeading || t == chroma.GenericSubheading || t == chroma.GenericStrong:
		return "1"
	case t == chroma.GenericPrompt || t == chroma.GenericOutput:
		return "90"
	}
	return ""
}

// ---- pager ----

func show(text string) {
	if isTerminal(os.Stdout) {
		if less, err := exec.LookPath("less"); err == nil {
			cmd := exec.Command(less, "-R", "-i", "-M")
			cmd.Stdin, cmd.Stdout, cmd.Stderr = strings.NewReader(text), os.Stdout, os.Stderr
			_ = cmd.Run() // salir con q antes del final no es un error
			return
		}
	}
	fmt.Print(text)
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
