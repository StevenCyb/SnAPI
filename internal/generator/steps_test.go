package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenCyb/SnAPI/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleProject() *models.Project {
	desc := "list users"
	return &models.Project{
		MainModule: &models.Module{Path: "github.com/example/app", GoVersion: "1.21"},
		SetupFuncs: []models.LifecycleFunc{
			{Package: "api", ImportPath: "github.com/example/app/api", Name: "Init", ReturnsError: true},
		},
		TeardownFuncs: []models.LifecycleFunc{
			{Package: "api", ImportPath: "github.com/example/app/api", Name: "Close"},
		},
		MiddlewareFuncs: []models.MiddlewareFunc{
			{Package: "api", ImportPath: "github.com/example/app/api", Name: "Auth"},
		},
		HandlerFuncs: []models.HandlerFunc{{
			Package: "api", ImportPath: "github.com/example/app/api", Name: "List",
			Meta: &models.HandlerMeta{
				Method: "GET", Path: "/users",
				Description: &desc,
				Middleware:  []string{"Auth"},
				Status:      []models.HandlerStatus{{Code: "200"}},
				Tags:        []string{"users"},
			},
			Imports: map[string]string{"runtime": "github.com/StevenCyb/SnAPI/pkg/runtime"},
		}},
	}
}

func TestGenerateServe_WritesMain(t *testing.T) {
	t.Parallel()
	dst := t.TempDir()
	g := NewGenerator(sampleProject(), dst, Config{Addr: ":9090"})

	require.NoError(t, g.generateServe())

	data, err := os.ReadFile(filepath.Join(dst, "main.go"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, `const addr = ":9090"`)
	assert.Contains(t, content, "registerRoutes(mux)")
	assert.NotContains(t, content, "registerSwaggerHandlers")
	assert.Contains(t, content, "api.Init()")
	assert.Contains(t, content, "api.Close()")
}

func TestGenerateServe_WithSwagger(t *testing.T) {
	t.Parallel()
	dst := t.TempDir()
	g := NewGenerator(sampleProject(), dst, Config{Swagger: &SwaggerConfig{}})

	require.NoError(t, g.generateServe())

	data, _ := os.ReadFile(filepath.Join(dst, "main.go"))
	assert.Contains(t, string(data), "registerSwaggerHandlers(mux)")
}

func TestGenerateRoutes_WritesRoutes(t *testing.T) {
	t.Parallel()
	dst := t.TempDir()
	g := NewGenerator(sampleProject(), dst, Config{})

	require.NoError(t, g.generateRoutes())

	data, err := os.ReadFile(filepath.Join(dst, "routes.go"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, `mux.HandleFunc("GET /users"`)
	assert.Contains(t, content, "api.List(req, resp)")
	assert.Contains(t, content, "api.Auth,")
	assert.Contains(t, content, `"github.com/example/app/api"`)
}

func TestGenerateRoutes_UnknownMiddleware(t *testing.T) {
	t.Parallel()
	proj := sampleProject()
	proj.HandlerFuncs[0].Meta.Middleware = []string{"Ghost"}
	proj.MiddlewareFuncs = nil

	g := NewGenerator(proj, t.TempDir(), Config{})
	err := g.generateRoutes()
	require.Error(t, err)
	var mwErr *MiddlewareNotFoundError
	require.ErrorAs(t, err, &mwErr)
	assert.Equal(t, "Ghost", mwErr.Name)
}

func TestGenerateSwagger_WritesFile(t *testing.T) {
	t.Parallel()
	dst := t.TempDir()
	g := NewGenerator(sampleProject(), dst,
		Config{Swagger: &SwaggerConfig{Path: "/docs"}})

	require.NoError(t, g.generateSwagger())

	data, err := os.ReadFile(filepath.Join(dst, "swagger.go"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, `"GET /docs"`)
	assert.Contains(t, content, `"GET /docs/index.html"`)
	assert.Contains(t, content, `"GET /docs/doc.json"`)
	assert.Contains(t, content, `\"openapi\":\"3.0.0\"`)
	assert.Contains(t, content, `\"/users\"`)
}

func TestGenerate_EndToEnd(t *testing.T) {
	t.Parallel()
	dst := t.TempDir()
	g := NewGenerator(sampleProject(), dst, Config{Swagger: &SwaggerConfig{}})

	require.NoError(t, g.Generate())

	for _, name := range []string{"go.mod", "main.go", "routes.go", "swagger.go"} {
		_, err := os.Stat(filepath.Join(dst, name))
		assert.NoError(t, err, "expected %s", name)
	}
}

func TestGenerateServe_WithHandlerStruct(t *testing.T) {
	t.Parallel()
	proj := sampleProject()
	proj.HandlerStructs = []models.HandlerStruct{
		{
			Package: "api", ImportPath: "github.com/example/app/api",
			Name: "CRUD", VarName: "crud",
			HasConstructor: true, ConstructorReturnsError: true,
			HasDestructor: true,
			Methods: []models.HandlerFunc{{
				Package: "api", Name: "List",
				Meta: &models.HandlerMeta{Method: "GET", Path: "/items"},
			}},
		},
	}
	dst := t.TempDir()
	g := NewGenerator(proj, dst, Config{})
	require.NoError(t, g.generateServe())

	data, err := os.ReadFile(filepath.Join(dst, "main.go"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "var crud = &api.CRUD{}")
	assert.Contains(t, content, "crud.Constructor()")
	assert.Contains(t, content, "crud.Destructor()")
}

func TestGenerateServe_HandlerStructNoLifecycle(t *testing.T) {
	t.Parallel()
	proj := sampleProject()
	proj.HandlerStructs = []models.HandlerStruct{
		{
			Package: "api", ImportPath: "github.com/example/app/api",
			Name: "Items", VarName: "items",
			HasConstructor: false, HasDestructor: false,
			Methods: []models.HandlerFunc{{
				Package: "api", Name: "List",
				Meta: &models.HandlerMeta{Method: "GET", Path: "/items"},
			}},
		},
	}
	dst := t.TempDir()
	g := NewGenerator(proj, dst, Config{})
	require.NoError(t, g.generateServe())

	data, err := os.ReadFile(filepath.Join(dst, "main.go"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "var items = &api.Items{}")
	assert.NotContains(t, content, "items.Constructor()")
	assert.NotContains(t, content, "items.Destructor()")
}

func TestGenerateRoutes_WithHandlerStruct(t *testing.T) {
	t.Parallel()
	proj := sampleProject()
	proj.HandlerStructs = []models.HandlerStruct{
		{
			Package: "api", ImportPath: "github.com/example/app/api",
			Name: "CRUD", VarName: "crud",
			Methods: []models.HandlerFunc{
				{Package: "api", Name: "List", Meta: &models.HandlerMeta{Method: "GET", Path: "/items"}},
				{Package: "api", Name: "Create", Meta: &models.HandlerMeta{Method: "POST", Path: "/items"}},
			},
		},
	}
	dst := t.TempDir()
	g := NewGenerator(proj, dst, Config{})
	require.NoError(t, g.generateRoutes())

	data, err := os.ReadFile(filepath.Join(dst, "routes.go"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, `mux.HandleFunc("GET /items"`)
	assert.Contains(t, content, "crud.List(req, resp)")
	assert.Contains(t, content, `mux.HandleFunc("POST /items"`)
	assert.Contains(t, content, "crud.Create(req, resp)")
}

func TestGenerateRoutes_HandlerStructPathPrefix(t *testing.T) {
	t.Parallel()
	proj := sampleProject()
	proj.HandlerStructs = []models.HandlerStruct{{
		Package: "api", ImportPath: "github.com/example/app/api",
		Name: "Books", VarName: "books",
		PathPrefix: "/books",
		Methods: []models.HandlerFunc{
			{Package: "api", Name: "List", Meta: &models.HandlerMeta{Method: "GET", Path: "/"}},
			{Package: "api", Name: "Get", Meta: &models.HandlerMeta{Method: "GET", Path: "/{id}"}},
		},
	}}
	dst := t.TempDir()
	g := NewGenerator(proj, dst, Config{})
	require.NoError(t, g.generateRoutes())

	data, _ := os.ReadFile(filepath.Join(dst, "routes.go"))
	content := string(data)
	// "/" collapsed into prefix → "GET /books", not "GET /books/"
	assert.Contains(t, content, `"GET /books"`)
	assert.NotContains(t, content, `"GET /books/"`)
	assert.Contains(t, content, `"GET /books/{id}"`)
}

func TestGenerateRoutes_HandlerStructInheritedMiddleware(t *testing.T) {
	t.Parallel()
	proj := sampleProject()
	proj.HandlerStructs = []models.HandlerStruct{{
		Package: "api", ImportPath: "github.com/example/app/api",
		Name: "Items", VarName: "items",
		Middleware: []string{"Auth"},
		Methods: []models.HandlerFunc{{
			Package: "api", Name: "List",
			Meta: &models.HandlerMeta{Method: "GET", Path: "/items", Middleware: []string{}},
		}},
	}}
	dst := t.TempDir()
	g := NewGenerator(proj, dst, Config{})
	require.NoError(t, g.generateRoutes())

	data, _ := os.ReadFile(filepath.Join(dst, "routes.go"))
	content := string(data)
	assert.Contains(t, content, "api.Auth,")
}

func TestGenerateRoutes_HandlerStructMergedMiddlewareOrder(t *testing.T) {
	t.Parallel()
	proj := sampleProject()
	proj.MiddlewareFuncs = []models.MiddlewareFunc{
		{Package: "api", ImportPath: "github.com/example/app/api", Name: "Auth"},
		{Package: "api", ImportPath: "github.com/example/app/api", Name: "Log"},
	}
	proj.HandlerStructs = []models.HandlerStruct{{
		Package: "api", ImportPath: "github.com/example/app/api",
		Name: "Items", VarName: "items",
		Middleware: []string{"Auth"},
		Methods: []models.HandlerFunc{{
			Package: "api", Name: "List",
			Meta: &models.HandlerMeta{Method: "GET", Path: "/items", Middleware: []string{"Log"}},
		}},
	}}
	dst := t.TempDir()
	g := NewGenerator(proj, dst, Config{})
	require.NoError(t, g.generateRoutes())

	data, _ := os.ReadFile(filepath.Join(dst, "routes.go"))
	content := string(data)
	authIdx := strings.Index(content, "api.Auth,")
	logIdx := strings.Index(content, "api.Log,")
	assert.Greater(t, authIdx, 0)
	assert.Greater(t, logIdx, 0)
	assert.Less(t, authIdx, logIdx, "struct-level Auth must appear before method-level Log")
}

func TestJoinPath(t *testing.T) {
	t.Parallel()
	cases := []struct{ prefix, path, want string }{
		{"", "/items", "/items"},
		{"/books", "/", "/books"},
		{"/books", "", "/books"},
		{"/books", "/{id}", "/books/{id}"},
		{"/books/", "/{id}", "/books/{id}"},
		{"/books", "/new", "/books/new"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, joinPath(tc.prefix, tc.path),
			"joinPath(%q, %q)", tc.prefix, tc.path)
	}
}

func TestGenerate_NoSwaggerSkipsSwaggerFile(t *testing.T) {
	t.Parallel()
	dst := t.TempDir()
	require.NoError(t, NewGenerator(sampleProject(), dst, Config{}).Generate())

	_, err := os.Stat(filepath.Join(dst, "swagger.go"))
	assert.True(t, os.IsNotExist(err))
}
