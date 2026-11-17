package main

// The MCP server: reads newline-delimited JSON-RPC from stdin, writes the
// responses to stdout. Tools are plain functions over the reader's own code
// (load, renderDoc, the Bubble Tea model), so the server exposes exactly what
// the binary does, headless.

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

type mcpTool struct {
	def  toolDef
	call func(*mcpServer, json.RawMessage) (any, error)
}

// mcpReadOnly is set once by runMCPServer. The core store helpers consult it so
// a headless session cannot write through a key either.
var mcpReadOnly bool

// mcpWriteTools are the tools that change persistent state. In read-only mode
// they are neither listed nor callable.
var mcpWriteTools = map[string]bool{
	"wr_bookmark_toggle": true,
	"wr_bookmark_delete": true,
	"wr_history_delete":  true,
	"wr_history_clear":   true,
	"wr_cache_clear":     true,
	"wr_config_set":      true,
}

var errReadOnly = errors.New("read-only MCP mode (WR_MCP_READONLY=1)")

type mcpServer struct {
	out *bufio.Writer

	mu       sync.Mutex // guards tools order, sessions and the id counter
	tools    map[string]mcpTool
	sess     map[string]*agentSession
	nextID   int
	readOnly bool // WR_MCP_READONLY: no tool may change the local store
}

func newMCPServer(w io.Writer) *mcpServer {
	return &mcpServer{
		out:      bufio.NewWriter(w),
		tools:    map[string]mcpTool{},
		sess:     map[string]*agentSession{},
		readOnly: mcpReadOnly,
	}
}

// runMCPServer serves MCP on stdin/stdout until the input ends or the client
// sends a parse error twice in a row.
func runMCPServer() error {
	cfg, _ := loadConfig()
	applyRuntime(cfg)
	mcpReadOnly = os.Getenv("WR_MCP_READONLY") != ""
	if mcpReadOnly {
		cacheOn = false // reads are fine, but do not write page data to disk
	}
	return newMCPServer(os.Stdout).serve(os.Stdin)
}

// serve reads newline-delimited JSON-RPC from in until it ends, answering
// every request on the spot.
func (s *mcpServer) serve(in io.Reader) error {
	registerTools(s)

	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 8<<20) // tool arguments can be long
	bad := 0
	for sc.Scan() {
		line := sc.Bytes()
		if len(trimSpace(line)) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil || req.Method == "" {
			s.replyError(req.IDOf(), rpcParseError, "could not parse the request")
			if bad++; bad >= 2 {
				return nil
			}
			continue
		}
		bad = 0
		if req.IsNotification() {
			// notifications/initialized and friends: nothing to answer.
			s.debugf("notification %s", req.Method)
			continue
		}
		s.dispatch(&req)
	}
	return sc.Err()
}

func (s *mcpServer) dispatch(req *rpcRequest) {
	modern, code, msg, data := s.requestEra(req)
	if code != 0 {
		s.replyErrorData(req.IDOf(), code, msg, data)
		return
	}
	switch req.Method {
	case "initialize":
		// initialize always selects the legacy handshake, even if the client
		// also sent modern _meta by mistake.
		s.reply(req, s.initialize(req.Params), false)
	case "server/discover":
		s.reply(req, s.discover(), true)
	case "ping":
		if modern {
			s.replyError(req.IDOf(), rpcMethodNotFound, "ping was removed in protocol 2026-07-28")
			return
		}
		s.reply(req, map[string]any{}, false)
	case "tools/list":
		list := toolsListResult{Tools: s.toolList()}
		if modern {
			list.TTLMs = 300000
			list.CacheScope = "public"
		}
		s.reply(req, list, modern)
	case "tools/call":
		s.reply(req, s.callTool(req.Params), modern)
	case "resources/list", "prompts/list":
		// declared neither in capabilities, but some clients ask anyway.
		s.reply(req, map[string]any{}, modern)
	default:
		s.replyError(req.IDOf(), rpcMethodNotFound, "no such method: "+req.Method)
	}
}

