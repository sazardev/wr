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
	if _, _, ok := cacheGet("https://a.test/x", false); ok {
		t.Fatal("hit en cache vacia")
	}
	if err := cachePut("https://a.test/x", false, "# Hola\n\ncuerpo\n"); err != nil {
		t.Fatal(err)
	}
	md, saved, ok := cacheGet("https://a.test/x", false)
	if !ok || md != "# Hola\n\ncuerpo\n" {
		t.Fatalf("ok=%v md=%q", ok, md)
	}
	if time.Since(saved) > time.Minute {
		t.Fatalf("fecha incorrecta: %v", saved)
	}
}

func TestCacheSeparatesNoLinks(t *testing.T) {
	isolate(t)
	_ = cachePut("https://a.test/x", false, "con [enlaces](u)")
	_ = cachePut("https://a.test/x", true, "sin enlaces")
	a, _, _ := cacheGet("https://a.test/x", false)
	b, _, _ := cacheGet("https://a.test/x", true)
	if a == b || !strings.Contains(a, "enlaces](u)") || b != "sin enlaces" {
		t.Fatalf("a=%q b=%q", a, b)
	}
}

func TestCacheRejectsMismatchedOrTamperedEntries(t *testing.T) {
	dir := isolate(t)
	_ = cachePut("https://a.test/x", false, "ok")
	path := cachePath(dir, "https://a.test/x", false)

	// la entrada dice ser de otra URL (colision o archivo manipulado)
	data, _ := os.ReadFile(path)
	_ = os.WriteFile(path, []byte(strings.Replace(string(data), "https://a.test/x", "https://evil.test/y", 1)), 0o600)
	if _, _, ok := cacheGet("https://a.test/x", false); ok {
		t.Fatal("acepto una entrada de otra URL")
	}
	// cabecera basura
	_ = os.WriteFile(path, []byte("basura sin cabecera"), 0o600)
	if _, _, ok := cacheGet("https://a.test/x", false); ok {
		t.Fatal("acepto una entrada sin cabecera")
	}
}

func TestCachePermissionsAndPrune(t *testing.T) {
	dir := isolate(t)
	_ = cachePut("https://a.test/x", false, "ok")
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Errorf("dir perms %v", fi.Mode().Perm())
	}
	if fi, _ := os.Stat(cachePath(dir, "https://a.test/x", false)); fi.Mode().Perm() != 0o600 {
		t.Errorf("file perms %v", fi.Mode().Perm())
	}

	old := filepath.Join(dir, "viejo.md")
	tmp := filepath.Join(dir, "tmp-huerfano")
	fresh := filepath.Join(dir, "tmp-reciente")
	for _, f := range []string{old, tmp, fresh} {
		_ = os.WriteFile(f, []byte("x"), 0o600)
	}
	_ = os.Chtimes(old, time.Now().Add(-31*24*time.Hour), time.Now().Add(-31*24*time.Hour))
	_ = os.Chtimes(tmp, time.Now().Add(-2*time.Hour), time.Now().Add(-2*time.Hour))
	pruneCache(dir)
	if _, err := os.Stat(old); err == nil {
		t.Error("no borro la entrada de >30 dias")
	}
	if _, err := os.Stat(tmp); err == nil {
		t.Error("no borro el temporal huerfano")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("borro un temporal reciente (podria estar en uso)")
	}
}

func TestCacheClear(t *testing.T) {
	dir := isolate(t)
	_ = cachePut("https://a.test/x", false, "ok")
	if err := cacheClear(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("la carpeta sigue existiendo")
	}
}

func TestSanitizeStripsEscapes(t *testing.T) {
	if got := sanitize("a\x1b[31mb\x07c\td\n"); got != "a[31mbc\td\n" {
		t.Fatalf("got %q", got)
	}
}

func TestHumanAge(t *testing.T) {
	for d, want := range map[time.Duration]string{
		5 * time.Second: "hace unos segundos",
		3 * time.Minute: "hace 3 min",
		5 * time.Hour:   "hace 5 h",
		72 * time.Hour:  "hace 3 dias",
	} {
		if got := humanAge(d); got != want {
			t.Errorf("%v: got %q want %q", d, got, want)
		}
	}
}
