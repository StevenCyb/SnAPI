package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenCyb/SnAPI/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testHandlerStructModule = "github.com/example/app"

func writeHandlerStructProject(t *testing.T, fileName, content string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module "+testHandlerStructModule+"\n\ngo 1.21\n"), 0o600))
	pkgDir := filepath.Join(dir, "api")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	filePath := filepath.Join(pkgDir, fileName)
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o600))
	return dir, filePath
}

func runHandlerStruct(t *testing.T, dir string) (*Parser, error) {
	t.Helper()
	p := NewParser(dir)
	require.NoError(t, p.parseModule())
	return p, p.parseHandlerStruct()
}

// ---------------------------------------------------------------------------
// toVarName
// ---------------------------------------------------------------------------

func TestToVarName(t *testing.T) {
	t.Parallel()
	cases := []struct{ in, want string }{
		{"CRUD", "crud"},
		{"MyHandler", "myHandler"},
		{"UserAPI", "userAPI"},
		{"API", "api"},
		{"already", "already"},
		{"A", "a"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, toVarName(tc.in), "toVarName(%q)", tc.in)
	}
}

// ---------------------------------------------------------------------------
// Valid handler struct
// ---------------------------------------------------------------------------

func TestParseHandlerStruct_Valid(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "crud.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

type CRUD struct{}

func (c *CRUD) Constructor() error { return nil }
func (c *CRUD) Destructor() {}

// @snapi.GET("/items")
func (c *CRUD) List(r runtime.Request, w runtime.Response) {}

// @snapi.POST("/items")
func (c *CRUD) Create(r runtime.Request, w runtime.Response) {}
`)
	p, err := runHandlerStruct(t, dir)
	require.NoError(t, err)
	require.Len(t, p.project.HandlerStructs, 1)

	hs := p.project.HandlerStructs[0]
	assert.Equal(t, "api", hs.Package)
	assert.Equal(t, testHandlerStructModule+"/api", hs.ImportPath)
	assert.Equal(t, "CRUD", hs.Name)
	assert.Equal(t, "crud", hs.VarName)
	assert.True(t, hs.ConstructorReturnsError)
	require.Len(t, hs.Methods, 2)

	names := []string{hs.Methods[0].Name, hs.Methods[1].Name}
	assert.ElementsMatch(t, []string{"List", "Create"}, names)
}

func TestParseHandlerStruct_ConstructorNoReturn(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "item.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

type Item struct{}

func (i *Item) Constructor() {}
func (i *Item) Destructor()  {}

// @snapi.GET("/items")
func (i *Item) List(r runtime.Request, w runtime.Response) {}
`)
	p, err := runHandlerStruct(t, dir)
	require.NoError(t, err)
	require.Len(t, p.project.HandlerStructs, 1)
	assert.False(t, p.project.HandlerStructs[0].ConstructorReturnsError)
}

// ---------------------------------------------------------------------------
// Missing Constructor / Destructor — now optional
// ---------------------------------------------------------------------------

func TestParseHandlerStruct_MissingConstructor(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "item.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

type Item struct{}

func (i *Item) Destructor() {}

// @snapi.GET("/items")
func (i *Item) List(r runtime.Request, w runtime.Response) {}
`)
	p, err := runHandlerStruct(t, dir)
	require.NoError(t, err)
	require.Len(t, p.project.HandlerStructs, 1)
	assert.False(t, p.project.HandlerStructs[0].HasConstructor)
	assert.True(t, p.project.HandlerStructs[0].HasDestructor)
}

func TestParseHandlerStruct_MissingDestructor(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "item.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

type Item struct{}

func (i *Item) Constructor() error { return nil }

// @snapi.GET("/items")
func (i *Item) List(r runtime.Request, w runtime.Response) {}
`)
	p, err := runHandlerStruct(t, dir)
	require.NoError(t, err)
	require.Len(t, p.project.HandlerStructs, 1)
	assert.True(t, p.project.HandlerStructs[0].HasConstructor)
	assert.False(t, p.project.HandlerStructs[0].HasDestructor)
}

