package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html/charset"
)

const (
	userAgent = "Mozilla/5.0 (X11; Linux x86_64) wr/2.0"
	maxBytes  = 8_000_000
)

// fetcher is how a document arrives: fetch in production, a stub in tests.
type fetcher func(src string) (body []byte, ctype string, err error)

// httpClient downloads pages. One client for the whole process, so connections
// to the same host are reused.
var httpClient = &http.Client{Timeout: 20 * time.Second}

// load downloads, converts and (for a URL) stores the result in the cache.
func load(src string) (string, error) { return loadWith(src, fetch) }

// loadWith is load with the transport injected, so tests can serve a document
// without a network.
func loadWith(src string, get fetcher) (string, error) {
	body, ctype, err := get(src)
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

func fetch(src string) ([]byte, string, error) {
	var r io.Reader
	var ctype string
	if isHTTP(src) {
		req, _ := http.NewRequest("GET", src, nil)
		req.Header.Set("User-Agent", userAgent)
		resp, err := httpClient.Do(req)
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
