package mcp

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
