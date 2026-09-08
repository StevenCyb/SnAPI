package generator

import (
	"regexp"

	"github.com/StevenCyb/SnAPI/internal/models"
)

//nolint:gochecknoglobals
var pathParamRegex = regexp.MustCompile(`\{([^/}]+)\}`)

// pathParameters extracts {name} placeholders from a route path and returns
// them as OpenAPI parameter objects. Manual @snapi.path annotations override
// the auto-derived entry (matched by name) for type/description.
func pathParameters(path string, overrides []models.HandlerParam) []map[string]any {
	matches := pathParamRegex.FindAllStringSubmatch(path, -1)
	overrideByName := map[string]models.HandlerParam{}
	for _, p := range overrides {
		overrideByName[p.Name] = p
	}
	seen := map[string]bool{}
	out := make([]map[string]any, 0, len(matches)+len(overrides))
	for _, m := range matches {
		name := m[1]
		seen[name] = true
		typ := "string"
		var desc *string
		if o, ok := overrideByName[name]; ok {
			if o.Type != "" {
				typ = o.Type
			}
			desc = o.Description
		}
		entry := map[string]any{
			"name":     name,
			"in":       "path",
			"required": true,
			"schema":   map[string]any{"type": typ},
		}
		if desc != nil {
			entry["description"] = *desc
		}
		out = append(out, entry)
	}
	// Manual path params not present in the route placeholder list still get emitted.
	for _, p := range overrides {
		if seen[p.Name] {
			continue
		}
		typ := p.Type
		if typ == "" {
			typ = "string"
		}
		entry := map[string]any{
			"name":     p.Name,
			"in":       "path",
			"required": true,
			"schema":   map[string]any{"type": typ},
		}
		if p.Description != nil {
			entry["description"] = *p.Description
		}
		out = append(out, entry)
	}
	return out
}

// operationParameters returns the combined path + query + header + cookie
// parameters for an operation as OpenAPI parameter objects.
func operationParameters(meta *models.HandlerMeta) []map[string]any {
	params := pathParameters(meta.Path, meta.Paths)
	params = append(params, paramList(meta.Queries, "query")...)
	params = append(params, paramList(meta.Headers, "header")...)
	params = append(params, paramList(meta.Cookies, "cookie")...)
	return params
}

func paramList(items []models.HandlerParam, in string) []map[string]any {
	if len(items) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, p := range items {
		entry := map[string]any{
			"name":     p.Name,
			"in":       in,
			"required": p.Required,
			"schema":   map[string]any{"type": p.Type},
		}
		if p.Description != nil {
			entry["description"] = *p.Description
		}
		out = append(out, entry)
	}
	return out
}
