package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"
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
func load(src string) (string, error) { return loadFrom(src, fetchURL) }

// loadWith is load with the transport injected, so tests can serve a document
// without a network.
func loadWith(src string, get fetcher) (string, error) {
	return loadFrom(src, func(s string) ([]byte, string, string, error) {
		body, ctype, err := get(s)
		return body, ctype, s, err
	})
}

// finalFetcher is a fetcher that also reports the address the document really
// came from, which differs from the requested one after a redirect.
type finalFetcher func(src string) (body []byte, ctype, final string, err error)

func loadFrom(src string, get finalFetcher) (string, error) {
	body, ctype, final, err := get(src)
	if err != nil {
		return "", err
	}
	// links are resolved against where the page lives now, not where it was asked for
	md, err := convert(body, final, ctype)
	if err != nil {
		return "", err
	}
	if isShellPage(md) {
		// a script-drawn page may advertise a text version of itself
		if alt := alternateLink(body, final); alt != "" && alt != final {
			if b2, c2, f2, err := get(alt); err == nil {
				if md2, err := convert(b2, f2, c2); err == nil && bodyWords(md2) > bodyWords(md) {
					md = md2
				}
			}
		}
	}
	if isShellPage(md) && isHTTP(final) {
		// still bare: run its scripts in a headless browser, when there is one
		if md2, ok := renderedMarkdown(final, md); ok {
			md = md2
		}
	}
	if cacheOn && isHTTP(src) && !isShellPage(md) { // a bare shell is not kept: it may render once a browser exists
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
	body, ctype, _, err := fetchURL(src)
	return body, ctype, err
}

// statusHint explains an HTTP error in terms of what the reader can do.
func statusHint(code int) string {
	switch code {
	case 401, 403:
		return "refused: the site blocks non-browser clients or needs a login, or your network blocks it (press x to open it in your browser)"
	case 404, 410:
		return "the page does not exist at that address"
	case 429:
		return "too many requests: wait a moment and reload"
	case 451:
		return "not available for legal reasons"
	}
	if code >= 500 {
		return "the server is failing: try again later"
	}
	return ""
}

// acceptLanguage is built from the environment so sites serve the reader's
// language first and English as a fallback.
func acceptLanguage() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v, _, _ := strings.Cut(os.Getenv(k), ".")
		if v == "" || v == "C" || v == "POSIX" {
			continue
		}
		lang := strings.ReplaceAll(v, "_", "-")
		base, _, regional := strings.Cut(lang, "-")
		switch {
		case base == "en":
			return "en"
		case regional:
			return lang + ", " + base + ";q=0.9, en;q=0.6"
		}
		return lang + ", en;q=0.6"
	}
	return "en"
}

// fetchURL is fetch plus the final address after redirects.
func fetchURL(src string) ([]byte, string, string, error) {
	var r io.Reader
	var ctype string
	final := src
	if isHTTP(src) {
		req, _ := http.NewRequest("GET", src, nil)
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", acceptLanguage())
		resp, err := httpClient.Do(req)
		if err != nil && transient(err) {
			time.Sleep(400 * time.Millisecond) // a reset or a stall is often gone on the second try
			req2, _ := http.NewRequest("GET", src, nil)
			req2.Header = req.Header.Clone()
			resp, err = httpClient.Do(req2)
		}
		if err != nil {
			return nil, "", src, fmt.Errorf("could not download: %s", explainNetErr(err))
		}
		defer resp.Body.Close()
		if resp.Request != nil && resp.Request.URL != nil {
			final = resp.Request.URL.String()
		}
		if resp.StatusCode >= 400 {
			who := errorPageTitle(resp.Body)
			msg := fmt.Sprintf("HTTP %d", resp.StatusCode)
			if who != "" {
				msg += " (“" + who + "”)"
			}
			if hint := statusHint(resp.StatusCode); hint != "" {
				msg += ": " + hint
			}
			return nil, "", final, fmt.Errorf("could not download: %s", msg)
		}
		ctype = resp.Header.Get("Content-Type")
		if mt := mediaType(ctype); unsupportedType(mt) {
			return nil, ctype, final, fmt.Errorf("cannot show %s here (press x to open it in your browser)", mt)
		}
		r = io.LimitReader(resp.Body, maxBytes)
	} else {
		f, err := os.Open(localPath(src))
		if err != nil {
			return nil, "", src, err
		}
		defer f.Close()
		r = io.LimitReader(f, maxBytes)
	}
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, ctype, final, err
	}
	return toUTF8(raw, ctype), ctype, final, nil
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

// transient reports a network failure worth one more try: a timeout or a
// connection that was reset or cut. DNS failures, refused connections and TLS
// errors are not: retrying them only delays the answer.
func transient(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET)
}

// explainNetErr says what went wrong without the Go plumbing around it
// (`Get "https://…": dial tcp …`), for the failures people actually meet.
func explainNetErr(err error) string {
	var dns *net.DNSError
	var ne net.Error
	switch {
	case errors.As(err, &dns) && dns.IsNotFound:
		return "no such host: " + dns.Name + " (check the address)"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection refused (is the server running?)"
	case errors.As(err, &ne) && ne.Timeout():
		return "timed out (the site is slow or unreachable)"
	case strings.Contains(err.Error(), "certificate"):
		return "the TLS certificate is not valid: " + shortErr(err)
	}
	return shortErr(err)
}

// shortErr drops the `Get "url": ` prefix of a net/http error.
func shortErr(err error) string {
	var ue interface{ Unwrap() error }
	if errors.As(err, &ue) && ue.Unwrap() != nil && strings.HasPrefix(err.Error(), "Get ") {
		return ue.Unwrap().Error()
	}
	return err.Error()
}

// errorPageTitle reads the <title> of an error page (a proxy or a firewall
// answers 403 with a page that says who it is), from the first 64 KB.
func errorPageTitle(body io.Reader) string {
	raw, _ := io.ReadAll(io.LimitReader(body, 64<<10))
	lower := strings.ToLower(string(raw))
	i := strings.Index(lower, "<title")
	if i < 0 {
		return ""
	}
	start := strings.Index(lower[i:], ">")
	end := strings.Index(lower[i:], "</title>")
	if start < 0 || end < 0 || end < start {
		return ""
	}
	t := strings.Join(strings.Fields(sanitize(string(raw[i+start+1:i+end]))), " ")
	if len([]rune(t)) > 60 {
		t = string([]rune(t)[:60]) + "…"
	}
	return t
}
