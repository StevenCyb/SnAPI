package parser

import (
	"strings"

	"github.com/StevenCyb/SnAPI/internal/models"
	"github.com/StevenCyb/SnAPI/internal/parser/utils"
)

// extractAnyHandlerMeta runs every handler-family extractor (HTTP, MCP tool,
// MCP resource, MCP prompt) against a function/method doc comment and
// ensures at most one family matched. filePath/funcName are used only to
// build the error when more than one family is present.
func extractAnyHandlerMeta(comments, filePath, funcName string) (
	httpMeta *models.HandlerMeta,
	tool *models.MCPToolMeta,
	resource *models.MCPResourceMeta,
	prompt *models.MCPPromptMeta,
	err error,
) {
	httpMeta = extractHandlerMeta(comments)
	tool = extractMCPToolMeta(comments)
	resource = extractMCPResourceMeta(comments)
	prompt = extractMCPPromptMeta(comments)

	var families []string
	if httpMeta != nil {
		families = append(families, "HTTP")
	}
	if tool != nil {
		families = append(families, "MCPTool")
	}
	if resource != nil {
		families = append(families, "MCPResource")
	}
	if prompt != nil {
		families = append(families, "MCPPrompt")
	}
	if len(families) > 1 {
		return nil, nil, nil, nil, &MultipleAnnotationFamiliesError{FilePath: filePath, FuncName: funcName, Families: families}
	}
	return httpMeta, tool, resource, prompt, nil
}

// appendMiddlewareNames appends the non-empty args of an
// @SnAPI.UseMiddleware annotation to dst.
func appendMiddlewareNames(dst []string, args []string) []string {
	for _, a := range args {
		if a != "" {
			dst = append(dst, a)
		}
	}
	return dst
}

// extractMCPToolMeta parses @SnAPI.MCPTool(name, description) plus the
// @SnAPI.Request(GoType) / @SnAPI.MCPOutput(GoType) / @SnAPI.UseMiddleware
// annotations that describe an MCP tool handler. Returns nil if no
// @SnAPI.MCPTool annotation is present.
func extractMCPToolMeta(comments string) *models.MCPToolMeta {
	anns := utils.ExtractAnnotation(comments)

	var meta *models.MCPToolMeta
	for _, ann := range anns {
		if strings.EqualFold(ann.Name, "mcptool") {
			meta = &models.MCPToolMeta{}
			if len(ann.Args) > 0 {
				meta.Name = ann.Args[0]
			}
			if len(ann.Args) > 1 {
				meta.Description = ann.Args[1]
			}
		}
	}
	if meta == nil {
		return nil
	}

	for _, ann := range anns {
		switch strings.ToLower(ann.Name) {
		case "request":
			// Reuses @SnAPI.Request's (mediaType, GoType) or (GoType) shape;
			// only the Go type matters for an MCP tool's input schema.
			if len(ann.Args) > 0 {
				meta.InputModel = ann.Args[len(ann.Args)-1]
			}
		case "mcpoutput":
			if len(ann.Args) > 0 {
				model := ann.Args[0]
				meta.OutputModel = &model
			}
		case "usemiddleware":
			meta.Middleware = appendMiddlewareNames(meta.Middleware, ann.Args)
		}
	}
	return meta
}

// extractMCPResourceMeta parses @SnAPI.MCPResource(uri, name, description,
// mimeType) plus @SnAPI.UseMiddleware. Returns nil if no @SnAPI.MCPResource
// annotation is present.
func extractMCPResourceMeta(comments string) *models.MCPResourceMeta {
	anns := utils.ExtractAnnotation(comments)

	var meta *models.MCPResourceMeta
	for _, ann := range anns {
		if strings.EqualFold(ann.Name, "mcpresource") {
			meta = &models.MCPResourceMeta{}
			if len(ann.Args) > 0 {
				meta.URI = ann.Args[0]
			}
			if len(ann.Args) > 1 {
				meta.Name = ann.Args[1]
			}
			if len(ann.Args) > 2 {
				meta.Description = ann.Args[2]
			}
			if len(ann.Args) > 3 {
				meta.MimeType = ann.Args[3]
			}
		}
	}
	if meta == nil {
		return nil
	}

	for _, ann := range anns {
		if strings.EqualFold(ann.Name, "usemiddleware") {
			meta.Middleware = appendMiddlewareNames(meta.Middleware, ann.Args)
		}
	}
	return meta
}

// parsePromptArg parses a single @SnAPI.MCPPromptArg(name, description?,
// required?) annotation's arguments. Unlike @SnAPI.Path/Query/Header/
// Cookie, a prompt argument has no "type".
func parsePromptArg(args []string) (models.HandlerParam, bool) {
	if len(args) == 0 || args[0] == "" {
		return models.HandlerParam{}, false
	}
	arg := models.HandlerParam{Name: args[0]}
	if len(args) > 1 && args[1] != "" {
		desc := args[1]
		arg.Description = &desc
	}
	if len(args) > 2 {
		arg.Required = strings.EqualFold(args[2], "true") || strings.EqualFold(args[2], "required")
	}
	return arg, true
}

// extractMCPPromptMeta parses @SnAPI.MCPPrompt(name, description) plus
// @SnAPI.MCPPromptArg(name, description, required) and @SnAPI.UseMiddleware.
// Returns nil if no @SnAPI.MCPPrompt annotation is present.
func extractMCPPromptMeta(comments string) *models.MCPPromptMeta {
	anns := utils.ExtractAnnotation(comments)

	var meta *models.MCPPromptMeta
	for _, ann := range anns {
		if strings.EqualFold(ann.Name, "mcpprompt") {
			meta = &models.MCPPromptMeta{}
			if len(ann.Args) > 0 {
				meta.Name = ann.Args[0]
			}
			if len(ann.Args) > 1 {
				meta.Description = ann.Args[1]
			}
		}
	}
	if meta == nil {
		return nil
	}

	for _, ann := range anns {
		switch strings.ToLower(ann.Name) {
		case "mcppromptarg":
			if arg, ok := parsePromptArg(ann.Args); ok {
				meta.Args = append(meta.Args, arg)
			}
		case "usemiddleware":
			meta.Middleware = appendMiddlewareNames(meta.Middleware, ann.Args)
		}
	}
	return meta
}
