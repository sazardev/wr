package main

// Headless sessions over the real Bubble Tea model: an agent opens a page,
// sends the same keys a person would press, and reads the screen. The model is
// driven without a terminal, so what it reports is exactly what the reader
// would show (m.render()), with animations off for determinism.
//
// Each session owns one goroutine pool that runs the commands Update returns
// (page loads) and feeds their messages back, so wr_session_key answers with
// the state right after the key and a load in flight shows up as loading=true
// on the next wr_session_screen.

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

type agentSession struct {
	id   string
	mu   sync.Mutex
	m    *model
	w, h int

	cmds   []tea.Cmd // commands Update returned, waiting to run
	done   chan struct{}
	closed bool
	flying atomic.Int32 // commands running right now
}

// ---- lifecycle ----

func (s *mcpServer) openSession(target string, w, h int, fresh bool) (*agentSession, error) {
	sess, err := newAgentSession(target, w, h, fresh)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.nextID++
	sess.id = fmt.Sprintf("s%d", s.nextID)
	s.sess[sess.id] = sess
	s.mu.Unlock()
	return sess, nil
}

// newAgentSession opens a headless reading session on target. The id is empty
// until the server registers it (the eval scenarios use it unregistered).
func newAgentSession(target string, w, h int, fresh bool) (*agentSession, error) {
	cfg, _ := loadConfig()
	cfg.Animations = false
	src, _ := normalizeInput(target, cfg.SearchEngine)
	if src == "" {
		return nil, argError("target is required")
	}
	if w <= 0 {
		w = 100
	}
	if h <= 0 {
		h = 30
	}
	sess := &agentSession{m: newModel(src, cfg, fresh, nil), w: w, h: h, done: make(chan struct{})}
	sess.apply(tea.WindowSizeMsg{Width: w, Height: h})
	sess.spawn([]tea.Cmd{sess.m.Init()})
	sess.waitSettled(2 * time.Second) // local pages and most fetches land now
	return sess, nil
}

func (s *mcpServer) getSession(id string) (*agentSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sess[id]
	if !ok || sess.isClosed() {
		return nil, argError("no such session: " + id)
	}
	return sess, nil
}

func (s *mcpServer) closeSession(id string) error {
	sess, err := s.getSession(id)
	if err != nil {
		return err
	}
	sess.close()
	s.mu.Lock()
	delete(s.sess, id)
	s.mu.Unlock()
	return nil
}

func (s *agentSession) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	close(s.done)
	s.mu.Unlock()
}

func (s *agentSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// apply hands a message to the model now and queues whatever commands it
// returns for the goroutine pool.
func (s *agentSession) apply(msg tea.Msg) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	var next tea.Cmd
	if b, ok := msg.(tea.BatchMsg); ok {
		s.cmds = append(s.cmds, b...)
	} else {
		_, next = s.m.Update(msg)
		if next != nil {
			s.cmds = append(s.cmds, next)
		}
	}
	cs := s.cmds
	s.cmds = nil
	s.mu.Unlock()
	s.spawn(cs)
}

func (s *agentSession) spawn(cmds []tea.Cmd) {
	for _, c := range cmds {
		if c == nil {
			continue
		}
		cc := c
		s.flying.Add(1)
		go func() {
			defer s.flying.Add(-1)
			select {
			case <-s.done:
				return
			default:
			}
			msg := cc()
			if msg == nil {
				return
			}
			if b, ok := msg.(tea.BatchMsg); ok {
				s.spawn([]tea.Cmd(b))
				return
			}
			s.apply(msg)
		}()
	}
}

