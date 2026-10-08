package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

// ---- turning what you type into a destination ----

// engineTemplate returns the URL template (with %s) for a search engine setting.
func engineTemplate(engine string) string {
	if t, ok := searchPresets[engine]; ok {
		return t
	}
	if strings.Contains(engine, "%s") {
		return engine
	}
	return searchPresets["duckduckgo"]
}

func searchURL(engine, query string) string {
	return strings.Replace(engineTemplate(engine), "%s", url.QueryEscape(query), 1)
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// looksLikeHost: example.com, example.com/path, localhost:8080, 127.0.0.1:3000
func looksLikeHost(s string) bool {
	if strings.ContainsAny(s, " \t") {
		return false
	}
	host := s
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if host == "" {
		return false
	}
	if strings.EqualFold(strings.Split(host, ":")[0], "localhost") {
		return true
	}
	h, port, hasPort := strings.Cut(host, ":")
	if hasPort {
		for _, r := range port {
			if !unicode.IsDigit(r) {
				return false
			}
		}
		host = h
	}
	if !strings.Contains(host, ".") || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}
	for _, r := range host {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '.') {
			return false
		}
	}
	return true
}

// normalizeInput decides what an address-bar entry means: a URL, a local file
// or a web search. search reports whether the engine was used.
func normalizeInput(in, engine string) (target string, search bool) {
	s := strings.TrimSpace(in)
	if s == "" {
		return "", false
	}
	switch {
	case strings.HasPrefix(s, "http://"), strings.HasPrefix(s, "https://"), strings.HasPrefix(s, "file://"):
		return s, false
	case strings.HasPrefix(s, "~/"):
		if h, err := os.UserHomeDir(); err == nil {
			s = filepath.Join(h, s[2:])
		}
	}
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") || fileExists(s) {
		return fileURL(s).String(), false // local files are always file:// URLs
	}
	if looksLikeHost(s) {
		return "https://" + s, false
	}
	return searchURL(engine, s), true
}

// ---- links ----

func fileURL(path string) *url.URL {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return &url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
}

// resolveLink turns a link found on the page at `base` into something wr can
// open. ok is false for schemes it cannot show (mailto:, tel:, javascript:...).
func resolveLink(base, href string) (string, bool) {
	href = strings.TrimSpace(href)
	if href == "" {
		return "", false
	}
	if strings.HasPrefix(href, "#") {
		return href, true
	}
	ref, err := url.Parse(href)
	if err != nil {
		return "", false
	}
	if strings.HasPrefix(href, "/") && (base == "" || !(strings.HasPrefix(base, "http://") || strings.HasPrefix(base, "https://"))) {
		return fileURL(href).String(), true // an absolute path from an older history entry or the start page
	}
	var b *url.URL
	switch {
	case base == "":
		b = &url.URL{}
	case strings.HasPrefix(base, "http://"), strings.HasPrefix(base, "https://"), strings.HasPrefix(base, "file://"):
		if b, err = url.Parse(base); err != nil {
			return "", false
		}
	default:
		b = fileURL(base)
	}
	r := b.ResolveReference(ref)
	switch r.Scheme {
	case "http", "https":
		// Search engines wrap results in a redirect: go straight to the target.
		if strings.HasSuffix(r.Host, "duckduckgo.com") && r.Path == "/l/" {
			if t := r.Query().Get("uddg"); t != "" {
				return t, true
			}
		}
		return r.String(), true
	case "file":
		return r.String(), true
	}
	return "", false
}

// slugify matches a heading to a "#fragment" the way sites do (lowercase,
// words joined by dashes).
func slugify(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			dash = false
		case (r == ' ' || r == '-' || r == '_') && !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// splitFragment separates "page#section".
func splitFragment(u string) (page, frag string) {
	if i := strings.Index(u, "#"); i >= 0 {
		return u[:i], u[i+1:]
	}
	return u, ""
}

// localPath returns the filesystem path of a file source.
func localPath(src string) string {
	if strings.HasPrefix(src, "file://") {
		if u, err := url.Parse(src); err == nil {
			return u.Path
		}
		return strings.TrimPrefix(src, "file://")
	}
	return src
}

// ---- opening things outside wr ----

// openExternal hands a URL to the system's browser.
func openExternal(u string) error {
	var cmd *exec.Cmd
	switch {
	case isWSL():
		if p, err := exec.LookPath("wslview"); err == nil {
			cmd = exec.Command(p, u)
		} else if p := windowsExe("explorer.exe", "/mnt/c/Windows/explorer.exe"); p != "" {
			cmd = exec.Command(p, u)
		}
	default:
		for _, name := range []string{"xdg-open", "open", "gio"} {
			if p, err := exec.LookPath(name); err == nil {
				cmd = exec.Command(p, u)
				break
			}
		}
	}
	if cmd == nil {
		return fmt.Errorf("no way to open a browser was found")
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// ---- the start page ----

func mdEscape(s string) string {
	r := strings.NewReplacer("[", "\\[", "]", "\\]", "\\", "\\\\", "`", "\\`")
	return r.Replace(strings.Join(strings.Fields(s), " "))
}

func visitTitle(v Visit) string {
	if strings.TrimSpace(v.Title) != "" {
		return v.Title
	}
	return v.URL
}

// startPageMD builds the page shown when wr starts without an address: your
// bookmarks and recent pages, all of them real links.
func startPageMD(c *Config, bookmarks, recent []Visit) string {
	open, menu := keyHint(c, actOpen), keyHint(c, actMenu)
	var b strings.Builder
	b.WriteString("# wr\n\n")
	fmt.Fprintf(&b, "A reader mode for your terminal. Press `%s` to open an address or search the web, and `%s` for the menu.\n\n", open, menu)
	if len(bookmarks) > 0 {
		b.WriteString("## Bookmarks\n\n")
		for _, v := range bookmarks[:min(len(bookmarks), 12)] {
			fmt.Fprintf(&b, "- [%s](<%s>)\n", mdEscape(visitTitle(v)), v.URL)
		}
		b.WriteString("\n")
	}
	if len(recent) > 0 {
		b.WriteString("## Recent\n\n")
		for _, v := range recent[:min(len(recent), 15)] {
			fmt.Fprintf(&b, "- [%s](<%s>)\n", mdEscape(visitTitle(v)), v.URL)
		}
		b.WriteString("\n")
	}
	if len(bookmarks) == 0 && len(recent) == 0 {
		fmt.Fprintf(&b, "Nothing here yet. Open a page with `%s`; the pages you visit show up here.\n", open)
	}
	return b.String()
}
