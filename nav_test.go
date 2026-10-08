package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNormalizeInput(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "notes.md")
	_ = os.WriteFile(file, []byte("# hi"), 0o644)

	for _, c := range []struct {
		in         string
		wantPrefix string
		search     bool
	}{
		{"https://example.com/a?b=1", "https://example.com/a?b=1", false},
		{"http://localhost:8080", "http://localhost:8080", false},
		{"file:///tmp/x.html", "file:///tmp/x.html", false},
		{"example.com", "https://example.com", false},
		{"example.com/some/path?q=1", "https://example.com/some/path?q=1", false},
		{"docs.rs", "https://docs.rs", false},
		{"localhost:3000/app", "https://localhost:3000/app", false},
		{"127.0.0.1:8080", "https://127.0.0.1:8080", false},
		{file, "file://" + filepath.ToSlash(file), false},
		{"/etc/hosts", "file:///etc/hosts", false},
		{"rust async book", "https://html.duckduckgo.com/html/?q=rust+async+book", true},
		{"how does tcp work?", "https://html.duckduckgo.com/html/?q=how+does+tcp+work%3F", true},
		{"golang", "https://html.duckduckgo.com/html/?q=golang", true},
		{"a.b c", "https://html.duckduckgo.com/html/?q=a.b+c", true}, // spaces: it is a search, not a host
	} {
		got, search := normalizeInput(c.in, "duckduckgo")
		if got != c.wantPrefix || search != c.search {
			t.Errorf("%q -> (%q, %v), want (%q, %v)", c.in, got, search, c.wantPrefix, c.search)
		}
	}
	if got, _ := normalizeInput("   ", "duckduckgo"); got != "" {
		t.Errorf("blank input must give nothing: %q", got)
	}
}

func TestSearchEngines(t *testing.T) {
	if got := searchURL("wikipedia", "go lang"); got != "https://en.wikipedia.org/w/index.php?search=go+lang" {
		t.Errorf("wikipedia: %q", got)
	}
	if got := searchURL("https://example.com/s?term=%s&x=1", "a b"); got != "https://example.com/s?term=a+b&x=1" {
		t.Errorf("custom template: %q", got)
	}
	if got := searchURL("nonsense", "q"); !strings.Contains(got, "duckduckgo") {
		t.Errorf("an unknown engine must fall back to the default: %q", got)
	}
}

func TestLooksLikeHost(t *testing.T) {
	for s, want := range map[string]bool{
		"example.com": true, "a.b.co.uk/path": true, "localhost": true, "localhost:80": true,
		"1.2.3.4:8080": true, "foo": false, "foo bar.com": false, ".com": false, "a.": false,
		"hello world": false, "x.y:abc": false, "under_score.com": false,
	} {
		if got := looksLikeHost(s); got != want {
			t.Errorf("looksLikeHost(%q)=%v want %v", s, got, want)
		}
	}
}

func TestResolveLink(t *testing.T) {
	for _, c := range []struct {
		base, href, want string
		ok               bool
	}{
		{"https://a.test/dir/page.html", "other.html", "https://a.test/dir/other.html", true},
		{"https://a.test/dir/page.html", "/root", "https://a.test/root", true},
		{"https://a.test/dir/page.html", "../up", "https://a.test/up", true},
		{"https://a.test/dir/page.html", "//cdn.test/x", "https://cdn.test/x", true},
		{"https://a.test/dir/page.html", "https://b.test/y", "https://b.test/y", true},
		{"https://a.test/p", "#section", "#section", true},
		{"https://a.test/p", "page2#top", "https://a.test/page2#top", true},
		{"https://a.test/p", "mailto:me@x.test", "", false},
		{"https://a.test/p", "javascript:void(0)", "", false},
		{"https://a.test/p", "tel:123", "", false},
		{"https://a.test/p", "", "", false},
		{"", "https://start.test", "https://start.test", true},
		{"", "relative", "", false}, // the start page has no base to resolve against
		{"https://html.duckduckgo.com/html/", "//duckduckgo.com/l/?uddg=https%3A%2F%2Ftarget.test%2Fa%3Fb%3D1&rut=x", "https://target.test/a?b=1", true},
	} {
		got, ok := resolveLink(c.base, c.href)
		if got != c.want || ok != c.ok {
			t.Errorf("resolveLink(%q,%q) = (%q,%v) want (%q,%v)", c.base, c.href, got, ok, c.want, c.ok)
		}
	}
	// links from a local file resolve against that file
	dir := t.TempDir()
	got, ok := resolveLink(filepath.Join(dir, "a.html"), "b.html")
	if !ok || got != "file://"+filepath.ToSlash(filepath.Join(dir, "b.html")) {
		t.Errorf("local relative link: %q %v", got, ok)
	}
}

