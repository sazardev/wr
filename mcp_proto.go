package main

import "encoding/json"

// MCP (Model Context Protocol) server: JSON-RPC 2.0 over stdio,
// newline-delimited. See https://modelcontextprotocol.io — this file holds the
// wire types only; the loop lives in mcp.go and the tools in mcp_tools.go,
// mcp_session.go and mcp_state.go.
//
// stdout carries the protocol and nothing else (an agent parses it), so the
// debug log goes to stderr and only when WR_MCP_DEBUG is set.

const (
	mcpProtocolVersion = "2025-06-18"
	mcpServerName      = "wr"

	// supported protocol versions, newest first: the client's is echoed back
	// when known, so old clients keep working.
	mcpCompatVersions = "2024-11-05, 2025-03-26, 2025-06-18"
)

// JSON-RPC 2.0 envelope.

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"` // absent or null = notification
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

const (
	rpcParseError     = -32700
	rpcInvalidRequest = -32600
	rpcMethodNotFound = -32601
	rpcInvalidParams  = -32602
	rpcInternalError  = -32603
)

// MCP initialize.

type mcpClientInfo struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
	ClientInfo      mcpClientInfo  `json:"clientInfo,omitempty"`
}

type initializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      mcpClientInfo  `json:"serverInfo"`
	Instructions    string         `json:"instructions,omitempty"`
}

// MCP tools.

type toolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

type toolsListResult struct {
	Tools []toolDef `json:"tools"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolResult struct {
	Content           []textContent `json:"content"`
	StructuredContent any           `json:"structuredContent,omitempty"`
	IsError           bool          `json:"isError,omitempty"`
}

func textResult(text string) toolResult {
	return toolResult{Content: []textContent{{Type: "text", Text: text}}}
}

// jsonSchema is the subset of JSON Schema a tool needs. Properties is kept as
// raw JSON so each tool writes its schema inline.
type jsonSchema = map[string]any

func objectSchema(properties jsonSchema, required ...string) jsonSchema {
	s := jsonSchema{"type": "object", "properties": properties}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func strProp(desc string) jsonSchema { return jsonSchema{"type": "string", "description": desc} }
func boolProp(desc string) jsonSchema {
	return jsonSchema{"type": "boolean", "description": desc}
}
func intProp(desc string) jsonSchema { return jsonSchema{"type": "number", "description": desc} }
