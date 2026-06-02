package generator

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/StevenCyb/SnAPI/internal/models"
)

// resolveModelSchema looks up the OpenAPI schema for `alias.typeName` by
// resolving the alias against the file's import map, walking to the package
// directory (using the module's go.mod replace directives), parsing the
// package's Go files and converting the type spec into a schema map. Returns
// (nil, nil) when the type can't be located locally (e.g. third-party).
func resolveModelSchema(mod *models.Module, imports map[string]string, alias, typeName string, registry map[string]any) (any, error) {
	if mod == nil || alias == "" || typeName == "" {
		return nil, nil
	}
	importPath, ok := imports[alias]
	if !ok {
		return nil, nil
	}
	pkgDir, ok := resolvePackageDir(mod, importPath)
	if !ok {
		return nil, nil
	}
	ts, pkgFiles, err := findTypeDecl(pkgDir, typeName)
	if err != nil || ts == nil {
		return nil, err
	}
	return exprToSchema(ts.Type, pkgFiles, mod, pkgDir, importPath, registry), nil
}

func resolvePackageDir(mod *models.Module, importPath string) (string, bool) {
	if mod.Path != "" && (importPath == mod.Path || strings.HasPrefix(importPath, mod.Path+"/")) {
		rel := strings.TrimPrefix(strings.TrimPrefix(importPath, mod.Path), "/")
		return filepath.Join(mod.Dir, rel), true
	}
	for _, rep := range mod.Replaces {
		if rep.OldPath == importPath || strings.HasPrefix(importPath, rep.OldPath+"/") {
			rel := strings.TrimPrefix(strings.TrimPrefix(importPath, rep.OldPath), "/")
			newPath := rep.NewPath
			if !filepath.IsAbs(newPath) {
				newPath = filepath.Join(mod.Dir, newPath)
			}
			return filepath.Join(newPath, rel), true
		}
	}
	return "", false
}

func findTypeDecl(pkgDir, typeName string) (*ast.TypeSpec, []*ast.File, error) {
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, nil, err
	}
	fSet := token.NewFileSet()
	var (
		files []*ast.File
		found *ast.TypeSpec
	)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fSet, filepath.Join(pkgDir, entry.Name()), nil, parser.ParseComments)
		if err != nil {
			continue
		}
		files = append(files, f)
		if found != nil {
			continue
		}
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == typeName {
					found = ts
				}
			}
		}
	}
	return found, files, nil
}

func exprToSchema(expr ast.Expr, pkgFiles []*ast.File, mod *models.Module, pkgDir, importPath string, registry map[string]any) any {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return exprToSchema(t.X, pkgFiles, mod, pkgDir, importPath, registry)
	case *ast.Ident:
		if s, ok := basicTypeSchema(t.Name); ok {
			return s
		}
		if local := findLocalTypeSpec(pkgFiles, t.Name); local != nil {
			if _, exists := registry[t.Name]; !exists {
				registry[t.Name] = map[string]any{"type": "object"}
				registry[t.Name] = exprToSchema(local.Type, pkgFiles, mod, pkgDir, importPath, registry)
			}
			return map[string]any{"$ref": "#/components/schemas/" + t.Name}
		}
		return map[string]any{"type": "object"}
	case *ast.SelectorExpr:
		// Cross-package references are emitted as opaque objects.
		return map[string]any{"type": "object"}
	case *ast.ArrayType:
		return map[string]any{
			"type":  "array",
			"items": exprToSchema(t.Elt, pkgFiles, mod, pkgDir, importPath, registry),
		}
	case *ast.MapType:
		return map[string]any{
			"type":                 "object",
			"additionalProperties": exprToSchema(t.Value, pkgFiles, mod, pkgDir, importPath, registry),
		}
	case *ast.StructType:
		return structSchema(t, pkgFiles, mod, pkgDir, importPath, registry)
	}
	return map[string]any{"type": "object"}
}

func structSchema(st *ast.StructType, pkgFiles []*ast.File, mod *models.Module, pkgDir, importPath string, registry map[string]any) any {
	properties := map[string]any{}
	var required []string
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		name, omitempty, skip := jsonFieldName(field)
		if skip {
			continue
		}
		for _, fname := range field.Names {
			if !fname.IsExported() {
				continue
			}
			prop := name
			if prop == "" {
				prop = fname.Name
			}
			properties[prop] = exprToSchema(field.Type, pkgFiles, mod, pkgDir, importPath, registry)
			if !omitempty {
				required = append(required, prop)
			}
		}
	}
	schema := map[string]any{"type": "object", "properties": properties}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func jsonFieldName(field *ast.Field) (name string, omitempty bool, skip bool) {
	if field.Tag == nil {
		return "", false, false
	}
	tag := reflect.StructTag(strings.Trim(field.Tag.Value, "`")).Get("json")
	if tag == "" {
		return "", false, false
	}
	parts := strings.Split(tag, ",")
	if parts[0] == "-" {
		return "", false, true
	}
	for _, p := range parts[1:] {
		if p == "omitempty" {
			omitempty = true
		}
	}
	return parts[0], omitempty, false
}

func findLocalTypeSpec(pkgFiles []*ast.File, name string) *ast.TypeSpec {
	for _, f := range pkgFiles {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == name {
					return ts
				}
			}
		}
	}
	return nil
}

func basicTypeSchema(name string) (map[string]any, bool) {
	switch name {
	case "string", "rune":
		return map[string]any{"type": "string"}, true
	case "bool":
		return map[string]any{"type": "boolean"}, true
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "byte", "uintptr":
		return map[string]any{"type": "integer"}, true
	case "float32", "float64":
		return map[string]any{"type": "number"}, true
	case "any", "interface{}":
		return map[string]any{}, true
	}
	return nil, false
}
