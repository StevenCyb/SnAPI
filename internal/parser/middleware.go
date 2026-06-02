package parser

import (
	"go/ast"
	"strings"

	"github.com/StevenCyb/SnAPI/internal/models"
	"github.com/StevenCyb/SnAPI/internal/parser/utils"
)

// parseMiddleware is a convenience wrapper around walkAndExtract that runs only
// the middleware extractor. Intended primarily for tests.
func (p *Parser) parseMiddleware() error {
	return p.walkAndExtract(p.middlewareExtractor)
}

// middlewareExtractor scans a single parsed file for @snapi.middleware annotated
// functions and appends them to p.project.
func (p *Parser) middlewareExtractor(fc fileCtx) error {
	var found []models.MiddlewareFunc
	for _, decl := range fc.File.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Doc == nil {
			continue
		}

		mw, err := processMiddlewareFunc(fn, fc.File, fc.ImportPath, fc.Path)
		if err != nil {
			return err
		}
		if mw != nil {
			found = append(found, *mw)
		}
	}

	if len(found) == 0 {
		return nil
	}
	p.mu.Lock()
	p.project.MiddlewareFuncs = append(p.project.MiddlewareFuncs, found...)
	p.mu.Unlock()
	return nil
}

// processMiddlewareFunc inspects a single *ast.FuncDecl and returns a
// MiddlewareFunc when the function is annotated with @snapi.middleware.
func processMiddlewareFunc(fn *ast.FuncDecl, node *ast.File, importPath, filePath string) (*models.MiddlewareFunc, error) {
	invalidErr := func(reason string) error {
		return &InvalidMiddlewareFuncError{FilePath: filePath, FuncName: fn.Name.Name, Reason: reason}
	}

	for _, ann := range utils.ExtractAnnotation(commentText(fn.Doc)) {
		if strings.ToLower(ann.Name) != "middleware" {
			continue
		}

		if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
			return nil, invalidErr("middleware functions must not return a value")
		}
		if fn.Type.Params == nil || len(fn.Type.Params.List) != 3 {
			return nil, invalidErr("middleware functions must have exactly three parameters: (runtime.Request, runtime.Response, runtime.HandlerFunc)")
		}
		paramTypes := make([]string, 0, 3)
		for _, param := range fn.Type.Params.List {
			sel, ok := param.Type.(*ast.SelectorExpr)
			if !ok {
				return nil, invalidErr("middleware function parameters must be of type runtime.Request, runtime.Response, and runtime.HandlerFunc")
			}
			paramTypes = append(paramTypes, sel.Sel.Name)
		}
		if paramTypes[0] != "Request" || paramTypes[1] != "Response" || paramTypes[2] != "HandlerFunc" {
			return nil, invalidErr("middleware function parameters must be of type runtime.Request, runtime.Response, and runtime.HandlerFunc")
		}

		return &models.MiddlewareFunc{Package: node.Name.Name, ImportPath: importPath, Name: fn.Name.Name}, nil
	}

	return nil, nil
}
