package generator

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"regexp"
	"strconv"
	"strings"
	texttmpl "text/template"

	"github.com/StevenCyb/SnAPI/internal/generator/utils"
	"github.com/StevenCyb/SnAPI/internal/models"
)

//go:embed template/swagger.tmpl
var swaggerTemplateString string

//go:embed template/swagger_ui.tmpl
var swaggerUITemplateString string

//nolint:gochecknoglobals // templates are immutable and safe for reuse
var (
	swaggerTemplate   = texttmpl.Must(texttmpl.New("swagger").Parse(swaggerTemplateString))
	swaggerUITemplate = template.Must(template.New("swagger-ui").Parse(swaggerUITemplateString))
)

type swaggerData struct {
	Path string
	HTML string
	JSON string
}

// generateSwagger writes swagger.go: the registerSwaggerHandlers function
// plus the embedded OpenAPI JSON spec and Swagger UI HTML.
func (g *Generator) generateSwagger() error {
	cfg := g.config.Swagger
	specJSON, err := g.buildOpenAPISpec()
	if err != nil {
		return &SwaggerGenerationError{Reason: "build spec", Err: err}
	}
	html, err := renderSwaggerUI(g.project.Config.Title, specJSON)
	if err != nil {
		return &SwaggerGenerationError{Reason: "render UI", Err: err}
	}

	data := swaggerData{
		Path: cfg.path(),
		HTML: escapeForGoStringLiteral(html),
		JSON: escapeForGoStringLiteral(specJSON),
	}
	if err := utils.RenderToFile(g.dst, swaggerTemplate, "swagger.go", data); err != nil {
		return &SwaggerGenerationError{Reason: "render swagger.go", Err: err}
	}
	return nil
}

// buildOpenAPISpec assembles the OpenAPI 3.0 document from the parsed project.
func (g *Generator) buildOpenAPISpec() (string, error) {
	paths := map[string]any{}
	schemas := map[string]any{}

	addHandler := func(h models.HandlerFunc, routePath string) {
		if h.Meta == nil {
			return
		}
		method := strings.ToLower(h.Meta.Method)
		pathItem, _ := paths[routePath].(map[string]any)
		if pathItem == nil {
			pathItem = map[string]any{}
			paths[routePath] = pathItem
		}
		op := map[string]any{
			"summary":     handlerSummary(h),
			"description": handlerDescription(h),
			"tags":        handlerTags(h),
			"responses":   handlerResponses(g.project.MainModule, h, schemas),
			"deprecated":  h.Meta.Deprecated,
		}
		if h.Meta.OperationID != nil {
			op["operationId"] = *h.Meta.OperationID
		}
		if sec := handlerSecurity(h.Meta); len(sec) > 0 {
			op["security"] = sec
		}
		if params := operationParameters(h.Meta); len(params) > 0 {
			op["parameters"] = params
		}
		if body := requestBody(g.project.MainModule, h, schemas); body != nil {
			op["requestBody"] = body
		}
		pathItem[method] = op
	}

	for _, h := range g.project.HandlerFuncs {
		addHandler(h, h.Meta.Path)
	}

	for _, s := range g.project.HandlerStructs {
		for _, m := range s.Methods {
			addHandler(m, joinPath(s.PathPrefix, m.Meta.Path))
		}
	}

	info := map[string]any{
		"title":       g.project.Config.Title,
		"description": g.project.Config.Description,
		"version":     g.project.Config.Version,
	}
	spec := map[string]any{
		"openapi": "3.0.0",
		"info":    info,
		"paths":   paths,
	}
	if servers := buildServers(g.project.Config.Servers); len(servers) > 0 {
		spec["servers"] = servers
	}
	components := map[string]any{}
	if len(schemas) > 0 {
		components["schemas"] = schemas
	}
	if secSchemes := buildSecuritySchemes(g.project.Config.SecuritySchemes); len(secSchemes) > 0 {
		components["securitySchemes"] = secSchemes
	}
	if len(components) > 0 {
		spec["components"] = components
	}

	out, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

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
			for k, v := range imports {
				effectiveImports[k] = v
			}
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

// renderSwaggerUI returns the minified HTML for the Swagger UI page that
// inlines the given JSON spec.
func renderSwaggerUI(title, specJSON string) (string, error) {
	cfg := map[string]any{
		"spec":                   json.RawMessage(specJSON),
		"deepLinking":            true,
		"docExpansion":           "list",
		"displayRequestDuration": true,
		"layout":                 "BaseLayout",
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := swaggerUITemplate.Execute(&buf, map[string]any{
		"Title":      title,
		"ConfigJSON": template.JS(cfgJSON), //nolint:gosec // configJSON is produced by json.Marshal
	}); err != nil {
		return "", err
	}
	return minifyHTML(buf.String()), nil
}

//nolint:gochecknoglobals
var whitespaceRegex = regexp.MustCompile(`\s+`)

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

func minifyHTML(in string) string {
	s := strings.NewReplacer("\t", " ", "\n", " ", "\r", " ").Replace(in)
	s = whitespaceRegex.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "> <", "><")
	return strings.TrimSpace(s)
}

// escapeForGoStringLiteral produces a string safe to embed between double
// quotes in generated Go source.
func escapeForGoStringLiteral(s string) string {
	q := strconv.Quote(s)
	return q[1 : len(q)-1]
}
