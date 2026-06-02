package parser

import (
	"go/ast"
	"strings"

	"github.com/StevenCyb/SnAPI/internal/models"
	"github.com/StevenCyb/SnAPI/internal/parser/utils"
)

// parseLifecycle is a convenience wrapper around walkAndExtract that runs only
// the lifecycle extractor. Intended primarily for tests.
func (p *Parser) parseLifecycle() error {
	return p.walkAndExtract(p.lifecycleExtractor)
}

// lifecycleExtractor scans a single parsed file for @snapi.setup / @snapi.teardown
// annotated functions and appends them to p.project.
func (p *Parser) lifecycleExtractor(fc fileCtx) error {
	var setups, teardowns []models.LifecycleFunc
	for _, decl := range fc.File.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Doc == nil {
			continue
		}

		setup, teardown, err := processLifecycleFunc(fn, fc.File, fc.ImportPath, fc.Path)
		if err != nil {
			return err
		}
		if setup != nil {
			setups = append(setups, *setup)
		}
		if teardown != nil {
			teardowns = append(teardowns, *teardown)
		}
	}

	if len(setups) == 0 && len(teardowns) == 0 {
		return nil
	}
	p.mu.Lock()
	p.project.SetupFuncs = append(p.project.SetupFuncs, setups...)
	p.project.TeardownFuncs = append(p.project.TeardownFuncs, teardowns...)
	p.mu.Unlock()
	return nil
}

// processLifecycleFunc inspects a single *ast.FuncDecl and determines whether it is a
// setup or teardown lifecycle function. It returns pointers to the corresponding
// LifecycleFunc (or nil if the function is not a lifecycle function) and any
// validation error.
func processLifecycleFunc(fn *ast.FuncDecl, node *ast.File, importPath, filePath string) (*models.LifecycleFunc, *models.LifecycleFunc, error) {
	invalidErr := func(reason string) error {
		return &InvalidLifecycleFuncError{FilePath: filePath, FuncName: fn.Name.Name, Reason: reason}
	}

	for _, ann := range utils.ExtractAnnotation(commentText(fn.Doc)) {
		switch strings.ToLower(ann.Name) {
		case "setup":
			if len(fn.Type.Params.List) > 0 {
				return nil, nil, invalidErr("lifecycle functions should not have parameters")
			}
			returnsError := false
			if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
				if len(fn.Type.Results.List) != 1 {
					return nil, nil, invalidErr("setup function must return at most one value")
				}
				if ident, ok := fn.Type.Results.List[0].Type.(*ast.Ident); !ok || ident.Name != "error" {
					return nil, nil, invalidErr("setup function return type must be error")
				}
				returnsError = true
			}
			return &models.LifecycleFunc{Package: node.Name.Name, ImportPath: importPath, Name: fn.Name.Name, ReturnsError: returnsError}, nil, nil
		case "teardown":
			if len(fn.Type.Params.List) > 0 {
				return nil, nil, invalidErr("lifecycle functions should not have parameters")
			}
			return nil, &models.LifecycleFunc{Package: node.Name.Name, ImportPath: importPath, Name: fn.Name.Name}, nil
		}
	}

	return nil, nil, nil
}
