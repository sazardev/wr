package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadWithCachesHTTPPages(t *testing.T) {
	isolate(t)
	old := cacheOn
	cacheOn = true
	t.Cleanup(func() { cacheOn = old })

	src := "https://a.test/article"
	body := "<html><head><title>T</title></head><body><p>hi there</p></body></html>"
	md, err := loadWith(src, func(got string) ([]byte, string, error) {
		if got != src {
			t.Errorf("fetcher called with %q", got)
		}
		return []byte(body), "text/html", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "hi there") {
		t.Errorf("markdown = %q", md)
	}
	cached, _, ok := cacheGet(src)
	if !ok || cached != md {
		t.Errorf("the page was not cached: ok=%v cached=%q", ok, cached)
	}
}

func TestLoadWithDoesNotCacheLocalFiles(t *testing.T) {
	isolate(t)
	old := cacheOn
	cacheOn = true
	t.Cleanup(func() { cacheOn = old })

	src := "notes.md"
	if _, err := loadWith(src, func(string) ([]byte, string, error) {
		return []byte("# local\n"), "text/markdown", nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := cacheGet(src); ok {
		t.Error("a local file was written to the URL cache")
	}
}

func TestFetchLocalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.md")
	if err := os.WriteFile(path, []byte("# hello\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, ctype, err := fetch(path)
	if err != nil {
		t.Fatal(err)
	}
	md, err := convert(body, path, ctype)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "# hello") {
		t.Errorf("markdown = %q", md)
	}
}

func TestFetchHTTP(t *testing.T) {
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		fmt.Fprint(w, "# served\n")
	}))
	defer srv.Close()

	body, ctype, err := fetch(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if ua != userAgent {
		t.Errorf("User-Agent = %q", ua)
	}
	if got := string(body); got != "# served\n" {
		t.Errorf("body = %q", got)
	}
	if mediaType(ctype) != "text/markdown" {
		t.Errorf("content type = %q", ctype)
	}
}

func TestFetchRejectsUnsupportedContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if _, _, err := fetch(srv.URL); err == nil || !strings.Contains(err.Error(), "image/png") {
		t.Errorf("err = %v, want an unsupported-type error", err)
	}
}

func TestFetchHTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	if _, _, err := fetch(srv.URL); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("err = %v, want an HTTP 404 error", err)
	}
}
