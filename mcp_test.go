package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- the harness the protocol tests share ----

// mcpTalk feeds request lines to a fresh server over a pipe and returns every
// response line it wrote. serve ends at EOF, so the buffer is complete after.
func mcpTalk(t *testing.T, reqs ...string) []rpcResponse {
	t.Helper()
	pr, pw := io.Pipe()
	out := &bytes.Buffer{}
	s := newMCPServer(out)
	done := make(chan error, 1)
	go func() { done <- s.serve(pr) }()
	go func() {
		for _, r := range reqs {
			fmt.Fprintln(pw, r)
		}
		pw.Close()
	}()
	if err := <-done; err != nil {
		t.Fatalf("serve: %v", err)
	}
	var out2 []rpcResponse
	for _, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		var r rpcResponse
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("bad response line %q: %v", line, err)
		}
		out2 = append(out2, r)
	}
	return out2
}

func call(t *testing.T, name string, args any) string {
	t.Helper()
	a, _ := json.Marshal(args)
	return fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, name, a)
}

func decodeTool(t *testing.T, resp rpcResponse) toolResult {
	t.Helper()
	if resp.Error != nil {
		t.Fatalf("rpc error: %+v", resp.Error)
	}
	raw, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatal(err)
	}
	var tr toolResult
	if err := json.Unmarshal(raw, &tr); err != nil {
		t.Fatal(err)
	}
	return tr
}

func toolText(t *testing.T, resp rpcResponse) string {
	t.Helper()
	tr := decodeTool(t, resp)
	if tr.IsError || len(tr.Content) == 0 {
		t.Fatalf("tool failed: %s", func() string {
			if len(tr.Content) > 0 {
				return tr.Content[0].Text
			}
			return "(no content)"
		}())
	}
	return tr.Content[0].Text
}

func toolStructured(t *testing.T, resp rpcResponse) map[string]any {
	t.Helper()
	return decodeTool(t, resp).StructuredContent.(map[string]any)
}

