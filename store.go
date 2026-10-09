package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Persistent browsing state, kept in the user's state directory
// (~/.local/state/wr): the visit history (append-only JSON lines) and the
// bookmarks (a small JSON file). Directory 0700, files 0600.

type Visit struct {
	URL   string
	Title string
	At    time.Time
}

type visitJSON struct {
	U string `json:"u"`
	T string `json:"t,omitempty"`
	A int64  `json:"a"`
}

func (v Visit) json() visitJSON  { return visitJSON{v.URL, v.Title, v.At.Unix()} }
func (j visitJSON) visit() Visit { return Visit{j.U, j.T, time.Unix(j.A, 0)} }

func stateDir() (string, error) {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "wr"), nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".local", "state", "wr"), nil
}

func statePath(name string) (string, error) {
	d, err := stateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, name), nil
}

func ensureStateDir() error {
	d, err := stateDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(d, 0o700)
}

// writeAtomic replaces path without ever leaving it half-written. The parent
// directory must already exist.
func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(tmp.Name())
		return werr
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

// ---- history ----

func readHistory() []Visit {
	path, err := statePath("history.jsonl")
	if err != nil {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []Visit
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		var j visitJSON
		if json.Unmarshal(sc.Bytes(), &j) == nil && j.U != "" {
			out = append(out, j.visit())
		}
	}
	return out
}

func writeHistory(vs []Visit) error {
	path, err := statePath("history.jsonl")
	if err != nil {
		return err
	}
	if err := ensureStateDir(); err != nil {
		return err
	}
	var b strings.Builder
	for _, v := range vs {
		line, _ := json.Marshal(v.json())
		b.Write(line)
		b.WriteByte('\n')
	}
	return writeAtomic(path, []byte(b.String()))
}

// historyAdd records a visit. Visiting the same page again right away only
// refreshes its title; the file is trimmed to `limit` entries when it grows.
func historyAdd(url, title string, limit int) error {
	vs := readHistory()
	if n := len(vs); n > 0 && vs[n-1].URL == url {
		if title != "" && vs[n-1].Title != title {
			vs[n-1].Title = title
			return writeHistory(vs)
		}
		return nil
	}
	v := Visit{url, title, time.Now()}
	if len(vs)+1 > limit*2 {
		vs = append(vs, v)
		return writeHistory(vs[len(vs)-limit:])
	}
	if err := ensureStateDir(); err != nil {
		return err
	}
	path, err := statePath("history.jsonl")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	line, _ := json.Marshal(v.json())
	_, err = f.Write(append(line, '\n'))
	return err
}

// historyList is the history, most recent first, one entry per page.
func historyList() []Visit {
	vs := readHistory()
	seen := map[string]bool{}
	var out []Visit
	for i := len(vs) - 1; i >= 0; i-- {
		if !seen[vs[i].URL] {
			seen[vs[i].URL] = true
			out = append(out, vs[i])
		}
	}
	return out
}

func historyDelete(url string) error {
	vs := readHistory()
	kept := vs[:0]
	for _, v := range vs {
		if v.URL != url {
			kept = append(kept, v)
		}
	}
	return writeHistory(kept)
}

func historyClear() error {
	path, err := statePath("history.jsonl")
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ---- bookmarks ----

func readBookmarks() []Visit {
	path, err := statePath("bookmarks.json")
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var js []visitJSON
	if json.Unmarshal(data, &js) != nil {
		return nil
	}
	out := make([]Visit, 0, len(js))
	for _, j := range js {
		if j.U != "" {
			out = append(out, j.visit())
		}
	}
	return out
}

func writeBookmarks(vs []Visit) error {
	path, err := statePath("bookmarks.json")
	if err != nil {
		return err
	}
	if err := ensureStateDir(); err != nil {
		return err
	}
	js := make([]visitJSON, len(vs))
	for i, v := range vs {
		js[i] = v.json()
	}
	data, _ := json.MarshalIndent(js, "", "  ")
	return writeAtomic(path, data)
}

// bookmarkList is newest first.
func bookmarkList() []Visit {
	vs := readBookmarks()
	for i, j := 0, len(vs)-1; i < j; i, j = i+1, j-1 {
		vs[i], vs[j] = vs[j], vs[i]
	}
	return vs
}

func isBookmarked(url string) bool {
	for _, v := range readBookmarks() {
		if v.URL == url {
			return true
		}
	}
	return false
}

// bookmarkToggle adds the page, or removes it if it was already bookmarked.
func bookmarkToggle(url, title string) (added bool, err error) {
	vs := readBookmarks()
	for i, v := range vs {
		if v.URL == url {
			return false, writeBookmarks(append(vs[:i:i], vs[i+1:]...))
		}
	}
	return true, writeBookmarks(append(vs, Visit{url, title, time.Now()}))
}

func bookmarkDelete(url string) error {
	vs := readBookmarks()
	kept := vs[:0]
	for _, v := range vs {
		if v.URL != url {
			kept = append(kept, v)
		}
	}
	return writeBookmarks(kept)
}

// humanAge is a short "how long ago" for lists.
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