func TestParseHandlerStruct_NeitherConstructorNorDestructor(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "item.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

type Item struct{}

// @snapi.GET("/items")
func (i *Item) List(r runtime.Request, w runtime.Response) {}
`)
	p, err := runHandlerStruct(t, dir)
	require.NoError(t, err)
	require.Len(t, p.project.HandlerStructs, 1)
	assert.False(t, p.project.HandlerStructs[0].HasConstructor)
	assert.False(t, p.project.HandlerStructs[0].HasDestructor)
}

// ---------------------------------------------------------------------------
// Invalid Constructor / Destructor signatures
// ---------------------------------------------------------------------------

func TestParseHandlerStruct_ConstructorWithParams(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "item.go", `package api

type Item struct{}

func (i *Item) Constructor(x int) error { return nil }
func (i *Item) Destructor() {}
`)
	_, err := runHandlerStruct(t, dir)
	require.Error(t, err)
	var hsErr *InvalidHandlerStructError
	require.ErrorAs(t, err, &hsErr)
	assert.Equal(t, "Constructor", hsErr.FuncName)
}

func TestParseHandlerStruct_ConstructorWrongReturnType(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "item.go", `package api

type Item struct{}

func (i *Item) Constructor() string { return "" }
func (i *Item) Destructor() {}
`)
	_, err := runHandlerStruct(t, dir)
	require.Error(t, err)
	var hsErr *InvalidHandlerStructError
	require.ErrorAs(t, err, &hsErr)
	assert.Equal(t, "Constructor", hsErr.FuncName)
}

func TestParseHandlerStruct_DestructorWithParams(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "item.go", `package api

type Item struct{}

func (i *Item) Constructor() error { return nil }
func (i *Item) Destructor(x int)   {}
`)
	_, err := runHandlerStruct(t, dir)
	require.Error(t, err)
	var hsErr *InvalidHandlerStructError
	require.ErrorAs(t, err, &hsErr)
	assert.Equal(t, "Destructor", hsErr.FuncName)
}

func TestParseHandlerStruct_DestructorWithReturn(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "item.go", `package api

type Item struct{}

func (i *Item) Constructor() error { return nil }
func (i *Item) Destructor() error  { return nil }
`)
	_, err := runHandlerStruct(t, dir)
	require.Error(t, err)
	var hsErr *InvalidHandlerStructError
	require.ErrorAs(t, err, &hsErr)
	assert.Equal(t, "Destructor", hsErr.FuncName)
}

// ---------------------------------------------------------------------------
// Handler method validation
// ---------------------------------------------------------------------------

func TestParseHandlerStruct_MethodTooFewParams(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "item.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

type Item struct{}

func (i *Item) Constructor() error { return nil }
func (i *Item) Destructor() {}

// @snapi.GET("/items")
func (i *Item) List(r runtime.Request) {}
`)
	_, err := runHandlerStruct(t, dir)
	require.ErrorIs(t, err, ErrExpectedAtLeast2Params)
}

func TestParseHandlerStruct_MethodWrongFirstParam(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "item.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

type Item struct{}

func (i *Item) Constructor() error { return nil }
func (i *Item) Destructor() {}

// @snapi.GET("/items")
func (i *Item) List(x int, w runtime.Response) {}
`)
	_, err := runHandlerStruct(t, dir)
	require.ErrorIs(t, err, ErrFirstParamMustBeRequest)
}

// ---------------------------------------------------------------------------
// Struct without @snapi methods is not treated as a handler struct
// ---------------------------------------------------------------------------

func TestParseHandlerStruct_NoAnnotatedMethods(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "helper.go", `package api

type Helper struct{}

func (h *Helper) Constructor() error { return nil }
func (h *Helper) Destructor() {}
func (h *Helper) DoSomething() {}
`)
	p, err := runHandlerStruct(t, dir)
	require.NoError(t, err)
	assert.Empty(t, p.project.HandlerStructs)
}

// ---------------------------------------------------------------------------
// handlerExtractor must ignore methods with receivers
// ---------------------------------------------------------------------------

func TestParseHandler_IgnoresStructMethods(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerProject(t, "mixed.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

type CRUD struct{}

// @snapi.GET("/items")  — this is a method, NOT a package-level handler
func (c *CRUD) List(r runtime.Request, w runtime.Response) {}
`)
	p, err := runHandler(t, dir)
	require.NoError(t, err)
	assert.Empty(t, p.project.HandlerFuncs, "method with @snapi must NOT appear in HandlerFuncs")
}

// ---------------------------------------------------------------------------
// Full Parse() surfaces struct handlers
// ---------------------------------------------------------------------------

