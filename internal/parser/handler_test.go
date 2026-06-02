package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testHandlerModule = "github.com/example/app"

func writeHandlerProject(t *testing.T, fileName, content string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module "+testHandlerModule+"\n\ngo 1.21\n"), 0o600))
	pkgDir := filepath.Join(dir, "api")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	filePath := filepath.Join(pkgDir, fileName)
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o600))
	return dir, filePath
}

func runHandler(t *testing.T, dir string) (*Parser, error) {
	t.Helper()
	p := NewParser(dir)
	require.NoError(t, p.parseModule())
	return p, p.parseHandler()
}

func TestExtractHandlerMeta_NoMethodReturnsNil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, extractHandlerMeta("@snapi.description(\"x\")"))
}

func TestExtractHandlerMeta_FullAnnotations(t *testing.T) {
	t.Parallel()

	comments := strings.Join([]string{
		`@snapi.POST("/users")`,
		`@snapi.description("create user")`,
		`@snapi.usemiddleware(Auth)`,
		`@snapi.usemiddleware(Logger)`,
		`@snapi.deprecated`,
		`@snapi.status(200, "OK")`,
		`@snapi.status(500)`,
		`@snapi.tags("a", "b")`,
		`@snapi.request("application/xml", model.User)`,
	}, "\n")
	meta := extractHandlerMeta(comments)
	require.NotNil(t, meta)
	assert.Equal(t, "POST", meta.Method)
	assert.Equal(t, "/users", meta.Path)
	require.NotNil(t, meta.Description)
	assert.Equal(t, "create user", *meta.Description)
	assert.Equal(t, []string{"Auth", "Logger"}, meta.Middleware)
	assert.True(t, meta.Deprecated)
	require.Len(t, meta.Status, 2)
	assert.Equal(t, "200", meta.Status[0].Code)
	require.NotNil(t, meta.Status[0].Description)
	assert.Equal(t, "OK", *meta.Status[0].Description)
	assert.Equal(t, "500", meta.Status[1].Code)
	assert.Nil(t, meta.Status[1].Description)
	assert.Equal(t, []string{"a", "b"}, meta.Tags)
	require.Len(t, meta.Requests, 1)
	assert.Equal(t, "application/xml", meta.Requests[0].ContentType)
	assert.Equal(t, "model.User", meta.Requests[0].Model)
}

func TestExtractHandlerMeta_DefaultPath(t *testing.T) {
	t.Parallel()
	meta := extractHandlerMeta(`@snapi.GET()`)
	require.NotNil(t, meta)
	assert.Equal(t, "GET", meta.Method)
	assert.Equal(t, "/", meta.Path)
}

func TestExtractHandlerMeta_RequestDefaultsToJSON(t *testing.T) {
	t.Parallel()
	meta := extractHandlerMeta("@snapi.GET(\"/\")\n@snapi.request(model.User)")
	require.NotNil(t, meta)
	require.Len(t, meta.Requests, 1)
	assert.Equal(t, "application/json", meta.Requests[0].ContentType)
	assert.Equal(t, "model.User", meta.Requests[0].Model)
}

func TestParseHandler_ValidHandler(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerProject(t, "file.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @snapi.GET("/users")
// @snapi.description("list users")
func List(r runtime.Request, w runtime.Response) {}
`)
	p, err := runHandler(t, dir)
	require.NoError(t, err)
	require.Len(t, p.project.HandlerFuncs, 1)
	h := p.project.HandlerFuncs[0]
	assert.Equal(t, "api", h.Package)
	assert.Equal(t, testHandlerModule+"/api", h.ImportPath)
	assert.Equal(t, "List", h.Name)
	require.NotNil(t, h.Meta)
	assert.Equal(t, "GET", h.Meta.Method)
	assert.Equal(t, "/users", h.Meta.Path)
	require.NotNil(t, h.Meta.Description)
	assert.Equal(t, "list users", *h.Meta.Description)
	assert.Empty(t, h.Services)
	assert.Equal(t, "github.com/StevenCyb/SnAPI/pkg/runtime", h.Imports["runtime"])
}

func TestParseHandler_WithServices(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerProject(t, "file.go", `package api

import (
	"github.com/StevenCyb/SnAPI/pkg/runtime"
	svc "github.com/example/app/services"
)

// @snapi.POST("/users")
func Create(r runtime.Request, w runtime.Response, s svc.UserService, n *svc.Notifier) {}
`)
	p, err := runHandler(t, dir)
	require.NoError(t, err)
	require.Len(t, p.project.HandlerFuncs, 1)
	h := p.project.HandlerFuncs[0]
	assert.Equal(t, []string{"svc.UserService", "*svc.Notifier"}, h.Services)
	assert.Equal(t, "github.com/example/app/services", h.Imports["svc"])
}

func TestParseHandler_TooFewParams(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerProject(t, "file.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @snapi.GET("/")
func Bad(r runtime.Request) {}
`)
	_, err := runHandler(t, dir)
	require.ErrorIs(t, err, ErrExpectedAtLeast2Params)
}