// waitSettled waits until no command is in flight (or the budget runs out), so
// opening a local page returns with the text already on screen.
func (s *agentSession) waitSettled(budget time.Duration) {
	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		if s.flying.Load() == 0 {
			time.Sleep(10 * time.Millisecond) // a command may be about to spawn
			if s.flying.Load() == 0 {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (s *agentSession) key(name string) error {
	msg, ok := keyPressMsg(name)
	if !ok {
		return argError("unknown key: " + name + " (try j, k, g, G, tab, enter, esc, pgdown, ctrl+d, shift+tab...)")
	}
	s.apply(msg)
	return nil
}

func (s *agentSession) typeText(text string) {
	for _, r := range text {
		s.apply(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}

// keyPressMsg maps a key name to the message the reader expects. The names are
// the canonical ones the docs use, so msg.String() matches the key bindings.
func keyPressMsg(name string) (tea.KeyPressMsg, bool) {
	name = strings.TrimSpace(name)
	var mod tea.KeyMod
	for {
		switch {
		case strings.HasPrefix(name, "ctrl+"):
			mod, name = tea.ModCtrl, name[5:]
		case strings.HasPrefix(name, "alt+"):
			mod, name = tea.ModAlt, name[4:]
		case strings.HasPrefix(name, "shift+"):
			mod, name = tea.ModShift, name[6:]
		default:
			return keyWithMod(name, mod)
		}
	}
}

func keyWithMod(name string, mod tea.KeyMod) (tea.KeyPressMsg, bool) {
	switch name {
	case "enter", "return":
		return tea.KeyPressMsg{Code: tea.KeyEnter, Mod: mod}, true
	case "esc", "escape":
		return tea.KeyPressMsg{Code: tea.KeyEscape, Mod: mod}, true
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: mod}, true
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace, Mod: mod}, true
	case "space", " ":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " ", Mod: mod}, true
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp, Mod: mod}, true
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown, Mod: mod}, true
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft, Mod: mod}, true
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight, Mod: mod}, true
	case "pgup", "pageup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp, Mod: mod}, true
	case "pgdown", "pagedown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown, Mod: mod}, true
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome, Mod: mod}, true
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd, Mod: mod}, true
	case "delete", "del":
		return tea.KeyPressMsg{Code: tea.KeyDelete, Mod: mod}, true
	}
	if r := []rune(name); len(r) == 1 {
		// a modifier needs a bare code: text takes over the identity of the key
		if mod == 0 {
			return tea.KeyPressMsg{Code: r[0], Text: string(r)}, true
		}
		return tea.KeyPressMsg{Code: r[0], Mod: mod}, true
	}
	return tea.KeyPressMsg{}, false
}

// ---- state ----

type sessScroll struct {
	Y       int `json:"y"`       // first visible line (0-based)
	Total   int `json:"total"`   // lines in the document
	Visible int `json:"visible"` // lines that fit on screen
	Percent int `json:"percent"` // 0-100
}

type sessState struct {
	URL         string     `json:"url"`
	Title       string     `json:"title"`
	Mode        string     `json:"mode"`
	Loading     bool       `json:"loading"`
	Error       string     `json:"error,omitempty"`
	Toast       string     `json:"toast,omitempty"`
	FromCache   bool       `json:"from_cache"`
	Bookmarked  bool       `json:"bookmarked"`
	Back        []string   `json:"back"`
	Forward     []string   `json:"forward"`
	Links       int        `json:"links"`
	FocusedLink string     `json:"focused_link,omitempty"`
	Query       string     `json:"query,omitempty"`
	Matches     int        `json:"matches"`
	Scroll      sessScroll `json:"scroll"`
}

func modeName(m mode) string {
	switch m {
	case modeRead:
		return "read"
	case modeSearch:
		return "search"
	case modeHints:
		return "hints"
	case modeOmni:
		return "omni"
	case modeMenu:
		return "menu"
	case modeConfirm:
		return "confirm"
	case modeTOC:
		return "toc"
	case modeSettings:
		return "settings"
	case modeKeys:
		return "keys"
	case modeList:
		return "list"
	}
	return "read"
}