func TestParser_Parse_PopulatesHandlerStructs(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "crud.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

type CRUD struct{}

func (c *CRUD) Constructor() error { return nil }
func (c *CRUD) Destructor() {}

// @snapi.GET("/items")
func (c *CRUD) List(r runtime.Request, w runtime.Response) {}
`)
	proj, err := NewParser(dir).Parse()
	require.NoError(t, err)
	require.Len(t, proj.HandlerStructs, 1)
	assert.Equal(t, "CRUD", proj.HandlerStructs[0].Name)
	assert.Equal(t, "crud", proj.HandlerStructs[0].VarName)
	require.Len(t, proj.HandlerStructs[0].Methods, 1)
	assert.Equal(t, "List", proj.HandlerStructs[0].Methods[0].Name)
}

// ---------------------------------------------------------------------------
// Struct-level @SnAPI.Path and @SnAPI.UseMiddleware
// ---------------------------------------------------------------------------

func TestParseHandlerStruct_PathPrefix(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "crud.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @SnAPI.Path("/books")
type Books struct{}

func (b *Books) Constructor() error { return nil }
func (b *Books) Destructor()        {}

// @SnAPI.GET("/")
func (b *Books) List(r runtime.Request, w runtime.Response) {}

// @SnAPI.GET("/{id}")
func (b *Books) Get(r runtime.Request, w runtime.Response) {}
`)
	p, err := runHandlerStruct(t, dir)
	require.NoError(t, err)
	require.Len(t, p.project.HandlerStructs, 1)
	hs := p.project.HandlerStructs[0]
	assert.Equal(t, "/books", hs.PathPrefix)
}

func TestParseHandlerStruct_StructMiddleware(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "crud.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @SnAPI.UseMiddleware(api.LoggingMiddleware)
// @SnAPI.UseMiddleware(api.AuthMiddleware)
type Items struct{}

// @SnAPI.GET("/items")
func (i *Items) List(r runtime.Request, w runtime.Response) {}
`)
	p, err := runHandlerStruct(t, dir)
	require.NoError(t, err)
	require.Len(t, p.project.HandlerStructs, 1)
	hs := p.project.HandlerStructs[0]
	assert.Equal(t, []string{"api.LoggingMiddleware", "api.AuthMiddleware"}, hs.Middleware)
}

func TestParseHandlerStruct_NoStructAnnotations(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "item.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

type Item struct{}

// @SnAPI.GET("/items")
func (i *Item) List(r runtime.Request, w runtime.Response) {}
`)
	p, err := runHandlerStruct(t, dir)
	require.NoError(t, err)
	require.Len(t, p.project.HandlerStructs, 1)
	hs := p.project.HandlerStructs[0]
	assert.Empty(t, hs.PathPrefix)
	assert.Empty(t, hs.Middleware)
	assert.Empty(t, hs.Tags)
}

// ---------------------------------------------------------------------------
// Struct-level @SnAPI.Tags inheritance
// ---------------------------------------------------------------------------

func TestParseHandlerStruct_StructTagsInheritedByMethods(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "crud.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @SnAPI.Tags("books")
type CRUD struct{}

// @SnAPI.GET("/")
func (c *CRUD) List(r runtime.Request, w runtime.Response) {}

// @SnAPI.POST("/")
// @SnAPI.Tags("write")
func (c *CRUD) Create(r runtime.Request, w runtime.Response) {}
`)
	p, err := runHandlerStruct(t, dir)
	require.NoError(t, err)
	require.Len(t, p.project.HandlerStructs, 1)
	hs := p.project.HandlerStructs[0]
	assert.Equal(t, []string{"books"}, hs.Tags)

	// List has no method-level tags → inherits struct tags only
	listMethod := findMethod(hs.Methods, "List")
	require.NotNil(t, listMethod)
	assert.Equal(t, []string{"books"}, listMethod.Meta.Tags)

	// Create has its own tag → struct tag prepended, method tag appended
	createMethod := findMethod(hs.Methods, "Create")
	require.NotNil(t, createMethod)
	assert.Equal(t, []string{"books", "write"}, createMethod.Meta.Tags)
}

func TestParseHandlerStruct_StructTagsDeduplicated(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "crud.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @SnAPI.Tags("books")
type CRUD struct{}

// @SnAPI.GET("/")
// @SnAPI.Tags("books", "extra")
func (c *CRUD) List(r runtime.Request, w runtime.Response) {}
`)
	p, err := runHandlerStruct(t, dir)
	require.NoError(t, err)
	listMethod := findMethod(p.project.HandlerStructs[0].Methods, "List")
	require.NotNil(t, listMethod)
	// "books" appears only once even though it is in both struct and method tags
	assert.Equal(t, []string{"books", "extra"}, listMethod.Meta.Tags)
}

func findMethod(methods []models.HandlerFunc, name string) *models.HandlerFunc {
	for i := range methods {
		if methods[i].Name == name {
			return &methods[i]
		}
	}
	return nil
}
