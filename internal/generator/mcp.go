package generator

import (
	_ "embed"
	"encoding/json"
	"maps"
	"strings"
	"text/template"

	"github.com/StevenCyb/SnAPI/internal/generator/utils"
	"github.com/StevenCyb/SnAPI/internal/models"
)

const (
	mcpSchemaRefPrefix = "#/$defs/"
	defaultMCPEndpoint = "/mcp"
)

//go:embed template/mcp.tmpl
var mcpTemplateString string

//nolint:gochecknoglobals // template is immutable and safe for reuse
var mcpTemplate = template.Must(template.New("mcp").Parse(mcpTemplateString))

type mcpData struct {
	Imports        []models.ProjectImport
	NeedsJSON      bool
	Endpoint       string
	ServerName     string
	ServerVersion  string
	Instructions   string
	AllowedOrigins []string
	Tools          []mcpToolEntry
	Resources      []mcpResourceEntry
	Prompts        []mcpPromptEntry
}

type mcpToolEntry struct {
	ToolName     string
	Description  string
	InputSchema  string
	OutputSchema string
	Package      string
	FuncName     string
	ReceiverVar  string
	Middlewares  []middlewareRef
}

type mcpResourceEntry struct {
	URI         string
	ResName     string
	Description string
	MimeType    string
	Package     string
	FuncName    string
	ReceiverVar string
	Middlewares []middlewareRef
}

type mcpPromptEntry struct {
	PromptName  string
	Description string
	Args        []mcpPromptArgEntry
	Package     string
	FuncName    string
	ReceiverVar string
	Middlewares []middlewareRef
}

type mcpPromptArgEntry struct {
	Name        string
	Description string
	Required    bool
}

// hasMCPFeature reports whether the project declares at least one
// @SnAPI.MCPTool, @SnAPI.MCPResource or @SnAPI.MCPPrompt.
func hasMCPFeature(p *models.Project) bool {
	for _, h := range p.HandlerFuncs {
		if h.MCPTool != nil || h.MCPResource != nil || h.MCPPrompt != nil {
			return true
		}
	}
	for _, s := range p.HandlerStructs {
		for _, m := range s.Methods {
			if m.MCPTool != nil || m.MCPResource != nil || m.MCPPrompt != nil {
				return true
			}
		}
	}
	return false
}

// generateMCP writes mcp.go: the registerMCPServer function that builds an
// mcp.Server from every discovered @SnAPI.MCPTool / @SnAPI.MCPResource /
// @SnAPI.MCPPrompt and mounts it on the mux.
func (g *Generator) generateMCP() error {
	c := &mcpCollector{lookup: middlewareLookup(g.project.MiddlewareFuncs)}

	for _, h := range g.project.HandlerFuncs {
		if err := c.collect(g, h, ""); err != nil {
			return &MCPGenerationError{Reason: "resolve tool/resource/prompt", Err: err}
		}
	}
	for _, s := range g.project.HandlerStructs {
		for _, m := range s.Methods {
			if err := c.collect(g, m, s.VarName); err != nil {
				return &MCPGenerationError{Reason: "resolve tool/resource/prompt", Err: err}
			}
		}
	}

	if err := utils.RenderToFile(g.dst, mcpTemplate, "mcp.go", c.buildData(g)); err != nil {
		return &MCPGenerationError{Reason: "render mcp.go", Err: err}
	}
	return nil
}

// mcpCollector accumulates tool/resource/prompt entries (and the package
// imports mcp.go needs) while walking the project's handler funcs/structs.
type mcpCollector struct {
	lookup    map[string]models.MiddlewareFunc
	tools     []mcpToolEntry
	resources []mcpResourceEntry
	prompts   []mcpPromptEntry
	pkgRefs   []models.ProjectImport
}

// addMiddlewareImports records the import of every middleware package
// mcp.go's own generated body references (each generated file needs its own
// import block - a middleware already imported by routes.go isn't
// automatically available here).
func (c *mcpCollector) addMiddlewareImports(refs []middlewareRef) {
	for _, ref := range refs {
		if mw, ok := c.lookup[ref.Package+"."+ref.Name]; ok {
			c.pkgRefs = append(c.pkgRefs, models.ProjectImport{Alias: mw.Package, Path: mw.ImportPath})
		}
	}
}

// collect dispatches h to the matching family collector, based on which of
// Meta/MCPTool/MCPResource/MCPPrompt is set. A plain HTTP handler (only
// Meta set) is silently skipped - it belongs to routes.go, not mcp.go.
func (c *mcpCollector) collect(g *Generator, h models.HandlerFunc, receiverVar string) error {
	var err error
	switch {
	case h.MCPTool != nil:
		err = c.collectTool(g, h, receiverVar)
	case h.MCPResource != nil:
		err = c.collectResource(h, receiverVar)
	case h.MCPPrompt != nil:
		err = c.collectPrompt(h, receiverVar)
	default:
		return nil
	}
	if err != nil {
		return err
	}
	if receiverVar == "" {
		c.pkgRefs = append(c.pkgRefs, models.ProjectImport{Alias: h.Package, Path: h.ImportPath})
	}
	return nil
}

