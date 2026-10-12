//go:build !windows

package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeBrowser installs a script as the browser for the duration of a test.
func fakeBrowser(t *testing.T, script string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fakebrowser")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldPath, oldTimeout := browserPath, renderTimeout
	browserOnce.Do(func() {})
	browserPath = path
	t.Cleanup(func() { browserPath, renderTimeout = oldPath, oldTimeout })
}

func TestHeadlessDOMReturnsTheBrowsersDocument(t *testing.T) {
	fakeBrowser(t, `echo '<html><body><p>rendered by the fake</p></body></html>'`)
	dom, err := headlessDOM("https://t.test/")
	if err != nil || !strings.Contains(string(dom), "rendered by the fake") {
		t.Errorf("dom=%q err=%v", dom, err)
	}
	if renderingNow.Load() != 0 {
		t.Error("the in-flight counter must return to zero")
	}
}

func TestHeadlessDOMRejectsNonDocuments(t *testing.T) {
	fakeBrowser(t, `echo 'chrome crashed'`)
	if _, err := headlessDOM("https://t.test/"); err == nil {
		t.Error("output that is not a document must be an error")
	}
}

func TestHeadlessTimeoutKillsTheWholeProcessGroup(t *testing.T) {
	// the "browser" forks a child that would outlive it, like Chromium's renderers
	marker := fmt.Sprintf("%d", 40000+rand.Intn(9999)) // a sleep nobody else runs
	fakeBrowser(t, "sleep "+marker+" &\nsleep "+marker)
	renderTimeout = 400 * time.Millisecond
	start := time.Now()
	_, err := headlessDOM("https://t.test/")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("err = %v, want a timeout", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("the timeout took %v", d)
	}
	time.Sleep(200 * time.Millisecond)
	if out, _ := exec.Command("pgrep", "-f", "sleep "+marker).Output(); len(strings.TrimSpace(string(out))) > 0 {
		t.Errorf("a child of the browser survived the timeout: %s", out)
	}
}

func TestCappedBufferKeepsAtMostMax(t *testing.T) {
	b := &cappedBuffer{max: 5}
	n, err := b.Write([]byte("abcdefgh"))
	if n != 8 || err != nil || b.String() != "abcde" {
		t.Errorf("n=%d err=%v kept=%q", n, err, b.String())
	}
	b.Write([]byte("zzz"))
	if b.String() != "abcde" {
		t.Errorf("kept %q after the cap", b.String())
	}
}
