package main

// Script-drawn pages (see shell.go) can only be read after their scripts run.
// wr does not ship a JavaScript engine, but when Chromium/Chrome is installed it
// can borrow one: the page is loaded in a headless instance (its own throwaway
// profile, the browser sandbox on), and the resulting DOM is extracted like any
// other HTML. It is only tried for pages that were detected as shells, so a
// normal page never pays for it.

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// jsRender is set from the configuration (render_js).
	jsRender = true
	// renderTimeout bounds the whole headless run.
	renderTimeout = 30 * time.Second
	// renderDOM is how a page is rendered; tests replace it.
	renderDOM = headlessDOM

	// renderingNow counts headless renders in flight, so the UI can say so.
	renderingNow atomic.Int32

	browserOnce sync.Once
	browserPath string
)

// browserCandidates are tried in order, by name on PATH. WR_BROWSER overrides.
var browserCandidates = []string{
	"chromium", "chromium-browser", "google-chrome", "google-chrome-stable",
	"chrome", "brave-browser", "microsoft-edge", "microsoft-edge-stable",
}

// findBrowser returns the headless-capable browser to use, or "".
func findBrowser() string {
	browserOnce.Do(func() {
		if p := os.Getenv("WR_BROWSER"); p != "" {
			if full, err := exec.LookPath(p); err == nil {
				browserPath = full
			}
			return
		}
		for _, name := range browserCandidates {
			if full, err := exec.LookPath(name); err == nil {
				browserPath = full
				return
			}
		}
	})
	return browserPath
}

// canRender reports whether script-drawn pages can be rendered here.
func canRender() bool { return jsRender && findBrowser() != "" }

// headlessDOM loads src in a headless browser and returns the DOM it ends up with.
func headlessDOM(src string) ([]byte, error) {
	bin := findBrowser()
	if bin == "" {
		return nil, errors.New("no browser found")
	}
	renderingNow.Add(1)
	defer renderingNow.Add(-1)
	profile, err := os.MkdirTemp("", "wr-render-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(profile)

	ctx, cancel := context.WithTimeout(context.Background(), renderTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin,
		"--headless=new", "--disable-gpu", "--user-data-dir="+profile,
		"--no-first-run", "--no-default-browser-check", "--disable-extensions",
		"--disable-background-networking", "--disable-sync", "--mute-audio",
		"--hide-scrollbars", "--disable-dev-shm-usage",
		"--user-agent="+userAgent,
		"--virtual-time-budget=10000",
		"--dump-dom", src)
	ownGroup(cmd)
	cmd.WaitDelay = 3 * time.Second // do not hang on a pipe a stray child still holds
	out := &cappedBuffer{max: maxBytes}
	cmd.Stdout = out
	if err := cmd.Run(); err != nil && out.Len() == 0 {
		if ctx.Err() != nil {
			return nil, errors.New("rendering timed out")
		}
		return nil, err
	}
	dom := out.Bytes()
	if !bytes.Contains(bytes.ToLower(dom[:min(len(dom), 4096)]), []byte("<html")) {
		return nil, errors.New("the browser returned no document")
	}
	return toUTF8(dom, "text/html; charset=utf-8"), nil
}

// renderedMarkdown renders a shell page with scripts and extracts it. It
// returns ok=false when it could not do better than the static extraction.
func renderedMarkdown(src string, current string) (string, bool) {
	if !jsRender || !isHTTP(src) || strings.HasPrefix(src, "file:") {
		return "", false
	}
	dom, err := renderDOM(src)
	if err != nil {
		return "", false
	}
	md, err := convert(dom, src, "text/html")
	// the static text includes the notice wr added; that is not page content
	if err != nil || bodyWords(md) <= bodyWords(current)-bodyWords(shellNotice()) {
		return "", false
	}
	return md, true
}

// cappedBuffer keeps at most max bytes and silently drops the rest, like the
// download cap, so a page that builds an enormous DOM cannot exhaust memory.
type cappedBuffer struct {
	bytes.Buffer
	max int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if room := c.max - c.Len(); room > 0 {
		c.Buffer.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}
