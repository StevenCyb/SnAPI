package parser

import (
	"go/ast"
	"strings"

	"github.com/StevenCyb/SnAPI/internal/models"
	"github.com/StevenCyb/SnAPI/internal/parser/utils"
)

// partialStructData accumulates per-struct information discovered across one
// or more files during the concurrent walk.
type partialStructData struct {
	Package          string
	ImportPath       string
	HasDecl          bool
	HasConstructor   bool
	CtorReturnsError bool
	HasDestructor    bool
	PathPrefix       string
	Middleware       []string
	Tags             []string
	Methods          []models.HandlerFunc
	Imports          map[string]string // merged alias -> import path from all method files
}

// parseHandlerStruct is a convenience wrapper around walkAndExtract that runs
// only the struct extractor. Intended primarily for tests.
func (p *Parser) parseHandlerStruct() error {
	if err := p.walkAndExtract(p.handlerStructExtractor); err != nil {
		return err
	}
	return p.assembleHandlerStructs()
}

// handlerStructExtractor scans a single file for:
//   - struct type declarations (marks the struct as declared)
//   - Constructor / Destructor methods on any struct
//   - @snapi.* annotated methods that become HTTP routes
func (p *Parser) handlerStructExtractor(fc fileCtx) error {
	imports := collectFileImports(fc.File)

	for _, decl := range fc.File.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			p.collectStructDecls(fc, d)
		case *ast.FuncDecl:
			if err := p.collectStructMethod(fc, d, imports); err != nil {
				return err
			}
		}
	}
	return nil
}

// collectStructDecls registers every struct type declaration in the file and
// parses optional struct-level @SnAPI.Path / @SnAPI.UseMiddleware annotations.
func (p *Parser) collectStructDecls(fc fileCtx, d *ast.GenDecl) {
	for _, spec := range d.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		if _, ok := ts.Type.(*ast.StructType); !ok {
			continue
		}

		// Doc comment may be on the TypeSpec (grouped decl) or on the GenDecl
		// (single-spec decl). Try TypeSpec first, fall back to GenDecl.
		doc := ts.Doc
		if doc == nil && len(d.Specs) == 1 {
			doc = d.Doc
		}
		pathPrefix, middleware, tags := extractStructMeta(commentText(doc))

		key := fc.ImportPath + "." + ts.Name.Name
		p.mu.Lock()
		c := p.getOrCreateCandidate(key, fc.File.Name.Name, fc.ImportPath)
		c.HasDecl = true
		if pathPrefix != "" {
			c.PathPrefix = pathPrefix
		}
		c.Middleware = append(c.Middleware, middleware...)
		c.Tags = append(c.Tags, tags...)
		p.mu.Unlock()
	}
}

// extractStructMeta parses @SnAPI.Path, @SnAPI.UseMiddleware, and @SnAPI.Tags
// from a struct's doc comment. All are optional.
func extractStructMeta(comments string) (pathPrefix string, middleware []string, tags []string) {
	for _, ann := range utils.ExtractAnnotation(comments) {
		switch strings.ToLower(ann.Name) {
		case "path":
			if len(ann.Args) > 0 && ann.Args[0] != "" {
				pathPrefix = ann.Args[0]
			}
		case "usemiddleware":
			for _, a := range ann.Args {
				if a != "" {
					middleware = append(middleware, a)
				}
			}
		case "tags":
			for _, a := range ann.Args {
				if a != "" {
					tags = append(tags, a)
				}
			}
		}
	}
	return
}

