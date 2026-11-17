package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoleMainIsUsedAsTheArticleRoot(t *testing.T) {
	page := `<html><head><title>Doc</title></head><body>` +
		`<div class="sidebar">` + strings.Repeat("navigation noise ", 40) + `</div>` +
		`<div role="main"><h1>Doc</h1><p>` + strings.Repeat("real article words ", 40) + `</p></div>` +
		`</body></html>`
	md, err := toMarkdown([]byte(page), "https://t.test/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "real article words") || strings.Contains(md, "navigation noise") {
		t.Errorf("role=main was not the root:\n%s", md)
	}
}

func TestAriaHiddenDecorationIsDropped(t *testing.T) {
	page := `<html><head><title>Doc</title></head><body><article><h1>Doc</h1><p>` +
		strings.Repeat("visible reading words ", 30) +
		`</p><span aria-hidden="true">decorative icon</span><div aria-hidden="true">menu noise</div></article></body></html>`
	md, err := toMarkdown([]byte(page), "https://t.test/")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(md, "decorative icon") || strings.Contains(md, "menu noise") {
		t.Errorf("aria-hidden decoration leaked into the page:\n%s", md)
	}
	if !strings.Contains(md, "visible reading words") {
		t.Errorf("the article was dropped:\n%s", md)
	}
}

func TestShellPageTriesItsAmpVersion(t *testing.T) {
	isolate(t)
	const page = "https://t.test/article"
	shell := `<html><head><title>Shell</title><link rel="amphtml" href="/article/amp">` +
		`<script type="module" src="/app.js"></script></head><body><div id="root"></div></body></html>`
	amp := `<html><head><title>Shell</title></head><body><article><h1>Full AMP article</h1><p>` +
		strings.Repeat("The complete text of the article, server-rendered for AMP. ", 12) + `</p></article></body></html>`
	get := func(src string) ([]byte, string, error) {
		if strings.HasSuffix(src, "/amp") {
			return []byte(amp), "text/html; charset=utf-8", nil
		}
		return []byte(shell), "text/html; charset=utf-8", nil
	}
	md, err := loadWith(page, get)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "Full AMP article") || isShellPage(md) {
		t.Errorf("the AMP fallback was not used:\n%s", md)
	}
	if alt := alternateLink([]byte(shell), page); alt != "" {
		t.Errorf("amphtml must not be read as a text alternate: %q", alt)
	}
	if got := ampLink([]byte(shell), page); got != page+"/amp" {
		t.Errorf("ampLink = %q", got)
	}
}

func TestAmpFallbackDoesNotReplaceBetterStaticContent(t *testing.T) {
	isolate(t)
	const page = "https://t.test/normal"
	html := `<html><head><title>Normal</title><link rel="amphtml" href="/normal/amp"></head><body><article><h1>Normal</h1><p>` +
		strings.Repeat("The page already has its whole article here. ", 30) + `</p></article></body></html>`
	amp := `<html><head><title>Normal</title></head><body><article><h1>Short AMP</h1><p>Too little.</p></article></body></html>`
	get := func(src string) ([]byte, string, error) {
		if strings.HasSuffix(src, "/amp") {
			return []byte(amp), "text/html; charset=utf-8", nil
		}
		return []byte(html), "text/html; charset=utf-8", nil
	}
	md, err := loadWith(page, get)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "already has its whole article") || strings.Contains(md, "Short AMP") {
		t.Errorf("a good page must not be replaced by a thin AMP copy:\n%s", md)
	}
}

func TestHTTPStatusStillReachesTheReaderAfterAmpChanges(t *testing.T) {
	// guard the load path against the alternate branch swallowing an error
	isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "missing") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<html><head><title>T</title></head><body><article><h1>T</h1><p>` +
			strings.Repeat("words ", 30) + `</p></article></body></html>`))
	}))
	defer srv.Close()
	if _, err := load(srv.URL + "/missing"); err == nil {
		t.Fatal("a 404 must still be an error")
	}
}

func TestAlternateLinksRefuseNonHTTPTargets(t *testing.T) {
	page := []byte(`<html><head><link rel="amphtml" href="javascript:alert(1)">` +
		`<link rel="alternate" type="text/markdown" href="data:text/markdown,x"></head><body></body></html>`)
	if got := ampLink(page, "https://t.test/a"); got != "" {
		t.Errorf("ampLink followed %q", got)
	}
	if got := alternateLink(page, "https://t.test/a"); got != "" {
		t.Errorf("alternateLink followed %q", got)
	}
}