// localPage writes a markdown file and returns its path.
func localPage(t *testing.T, md string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "page.md")
	if err := os.WriteFile(p, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// localSite writes two html pages that link to each other and returns the first.
func localSite(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.html")
	b := filepath.Join(dir, "b.html")
	fill := strings.Repeat("<p>Filler paragraph with plenty of words to fill the screen.</p>", 40)
	os.WriteFile(a, []byte(`<html><head><title>A</title></head><body><article><h1>Page A</h1>`+
		`<p>Intro with a <a href="b.html">link to page b</a>.</p>`+fill+`</article></body></html>`), 0o644)
	os.WriteFile(b, []byte(`<html><head><title>B</title></head><body><article><h1>Page B</h1></article></body></html>`), 0o644)
	return a
}

// ---- protocol ----

func TestMCPHandshake(t *testing.T) {
	resp := mcpTalk(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"test","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`, // a notification: no answer
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":4,"method":"resources/list"}`,
		`{"jsonrpc":"2.0","id":5,"method":"no/such"}`,
		`{oops`,
	)
	if len(resp) != 5 {
		t.Fatalf("got %d responses, want 5 (the notification must not be answered)", len(resp))
	}

	// initialize
	var init initializeResult
	raw, _ := json.Marshal(resp[0].Result)
	if err := json.Unmarshal(raw, &init); err != nil {
		t.Fatal(err)
	}
	if init.ProtocolVersion != "2025-06-18" {
		t.Errorf("protocol version: %q", init.ProtocolVersion)
	}
	if init.ServerInfo.Name != "wr" || init.ServerInfo.Version == "" {
		t.Errorf("server info: %+v", init.ServerInfo)
	}
	if _, ok := init.Capabilities["tools"]; !ok {
		t.Errorf("no tools capability: %+v", init.Capabilities)
	}
	if init.Instructions == "" {
		t.Error("no instructions")
	}

	// ping
	if m, ok := resp[1].Result.(map[string]any); !ok || len(m) != 0 {
		t.Errorf("ping result: %#v", resp[1].Result)
	}

	// tools/list: every known tool, each with a schema
	var list toolsListResult
	raw, _ = json.Marshal(resp[2].Result)
	if err := json.Unmarshal(raw, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Tools) != len(toolNames) {
		t.Errorf("listed %d tools, want %d", len(list.Tools), len(toolNames))
	}
	for _, name := range toolNames {
		found := false
		for _, tl := range list.Tools {
			if tl.Name == name {
				found = true
				if tl.Description == "" {
					t.Errorf("%s: no description", name)
				}
				if _, ok := tl.InputSchema.(map[string]any); !ok {
					t.Errorf("%s: bad input schema", name)
				}
			}
		}
		if !found {
			t.Errorf("tool %s not listed", name)
		}
	}

	// resources/list answers empty, an unknown method is a proper error
	if m, ok := resp[3].Result.(map[string]any); !ok || len(m) != 0 {
		t.Errorf("resources/list: %#v", resp[3].Result)
	}
	if resp[4].Error == nil || resp[4].Error.Code != rpcMethodNotFound {
		t.Errorf("unknown method: %+v", resp[4].Error)
	}
}

func TestMCPOldProtocolVersion(t *testing.T) {
	resp := mcpTalk(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`)
	var init initializeResult
	raw, _ := json.Marshal(resp[0].Result)
	json.Unmarshal(raw, &init)
	if init.ProtocolVersion != "2024-11-05" {
		t.Errorf("client version not echoed: %q", init.ProtocolVersion)
	}
}

func TestMCPBadRequests(t *testing.T) {
	// an unparseable line has no id, so it cannot be answered: nothing comes
	// back for it, and the server keeps going for the next line.
	resp := mcpTalk(t, `{not json`, `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"wr_nope"}}`)
	if len(resp) != 1 {
		t.Fatalf("got %d responses, want 1", len(resp))
	}
	tr := decodeTool(t, resp[0])
	if !tr.IsError {
		t.Error("calling an unknown tool must be a tool error")
	}
}

// ---- reading tools ----

func TestMCPReadTools(t *testing.T) {
	isolateAll(t)
	p := localPage(t, "# Local Title\n\nHello page body.\n\nA [link](https://x.test/t) here.\n")

	resp := mcpTalk(t,
		call(t, "wr_read", map[string]any{"target": p}),
		call(t, "wr_read", map[string]any{"target": p, "strip_links": true}),
		call(t, "wr_read", map[string]any{"target": ""}),
		call(t, "wr_render", map[string]any{"target": p, "width": 40}),
		call(t, "wr_render", map[string]any{"target": p, "ansi": true}),
		call(t, "wr_links", map[string]any{"target": p}),
		call(t, "wr_resolve_input", map[string]any{"input": "go book"}),
		call(t, "wr_resolve_input", map[string]any{"input": "example.com"}),
		call(t, "wr_resolve_input", map[string]any{"input": p}),
	)

	st := toolStructured(t, resp[0])
	if st["title"] != "Local Title" {
		t.Errorf("title: %v", st["title"])
	}
	if !strings.Contains(st["markdown"].(string), "Hello page body.") {
		t.Errorf("markdown: %v", st["markdown"])
	}
	if st["search"] != false {
		t.Errorf("a local file is not a search: %v", st["search"])
	}
	if !strings.HasPrefix(st["url"].(string), "file://") {
		t.Errorf("url: %v", st["url"])
	}

	stripped := toolStructured(t, resp[1])
	if strings.Contains(stripped["markdown"].(string), "https://x.test/t") {
		t.Error("strip_links kept the url")
	}
	if !strings.Contains(stripped["markdown"].(string), "link") {
		t.Error("strip_links dropped the text")
	}

	if !decodeTool(t, resp[2]).IsError {
		t.Error("an empty target must fail")
	}

	rendered := toolStructured(t, resp[3])
	if rendered["width"].(float64) != 40 {
		t.Errorf("width: %v", rendered["width"])
	}
	if rendered["lines"].(float64) < 3 {
		t.Errorf("lines: %v", rendered["lines"])
	}
	if strings.Contains(rendered["text"].(string), "\x1b[") {
		t.Error("plain render must not keep ANSI")
	}
	if !strings.Contains(toolStructured(t, resp[4])["text"].(string), "\x1b[") {
		t.Error("ansi render must keep ANSI")
	}

	links := toolStructured(t, resp[5])
	if links["count"].(float64) != 1 {
		t.Fatalf("links: %v", links)
	}
	l := links["links"].([]any)[0].(map[string]any)
	if l["url"] != "https://x.test/t" || l["text"] != "link" {
		t.Errorf("link: %v", l)
	}
	if l["kind"] != "web" {
		t.Errorf("kind: %v", l["kind"])
	}

	q := toolStructured(t, resp[6])
	if q["search"] != true || !strings.Contains(q["url"].(string), "duckduckgo") {
		t.Errorf("search resolve: %v", q)
	}
	host := toolStructured(t, resp[7])
	if host["search"] != false || host["url"] != "https://example.com" {
		t.Errorf("host resolve: %v", host)
	}
	file := toolStructured(t, resp[8])
	if file["search"] != false || !strings.HasPrefix(file["url"].(string), "file://") {
		t.Errorf("file resolve: %v", file)
	}
}

// ---- sessions ----

func TestMCPSessionLifecycle(t *testing.T) {
	isolateAll(t)
	a := localSite(t)
	s := newMCPServer(io.Discard)

	sess, err := s.openSession(a, 80, 24, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sess.screen(false), "Page A") {
		t.Errorf("screen: %q", sess.screen(false))
	}
	if sess.state().Links != 0 && sess.state().FocusedLink == "" {
		// the link exists: Page B
	}
	if err := sess.key("tab"); err != nil {
		t.Fatal(err)
	}
	if sess.state().FocusedLink == "" {
		t.Error("tab did not focus a link")
	}
	if err := sess.key("enter"); err != nil {
		t.Fatal(err)
	}
	sess.waitSettled(2e9)
	st := sess.state()
	if !strings.HasSuffix(st.URL, "b.html") {
		t.Errorf("enter did not follow the link: %q", st.URL)
	}
	if len(st.Back) != 1 {
		t.Errorf("back stack: %v", st.Back)
	}
	if err := sess.key("H"); err != nil {
		t.Fatal(err)
	}
	sess.waitSettled(2e9)
	if !strings.HasSuffix(sess.state().URL, "a.html") {
		t.Errorf("H did not go back: %q", sess.state().URL)
	}

	// keys map to the canonical names the bindings use
	for _, name := range []string{"j", "k", "g", "G", "/", "o", "t", "n", "N", "f", "m", ",", "?", "q", "y", "r", "v", "'", "*", "~", "x", "H", "L", "[", "]", "d", "u", "b"} {
		if _, ok := keyPressMsg(name); !ok {
			t.Errorf("key %q not mapped", name)
		}
	}
	for _, name := range []string{"ctrl+d", "ctrl+f", "ctrl+b", "ctrl+u", "shift+tab", "alt+left", "alt+right"} {
		msg, ok := keyPressMsg(name)
		if !ok {
			t.Fatalf("key %q not mapped", name)
		}
		if msg.String() != name {
			t.Errorf("key %q: msg.String() = %q", name, msg.String())
		}
	}

	if err := s.closeSession(sess.id); err != nil {
		t.Fatal(err)
	}
	if !sess.isClosed() {
		t.Error("close did not mark the session closed")
	}
	if _, err := s.getSession(sess.id); err == nil {
		t.Error("a closed session must not resolve")
	}
}

// ---- state tools ----

func TestMCPStateTools(t *testing.T) {
	isolateAll(t)

	resp := mcpTalk(t,
		call(t, "wr_history", map[string]any{}),
		call(t, "wr_bookmarks", map[string]any{}),
		call(t, "wr_bookmark_toggle", map[string]any{"target": "example.com/x", "title": "X"}),
		call(t, "wr_bookmarks", map[string]any{}),
		call(t, "wr_bookmark_toggle", map[string]any{"target": "example.com/x"}),
		call(t, "wr_bookmark_delete", map[string]any{"target": "example.com/x"}),
		call(t, "wr_cache_stats", map[string]any{}),
		call(t, "wr_cache_clear", map[string]any{}),
		call(t, "wr_config_get", map[string]any{}),
		call(t, "wr_config_set", map[string]any{"settings": map[string]any{"width": 120, "animations": false, "links": "inline"}}),
		call(t, "wr_config_get", map[string]any{}),
		call(t, "wr_config_set", map[string]any{"settings": map[string]any{"nope": true}}),
	)

	if h := toolStructured(t, resp[0]); h["count"].(float64) != 0 {
		t.Errorf("history not empty: %v", h)
	}
	if b := toolStructured(t, resp[1]); b["count"].(float64) != 0 {
		t.Errorf("bookmarks not empty: %v", b)
	}

	tog := toolStructured(t, resp[2])
	if tog["bookmarked"] != true || tog["url"] != "https://example.com/x" {
		t.Errorf("toggle: %v", tog)
	}
	if b := toolStructured(t, resp[3]); b["count"].(float64) != 1 {
		t.Errorf("bookmarks after add: %v", b)
	}
	if tog := toolStructured(t, resp[4]); tog["bookmarked"] != false {
		t.Errorf("toggle off: %v", tog)
	}
	if del := toolStructured(t, resp[5]); del["removed"] != "https://example.com/x" {
		t.Errorf("delete: %v", del)
	}

	if c := toolStructured(t, resp[6]); c["pages"].(float64) < 0 {
		t.Errorf("cache stats: %v", c)
	}
	if _, ok := toolStructured(t, resp[7])["cleared_pages"]; !ok {
		t.Error("cache clear did not report")
	}

	cfg := toolStructured(t, resp[8])["config"].(map[string]any)
	if cfg["width"] != float64(100) || cfg["links"] != "footnotes" {
		t.Errorf("default config: %v", cfg)
	}
	set := toolStructured(t, resp[9])
	if set["saved"] != true {
		t.Errorf("save: %v", set)
	}
	if set["config"].(map[string]any)["width"] != float64(120) {
		t.Error("width not applied")
	}
	if toolStructured(t, resp[10])["config"].(map[string]any)["width"] != float64(120) {
		t.Error("width not persisted to disk")
	}
	if !decodeTool(t, resp[11]).IsError {
		t.Error("an unknown setting must fail")
	}
}

func TestMCPUnknownSessionToolError(t *testing.T) {
	resp := mcpTalk(t, call(t, "wr_session_key", map[string]any{"session_id": "nope", "key": "j"}))
	if !decodeTool(t, resp[0]).IsError {
		t.Error("an unknown session must be a tool error")
	}
	resp = mcpTalk(t, call(t, "wr_session_open", map[string]any{"target": ""}))
	if !decodeTool(t, resp[0]).IsError {
		t.Error("an empty target must be a tool error")
	}
}