// collectStructMethod handles a single method declaration (FuncDecl with a
// receiver). It records Constructor, Destructor, and @snapi.* handler methods.
func (p *Parser) collectStructMethod(fc fileCtx, fn *ast.FuncDecl, imports map[string]string) error {
	typeName, ok := receiverTypeName(fn)
	if !ok {
		return nil // not a method
	}
	key := fc.ImportPath + "." + typeName

	switch fn.Name.Name {
	case "Constructor":
		returnsError, err := validateConstructorMethod(fn, fc.Path, typeName)
		if err != nil {
			return err
		}
		p.mu.Lock()
		c := p.getOrCreateCandidate(key, fc.File.Name.Name, fc.ImportPath)
		c.HasConstructor = true
		c.CtorReturnsError = returnsError
		p.mu.Unlock()

	case "Destructor":
		if err := validateDestructorMethod(fn, fc.Path, typeName); err != nil {
			return err
		}
		p.mu.Lock()
		c := p.getOrCreateCandidate(key, fc.File.Name.Name, fc.ImportPath)
		c.HasDestructor = true
		p.mu.Unlock()

	default:
		if fn.Doc == nil {
			return nil
		}
		httpMeta, tool, resource, prompt, err := extractAnyHandlerMeta(commentText(fn.Doc), fc.Path, fn.Name.Name)
		if err != nil {
			return err
		}
		if httpMeta == nil && tool == nil && resource == nil && prompt == nil {
			return nil
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
		method := models.HandlerFunc{
			Package:     fc.File.Name.Name,
			ImportPath:  fc.ImportPath,
			Name:        fn.Name.Name,
			Meta:        httpMeta,
			MCPTool:     tool,
			MCPResource: resource,
			MCPPrompt:   prompt,
			Imports:     imports,
		}
		p.mu.Lock()
		c := p.getOrCreateCandidate(key, fc.File.Name.Name, fc.ImportPath)
		c.Methods = append(c.Methods, method)
		if c.Imports == nil {
			c.Imports = make(map[string]string, len(imports))
		}
		for k, v := range imports {
			c.Imports[k] = v
		}
		p.mu.Unlock()
	}
	return nil
}

// assembleHandlerStructs finalises the struct handler list after all files
// have been walked. Only structs with at least one @snapi.* method are included.
func (p *Parser) assembleHandlerStructs() error {
	for key, data := range p.structCandidates {
		if len(data.Methods) == 0 {
			continue // no handler methods → not a handler struct
		}
		typeName := key[strings.LastIndex(key, ".")+1:]
		if !data.HasDecl {
			return &InvalidHandlerStructError{TypeName: typeName, Reason: "struct declaration not found in project"}
		}
		p.project.HandlerStructs = append(p.project.HandlerStructs, models.HandlerStruct{
			Package:                 data.Package,
			ImportPath:              data.ImportPath,
			Name:                    typeName,
			VarName:                 toVarName(typeName),
			HasConstructor:          data.HasConstructor,
			ConstructorReturnsError: data.CtorReturnsError,
			HasDestructor:           data.HasDestructor,
			PathPrefix:              data.PathPrefix,
			Middleware:              data.Middleware,
			Tags:                    data.Tags,
			Methods:                 mergeStructTags(data.Tags, data.Methods),
		})
	}
	return nil
}

// getOrCreateCandidate returns the partialStructData for key, creating it with
// the given package/importPath if absent. Must be called with p.mu held.
func (p *Parser) getOrCreateCandidate(key, pkg, importPath string) *partialStructData {
	if c, ok := p.structCandidates[key]; ok {
		return c
	}
	c := &partialStructData{Package: pkg, ImportPath: importPath}
	p.structCandidates[key] = c
	return c
}

// receiverTypeName extracts the base type name from a method's receiver.
// Returns ("", false) for non-methods or unsupported receiver expressions.
func receiverTypeName(fn *ast.FuncDecl) (string, bool) {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return "", false
	}
	switch t := fn.Recv.List[0].Type.(type) {
	case *ast.Ident:
		return t.Name, true
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name, true
		}
	}
	return "", false
}

// validateConstructorMethod checks that a Constructor method has no parameters
// and returns at most a single error value. Returns whether it returns error.
func validateConstructorMethod(fn *ast.FuncDecl, filePath, typeName string) (bool, error) {
	errFor := func(reason string) error {
		return &InvalidHandlerStructError{FilePath: filePath, TypeName: typeName, FuncName: "Constructor", Reason: reason}
	}
	if len(fn.Type.Params.List) > 0 {
		return false, errFor("must not have parameters")
	}
	if fn.Type.Results == nil || len(fn.Type.Results.List) == 0 {
		return false, nil
	}
	if len(fn.Type.Results.List) != 1 {
		return false, errFor("must return at most one value")
	}
	ident, ok := fn.Type.Results.List[0].Type.(*ast.Ident)
	if !ok || ident.Name != "error" {
		return false, errFor("return type must be error")
	}
	return true, nil
}

// validateDestructorMethod checks that a Destructor method has no parameters
// and no return values.
func validateDestructorMethod(fn *ast.FuncDecl, filePath, typeName string) error {
	errFor := func(reason string) error {
		return &InvalidHandlerStructError{FilePath: filePath, TypeName: typeName, FuncName: "Destructor", Reason: reason}
	}
	if len(fn.Type.Params.List) > 0 {
		return errFor("must not have parameters")
	}
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		return errFor("must not return any values")
	}
	return nil
}

// mergeStructTags returns a copy of methods where structTags are prepended to
// each method's existing tags (deduplicating; struct tags come first as the
// broader grouping, method-level tags follow).
func mergeStructTags(structTags []string, methods []models.HandlerFunc) []models.HandlerFunc {
	if len(structTags) == 0 {
		return methods
	}
	out := make([]models.HandlerFunc, len(methods))
	for i, m := range methods {
		mc := m
		if m.Meta != nil {
			mergedMeta := *m.Meta
			seen := make(map[string]bool, len(structTags)+len(m.Meta.Tags))
			merged := make([]string, 0, len(structTags)+len(m.Meta.Tags))
			for _, t := range structTags {
				if !seen[t] {
					seen[t] = true
					merged = append(merged, t)
				}
			}
			for _, t := range m.Meta.Tags {
				if !seen[t] {
					seen[t] = true
					merged = append(merged, t)
				}
			}
			mergedMeta.Tags = merged
			mc.Meta = &mergedMeta
		}
		out[i] = mc
	}
	return out
}

// toVarName converts a Go type name to a lower-camel-case variable name.
// Leading consecutive uppercase letters are lowercased:
//
//	CRUD → crud, MyHandler → myHandler, UserAPI → userAPI
func toVarName(typeName string) string {
	if len(typeName) == 0 {
		return typeName
	}
	i := 0
	for i < len(typeName) && typeName[i] >= 'A' && typeName[i] <= 'Z' {
		i++
	}
	switch i {
	case 0:
		return typeName
	case 1:
		return strings.ToLower(typeName[:1]) + typeName[1:]
	default:
		if i < len(typeName) {
			// e.g. "UserAPI" → u=1 ... but "CRUD" all uppercase
			return strings.ToLower(typeName[:i-1]) + typeName[i-1:]
		}
		return strings.ToLower(typeName)
	}
}
