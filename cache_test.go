package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func isolate(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", d)
	return filepath.Join(d, "wr")
}

func TestCacheRoundTrip(t *testing.T) {
	isolate(t)
	if _, _, ok := cacheGet("https://a.test/x"); ok {
		t.Fatal("hit on an empty cache")
	}
	if err := cachePut("https://a.test/x", "# Hello\n\nbody\n"); err != nil {
		t.Fatal(err)
	}
	md, saved, ok := cacheGet("https://a.test/x")
	if !ok || md != "# Hello\n\nbody\n" {
		t.Fatalf("ok=%v md=%q", ok, md)
	}
	if time.Since(saved) > time.Minute {
		t.Fatalf("wrong date: %v", saved)
	}
}

func TestCacheDeleteAndStats(t *testing.T) {
	isolate(t)
	_ = cachePut("https://a.test/1", "one")
	_ = cachePut("https://a.test/2", "two two")
	if n, size := cacheStats(); n != 2 || size <= 0 {
		t.Fatalf("stats: n=%d size=%d", n, size)
	}
	if err := cacheDelete("https://a.test/1"); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := cacheGet("https://a.test/1"); ok {
		t.Fatal("the deleted entry is still there")
	}
	if _, _, ok := cacheGet("https://a.test/2"); !ok {
		t.Fatal("deleted the wrong entry")
	}
	if err := cacheDelete("https://a.test/no-existe"); err != nil {
		t.Fatalf("deleting something missing is not an error: %v", err)
	}
	if got := humanSize(2048); got != "2 KB" {
		t.Fatalf("humanSize: %q", got)
	}
}

func TestCacheRejectsMismatchedOrTamperedEntries(t *testing.T) {
	dir := isolate(t)
	_ = cachePut("https://a.test/x", "ok")
	path := cachePath(dir, "https://a.test/x")

	// the entry claims to be for another URL (collision or tampered file)
	data, _ := os.ReadFile(path)
	_ = os.WriteFile(path, []byte(strings.Replace(string(data), "https://a.test/x", "https://evil.test/y", 1)), 0o600)
	if _, _, ok := cacheGet("https://a.test/x"); ok {
		t.Fatal("accepted an entry for another URL")
	}
	// garbage header
	_ = os.WriteFile(path, []byte("garbage without a header"), 0o600)
	if _, _, ok := cacheGet("https://a.test/x"); ok {
		t.Fatal("accepted an entry without a header")
	}
}

func TestCachePermissionsAndPrune(t *testing.T) {
	dir := isolate(t)
	_ = cachePut("https://a.test/x", "ok")
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Errorf("dir perms %v", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(cachePath(dir, "https://a.test/x")); fi.Mode().Perm() != 0o600 {
		t.Errorf("file perms %v", fi.Mode().Perm())
	}

	old := filepath.Join(dir, "old.md")
	tmp := filepath.Join(dir, "tmp-orphan")
	fresh := filepath.Join(dir, "tmp-recent")
	for _, f := range []string{old, tmp, fresh} {
		_ = os.WriteFile(f, []byte("x"), 0o600)
	}
	_ = os.Chtimes(old, time.Now().Add(-31*24*time.Hour), time.Now().Add(-31*24*time.Hour))
	_ = os.Chtimes(tmp, time.Now().Add(-2*time.Hour), time.Now().Add(-2*time.Hour))
	pruneCache(dir)
	if _, err := os.Stat(old); err == nil {
		t.Error("did not delete the entry older than 30 days")
	}
	if _, err := os.Stat(tmp); err == nil {
		t.Error("did not delete the orphaned temp file")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("deleted a recent temp file (it may be in use)")
	}
}

func TestCacheClear(t *testing.T) {
	dir := isolate(t)
	_ = cachePut("https://a.test/x", "ok")
	if err := cacheClear(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("the directory still exists")
	}
}

func TestSanitizeStripsEscapes(t *testing.T) {
	if got := sanitize("a\x1b[31mb\x07c\td\n"); got != "a[31mbc\td\n" {
		t.Fatalf("got %q", got)
	}
}
