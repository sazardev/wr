package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// On-disk cache of the extracted Markdown, per URL. It lets a page you already
// read open instantly; while you read it is refreshed in the background for
// next time. It lives in the user cache directory (~/.cache/wr on Linux), with
// permissions 0700/0600.

const (
	cacheVersion = 2 // bump if the generated Markdown format changes
	cacheTmpAge  = time.Hour
)

// cacheMaxAge is set from cache_days in the configuration.
var cacheMaxAge = 30 * 24 * time.Hour

func isHTTP(src string) bool {
	return strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")
}

func cacheDir() (string, error) {
	d, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "wr"), nil
}

func cachePath(dir, src string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%s", cacheVersion, src)))
	return filepath.Join(dir, hex.EncodeToString(sum[:16])+".md")
}

// cacheGet returns the stored Markdown and when it was stored.
func cacheGet(src string) (md string, saved time.Time, ok bool) {
	dir, err := cacheDir()
	if err != nil {
		return "", time.Time{}, false
	}
	data, err := os.ReadFile(cachePath(dir, src))
	if err != nil {
		return "", time.Time{}, false
	}
	header, body, found := strings.Cut(string(data), "\n")
	f := strings.SplitN(header, " ", 4)
	if !found || len(f) != 4 || f[0] != "wr-cache" || f[1] != strconv.Itoa(cacheVersion) || f[3] != src {
		return "", time.Time{}, false // old format or key collision
	}
	secs, err := strconv.ParseInt(f[2], 10, 64)
	if err != nil {
		return "", time.Time{}, false
	}
	return body, time.Unix(secs, 0), true
}

// cachePut writes atomically (temp file + rename), so an interrupted refresh
// never leaves a half-written entry.
func cachePut(src, md string) error {
	dir, err := cacheDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "tmp-*")
	if err != nil {
		return err
	}
	_, werr := fmt.Fprintf(tmp, "wr-cache %d %d %s\n%s", cacheVersion, time.Now().Unix(), src, md)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(tmp.Name())
		return werr
	}
	if err := os.Rename(tmp.Name(), cachePath(dir, src)); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	pruneCache(dir)
	return nil
}

// pruneCache deletes entries older than cacheMaxAge and orphaned temp files.
func pruneCache(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		age := now.Sub(info.ModTime())
		if age > cacheMaxAge || (strings.HasPrefix(e.Name(), "tmp-") && age > cacheTmpAge) {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

func cacheClear() error {
	dir, err := cacheDir()
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func humanAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// cacheDelete removes the entry for one URL.
func cacheDelete(src string) error {
	dir, err := cacheDir()
	if err != nil {
		return err
	}
	err = os.Remove(cachePath(dir, src))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// cacheStats counts stored pages and their size in bytes.
func cacheStats() (pages int, bytes int64) {
	dir, err := cacheDir()
	if err != nil {
		return 0, 0
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "tmp-") || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		if info, err := e.Info(); err == nil && info.Mode().IsRegular() {
			pages++
			bytes += info.Size()
		}
	}
	return pages, bytes
}

func humanSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.0f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}
