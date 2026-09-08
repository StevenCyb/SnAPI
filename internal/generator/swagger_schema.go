package generator

import (
	"maps"
	"strings"

	"github.com/StevenCyb/SnAPI/internal/models"
)

// modelSchemaRef registers the model with the schema registry (if not already
// present) and returns an OpenAPI schema referencing it. A leading "[]"
// produces an array schema wrapping the model ref.
// selfPkg / selfImportPath represent the handler's own package so that types
// like "api.Book" can be resolved even when the package doesn't import itself.
func modelSchemaRef(mod *models.Module, imports map[string]string, selfPkg, selfImportPath string, schemas map[string]any, ref string) map[string]any {
	isArray := strings.HasPrefix(ref, "[]")
	ref = strings.TrimPrefix(ref, "[]")

	modelName := ref
	alias := ""
	if idx := strings.LastIndex(modelName, "."); idx >= 0 {
		alias, modelName = modelName[:idx], modelName[idx+1:]
	}
	if alias == "" {
		if schema, ok := basicTypeSchema(modelName); ok {
			if isArray {
				return map[string]any{"type": "array", "items": schema}
			}
			return schema
		}
	}

	// Build an effective import map that includes the handler's own package so
	// that references like "api.Book" (same package as the handler) resolve.
	effectiveImports := imports
	if selfPkg != "" && selfImportPath != "" {
		if _, ok := imports[selfPkg]; !ok {
			effectiveImports = make(map[string]string, len(imports)+1)
			maps.Copy(effectiveImports, imports)
			effectiveImports[selfPkg] = selfImportPath
		}
	}

	if _, exists := schemas[modelName]; !exists {
		if s, err := resolveModelSchema(mod, effectiveImports, alias, modelName, openAPISchemaRefPrefix, schemas); err == nil && s != nil {
			schemas[modelName] = s
		}
	}
	item := map[string]any{"$ref": "#/components/schemas/" + modelName}
	if isArray {
		return map[string]any{"type": "array", "items": item}
	}
	return item
}

func handlerResponses(mod *models.Module, h models.HandlerFunc, schemas map[string]any) map[string]any {
	out := map[string]any{}
	for _, st := range h.Meta.Status {
		desc := ""
		if st.Description != nil {
			desc = *st.Description
		}
		out[st.Code] = map[string]any{"description": desc}
	}

	for _, resp := range h.Meta.Responses {
		entry, _ := out[resp.Code].(map[string]any)
		if entry == nil {
			entry = map[string]any{"description": ""}
			out[resp.Code] = entry
		}
		content, _ := entry["content"].(map[string]any)
		if content == nil {
			content = map[string]any{}
			entry["content"] = content
		}
		content[resp.ContentType] = map[string]any{
			"schema": modelSchemaRef(mod, h.Imports, h.Package, h.ImportPath, schemas, resp.Model),
		}
	}

	for _, rh := range h.Meta.ResponseHeaders {
		entry, _ := out[rh.Code].(map[string]any)
		if entry == nil {
			entry = map[string]any{"description": ""}
			out[rh.Code] = entry
		}
		headers, _ := entry["headers"].(map[string]any)
		if headers == nil {
			headers = map[string]any{}
			entry["headers"] = headers
		}
		header := map[string]any{
			"schema": map[string]any{"type": rh.Type},
		}
		if rh.Description != nil {
			header["description"] = *rh.Description
		}
		headers[rh.Name] = header
	}

	if len(out) == 0 {
		return map[string]any{
			"200": map[string]any{"description": "Success"},
			"500": map[string]any{"description": "Internal Server Error"},
		}
	}
	return out
}

func requestBody(mod *models.Module, h models.HandlerFunc, schemas map[string]any) map[string]any {
	method := strings.ToLower(h.Meta.Method)
	if len(h.Meta.Requests) == 0 {
		if method != "post" && method != "put" && method != "patch" {
			return nil
		}
		return map[string]any{
			"required": true,
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{"type": "object"},
				},
			},
		}
	}

	content := map[string]any{}
	for _, req := range h.Meta.Requests {
		content[req.ContentType] = map[string]any{
			"schema": modelSchemaRef(mod, h.Imports, h.Package, h.ImportPath, schemas, req.Model),
		}
	}
	return map[string]any{
		"required": true,
		"content":  content,
	}
}