func (s *agentSession) state() sessState {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := s.m
	st := sessState{
		URL:         m.src,
		Title:       m.title(),
		Mode:        modeName(m.mode),
		Loading:     m.loading,
		Toast:       m.toast,
		FromCache:   m.fromCache,
		Bookmarked:  m.bookmarked,
		Query:       m.query,
		Matches:     len(m.matches),
		FocusedLink: m.focusedURL(),
	}
	if m.loadErr != nil {
		st.Error = m.loadErr.Error()
	}
	st.Back = navURLs(m.back)
	st.Forward = navURLs(m.fwd)
	st.Links = len(m.linksOrder())
	_, _, bodyH, _ := m.geometry()
	st.Scroll = sessScroll{Y: m.y, Total: len(docLinesOf(m)), Visible: bodyH}
	if st.Scroll.Total > st.Scroll.Visible {
		st.Scroll.Percent = st.Scroll.Y * 100 / (st.Scroll.Total - st.Scroll.Visible)
		if st.Scroll.Percent > 100 {
			st.Scroll.Percent = 100
		}
	}
	return st
}

func navURLs(entries []navEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.src)
	}
	return out
}

// screen renders the model exactly like the terminal view would.
func (s *agentSession) screen(ansiOut bool) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ansiOut {
		return s.m.render()
	}
	var b strings.Builder
	for i, l := range strings.Split(s.m.render(), "\n") {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(ansi.Strip(l))
	}
	return b.String()
}

// docLines is len(m.doc.Lines) without a nil panic on the loading screen.
func docLinesOf(m *model) []string {
	if m.doc == nil {
		return nil
	}
	return m.doc.Lines
}

// ---- tools ----

