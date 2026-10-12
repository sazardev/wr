package main

// The MCP server: reads newline-delimited JSON-RPC from stdin, writes the
// responses to stdout. Tools are plain functions over the reader's own code
// (load, renderDoc, the Bubble Tea model), so the server exposes exactly what
// the binary does, headless.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
)

type mcpTool struct {
	def  toolDef
	call func(*mcpServer, json.RawMessage) (any, error)
}

type mcpServer struct {
	out *bufio.Writer

	mu     sync.Mutex // guards tools order, sessions and the id counter
	tools  map[string]mcpTool
	sess   map[string]*agentSession
	nextID int
}

func newMCPServer(w io.Writer) *mcpServer {
	return &mcpServer{
		out:   bufio.NewWriter(w),
		tools: map[string]mcpTool{},
		sess:  map[string]*agentSession{},
	}
}

// runMCPServer serves MCP on stdin/stdout until the input ends or the client
// sends a parse error twice in a row.
func runMCPServer() error {
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
	switch req.Method {
	case "initialize":
		s.reply(req.ID, s.initialize(req.Params))
	case "ping":
		s.reply(req.ID, map[string]any{})
	case "tools/list":
		s.reply(req.ID, toolsListResult{Tools: s.toolList()})
	case "tools/call":
		s.reply(req.ID, s.callTool(req.Params))
	case "resources/list", "prompts/list":
		// declared neither in capabilities, but some clients ask anyway.
		s.reply(req.ID, map[string]any{})
	default:
		s.replyError(req.ID, rpcMethodNotFound, "no such method: "+req.Method)
	}
}

func (s *mcpServer) initialize(params json.RawMessage) any {
	var p initializeParams
	_ = json.Unmarshal(params, &p) // a bad params still gets a working server
	version := mcpProtocolVersion
	for _, v := range []string{mcpProtocolVersion, "2025-03-26", "2024-11-05"} {
		if p.ProtocolVersion == v {
			version = v
			break
		}
	}
	s.debugf("client %s %s, protocol %s", p.ClientInfo.Name, p.ClientInfo.Version, p.ProtocolVersion)
	return initializeResult{
		ProtocolVersion: version,
		Capabilities:    map[string]any{"tools": map[string]any{"listChanged": false}},
		ServerInfo:      mcpClientInfo{Name: mcpServerName, Version: versionLine()},
		Instructions: "wr reads web pages and files into formatted Markdown. " +
			"wr_read/wr_render/wr_links fetch and render one page; " +
			"wr_session_* drives the interactive reader headless (keys, then read the screen); " +
			"wr_history/wr_bookmarks/wr_config_* manage the local store. " +
			"A target is an URL, a local file or search words, exactly like the address bar.",
	}
}

func (s *mcpServer) callTool(params json.RawMessage) any {
	var p toolCallParams
	if err := json.Unmarshal(params, &p); err != nil || p.Name == "" {
		return errResult(rpcInvalidParams, "tools/call needs a name")
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

func (s *mcpServer) reply(id json.RawMessage, result any) {
	resp := rpcResponse{JSONRPC: "2.0", ID: id, Result: result}
	data, err := json.Marshal(resp)
	if err != nil {
		s.replyError(id, rpcInternalError, err.Error())
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.out.Write(data)
	s.out.WriteByte('\n')
	s.out.Flush()
}

func (s *mcpServer) replyError(id json.RawMessage, code int, msg string) {
	if id == nil {
		return // a notification cannot be answered
	}
	resp := rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
	data, _ := json.Marshal(resp)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.out.Write(data)
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
