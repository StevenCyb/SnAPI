package parser

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"slices"
	"strings"

	"github.com/StevenCyb/SnAPI/internal/models"
	"github.com/StevenCyb/SnAPI/internal/parser/utils"
)

const defaultRequestContentType = "application/json"

func isHTTPMethod(name string) bool {
	switch name {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
		return true
	}
	return false
}

// parseHandler is a convenience wrapper around walkAndExtract that runs only
// the handler extractor. Intended primarily for tests.
func (p *Parser) parseHandler() error {
	return p.walkAndExtract(p.handlerExtractor)
}

// handlerExtractor scans a single parsed file for handler functions (any
// function whose doc comment contains an HTTP method annotation) and appends
// them to p.project.
func (p *Parser) handlerExtractor(fc fileCtx) error {
	imports := collectFileImports(fc.File)

	var found []models.HandlerFunc
	for _, decl := range fc.File.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Doc == nil {
			continue
		}
		meta := extractHandlerMeta(commentText(fn.Doc))
		if meta == nil {
			continue
		}

		params := fn.Type.Params.List
		if len(params) < 2 {
			return ErrExpectedAtLeast2Params
		}
		if !isRuntimeSelector(params[0].Type, "Request") {
			return ErrFirstParamMustBeRequest
		}
		if !isRuntimeSelector(params[1].Type, "Response") {
			return ErrSecondParamMustBeResponse
		}

		services, err := collectHandlerServices(fc, fn, params[2:])
		if err != nil {
			return err
		}

		found = append(found, models.HandlerFunc{
			Package:    fc.File.Name.Name,
			ImportPath: fc.ImportPath,
			Name:       fn.Name.Name,
			Meta:       meta,
			Services:   services,
			Imports:    imports,
		})
	}

	if len(found) == 0 {
		return nil
	}
	p.mu.Lock()
	p.project.HandlerFuncs = append(p.project.HandlerFuncs, found...)
	p.mu.Unlock()
	return nil
}

// extractHandlerMeta parses @snapi.* annotations on a handler function. Returns
// nil if the function carries no HTTP method annotation (i.e. it is not a
// handler).
func extractHandlerMeta(comments string) *models.HandlerMeta {
	meta := &models.HandlerMeta{Path: "/"}
	hasMethod := false

	for _, ann := range utils.ExtractAnnotation(comments) {
		upperName := strings.ToUpper(ann.Name)
		if isHTTPMethod(upperName) {
			meta.Method = upperName
			if len(ann.Args) > 0 && ann.Args[0] != "" {
				meta.Path = ann.Args[0]
			}
			hasMethod = true
			continue
		}

		switch strings.ToLower(ann.Name) {
		case "summary":
			if len(ann.Args) > 0 {
				s := ann.Args[0]
				meta.Summary = &s
			}
		case "description":
			if len(ann.Args) > 0 {
				d := ann.Args[0]
				meta.Description = &d
			}
		case "operationid":
			if len(ann.Args) > 0 {
				id := ann.Args[0]
				meta.OperationID = &id
			}
		case "usemiddleware":
			for _, a := range ann.Args {
				if a != "" {
					meta.Middleware = append(meta.Middleware, a)
				}
			}
		case "security":
			for _, a := range ann.Args {
				if a != "" {
					meta.Security = append(meta.Security, a)
				}
			}
		case "deprecated":
			meta.Deprecated = true
		case "status":
			if len(ann.Args) == 0 {
				continue
			}
			s := models.HandlerStatus{Code: ann.Args[0]}
			if len(ann.Args) > 1 && ann.Args[1] != "" {
				d := ann.Args[1]
				s.Description = &d
			}
			meta.Status = append(meta.Status, s)
		case "tags":
			for _, a := range ann.Args {
				if a != "" {
					meta.Tags = append(meta.Tags, a)
				}
			}
		case "request":
			if len(ann.Args) == 0 {
				continue
			}
			req := models.HandlerRequest{ContentType: defaultRequestContentType}
			if len(ann.Args) == 1 {
				req.Model = ann.Args[0]
			} else {
				req.ContentType = ann.Args[0]
				req.Model = ann.Args[1]
			}
			meta.Requests = append(meta.Requests, req)
		case "path":
			if p, ok := parseHandlerParam(ann.Args, true); ok {
				meta.Paths = append(meta.Paths, p)
			}
		case "query":
			if p, ok := parseHandlerParam(ann.Args, false); ok {
				meta.Queries = append(meta.Queries, p)
			}
		case "header":
			if p, ok := parseHandlerParam(ann.Args, false); ok {
				meta.Headers = append(meta.Headers, p)
			}
		case "cookie":
			if p, ok := parseHandlerParam(ann.Args, false); ok {
				meta.Cookies = append(meta.Cookies, p)
			}
		case "response":
			if len(ann.Args) < 3 {
				continue
			}
			meta.Responses = append(meta.Responses, models.HandlerResponse{
				Code:        ann.Args[0],
				ContentType: ann.Args[1],
				Model:       ann.Args[2],
			})
		case "responseheader":
			if len(ann.Args) < 3 {
				continue
			}
			h := models.HandlerResponseHeader{
				Code: ann.Args[0],
				Name: ann.Args[1],
				Type: ann.Args[2],
			}
			if len(ann.Args) > 3 && ann.Args[3] != "" {
				d := ann.Args[3]
				h.Description = &d
			}
			meta.ResponseHeaders = append(meta.ResponseHeaders, h)
		}
	}

	if !hasMethod {
		return nil
	}
	return meta
}