func TestParseHandler_WrongFirstParam(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerProject(t, "file.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @snapi.GET("/")
func Bad(r int, w runtime.Response) {}
`)
	_, err := runHandler(t, dir)
	require.ErrorIs(t, err, ErrFirstParamMustBeRequest)
}

func TestParseHandler_WrongSecondParam(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerProject(t, "file.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @snapi.GET("/")
func Bad(r runtime.Request, w int) {}
`)
	_, err := runHandler(t, dir)
	require.ErrorIs(t, err, ErrSecondParamMustBeResponse)
}

func TestParseHandler_DuplicateServiceParam(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerProject(t, "file.go", `package api

import (
	"github.com/StevenCyb/SnAPI/pkg/runtime"
	svc "github.com/example/app/services"
)

// @snapi.GET("/")
func Bad(r runtime.Request, w runtime.Response, a svc.S, b svc.S) {}
`)
	_, err := runHandler(t, dir)
	require.Error(t, err)
	var svcErr *InvalidServiceParamError
	require.ErrorAs(t, err, &svcErr)
	assert.Contains(t, svcErr.Reason, "duplicate")
}

func TestParseHandler_ServiceParamMultiName(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerProject(t, "file.go", `package api

import (
	"github.com/StevenCyb/SnAPI/pkg/runtime"
	svc "github.com/example/app/services"
)

// @snapi.GET("/")
func Bad(r runtime.Request, w runtime.Response, a, b svc.S) {}
`)
	_, err := runHandler(t, dir)
	require.Error(t, err)
	var svcErr *InvalidServiceParamError
	require.ErrorAs(t, err, &svcErr)
	assert.Contains(t, svcErr.Reason, "single name")
}

func TestParseHandler_NoHandlersInFile(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerProject(t, "file.go", `package api

func Helper() {}
`)
	p, err := runHandler(t, dir)
	require.NoError(t, err)
	assert.Nil(t, p.project.HandlerFuncs)
}

func TestParseHandler_SkipMainPackage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module "+testHandlerModule+"\n\ngo 1.21\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @snapi.GET("/")
func List(r runtime.Request, w runtime.Response) {}
`), 0o600))

	p := NewParser(dir)
	require.NoError(t, p.parseModule())
	require.NoError(t, p.parseHandler())
	assert.Nil(t, p.project.HandlerFuncs)
}

func TestParseHandler_SkipsAnonymousAndDotImports(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerProject(t, "file.go", `package api

import (
	"github.com/StevenCyb/SnAPI/pkg/runtime"
	_ "github.com/example/app/ignored"
	. "github.com/example/app/dotted"
)

// @snapi.GET("/")
func List(r runtime.Request, w runtime.Response) {}
`)
	p, err := runHandler(t, dir)
	require.NoError(t, err)
	require.Len(t, p.project.HandlerFuncs, 1)
	h := p.project.HandlerFuncs[0]
	_, hasUnderscore := h.Imports["_"]
	assert.False(t, hasUnderscore)
	_, hasDot := h.Imports["."]
	assert.False(t, hasDot)
}

func TestParser_Parse_PopulatesHandlers(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerProject(t, "file.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @snapi.GET("/users")
func List(r runtime.Request, w runtime.Response) {}
`)
	proj, err := NewParser(dir).Parse()
	require.NoError(t, err)
	require.Len(t, proj.HandlerFuncs, 1)
	assert.Equal(t, "List", proj.HandlerFuncs[0].Name)
}
