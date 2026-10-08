package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/vt"
)

// End-to-end tests: the real Bubble Tea program (real renderer, real escape
// sequences) writes into Charm's terminal emulator, and we read the screen.
//
// Note: the emulator does not reproduce the renderer's scroll-region
// optimizations (DECSTBM + line feeds), so after a long vertical scroll its
// screen can be wrong even though tmux and real terminals draw it correctly.
// These tests therefore avoid vertical scrolling; the scroll animation itself
// is covered by the model-level tests in tui_test.go.

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) snapshot() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.b.Bytes()...)
}

type session struct {
	t    *testing.T
	p    *tea.Program
	out  *syncBuf
	done chan struct{}
	w, h int
}

func startSession(t *testing.T, w, h int) *session {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	s := &session{t: t, out: &syncBuf{}, done: make(chan struct{}), w: w, h: h}
	s.p = tea.NewProgram(newModel("https://t.test/x", defaultConfig(), false, nil),
		tea.WithInput(nil), tea.WithOutput(s.out), tea.WithWindowSize(w, h))
	go func() { _, _ = s.p.Run(); close(s.done) }()
	t.Cleanup(func() { s.p.Quit(); <-s.done })
	time.Sleep(50 * time.Millisecond)
	s.p.Send(loadedMsg{md: longDoc()})
	return s
}

func (s *session) screen() string {
	em := vt.NewEmulator(s.w, s.h)
	_, _ = em.Write(s.out.snapshot())
	return em.String()
}

// waitFor polls the emulated screen until it contains every wanted text.
func (s *session) waitFor(wants ...string) string {
	s.t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	var scr string
	for time.Now().Before(deadline) {
		scr = s.screen()
		ok := true
		for _, w := range wants {
			ok = ok && strings.Contains(scr, w)
		}
		if ok {
			return scr
		}
		time.Sleep(20 * time.Millisecond)
	}
	s.t.Fatalf("timed out waiting for %q on screen:\n%s", wants, scr)
	return ""
}

func (s *session) key(text string) { s.p.Send(tea.KeyPressMsg{Code: []rune(text)[0], Text: text}) }

func TestE2EMenuHasNoDocumentTextBesidePanel(t *testing.T) {
	s := startSession(t, 100, 28)
	s.waitFor("Document")
	s.key("m")
	scr := s.waitFor("Reload page", "Quit")
	for _, l := range strings.Split(scr, "\n") {
		if i := strings.Index(l, "│"); i > 0 && (strings.Contains(l, "Reload") || strings.Contains(l, "Section index")) {
			if strings.TrimSpace(l[:i]) != "" {
				t.Errorf("document text left of the panel: %q", l)
			}
		}
	}
}

func TestE2EFooterShowsHostAndNoTitle(t *testing.T) {
	s := startSession(t, 100, 28)
	scr := s.waitFor("t.test", "search")
	lines := strings.Split(scr, "\n")
	status := lines[len(lines)-2]
	if !strings.Contains(status, "t.test") || strings.Contains(status, "Document") {
		t.Errorf("status row: %q", status)
	}
}
