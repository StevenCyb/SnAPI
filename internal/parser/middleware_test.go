package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenCyb/SnAPI/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testMiddlewareModule = "github.com/example/app"

func writeMiddlewareProject(t *testing.T, fileName, content string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module "+testMiddlewareModule+"\n\ngo 1.21\n"), 0o600))
	pkgDir := filepath.Join(dir, "api")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	filePath := filepath.Join(pkgDir, fileName)
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o600))
	return dir, filePath
}

func runMiddleware(t *testing.T, dir string) (*Parser, error) {
	t.Helper()
	p := NewParser(dir)
	require.NoError(t, p.parseModule())
	return p, p.parseMiddleware()
}

func middlewareFuncTester(t *testing.T, content string, expected []models.MiddlewareFunc, expectErr bool) {
	t.Helper()
	dir, _ := writeMiddlewareProject(t, "file.go", content)
	p, err := runMiddleware(t, dir)
	if expectErr {
		require.Error(t, err)
		var mwErr *InvalidMiddlewareFuncError
		require.ErrorAs(t, err, &mwErr)
		return
	}
	require.NoError(t, err)
	assert.Equal(t, expected, p.project.MiddlewareFuncs)
}

func TestParseMiddleware_NoMiddleware(t *testing.T) {
	t.Parallel()

	middlewareFuncTester(t, `package api

func Something() {}
`, nil, false)
}

func TestParseMiddleware_ValidMiddleware(t *testing.T) {
	t.Parallel()

	middlewareFuncTester(t, `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @snapi.middleware
func MyMiddleware(runtime.Request, runtime.Response, runtime.HandlerFunc) {}
`,
		[]models.MiddlewareFunc{{
			Package:    "api",
			ImportPath: testMiddlewareModule + "/api",
			Name:       "MyMiddleware",
		}}, false)
}

func TestParseMiddleware_InvalidReturn(t *testing.T) {
	t.Parallel()

	middlewareFuncTester(t, `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @snapi.middleware
func BadMiddleware(runtime.Request, runtime.Response, runtime.HandlerFunc) error { return nil }
`, nil, true)
}

func TestParseMiddleware_WrongParamCount(t *testing.T) {
	t.Parallel()

	middlewareFuncTester(t, `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @snapi.middleware
func TooFew(runtime.Request, runtime.Response) {}
`, nil, true)
}

func TestParseMiddleware_WrongParamTypes(t *testing.T) {
	t.Parallel()

	middlewareFuncTester(t, `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @snapi.middleware
func BadTypes(runtime.Request, int, runtime.HandlerFunc) {}
`, nil, true)
}

func TestParseMiddleware_NonSelectorParam(t *testing.T) {
	t.Parallel()

	middlewareFuncTester(t, `package api

// @snapi.middleware
func BadParam(int, string, bool) {}
`, nil, true)
}

func TestParseMiddleware_WrongSelectorOrder(t *testing.T) {
	t.Parallel()

	middlewareFuncTester(t, `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @snapi.middleware
func WrongOrder(runtime.Response, runtime.Request, runtime.HandlerFunc) {}
`, nil, true)
}

func TestParseMiddleware_SkipMainPackage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module "+testMiddlewareModule+"\n\ngo 1.21\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @snapi.middleware
func MyMiddleware(runtime.Request, runtime.Response, runtime.HandlerFunc) {}
`), 0o600))

	p := NewParser(dir)
	require.NoError(t, p.parseModule())
	require.NoError(t, p.parseMiddleware())
	assert.Nil(t, p.project.MiddlewareFuncs)
}

func TestParseMiddleware_SkipTestFiles(t *testing.T) {
	t.Parallel()

	dir, _ := writeMiddlewareProject(t, "file_test.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @snapi.middleware
func MyMiddleware(runtime.Request, runtime.Response, runtime.HandlerFunc) {}
`)
	p, err := runMiddleware(t, dir)
	require.NoError(t, err)
	assert.Nil(t, p.project.MiddlewareFuncs)
}

func TestParseMiddleware_ParseError(t *testing.T) {
	t.Parallel()

	dir, _ := writeMiddlewareProject(t, "broken.go", "package api\n\nthis is not valid go\n")
	_, err := runMiddleware(t, dir)
	require.Error(t, err)
	var pErr *ParsingFileError
	require.ErrorAs(t, err, &pErr)
}

func TestParseMiddleware_RequiresMainModule(t *testing.T) {
	t.Parallel()

	p := NewParser(t.TempDir())
	err := p.parseMiddleware()
	require.ErrorIs(t, err, ErrModulePathNotFound)
}

func TestParser_Parse_PopulatesMiddleware(t *testing.T) {
	t.Parallel()

	dir, _ := writeMiddlewareProject(t, "file.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @snapi.middleware
func MyMiddleware(runtime.Request, runtime.Response, runtime.HandlerFunc) {}
`)
	proj, err := NewParser(dir).Parse()
	require.NoError(t, err)
	require.Len(t, proj.MiddlewareFuncs, 1)
	assert.Equal(t, "MyMiddleware", proj.MiddlewareFuncs[0].Name)
}
