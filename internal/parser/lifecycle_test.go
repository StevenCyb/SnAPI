package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenCyb/SnAPI/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testLifecycleModule = "github.com/example/app"
	testLifecyclePkg    = "api"
)

func writeLifecycleProject(t *testing.T, fileName, content string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module "+testLifecycleModule+"\n\ngo 1.21\n"), 0o600))
	pkgDir := filepath.Join(dir, "api")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	filePath := filepath.Join(pkgDir, fileName)
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o600))
	return dir, filePath
}

func runLifecycle(t *testing.T, dir string) (*Parser, error) {
	t.Helper()
	p := NewParser(dir)
	require.NoError(t, p.parseModule())
	return p, p.parseLifecycle()
}

func lifecycleFuncTester(t *testing.T, content string, expectedSetup, expectedTeardown []models.LifecycleFunc, expectErr bool) {
	t.Helper()
	dir, _ := writeLifecycleProject(t, "file.go", content)
	p, err := runLifecycle(t, dir)
	if expectErr {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)
	assert.Equal(t, expectedSetup, p.project.SetupFuncs)
	assert.Equal(t, expectedTeardown, p.project.TeardownFuncs)
}

func TestParseLifecycle_NoLifecycleFuncs(t *testing.T) {
	t.Parallel()

	lifecycleFuncTester(t, `package api

func Something() {}
`, nil, nil, false)
}

func TestParseLifecycle_SingleSetupFunc(t *testing.T) {
	t.Parallel()

	lifecycleFuncTester(t, `package api

// @snapi.setup
func Setup() error { return nil }
`,
		[]models.LifecycleFunc{{
			Package:      testLifecyclePkg,
			ImportPath:   testLifecycleModule + "/api",
			Name:         "Setup",
			ReturnsError: true,
		}}, nil, false)
}

func TestParseLifecycle_SetupFuncWithoutErrorReturn(t *testing.T) {
	t.Parallel()

	lifecycleFuncTester(t, `package api

// @snapi.setup
func Setup() {}
`,
		[]models.LifecycleFunc{{
			Package:    testLifecyclePkg,
			ImportPath: testLifecycleModule + "/api",
			Name:       "Setup",
		}}, nil, false)
}

func TestParseLifecycle_SetupFuncWrongReturnType(t *testing.T) {
	t.Parallel()

	dir, _ := writeLifecycleProject(t, "file.go", `package api

// @snapi.setup
func Setup() int { return 1 }
`)
	_, err := runLifecycle(t, dir)
	require.Error(t, err)
	var lifecycleErr *InvalidLifecycleFuncError
	require.ErrorAs(t, err, &lifecycleErr)
	assert.Contains(t, lifecycleErr.Reason, "setup function return type must be error")
}

func TestParseLifecycle_SetupFuncTooManyReturns(t *testing.T) {
	t.Parallel()

	dir, _ := writeLifecycleProject(t, "file.go", `package api

// @snapi.setup
func Setup() (error, error) { return nil, nil }
`)
	_, err := runLifecycle(t, dir)
	require.Error(t, err)
	var lifecycleErr *InvalidLifecycleFuncError
	require.ErrorAs(t, err, &lifecycleErr)
	assert.Contains(t, lifecycleErr.Reason, "setup function must return at most one value")
}

func TestParseLifecycle_SingleTeardownFunc(t *testing.T) {
	t.Parallel()

	lifecycleFuncTester(t, `package api

// @snapi.teardown
func Teardown() {}
`, nil, []models.LifecycleFunc{{
		Package:    testLifecyclePkg,
		ImportPath: testLifecycleModule + "/api",
		Name:       "Teardown",
	}}, false)
}

func TestParseLifecycle_BothLifecycleFuncs(t *testing.T) {
	t.Parallel()

	lifecycleFuncTester(t, `package api

// @snapi.setup
func Setup() error { return nil }

// @snapi.teardown
func Teardown() {}
`,
		[]models.LifecycleFunc{{
			Package:      testLifecyclePkg,
			ImportPath:   testLifecycleModule + "/api",
			Name:         "Setup",
			ReturnsError: true,
		}},
		[]models.LifecycleFunc{{
			Package:    testLifecyclePkg,
			ImportPath: testLifecycleModule + "/api",
			Name:       "Teardown",
		}}, false)
}

