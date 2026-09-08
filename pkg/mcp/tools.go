package mcp

import (
	"encoding/json"
	"net/http"

	"github.com/StevenCyb/SnAPI/internal/wrapper"
	"github.com/StevenCyb/SnAPI/pkg/runtime"
)

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
