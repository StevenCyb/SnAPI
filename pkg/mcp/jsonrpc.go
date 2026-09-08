// Package mcp implements the server side of the MCP (Model Context
// Protocol) Streamable HTTP transport, spec revision 2026-07-28. This
// revision is stateless per request: there is no session, no `initialize`
// handshake, and no `Mcp-Session-Id`. See https://modelcontextprotocol.io/specification/2026-07-28
// for the authoritative spec.
package mcp

import (
	"encoding/json"
	"net/http"
)

// ProtocolVersion is the MCP spec revision this server implements.
const ProtocolVersion = "2026-07-28"

const jsonRPCVersion = "2.0"

// JSON-RPC 2.0 error codes, plus the MCP-reserved sub-range (-32020..-32099).
const (
	CodeParseError                 = -32700
	CodeInvalidRequest             = -32600
	CodeMethodNotFound             = -32601
	CodeInvalidParams              = -32602
	CodeInternalError              = -32603
	CodeHeaderMismatch             = -32020
	CodeUnsupportedProtocolVersion = -32022
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// rpcErrResult is an application-level JSON-RPC error produced while
// dispatching a request (as opposed to a transport/header validation error,
// which is built inline where it's detected).
type rpcErrResult struct {
	Code    int
	Message string
}

func writeJSONRPCResult(w http.ResponseWriter, id json.RawMessage, result any) {
	writeJSON(w, http.StatusOK, rpcResponse{JSONRPC: jsonRPCVersion, ID: id, Result: result})
}

func writeJSONRPCError(w http.ResponseWriter, status int, id json.RawMessage, code int, message string) {
	writeJSON(w, status, rpcResponse{JSONRPC: jsonRPCVersion, ID: id, Error: &rpcError{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, v rpcResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// The response is already written (status line + headers); an encode
	// failure here (a non-serializable value reaching Result, which is
	// only ever a map/slice/RawMessage built by this package) can't be
	// reported to the client at this point.
	if err := json.NewEncoder(w).Encode(v); err != nil {
		return
	}
}