func registerSessionTools(s *mcpServer) {
	tools := []mcpTool{
		{
			def: toolDef{
				Name: "wr_session_open",
				Description: "Open a page in a headless reading session (the interactive reader, driven by keys). " +
					"Returns the screen and state right away; a network load still in flight shows as loading=true.",
				InputSchema: objectSchema(jsonSchema{
					"target": strProp("an URL, a local file path, or search words"),
					"width":  intProp("terminal width in columns (default 100)"),
					"height": intProp("terminal height in rows (default 30)"),
					"fresh":  boolProp("ignore the cache for this first load (default false)"),
				}, "target"),
			},
			call: func(s *mcpServer, raw json.RawMessage) (any, error) {
				var a struct {
					Target string `json:"target"`
					Width  int    `json:"width"`
					Height int    `json:"height"`
					Fresh  bool   `json:"fresh"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return nil, argError("bad arguments: " + err.Error())
				}
				sess, err := s.openSession(a.Target, a.Width, a.Height, a.Fresh)
				if err != nil {
					return nil, err
				}
				return sess.snapshot(false), nil
			},
		},
		{
			def: toolDef{
				Name: "wr_session_key",
				Description: "Send a key to a session, like a person pressing it. Names are the documented ones: " +
					"letters (j k g G t n N / o v ' * ~ x f m , ? q H L [ ] d u b y r), enter, esc, tab, shift+tab, " +
					"space, up, down, left, right, pgup, pgdown, home, end, backspace, delete, and ctrl+d / ctrl+f / " +
					"ctrl+u / ctrl+b / alt+left / alt+right. After pressing o (address bar) or / (search), send the text with wr_session_type.",
				InputSchema: objectSchema(jsonSchema{
					"session_id": strProp("the session to drive"),
					"key":        strProp("the key to press"),
					"repeat":     intProp("press it this many times (default 1)"),
				}, "session_id", "key"),
			},
			call: func(s *mcpServer, raw json.RawMessage) (any, error) {
				var a struct {
					SessionID string `json:"session_id"`
					Key       string `json:"key"`
					Repeat    int    `json:"repeat"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return nil, argError("bad arguments: " + err.Error())
				}
				sess, err := s.getSession(a.SessionID)
				if err != nil {
					return nil, err
				}
				n := a.Repeat
				if n <= 0 {
					n = 1
				}
				if n > 1000 {
					n = 1000
				}
				for i := 0; i < n; i++ {
					if err := sess.key(a.Key); err != nil {
						return nil, err
					}
				}
				sess.waitSettled(300 * time.Millisecond) // give a navigation a chance to land
				return sess.snapshot(false), nil
			},
		},
		{
			def: toolDef{
				Name:        "wr_session_type",
				Description: "Type text into a session (the address bar after o, the search line after /, a link label after f).",
				InputSchema: objectSchema(jsonSchema{
					"session_id": strProp("the session to drive"),
					"text":       strProp("the text to type, character by character"),
				}, "session_id", "text"),
			},
			call: func(s *mcpServer, raw json.RawMessage) (any, error) {
				var a struct {
					SessionID string `json:"session_id"`
					Text      string `json:"text"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return nil, argError("bad arguments: " + err.Error())
				}
				sess, err := s.getSession(a.SessionID)
				if err != nil {
					return nil, err
				}
				if len(a.Text) > 4096 {
					return nil, argError("text too long")
				}
				sess.typeText(a.Text)
				sess.waitSettled(300 * time.Millisecond)
				return sess.snapshot(false), nil
			},
		},
		{
			def: toolDef{
				Name: "wr_session_screen",
				Description: "Read a session: the current screen (plain text, or ANSI with colors) plus the structured state " +
					"(url, title, mode, scroll, back/forward, focused link, toasts, errors).",
				InputSchema: objectSchema(jsonSchema{
					"session_id": strProp("the session to read"),
					"ansi":       boolProp("keep the ANSI colors in the screen (default false)"),
				}, "session_id"),
			},
			call: func(s *mcpServer, raw json.RawMessage) (any, error) {
				var a struct {
					SessionID string `json:"session_id"`
					ANSI      bool   `json:"ansi"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return nil, argError("bad arguments: " + err.Error())
				}
				sess, err := s.getSession(a.SessionID)
				if err != nil {
					return nil, err
				}
				return sess.snapshot(a.ANSI), nil
			},
		},
		{
			def: toolDef{
				Name:        "wr_session_close",
				Description: "Close a session and free its state. Ids are reusable after closing.",
				InputSchema: objectSchema(jsonSchema{
					"session_id": strProp("the session to close"),
				}, "session_id"),
			},
			call: func(s *mcpServer, raw json.RawMessage) (any, error) {
				var a struct {
					SessionID string `json:"session_id"`
				}
				if err := json.Unmarshal(raw, &a); err != nil {
					return nil, argError("bad arguments: " + err.Error())
				}
				if err := s.closeSession(a.SessionID); err != nil {
					return nil, err
				}
				return map[string]any{"closed": a.SessionID}, nil
			},
		},
		{
			def: toolDef{
				Name:        "wr_session_list",
				Description: "List the open sessions with their url, title and mode.",
				InputSchema: objectSchema(jsonSchema{}),
			},
			call: func(s *mcpServer, raw json.RawMessage) (any, error) {
				s.mu.Lock()
				ids := make([]string, 0, len(s.sess))
				for id := range s.sess {
					ids = append(ids, id)
				}
				s.mu.Unlock()
				out := make([]map[string]any, 0, len(ids))
				for _, id := range ids {
					sess, err := s.getSession(id)
					if err != nil {
						continue
					}
					st := sess.state()
					out = append(out, map[string]any{
						"session_id": id, "url": st.URL, "title": st.Title, "mode": st.Mode, "loading": st.Loading,
					})
				}
				return map[string]any{"count": len(out), "sessions": out}, nil
			},
		},
	}
	for _, t := range tools {
		s.register(t)
	}
}

type sessSnapshot struct {
	SessionID string    `json:"session_id"`
	State     sessState `json:"state"`
	Screen    string    `json:"screen"`
}

func (s *agentSession) snapshot(ansiOut bool) sessSnapshot {
	return sessSnapshot{SessionID: s.id, State: s.state(), Screen: s.screen(ansiOut)}
}