// requestEra decides whether a request uses the stateless per-request metadata
// or the old initialize handshake. A request without _meta is legacy, so every
// existing client keeps working; a modern request is validated before it is
// dispatched, as the 2026-07-28 revision requires.
func (s *mcpServer) requestEra(req *rpcRequest) (modern bool, code int, msg string, data any) {
	if req.Method == "initialize" {
		return false, 0, "", nil
	}
	raw, ok := jsonField(req.Params, "_meta")
	if !ok || len(raw) == 0 || string(raw) == "null" {
		if req.Method == "server/discover" {
			return false, rpcInvalidParams, "server/discover requires the 2026-07-28 per-request _meta fields", nil
		}
		return false, 0, "", nil
	}
	var meta requestMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return false, rpcInvalidParams, "_meta must be an object with io.modelcontextprotocol/* fields", nil
	}
	if meta.ProtocolVersion == "" {
		return false, rpcInvalidParams, "_meta is missing " + metaProtocolVersion, nil
	}
	if meta.ProtocolVersion != mcpProtocolVersion {
		return false, rpcUnsupportedProtocolVersion, "Unsupported protocol version",
			map[string]any{"supported": mcpModernVersions, "requested": meta.ProtocolVersion}
	}
	if len(meta.ClientCapabilities) == 0 || string(meta.ClientCapabilities) == "null" {
		return false, rpcInvalidParams, "_meta is missing " + metaClientCapabilities, nil
	}
	var caps map[string]any
	if err := json.Unmarshal(meta.ClientCapabilities, &caps); err != nil {
		return false, rpcInvalidParams, metaClientCapabilities + " must be an object", nil
	}
	return true, 0, "", nil
}

func (s *mcpServer) initialize(params json.RawMessage) any {
	var p initializeParams
	_ = json.Unmarshal(params, &p) // a bad params still gets a working server
	version := mcpLegacyVersion
	for _, v := range mcpLegacyVersions {
		if p.ProtocolVersion == v {
			version = v
			break
		}
	}
	s.debugf("client %s %s, protocol %s", p.ClientInfo.Name, p.ClientInfo.Version, p.ProtocolVersion)
	return initializeResult{
		ProtocolVersion: version,
		Capabilities:    mcpCapabilities(),
		ServerInfo:      mcpClientInfo{Name: mcpServerName, Version: versionLine()},
		Instructions:    mcpInstructions(),
	}
}

// discover is the mandatory modern entry point: versions, capabilities and
// identity in one cacheable result.
func (s *mcpServer) discover() discoverResult {
	return discoverResult{
		SupportedVersions: mcpModernVersions,
		Capabilities:      mcpCapabilities(),
		Instructions:      mcpInstructions(),
		TTLMs:             3600000,
		CacheScope:        "public",
	}
}

func mcpCapabilities() map[string]any {
	return map[string]any{"tools": map[string]any{"listChanged": false}}
}

func mcpInstructions() string {
	return "wr reads web pages and files into formatted Markdown. " +
		"wr_read/wr_render/wr_links fetch and render one page; " +
		"wr_session_* drives the interactive reader headless (keys, then read the screen); " +
		"wr_history/wr_bookmarks/wr_config_* manage the local store. " +
		"A target is an URL, a local file or search words, exactly like the address bar."
}

func (s *mcpServer) callTool(params json.RawMessage) any {
	var p toolCallParams
	if err := json.Unmarshal(params, &p); err != nil || p.Name == "" {
		return errResult(rpcInvalidParams, "tools/call needs a name")
	}
	if s.readOnly && mcpWriteTools[p.Name] {
		return errResult(rpcInvalidParams, p.Name+" is disabled: "+errReadOnly.Error())
	}
	s.mu.Lock()
	t, ok := s.tools[p.Name]
	s.mu.Unlock()
	if !ok {
		return errResult(rpcInvalidParams, "no such tool: "+p.Name)
	}
	s.debugf("tool %s", p.Name)
	res, err := t.call(s, p.Arguments)
	if err != nil {
		return toolResult{
			Content:           []textContent{{Type: "text", Text: err.Error()}},
			StructuredContent: map[string]any{"error": err.Error()},
			IsError:           true,
		}
	}
	out := toolResult{Content: []textContent{{Type: "text", Text: renderJSON(res)}}}
	if res != nil {
		out.StructuredContent = res
	}
	return out
}