func TestSlugifyAndFragments(t *testing.T) {
	for in, want := range map[string]string{
		"Hello World": "hello-world", "  Spaces   everywhere ": "spaces-everywhere",
		"Step 1: Setup!": "step-1-setup", "snake_case_title": "snake-case-title", "Ünïcode ok": "ünïcode-ok",
	} {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q)=%q want %q", in, got, want)
		}
	}
	if p, f := splitFragment("https://a.test/p#sec"); p != "https://a.test/p" || f != "sec" {
		t.Errorf("%q %q", p, f)
	}
	if p, f := splitFragment("#only"); p != "" || f != "only" {
		t.Errorf("%q %q", p, f)
	}
}

func TestLocalPath(t *testing.T) {
	if got := localPath("file:///tmp/a%20b.html"); got != "/tmp/a b.html" {
		t.Errorf("got %q", got)
	}
	if got := localPath("/plain/path.html"); got != "/plain/path.html" {
		t.Errorf("got %q", got)
	}
}

func TestStartPageListsBookmarksAndRecentAsLinks(t *testing.T) {
	cfg := defaultConfig()
	md := startPageMD(&cfg, []Visit{{"https://a.test/x", "Bookmarked [one]", time.Now()}}, []Visit{{"https://b.test", "", time.Now()}})
	for _, want := range []string{"# wr", "## Bookmarks", `[Bookmarked \[one\]](<https://a.test/x>)`, "## Recent", "[https://b.test](<https://b.test>)", "`o`", "`m`"} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
	empty := startPageMD(&cfg, nil, nil)
	if !strings.Contains(empty, "Nothing here yet") || strings.Contains(empty, "## Recent") {
		t.Errorf("empty start page:\n%s", empty)
	}
	// and it renders into clickable links
	d := renderDoc(md, 80, cfg)
	if len(d.Links) < 2 {
		t.Errorf("the start page must contain clickable links, got %d", len(d.Links))
	}
}

// ---- persistent store ----

func stateEnv(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
}

func TestHistoryAddListAndDedupe(t *testing.T) {
	stateEnv(t)
	for _, v := range [][2]string{{"https://a.test", "A"}, {"https://b.test", "B"}, {"https://b.test", "B"}, {"https://a.test", "A again"}} {
		if err := historyAdd(v[0], v[1], 100); err != nil {
			t.Fatal(err)
		}
	}
	got := historyList()
	if len(got) != 2 || got[0].URL != "https://a.test" || got[0].Title != "A again" || got[1].URL != "https://b.test" {
		t.Fatalf("most recent first, one entry per page: %+v", got)
	}
	// visiting the same page twice in a row does not add a line, it only refreshes the title
	if n := len(readHistory()); n != 3 {
		t.Errorf("expected 3 raw entries, got %d", n)
	}
}

func TestHistoryDeleteClearAndLimit(t *testing.T) {
	stateEnv(t)
	for i := 0; i < 30; i++ {
		_ = historyAdd("https://t.test/"+string(rune('a'+i%26))+string(rune('a'+i/26)), "t", 5)
	}
	if n := len(readHistory()); n > 10 { // trimmed once it passes twice the limit
		t.Errorf("history grew past its limit: %d", n)
	}
	_ = historyAdd("https://keep.test", "k", 5)
	_ = historyAdd("https://drop.test", "d", 5)
	_ = historyDelete("https://drop.test")
	for _, v := range historyList() {
		if v.URL == "https://drop.test" {
			t.Error("delete did not remove the entry")
		}
	}
	if err := historyClear(); err != nil || len(historyList()) != 0 {
		t.Errorf("clear: %v %d", err, len(historyList()))
	}
	if err := historyClear(); err != nil {
		t.Errorf("clearing twice is not an error: %v", err)
	}
}

func TestStoreFilePermissions(t *testing.T) {
	stateEnv(t)
	_ = historyAdd("https://a.test", "A", 10)
	_, _ = bookmarkToggle("https://a.test", "A")
	dir, _ := stateDir()
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Errorf("dir perms %v", fi.Mode().Perm())
	}
	for _, name := range []string{"history.jsonl", "bookmarks.json"} {
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil || fi.Mode().Perm()&0o077 != 0 {
			t.Errorf("%s must be private: %v %v", name, err, fi)
		}
	}
}

func TestBookmarks(t *testing.T) {
	stateEnv(t)
	if isBookmarked("https://a.test") {
		t.Fatal("nothing is bookmarked yet")
	}
	added, err := bookmarkToggle("https://a.test", "A")
	if err != nil || !added || !isBookmarked("https://a.test") {
		t.Fatalf("add: %v %v", added, err)
	}
	_, _ = bookmarkToggle("https://b.test", "B")
	list := bookmarkList()
	if len(list) != 2 || list[0].URL != "https://b.test" { // newest first
		t.Errorf("order: %+v", list)
	}
	added, _ = bookmarkToggle("https://a.test", "A")
	if added || isBookmarked("https://a.test") {
		t.Error("toggling a bookmarked page must remove it")
	}
	_ = bookmarkDelete("https://b.test")
	if len(bookmarkList()) != 0 {
		t.Error("delete failed")
	}
}

