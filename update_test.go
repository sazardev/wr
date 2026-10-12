package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// withVersion pretends to be a given release for one test.
func withVersion(t *testing.T, v string) {
	t.Helper()
	old := version
	version = v
	t.Cleanup(func() { version = old })
}

func TestIsNewer(t *testing.T) {
	for _, c := range []struct {
		latest, current string
		want            bool
	}{
		{"v0.6.0", "v0.5.1", true},
		{"v0.5.1", "v0.5.1", false},
		{"v0.5.1", "v0.6.0", false},
		{"v1.0.0", "v0.9.9", true},
		{"v0.5.10", "v0.5.9", true}, // numeric, not lexical
		{"v0.6.0", "v0.5.2-0.20261012163403-9420c56ed71b+dirty", true},
		{"v0.5.2", "v0.5.2-0.20261012163403-9420c56ed71b", true}, // a build before the release
		{"v0.5.1", "v0.5.2-0.20261012163403-9420c56ed71b", false},
		{"v0.6.0", "dev", false},
		{"v0.6.0-rc1", "v0.5.1", false}, // never offer a pre-release
		{"nonsense", "v0.5.1", false},
	} {
		if got := isNewer(c.latest, c.current); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}

func TestShortVersion(t *testing.T) {
	for in, want := range map[string]string{
		"v0.6.0": "v0.6.0",
		"v0.5.2-0.20261012163403-9420c56ed71b+dirty": "v0.5.2-dev",
		"dev": "dev",
	} {
		withVersion(t, in)
		if got := shortVersion(); got != want {
			t.Errorf("shortVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

// releaseServer serves a release JSON and counts the requests.
func releaseServer(t *testing.T, rel releaseInfo, hits *atomic.Int32) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		json.NewEncoder(w).Encode(rel)
	}))
	t.Cleanup(srv.Close)
	old := releaseAPI
	releaseAPI = srv.URL
	t.Cleanup(func() { releaseAPI = old })
}

func TestCheckForUpdateCachesAndAnnouncesOnce(t *testing.T) {
	isolateAll(t)
	withVersion(t, "v0.5.1")
	var hits atomic.Int32
	releaseServer(t, releaseInfo{Tag: "v0.6.0", Body: "notes", URL: "https://x/r"}, &hits)

	res := checkForUpdate(false)
	if res.Err != nil || !res.Newer || !res.Announce || res.Latest.Tag != "v0.6.0" || hits.Load() != 1 {
		t.Fatalf("first check: %+v hits=%d", res, hits.Load())
	}
	markAnnounced("v0.6.0")
	res = checkForUpdate(false)
	if !res.Newer || res.Announce || hits.Load() != 1 {
		t.Errorf("second check must come from the cache and not announce again: %+v hits=%d", res, hits.Load())
	}
	if res = checkForUpdate(true); hits.Load() != 2 || !res.Newer {
		t.Errorf("force must ask again: hits=%d %+v", hits.Load(), res)
	}
}

func TestCheckForUpdateSurvivesAnOfflineServer(t *testing.T) {
	isolateAll(t)
	withVersion(t, "v0.5.1")
	old := releaseAPI
	releaseAPI = "http://127.0.0.1:1/nope"
	t.Cleanup(func() { releaseAPI = old })

	res := checkForUpdate(false)
	if res.Err == nil || res.Newer || res.Latest != nil {
		t.Errorf("offline with no memory: %+v", res)
	}
	// with a remembered release, an offline check still knows about it
	st := loadUpdateState()
	st.Latest, st.Checked, st.Failed = &releaseInfo{Tag: "v0.6.0"}, time.Now().Add(-24*time.Hour), time.Time{}
	st.save()
	res = checkForUpdate(false)
	if res.Err == nil || !res.Newer || res.Latest == nil {
		t.Errorf("offline with a remembered release: %+v", res)
	}
}

func TestFirstRunAfterAnUpdateCarriesItsNotes(t *testing.T) {
	isolateAll(t)
	withVersion(t, "v0.5.1")
	var hits atomic.Int32
	releaseServer(t, releaseInfo{Tag: "v0.6.0", Body: "## 0.6.0\n* new thing"}, &hits)
	checkForUpdate(true) // runs as v0.5.1: records Seen = v0.5.1

	withVersion(t, "v0.6.0") // the update was installed and wr restarted
	res := checkForUpdate(false)
	if res.JustUpdate == nil || res.JustUpdate.Tag != "v0.6.0" || res.Newer {
		t.Errorf("the first run after the update must carry its notes: %+v", res)
	}
	if res = checkForUpdate(false); res.JustUpdate != nil {
		t.Error("the notes must be offered only once")
	}
}

func TestUpdatesPossibleHonorsEveryOffSwitch(t *testing.T) {
	withVersion(t, "v0.5.1")
	old := updateEnabled
	t.Cleanup(func() { updateEnabled = old })
	updateEnabled = false
	if updatesPossible() {
		t.Error("tests, pipes and MCP must never check")
	}
	updateEnabled = true
	if !updatesPossible() {
		t.Error("the interactive reader on a release build should check")
	}
	t.Setenv("WR_NO_UPDATE_CHECK", "1")
	if updatesPossible() {
		t.Error("WR_NO_UPDATE_CHECK must turn it off")
	}
	t.Setenv("WR_NO_UPDATE_CHECK", "")
	withVersion(t, "dev")
	if updatesPossible() {
		t.Error("a build with no version has nothing to compare")
	}
}

// ---- applying ----

// fakeRelease builds an archive holding a script that reports version out, and
// serves it with its checksums. It returns the release to apply.
func fakeRelease(t *testing.T, tag, reports string, corrupt bool) *releaseInfo {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	script := "#!/bin/sh\necho 'wr " + reports + "'\n"
	tw.WriteHeader(&tar.Header{Name: "README.md", Mode: 0o644, Size: 2, Typeflag: tar.TypeReg})
	tw.Write([]byte("hi"))
	tw.WriteHeader(&tar.Header{Name: "wr", Mode: 0o755, Size: int64(len(script)), Typeflag: tar.TypeReg})
	tw.Write([]byte(script))
	tw.Close()
	gz.Close()
	archive := buf.Bytes()
	sum := sha256.Sum256(archive)
	hexsum := hex.EncodeToString(sum[:])
	if corrupt {
		hexsum = strings.Repeat("0", 64)
	}
	name := platformAsset()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, name):
			w.Write(archive)
		case strings.HasSuffix(r.URL.Path, "checksums.txt"):
			fmt.Fprintf(w, "%s  %s\n%s  other.tar.gz\n", hexsum, name, strings.Repeat("1", 64))
		}
	}))
	t.Cleanup(srv.Close)
	oldPrefix := assetPrefix
	assetPrefix = srv.URL + "/"
	t.Cleanup(func() { assetPrefix = oldPrefix })
	rel := &releaseInfo{Tag: tag}
	rel.Assets = append(rel.Assets,
		struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{name, srv.URL + "/" + name},
		struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{"checksums.txt", srv.URL + "/checksums.txt"})
	return rel
}

