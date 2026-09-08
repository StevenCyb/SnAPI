package generator

import (
	"fmt"
	"strings"

	"github.com/StevenCyb/SnAPI/internal/models"
)

func handlerSummary(h models.HandlerFunc) string {
	if h.Meta.Summary != nil {
		return *h.Meta.Summary
	}
	return fmt.Sprintf("%s.%s", h.Package, h.Name)
}

func handlerSecurity(meta *models.HandlerMeta) []map[string]any {
	if len(meta.Security) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(meta.Security))
	for _, name := range meta.Security {
		out = append(out, map[string]any{name: []string{}})
	}
	return out
}

func buildServers(servers []models.ProjectServer) []map[string]any {
	if len(servers) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(servers))
	for _, s := range servers {
		entry := map[string]any{"url": s.URL}
		if s.Description != "" {
			entry["description"] = s.Description
		}
		out = append(out, entry)
	}
	return out
}

func buildSecuritySchemes(schemes []models.SecurityScheme) map[string]any {
	if len(schemes) == 0 {
		return nil
	}
	out := map[string]any{}
	for _, s := range schemes {
		entry := map[string]any{"type": s.Type}
		switch strings.ToLower(s.Type) {
		case "http":
			if s.Scheme != "" {
				entry["scheme"] = s.Scheme
			}
			if s.BearerFormat != "" {
				entry["bearerFormat"] = s.BearerFormat
			}
		case "apikey":
			if s.In != "" {
				entry["in"] = s.In
			}
			if s.ParamName != "" {
				entry["name"] = s.ParamName
			}
		}
		out[s.Name] = entry
	}
	return out
}

func handlerDescription(h models.HandlerFunc) string {
	if h.Meta.Description != nil {
		return *h.Meta.Description
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Go Handler: %s\n\n", h.ImportPath)
	if len(h.Services) > 0 {
		fmt.Fprintf(&b, "**Services:** `%s`\n\n", strings.Join(h.Services, "`, `"))
	}
	if len(h.Meta.Middleware) > 0 {
		fmt.Fprintf(&b, "**Middleware Pipeline:** %s", strings.Join(h.Meta.Middleware, " → "))
	}
	return b.String()
}

func handlerTags(h models.HandlerFunc) []string {
	if len(h.Meta.Tags) > 0 {
		return h.Meta.Tags
	}
	return []string{h.Package}
}
