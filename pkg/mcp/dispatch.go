package mcp

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/StevenCyb/SnAPI/internal/wrapper"
	"github.com/StevenCyb/SnAPI/pkg/runtime"
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

func (s *Server) discover() map[string]any {
	caps := map[string]any{}
	if len(s.tools) > 0 {
		caps["tools"] = map[string]any{}
	}
	if len(s.resources) > 0 || len(s.resourceTemplates) > 0 {
		caps["resources"] = map[string]any{}
	}
	if len(s.prompts) > 0 {
		caps["prompts"] = map[string]any{}
	}

	result := map[string]any{
		keyResultType:       resultTypeComplete,
		"supportedVersions": []string{ProtocolVersion},
		"capabilities":      caps,
		"_meta": map[string]any{
			"io.modelcontextprotocol/serverInfo": map[string]any{
				keyName:   s.info.Name,
				"version": s.info.Version,
			},
		},
		keyTTLMs:      defaultListTTLMs,
		keyCacheScope: cacheScopePublic,
	}
	if s.info.Instructions != "" {
		result["instructions"] = s.info.Instructions
	}
	return result
}

func rawOrEmptyObjectSchema(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`{"type":"object","additionalProperties":false}`)
	}
	return raw
}

func (s *Server) listTools() map[string]any {
	tools := make([]map[string]any, 0, len(s.tools))
	for _, t := range s.tools {
		entry := map[string]any{
			keyName:       t.Name,
			"description": t.Description,
			"inputSchema": rawOrEmptyObjectSchema(t.InputSchema),
		}
		if len(t.OutputSchema) > 0 {
			entry["outputSchema"] = t.OutputSchema
		}
		tools = append(tools, entry)
	}
	return map[string]any{keyResultType: resultTypeComplete, "tools": tools, keyTTLMs: defaultListTTLMs, keyCacheScope: cacheScopePublic}
}

func (s *Server) callTool(r *http.Request, params map[string]any) (any, *rpcErrResult) {
	name, _ := params[keyName].(string)
	tool, ok := s.toolByName[name]
	if !ok {
		return nil, &rpcErrResult{CodeInvalidParams, "unknown tool: " + name}
	}

	argsRaw, err := json.Marshal(params["arguments"])
	if err != nil {
		return nil, &rpcErrResult{CodeInternalError, "failed to encode tool arguments"}
	}
	req := wrapper.NewMCPRequest(r, argsRaw, nil)
	resp := wrapper.NewMCPResponse(wrapper.MCPKindTool)

	runtime.Chain(tool.Handler, tool.Middlewares...)(req, resp)

	if resp.ProtocolErr != nil {
		return nil, &rpcErrResult{resp.ProtocolErr.Code, resp.ProtocolErr.Message}
	}

	result := map[string]any{
		keyResultType: resultTypeComplete,
		"content":     resp.Content,
		"isError":     resp.IsError,
	}
	if resp.StructuredContent != nil {
		result["structuredContent"] = resp.StructuredContent
	}
	return result, nil
}

func resourceListEntry(res Resource, uriField string) map[string]any {
	entry := map[string]any{uriField: res.URI, keyName: res.Name}
	if res.Description != "" {
		entry["description"] = res.Description
	}
	if res.MimeType != "" {
		entry["mimeType"] = res.MimeType
	}
	return entry
}

func (s *Server) listResources() map[string]any {
	items := make([]map[string]any, 0, len(s.resources))
	for _, res := range s.resources {
		items = append(items, resourceListEntry(res, "uri"))
	}
	return map[string]any{keyResultType: resultTypeComplete, "resources": items, keyTTLMs: defaultListTTLMs, keyCacheScope: cacheScopePublic}
}

func (s *Server) listResourceTemplates() map[string]any {
	items := make([]map[string]any, 0, len(s.resourceTemplates))
	for _, res := range s.resourceTemplates {
		items = append(items, resourceListEntry(res, "uriTemplate"))
	}
	return map[string]any{keyResultType: resultTypeComplete, "resourceTemplates": items, keyTTLMs: defaultListTTLMs, keyCacheScope: cacheScopePublic}
}

func (s *Server) readResource(r *http.Request, params map[string]any) (any, *rpcErrResult) {
	uri, _ := params["uri"].(string)

	if res, ok := s.resourceByURI[uri]; ok {
		return s.invokeResource(r, res, uri, nil)
	}
	for _, tmpl := range s.resourceTemplates {
		if values, matched := matchTemplate(tmpl.URI, uri); matched {
			return s.invokeResource(r, tmpl, uri, values)
		}
	}
	return nil, &rpcErrResult{CodeInvalidParams, "resource not found: " + uri}
}

func (s *Server) invokeResource(r *http.Request, res Resource, uri string, values map[string]string) (any, *rpcErrResult) {
	req := wrapper.NewMCPRequest(r, nil, values)
	resp := wrapper.NewMCPResponse(wrapper.MCPKindResource)
	resp.ResourceURI = uri
	resp.ResourceMimeType = res.MimeType

	runtime.Chain(res.Handler, res.Middlewares...)(req, resp)

	if resp.ProtocolErr != nil {
		return nil, &rpcErrResult{resp.ProtocolErr.Code, resp.ProtocolErr.Message}
	}
	return map[string]any{
		keyResultType: resultTypeComplete,
		"contents":    resp.Content,
		keyTTLMs:      defaultReadTTLMs,
		keyCacheScope: "private",
	}, nil
}

func (s *Server) listPrompts() map[string]any {
	items := make([]map[string]any, 0, len(s.prompts))
	for _, p := range s.prompts {
		entry := map[string]any{keyName: p.Name}
		if p.Description != "" {
			entry["description"] = p.Description
		}
		if len(p.Args) > 0 {
			args := make([]map[string]any, 0, len(p.Args))
			for _, a := range p.Args {
				argEntry := map[string]any{keyName: a.Name, "required": a.Required}
				if a.Description != "" {
					argEntry["description"] = a.Description
				}
				args = append(args, argEntry)
			}
			entry["arguments"] = args
		}
		items = append(items, entry)
	}
	return map[string]any{keyResultType: resultTypeComplete, "prompts": items, keyTTLMs: defaultListTTLMs, keyCacheScope: cacheScopePublic}
}

func (s *Server) getPrompt(r *http.Request, params map[string]any) (any, *rpcErrResult) {
	name, _ := params[keyName].(string)
	p, ok := s.promptByName[name]
	if !ok {
		return nil, &rpcErrResult{CodeInvalidParams, "unknown prompt: " + name}
	}

	values := map[string]string{}
	if argsMap, ok := params["arguments"].(map[string]any); ok {
		for k, v := range argsMap {
			if str, ok := v.(string); ok {
				values[k] = str
			}
		}
	}

	req := wrapper.NewMCPRequest(r, nil, values)
	resp := wrapper.NewMCPResponse(wrapper.MCPKindPrompt)

	runtime.Chain(p.Handler, p.Middlewares...)(req, resp)

	if resp.ProtocolErr != nil {
		return nil, &rpcErrResult{resp.ProtocolErr.Code, resp.ProtocolErr.Message}
	}

	result := map[string]any{keyResultType: resultTypeComplete, "messages": resp.Content}
	if p.Description != "" {
		result["description"] = p.Description
	}
	return result, nil
}