func target(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "wr")
	if err := os.WriteFile(p, []byte("OLD BINARY"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestApplyUpdateReplacesTheBinary(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("release builds exist for linux and darwin")
	}
	rel := fakeRelease(t, "v9.9.9", "v9.9.9", false)
	p := target(t)
	var steps []string
	if err := applyUpdateTo(rel, p, func(s string) { steps = append(steps, s) }); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "wr v9.9.9") {
		t.Errorf("the binary was not replaced: %q", b)
	}
	if st, _ := os.Stat(p); st.Mode().Perm()&0o111 == 0 {
		t.Error("the new binary must be executable")
	}
	if len(steps) < 3 {
		t.Errorf("progress steps missing: %v", steps)
	}
	if left, _ := filepath.Glob(filepath.Join(filepath.Dir(p), ".wr-update-*")); len(left) != 0 {
		t.Errorf("temp files left behind: %v", left)
	}
}

func TestApplyUpdateRefusesAnythingSuspicious(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("release builds exist for linux and darwin")
	}
	for name, c := range map[string]struct {
		build func() *releaseInfo
		want  string
	}{
		"corrupt download":     {func() *releaseInfo { return fakeRelease(t, "v9.9.9", "v9.9.9", true) }, "checksum mismatch"},
		"wrong version inside": {func() *releaseInfo { return fakeRelease(t, "v9.9.9", "v1.0.0", false) }, "reports"},
	} {
		p := target(t)
		err := applyUpdateTo(c.build(), p, func(string) {})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want it to mention %q", name, err, c.want)
		}
		if b, _ := os.ReadFile(p); string(b) != "OLD BINARY" {
			t.Errorf("%s: the installed binary must stay untouched, got %q", name, b)
		}
	}

	// an asset outside the project's release URL is never downloaded
	rel := fakeRelease(t, "v9.9.9", "v9.9.9", false)
	rel.Assets[0].URL = "https://evil.example/wr.tar.gz"
	p := target(t)
	if err := applyUpdateTo(rel, p, func(string) {}); err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Errorf("a foreign URL must be refused: %v", err)
	}

	// a release without a build for this system
	if err := applyUpdateTo(&releaseInfo{Tag: "v9.9.9"}, p, func(string) {}); err == nil || !strings.Contains(err.Error(), "no build") {
		t.Errorf("missing assets: %v", err)
	}
}

func TestApplyUpdateExplainsAnUnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 || (runtime.GOOS != "linux" && runtime.GOOS != "darwin") {
		t.Skip("needs a non-root user on linux or darwin")
	}
	rel := fakeRelease(t, "v9.9.9", "v9.9.9", false)
	dir := t.TempDir()
	p := filepath.Join(dir, "wr")
	os.WriteFile(p, []byte("OLD"), 0o755)
	os.Chmod(dir, 0o555)
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	err := applyUpdateTo(rel, p, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "install.sh") {
		t.Errorf("the error must say how to install otherwise: %v", err)
	}
}

func TestChecksumFor(t *testing.T) {
	sums := []byte("abc123  wr_linux_amd64.tar.gz\nDEF456 *wr_darwin_arm64.tar.gz\n")
	if got := checksumFor(sums, "wr_linux_amd64.tar.gz"); got != "abc123" {
		t.Errorf("got %q", got)
	}
	if got := checksumFor(sums, "wr_darwin_arm64.tar.gz"); got != "def456" {
		t.Errorf("binary-mode marker and case: got %q", got)
	}
	if got := checksumFor(sums, "missing"); got != "" {
		t.Errorf("got %q", got)
	}
}

// ---- release notes ----

const releasePleaseBody = `:robot: I have created a release *beep* *boop*
---


## [0.6.0](https://github.com/sazardev/wr/compare/v0.5.1...v0.6.0) (2026-10-12)


### Features

* **mcp:** agent harness — MCP server ([670841b](https://github.com/sazardev/wr/commit/670841b))
* read script-drawn pages, with a [guide](https://x/y) and ` + "`code`" + ` ([9420c56](https://github.com/sazardev/wr/commit/9420c56))


### Bug Fixes

* links follow redirects ([abc1234](https://github.com/sazardev/wr/commit/abc1234))

---
This PR was generated with [Release Please](https://github.com/googleapis/release-please).`

func TestReleaseNotesReadAsPlainText(t *testing.T) {
	var got []string
	for _, n := range releaseNotes(releasePleaseBody) {
		got = append(got, fmt.Sprintf("%d:%s", n.kind, n.text))
	}
	want := []string{
		"3:2026-10-12",
		"1:Features",
		"2:mcp: agent harness — MCP server",
		"2:read script-drawn pages, with a guide and code",
		"1:Bug Fixes",
		"2:links follow redirects",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("notes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	for _, n := range releaseNotes("a \x1b[31mred\x1b[0m line") {
		if strings.Contains(n.text, "\x1b") {
			t.Error("release notes are untrusted: control characters must go")
		}
	}
}