func (c *mcpCollector) collectTool(g *Generator, h models.HandlerFunc, receiverVar string) error {
	esc := escapeForGoStringLiteral

	refs, err := resolveMiddlewareNames(h.Name, h.Package, h.MCPTool.Middleware, c.lookup)
	if err != nil {
		return err
	}
	inputSchema, err := g.mcpSchemaJSON(h, h.MCPTool.InputModel)
	if err != nil {
		return err
	}
	var outputSchema string
	if h.MCPTool.OutputModel != nil {
		if outputSchema, err = g.mcpSchemaJSON(h, *h.MCPTool.OutputModel); err != nil {
			return err
		}
	}

	c.tools = append(c.tools, mcpToolEntry{
		ToolName: esc(h.MCPTool.Name), Description: esc(h.MCPTool.Description),
		InputSchema: inputSchema, OutputSchema: outputSchema,
		Package: h.Package, FuncName: h.Name, ReceiverVar: receiverVar,
		Middlewares: refs,
	})
	c.addMiddlewareImports(refs)
	return nil
}

func (c *mcpCollector) collectResource(h models.HandlerFunc, receiverVar string) error {
	esc := escapeForGoStringLiteral

	refs, err := resolveMiddlewareNames(h.Name, h.Package, h.MCPResource.Middleware, c.lookup)
	if err != nil {
		return err
	}

	c.resources = append(c.resources, mcpResourceEntry{
		URI: esc(h.MCPResource.URI), ResName: esc(h.MCPResource.Name),
		Description: esc(h.MCPResource.Description), MimeType: esc(h.MCPResource.MimeType),
		Package: h.Package, FuncName: h.Name, ReceiverVar: receiverVar,
		Middlewares: refs,
	})
	c.addMiddlewareImports(refs)
	return nil
}

func (c *mcpCollector) collectPrompt(h models.HandlerFunc, receiverVar string) error {
	esc := escapeForGoStringLiteral

	refs, err := resolveMiddlewareNames(h.Name, h.Package, h.MCPPrompt.Middleware, c.lookup)
	if err != nil {
		return err
	}

	args := make([]mcpPromptArgEntry, 0, len(h.MCPPrompt.Args))
	for _, a := range h.MCPPrompt.Args {
		var desc string
		if a.Description != nil {
			desc = *a.Description
		}
		args = append(args, mcpPromptArgEntry{Name: esc(a.Name), Description: esc(desc), Required: a.Required})
	}

	c.prompts = append(c.prompts, mcpPromptEntry{
		PromptName: esc(h.MCPPrompt.Name), Description: esc(h.MCPPrompt.Description), Args: args,
		Package: h.Package, FuncName: h.Name, ReceiverVar: receiverVar,
		Middlewares: refs,
	})
	c.addMiddlewareImports(refs)
	return nil
}

// buildData assembles the final template data once every handler/struct has
// been collected.
func (c *mcpCollector) buildData(g *Generator) mcpData {
	esc := escapeForGoStringLiteral

	endpoint := g.project.Config.MCPEndpoint
	if endpoint == "" {
		endpoint = defaultMCPEndpoint
	}
	var instructions string
	if g.project.Config.MCPInstructions != nil {
		instructions = *g.project.Config.MCPInstructions
	}

	allowedOrigins := make([]string, len(g.project.Config.MCPAllowedOrigins))
	for i, o := range g.project.Config.MCPAllowedOrigins {
		allowedOrigins[i] = esc(o)
	}

	var needsJSON bool
	for _, t := range c.tools {
		if t.InputSchema != "" || t.OutputSchema != "" {
			needsJSON = true
			break
		}
	}

	return mcpData{
		Imports:        collectPackages(c.pkgRefs),
		NeedsJSON:      needsJSON,
		Endpoint:       esc(endpoint),
		ServerName:     esc(g.project.Config.Title),
		ServerVersion:  esc(g.project.Config.Version),
		Instructions:   esc(instructions),
		AllowedOrigins: allowedOrigins,
		Tools:          c.tools,
		Resources:      c.resources,
		Prompts:        c.prompts,
	}
}

// mcpSchemaJSON resolves ref (a Go type name, bare or "pkg.Type") against
// h's own imports into a standalone JSON Schema string suitable for
// embedding as a Go string literal, inlining any nested named types it
// references as "$defs".
func (g *Generator) mcpSchemaJSON(h models.HandlerFunc, ref string) (string, error) {
	if ref == "" {
		return "", nil
	}
	alias, typeName := h.Package, ref
	if idx := strings.LastIndex(ref, "."); idx >= 0 {
		alias, typeName = ref[:idx], ref[idx+1:]
	}

	imports := h.Imports
	if _, ok := imports[alias]; !ok {
		merged := make(map[string]string, len(h.Imports)+1)
		maps.Copy(merged, h.Imports)
		merged[h.Package] = h.ImportPath
		imports = merged
	}

	registry := map[string]any{}
	schema, err := resolveModelSchema(g.project.MainModule, imports, alias, typeName, mcpSchemaRefPrefix, registry)
	if err != nil || schema == nil {
		return "", err
	}
	if m, ok := schema.(map[string]any); ok && len(registry) > 0 {
		m["$defs"] = registry
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return "", err
	}
	return escapeForGoStringLiteral(string(raw)), nil
}