// parseHandlerParam parses a (name, type, description?, required?) argument
// list into a HandlerParam. `defaultRequired` is used when no explicit required
// flag is supplied (path params default to true, the rest to false).
func parseHandlerParam(args []string, defaultRequired bool) (models.HandlerParam, bool) {
	if len(args) < 2 {
		return models.HandlerParam{}, false
	}
	p := models.HandlerParam{Name: args[0], Type: args[1], Required: defaultRequired}
	if len(args) > 2 && args[2] != "" {
		d := args[2]
		p.Description = &d
	}
	if len(args) > 3 {
		p.Required = strings.EqualFold(args[3], "true") || strings.EqualFold(args[3], "required")
	}
	return p, true
}

// collectHandlerServices renders the types of every service parameter (those
// after the mandatory request/response pair) and validates them.
func collectHandlerServices(fc fileCtx, fn *ast.FuncDecl, params []*ast.Field) ([]string, error) {
	var services []string
	for _, param := range params {
		typeStr := renderExpr(fc.FSet, param.Type)
		if slices.Contains(services, typeStr) {
			return nil, &InvalidServiceParamError{
				FilePath: fc.Path, FuncName: fn.Name.Name, ParamType: typeStr,
				Reason: "duplicate service parameter",
			}
		}
		if len(param.Names) > 1 {
			return nil, &InvalidServiceParamError{
				FilePath: fc.Path, FuncName: fn.Name.Name, ParamType: typeStr,
				Reason: "service parameter must have a single name",
			}
		}
		services = append(services, typeStr)
	}
	return services, nil
}

// collectFileImports builds an alias -> import path map for a file. When no
// explicit alias is set, the last segment of the import path is used (matching
// the default Go selector behavior). Anonymous (`_`) and dot imports are
// skipped.
func collectFileImports(file *ast.File) map[string]string {
	imports := make(map[string]string, len(file.Imports))
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		alias := ""
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		if alias == "" {
			alias = path
			if idx := strings.LastIndex(path, "/"); idx >= 0 {
				alias = path[idx+1:]
			}
		}
		if alias == "_" || alias == "." {
			continue
		}
		imports[alias] = path
	}
	return imports
}

// isRuntimeSelector reports whether expr is the selector expression
// `runtime.<sel>`.
func isRuntimeSelector(expr ast.Expr, sel string) bool {
	se, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	x, ok := se.X.(*ast.Ident)
	return ok && x.Name == "runtime" && se.Sel.Name == sel
}

// renderExpr returns the textual representation of an AST expression.
func renderExpr(fSet *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, fSet, expr)
	return buf.String()
}