func (s *mcpServer) toolList() []toolDef {
	names := toolNames
	out := make([]toolDef, 0, len(names))
	s.mu.Lock()
	for _, n := range names {
		if s.readOnly && mcpWriteTools[n] {
			continue
		}
		if t, ok := s.tools[n]; ok {
			out = append(out, t.def)
		}
	}
	s.mu.Unlock()
	return out
}

func (s *mcpServer) register(t mcpTool) {
	s.mu.Lock()
	s.tools[t.def.Name] = t
	s.mu.Unlock()
}

// ---- writing ----

func (s *mcpServer) reply(req *rpcRequest, result any, modern bool) {
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	if modern {
		raw, err := modernResult(result)
		if err != nil {
			s.replyError(req.ID, rpcInternalError, err.Error())
			return
		}
		resp.Result = raw
	} else {
		resp.Result = result
	}
	data, err := json.Marshal(resp)
	if err != nil {
		s.replyError(req.ID, rpcInternalError, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.out.Write(data)
	s.out.WriteByte('\n')
	s.out.Flush()
}

// modernResult adds the required resultType and the server identity in _meta to
// a result. The protocol keeps them out of the legacy responses.
func modernResult(result any) (json.RawMessage, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	if _, ok := obj["resultType"]; !ok {
		obj["resultType"] = json.RawMessage(`"complete"`)
	}
	meta := map[string]json.RawMessage{}
	if raw, ok := obj["_meta"]; ok {
		_ = json.Unmarshal(raw, &meta)
	}
	if _, ok := meta[metaServerInfo]; !ok {
		info, err := json.Marshal(mcpClientInfo{Name: mcpServerName, Version: versionLine()})
		if err != nil {
			return nil, err
		}
		meta[metaServerInfo] = info
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	obj["_meta"] = metaJSON
	return json.Marshal(obj)
}

func (s *mcpServer) replyError(id json.RawMessage, code int, msg string) {
	s.replyErrorData(id, code, msg, nil)
}

func (s *mcpServer) replyErrorData(id json.RawMessage, code int, msg string, data any) {
	if id == nil {
		return // a notification cannot be answered
	}
	resp := rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg, Data: data}}
	payload, _ := json.Marshal(resp)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.out.Write(payload)
	s.out.WriteByte('\n')
	s.out.Flush()
}

func errResult(code int, msg string) any {
	return toolResult{
		Content:           []textContent{{Type: "text", Text: msg}},
		StructuredContent: map[string]any{"error": msg},
		IsError:           true,
	}
}

func (s *mcpServer) debugf(format string, args ...any) {
	if os.Getenv("WR_MCP_DEBUG") == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "wr-mcp: "+format+"\n", args...)
}

// ---- small protocol helpers ----

// jsonField returns one raw field of a JSON object, if params is an object and
// the field exists.
func jsonField(raw json.RawMessage, name string) (json.RawMessage, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, false
	}
	v, ok := obj[name]
	return v, ok
}

func (r rpcRequest) IsNotification() bool { return len(r.ID) == 0 || string(r.ID) == "null" }

func (r rpcRequest) IDOf() json.RawMessage {
	if r.IsNotification() {
		return nil
	}
	return r.ID
}

func trimSpace(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && (b[i] == ' ' || b[i] == '\t' || b[i] == '\r' || b[i] == '\n') {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\t' || b[j-1] == '\r' || b[j-1] == '\n') {
		j--
	}
	return b[i:j]
}

// renderJSON is one compact JSON document (the text half of a tool result).
func renderJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(data)
}
