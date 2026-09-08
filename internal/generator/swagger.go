package generator

import (
	_ "embed"
	"encoding/json"
	"strings"
	texttmpl "text/template"

	"github.com/StevenCyb/SnAPI/internal/generator/utils"
	"github.com/StevenCyb/SnAPI/internal/models"
)

//go:embed template/swagger.tmpl
var swaggerTemplateString string

//nolint:gochecknoglobals // templates are immutable and safe for reuse
var swaggerTemplate = texttmpl.Must(texttmpl.New("swagger").Parse(swaggerTemplateString))

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
		g.addOpenAPIOperation(paths, schemas, h, routePath)
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

func (g *Generator) addOpenAPIOperation(paths, schemas map[string]any, h models.HandlerFunc, routePath string) {
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
