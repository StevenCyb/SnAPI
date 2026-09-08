package mcp

import (
	"net/http"

	"github.com/StevenCyb/SnAPI/internal/wrapper"
	"github.com/StevenCyb/SnAPI/pkg/runtime"
)

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