func TestStoreSurvivesGarbage(t *testing.T) {
	stateEnv(t)
	dir, _ := stateDir()
	_ = os.MkdirAll(dir, 0o700)
	_ = os.WriteFile(filepath.Join(dir, "history.jsonl"), []byte("not json\n{\"u\":\"https://ok.test\",\"a\":1}\n{}\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "bookmarks.json"), []byte("{broken"), 0o600)
	if got := historyList(); len(got) != 1 || got[0].URL != "https://ok.test" {
		t.Errorf("bad lines must be skipped: %+v", got)
	}
	if len(bookmarkList()) != 0 {
		t.Error("a broken bookmarks file must read as empty")
	}
}

func TestHumanAge(t *testing.T) {
	for d, want := range map[time.Duration]string{
		5 * time.Second: "just now", 3 * time.Minute: "3 min ago", 5 * time.Hour: "5h ago", 72 * time.Hour: "3d ago",
	} {
		if got := humanAge(d); got != want {
			t.Errorf("%v: got %q want %q", d, got, want)
		}
	}
}

// ---- content types ----

func TestConvertByContentType(t *testing.T) {
	html, err := convert([]byte("<html><head><title>T</title></head><body><article><h1>Hi</h1><p>x</p></article></body></html>"), "https://a.test/", "text/html; charset=utf-8")
	if err != nil || !strings.Contains(html, "# Hi") {
		t.Errorf("html: %q %v", html, err)
	}
	md, _ := convert([]byte("# Raw\n\n- a\n"), "https://a.test/README.md", "text/plain")
	if md != "# Raw\n\n- a\n" {
		t.Errorf("a .md served as text must be used as Markdown: %q", md)
	}
	if md2, _ := convert([]byte("# Raw\n"), "https://a.test/x", "text/markdown"); md2 != "# Raw\n" {
		t.Errorf("markdown type: %q", md2)
	}
	txt, _ := convert([]byte("line1\nline2\n"), "https://a.test/log.txt", "text/plain")
	if txt != "```\nline1\nline2\n```\n" {
		t.Errorf("plain text goes in a code block: %q", txt)
	}
	js, _ := convert([]byte(`{"a":1}`), "https://a.test/api", "application/json")
	if !strings.HasPrefix(js, "```json\n") {
		t.Errorf("json: %q", js)
	}
	xml, _ := convert([]byte("<a/>"), "https://a.test/feed", "application/rss+xml")
	if !strings.HasPrefix(xml, "```xml\n") {
		t.Errorf("xml: %q", xml)
	}
	if _, err := convert([]byte("\x89PNG"), "https://a.test/i.png", "image/png"); err == nil || !strings.Contains(err.Error(), "image/png") {
		t.Errorf("an image must be reported, not shown: %v", err)
	}
	if _, err := convert(nil, "https://a.test/f.pdf", "application/pdf"); err == nil || !strings.Contains(err.Error(), "browser") {
		t.Errorf("a pdf must point to the browser: %v", err)
	}
	// a document containing code fences must not break out of its own fence
	nested, _ := convert([]byte("```\ncode\n```\n"), "https://a.test/n.txt", "text/plain")
	if !strings.HasPrefix(nested, "````\n") {
		t.Errorf("the fence must be longer than any run in the text: %q", nested)
	}
}

func TestFetchLocalFilesAndTypes(t *testing.T) {
	dir := t.TempDir()
	html := filepath.Join(dir, "p.html")
	_ = os.WriteFile(html, []byte("<article><h1>Local</h1></article>"), 0o644)
	body, ctype, err := fetch(html)
	if err != nil || ctype != "" || !strings.Contains(string(body), "Local") {
		t.Errorf("fetch: %q %q %v", body, ctype, err)
	}
	if md, err := load("file://" + filepath.ToSlash(html)); err != nil || !strings.Contains(md, "# Local") {
		t.Errorf("a file:// URL must load: %q %v", md, err)
	}
	if _, _, err := fetch(filepath.Join(dir, "missing.html")); err == nil {
		t.Error("a missing file must be an error")
	}
	notes := filepath.Join(dir, "n.md")
	_ = os.WriteFile(notes, []byte("# Notes\n"), 0o644)
	if md, _ := load(notes); md != "# Notes\n" {
		t.Errorf("a local .md is used as Markdown: %q", md)
	}
}