func TestParseLifecycle_InvalidSetupWithParams(t *testing.T) {
	t.Parallel()

	dir, _ := writeLifecycleProject(t, "file.go", `package api

// @snapi.setup
func Setup(a int) {}
`)
	_, err := runLifecycle(t, dir)
	require.Error(t, err)
	var lifecycleErr *InvalidLifecycleFuncError
	require.ErrorAs(t, err, &lifecycleErr)
	assert.Contains(t, lifecycleErr.Reason, "lifecycle functions should not have parameters")
}

func TestParseLifecycle_InvalidTeardownWithParams(t *testing.T) {
	t.Parallel()

	dir, _ := writeLifecycleProject(t, "file.go", `package api

// @snapi.teardown
func Teardown(a int) {}
`)
	_, err := runLifecycle(t, dir)
	require.Error(t, err)
	var lifecycleErr *InvalidLifecycleFuncError
	require.ErrorAs(t, err, &lifecycleErr)
	assert.Contains(t, lifecycleErr.Reason, "lifecycle functions should not have parameters")
}

func TestParseLifecycle_SkipMainPackage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module "+testLifecycleModule+"\n\ngo 1.21\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

// @snapi.setup
func Setup() {}
`), 0o600))

	p := NewParser(dir)
	require.NoError(t, p.parseModule())
	require.NoError(t, p.parseLifecycle())
	assert.Nil(t, p.project.SetupFuncs)
	assert.Nil(t, p.project.TeardownFuncs)
}

func TestParseLifecycle_SkipTestFiles(t *testing.T) {
	t.Parallel()

	dir, _ := writeLifecycleProject(t, "file_test.go", `package api

// @snapi.setup
func Setup() {}
`)
	p, err := runLifecycle(t, dir)
	require.NoError(t, err)
	assert.Nil(t, p.project.SetupFuncs)
}

func TestParseLifecycle_ParseError(t *testing.T) {
	t.Parallel()

	dir, _ := writeLifecycleProject(t, "broken.go", "package api\n\nthis is not valid go\n")
	_, err := runLifecycle(t, dir)
	require.Error(t, err)
	var pErr *ParsingFileError
	require.ErrorAs(t, err, &pErr)
}

func TestParseLifecycle_ImportPathRoot(t *testing.T) {
	t.Parallel()

	// File directly in module root → import path == module path
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module "+testLifecycleModule+"\n\ngo 1.21\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.go"), []byte(`package app

// @snapi.setup
func Setup() {}
`), 0o600))

	p := NewParser(dir)
	require.NoError(t, p.parseModule())
	require.NoError(t, p.parseLifecycle())
	require.Len(t, p.project.SetupFuncs, 1)
	assert.Equal(t, testLifecycleModule, p.project.SetupFuncs[0].ImportPath)
}

func TestParseLifecycle_RequiresMainModule(t *testing.T) {
	t.Parallel()

	p := NewParser(t.TempDir())
	err := p.parseLifecycle()
	require.ErrorIs(t, err, ErrModulePathNotFound)
}

func TestParser_Parse_PopulatesLifecycle(t *testing.T) {
	t.Parallel()

	dir, _ := writeLifecycleProject(t, "file.go", `package api

// @snapi.setup
func Setup() error { return nil }

// @snapi.teardown
func Teardown() {}
`)
	proj, err := NewParser(dir).Parse()
	require.NoError(t, err)
	require.Len(t, proj.SetupFuncs, 1)
	require.Len(t, proj.TeardownFuncs, 1)
	assert.Equal(t, "Setup", proj.SetupFuncs[0].Name)
	assert.Equal(t, "Teardown", proj.TeardownFuncs[0].Name)
}
