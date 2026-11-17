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
	for _, version := range []string{"2025-11-25", "2024-11-05"} {
		resp := mcpTalk(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"`+version+`"}}`)
		var init initializeResult
		raw, _ := json.Marshal(resp[0].Result)
		json.Unmarshal(raw, &init)
		if init.ProtocolVersion != version {
			t.Errorf("client version not echoed: %q, want %q", init.ProtocolVersion, version)
		}
	}
}

// modernParams builds the per-request _meta of the stateless revision.
func modernParams(extra string) string {
	if extra != "" {
		extra = "," + extra
	}
	return `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28",` +
		`"io.modelcontextprotocol/clientInfo":{"name":"test","version":"1"},` +
		`"io.modelcontextprotocol/clientCapabilities":{}` + extra + `}`
}

func resultMap(t *testing.T, resp rpcResponse) map[string]any {
	t.Helper()
	if resp.Error != nil {
		t.Fatalf("rpc error: %+v", resp.Error)
	}
	m, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result is %T, want an object", resp.Result)
	}
	return m
}

func TestMCPModernDiscover(t *testing.T) {
	resp := mcpTalk(t, `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{`+modernParams("")+`}}`)
	if len(resp) != 1 || resp[0].Error != nil {
		t.Fatalf("discover: %+v", resp)
	}
	m := resultMap(t, resp[0])
	if m["resultType"] != "complete" {
		t.Errorf("resultType: %#v", m["resultType"])
	}
	versions, ok := m["supportedVersions"].([]any)
	if !ok || len(versions) != 1 || versions[0] != "2026-07-28" {
		t.Errorf("supportedVersions: %#v", m["supportedVersions"])
	}
	if caps, ok := m["capabilities"].(map[string]any); !ok || caps["tools"] == nil {
		t.Errorf("capabilities: %#v", m["capabilities"])
	}
	meta, ok := m["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("_meta: %#v", m["_meta"])
	}
	info, ok := meta["io.modelcontextprotocol/serverInfo"].(map[string]any)
	if !ok || info["name"] != "wr" || info["version"] == "" {
		t.Errorf("serverInfo: %#v", meta)
	}
	if m["ttlMs"].(float64) <= 0 || m["cacheScope"] != "public" {
		t.Errorf("cache hints: ttl=%v scope=%v", m["ttlMs"], m["cacheScope"])
	}
}

func TestMCPModernTools(t *testing.T) {
	resp := mcpTalk(t,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{`+modernParams("")+`}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"wr_resolve_input",`+
			`"arguments":{"input":"example.com"},`+modernParams("")+`}}`,
	)
	list := resultMap(t, resp[0])
	if list["resultType"] != "complete" || list["cacheScope"] != "public" {
		t.Errorf("tools/list: %#v", list)
	}
	if _, ok := list["ttlMs"].(float64); !ok {
		t.Errorf("tools/list has no ttlMs: %#v", list)
	}
	if tools, ok := list["tools"].([]any); !ok || len(tools) != len(toolNames) {
		t.Errorf("tools: %#v", list["tools"])
	}
	call := resultMap(t, resp[1])
	if call["resultType"] != "complete" {
		t.Errorf("tools/call resultType: %#v", call["resultType"])
	}
	sc, ok := call["structuredContent"].(map[string]any)
	if !ok || sc["url"] != "https://example.com" {
		t.Errorf("tools/call: %#v", call)
	}
	if meta, ok := call["_meta"].(map[string]any); !ok || meta["io.modelcontextprotocol/serverInfo"] == nil {
		t.Errorf("tools/call _meta: %#v", call["_meta"])
	}
}

func TestMCPModernErrors(t *testing.T) {
	badVersion := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"1900-01-01",` +
		`"io.modelcontextprotocol/clientCapabilities":{}}}}`
	noCaps := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{"_meta":{` +
		`"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`
	noMeta := `{"jsonrpc":"2.0","id":3,"method":"server/discover"}`
	ping := `{"jsonrpc":"2.0","id":4,"method":"ping","params":{` + modernParams("") + `}}`
	resp := mcpTalk(t, badVersion, noCaps, noMeta, ping)

	if resp[0].Error == nil || resp[0].Error.Code != rpcUnsupportedProtocolVersion {
		t.Fatalf("unsupported version: %+v", resp[0].Error)
	}
	data, ok := resp[0].Error.Data.(map[string]any)
	if !ok || data["requested"] != "1900-01-01" {
		t.Errorf("unsupported version data: %#v", resp[0].Error.Data)
	}
	if supported, ok := data["supported"].([]any); !ok || len(supported) != 1 || supported[0] != "2026-07-28" {
		t.Errorf("supported versions: %#v", data["supported"])
	}
	if resp[1].Error == nil || resp[1].Error.Code != rpcInvalidParams {
		t.Errorf("missing capabilities: %+v", resp[1].Error)
	}
	if resp[2].Error == nil || resp[2].Error.Code != rpcInvalidParams {
		t.Errorf("discover without _meta: %+v", resp[2].Error)
	}
	if resp[3].Error == nil || resp[3].Error.Code != rpcMethodNotFound {
		t.Errorf("modern ping: %+v", resp[3].Error)
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

func TestMCPReadOnly(t *testing.T) {
	isolateAll(t)
	old := mcpReadOnly
	mcpReadOnly = true
	oldCache := cacheOn
	cacheOn = true
	t.Cleanup(func() { mcpReadOnly = old; cacheOn = oldCache })

	resp := mcpTalk(t,
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`,
		call(t, "wr_config_set", map[string]any{"settings": map[string]any{"width": 90}}),
		call(t, "wr_bookmark_toggle", map[string]any{"target": "https://a.test"}),
	)
	list := resultMap(t, resp[0])
	listed, _ := list["tools"].([]any)
	for _, raw := range listed {
		tool, _ := raw.(map[string]any)
		name, _ := tool["name"].(string)
		if mcpWriteTools[name] {
			t.Errorf("read-only mode listed a writing tool: %s", name)
		}
	}
	if !decodeTool(t, resp[1]).IsError {
		t.Error("wr_config_set must be refused in read-only mode")
	}
	if !decodeTool(t, resp[2]).IsError {
		t.Error("wr_bookmark_toggle must be refused in read-only mode")
	}

	// The core helpers are guarded too, so a headless session cannot write
	// through a key or a menu entry.
	if err := cachePut("https://a.test", "body"); err != nil {
		t.Errorf("cachePut: %v", err)
	}
	if _, _, ok := cacheGet("https://a.test"); ok {
		t.Error("read-only mode must not write page data to the cache")
	}
	if err := historyAdd("https://a.test", "A", 100); err != nil {
		t.Errorf("historyAdd: %v", err)
	}
	if n := len(readHistory()); n != 0 {
		t.Errorf("read-only mode wrote %d history entries", n)
	}
	if _, err := bookmarkToggle("https://a.test", "A"); err != errReadOnly {
		t.Errorf("bookmarkToggle: %v", err)
	}
	if err := saveConfig(defaultConfig()); err != errReadOnly {
		t.Errorf("saveConfig: %v", err)
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
