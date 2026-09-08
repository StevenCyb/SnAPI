package mcp

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// Default freshness hints for list/read results (the CacheableResult
// shape). Not currently configurable via annotations.
const (
	defaultListTTLMs = 300000 // 5 minutes
	defaultReadTTLMs = 60000  // 1 minute
)

// protocolVersionHeader is "Mcp-Protocol-Version" in canonical MIME header
// form (net/http.Header canonicalizes lookups/writes to this form
// regardless of the casing used, so this matches the spec's
// "MCP-Protocol-Version" on the wire).
const protocolVersionHeader = "Mcp-Protocol-Version"

// Repeated JSON result field names/values.
const (
	keyResultType      = "resultType"
	resultTypeComplete = "complete"
	keyTTLMs           = "ttlMs"
	keyCacheScope      = "cacheScope"
	cacheScopePublic   = "public"
	keyName            = "name"
)

// ServeHTTP implements the MCP endpoint. Only POST is defined by this
// protocol revision - GET and DELETE (used by earlier revisions for the
// SSE stream and session termination, both removed here) get 405.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" && !s.originAllowed(origin) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.handlePost(w, r)
}

func (s *Server) handlePost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONRPCError(w, http.StatusBadRequest, nil, CodeParseError, "failed to read request body")
		return
	}

	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil || req.Method == "" || req.JSONRPC != jsonRPCVersion {
		// A JSON-RPC batch (array body) also lands here: batching was
		// removed in this protocol revision, so it is rejected the same
		// way as any other malformed request.
		writeJSONRPCError(w, http.StatusBadRequest, nil, CodeInvalidRequest, "invalid JSON-RPC request")
		return
	}

	if len(req.ID) == 0 {
		// This protocol revision defines no client-to-server notification
		// over Streamable HTTP; accept and discard per the generic
		// notification rule (202, no body).
		w.WriteHeader(http.StatusAccepted)
		return
	}

	meta, params, err := parseParams(req.Params)
	if err != nil {
		writeJSONRPCError(w, http.StatusBadRequest, req.ID, CodeInvalidParams, "invalid params")
		return
	}

	if herr := s.validateHeaders(r, req, meta, params); herr != nil {
		writeJSONRPCError(w, herr.status, req.ID, herr.code, herr.message)
		return
	}

	result, rpcErr := s.dispatch(r, req.Method, params)
	if rpcErr != nil {
		status := http.StatusOK
		if rpcErr.Code == CodeMethodNotFound {
			status = http.StatusNotFound
		}
		writeJSONRPCError(w, status, req.ID, rpcErr.Code, rpcErr.Message)
		return
	}

	writeJSONRPCResult(w, req.ID, result)
}

func parseParams(raw json.RawMessage) (meta, params map[string]any, err error) {
	params = map[string]any{}
	if len(raw) == 0 {
		return map[string]any{}, params, nil
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, nil, err
	}
	meta, _ = params["_meta"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
	}
	return meta, params, nil
}

type headerErr struct {
	status  int
	code    int
	message string
}

// validateHeaders checks the request-metadata headers this transport
// revision mirrors from the body (MCP-Protocol-Version, Mcp-Method, and -
// for tools/call, resources/read, prompts/get - Mcp-Name) against the body
// values, per the spec's Server Validation rules.
func (s *Server) validateHeaders(r *http.Request, req rpcRequest, meta, params map[string]any) *headerErr {
	headerVersion := r.Header.Get(protocolVersionHeader)
	if headerVersion == "" {
		return &headerErr{http.StatusBadRequest, CodeHeaderMismatch, "missing MCP-Protocol-Version header"}
	}
	if bodyVersion, _ := meta["io.modelcontextprotocol/protocolVersion"].(string); bodyVersion != "" && bodyVersion != headerVersion {
		return &headerErr{http.StatusBadRequest, CodeHeaderMismatch, "MCP-Protocol-Version header does not match request body"}
	}
	if headerVersion != ProtocolVersion {
		return &headerErr{http.StatusBadRequest, CodeUnsupportedProtocolVersion, "unsupported protocol version: " + headerVersion}
	}

	headerMethod := r.Header.Get("Mcp-Method")
	if headerMethod == "" {
		return &headerErr{http.StatusBadRequest, CodeHeaderMismatch, "missing Mcp-Method header"}
	}
	if headerMethod != req.Method {
		return &headerErr{http.StatusBadRequest, CodeHeaderMismatch, "Mcp-Method header does not match request body"}
	}

	if req.Method == "tools/call" || req.Method == "resources/read" || req.Method == "prompts/get" {
		bodyName, _ := params[keyName].(string)
		if bodyName == "" {
			bodyName, _ = params["uri"].(string)
		}
		headerName := decodeMcpName(r.Header.Get("Mcp-Name"))
		if headerName == "" {
			return &headerErr{http.StatusBadRequest, CodeHeaderMismatch, "missing Mcp-Name header"}
		}
		if headerName != bodyName {
			return &headerErr{http.StatusBadRequest, CodeHeaderMismatch, "Mcp-Name header does not match request body"}
		}
	}
	return nil
}

// decodeMcpName decodes the "=?base64?...?=" sentinel encoding used for
// Mcp-Name values that aren't safely representable as plain header text.
func decodeMcpName(v string) string {
	const prefix, suffix = "=?base64?", "?="
	if strings.HasPrefix(v, prefix) && strings.HasSuffix(v, suffix) {
		if dec, err := base64.StdEncoding.DecodeString(v[len(prefix) : len(v)-len(suffix)]); err == nil {
			return string(dec)
		}
	}
	return v
}

func (s *Server) dispatch(r *http.Request, method string, params map[string]any) (any, *rpcErrResult) {
	switch method {
	case "server/discover":
		return s.discover(), nil
	case "tools/list":
		return s.listTools(), nil
	case "tools/call":
		return s.callTool(r, params)
	case "resources/list":
		return s.listResources(), nil
	case "resources/templates/list":
		return s.listResourceTemplates(), nil
	case "resources/read":
		return s.readResource(r, params)
	case "prompts/list":
		return s.listPrompts(), nil
	case "prompts/get":
		return s.getPrompt(r, params)
	default:
		return nil, &rpcErrResult{CodeMethodNotFound, "method not found: " + method}
	}
}
