package mcp

import (
	"net/http"

	"github.com/StevenCyb/SnAPI/internal/wrapper"
	"github.com/StevenCyb/SnAPI/pkg/runtime"
)

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
