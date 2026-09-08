package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractMCPToolMeta_NoAnnotationReturnsNil(t *testing.T) {
	t.Parallel()
	assert.Nil(t, extractMCPToolMeta(`@snapi.description("x")`))
}

func TestExtractMCPToolMeta_FullAnnotations(t *testing.T) {
	t.Parallel()

	comments := strings.Join([]string{
		`@snapi.mcptool("greet", "greets someone")`,
		`@snapi.request(model.GreetArgs)`,
		`@snapi.mcpoutput(model.GreetResult)`,
		`@snapi.usemiddleware(Auth)`,
	}, "\n")

	meta := extractMCPToolMeta(comments)
	require.NotNil(t, meta)
	assert.Equal(t, "greet", meta.Name)
	assert.Equal(t, "greets someone", meta.Description)
	assert.Equal(t, "model.GreetArgs", meta.InputModel)
	require.NotNil(t, meta.OutputModel)
	assert.Equal(t, "model.GreetResult", *meta.OutputModel)
	assert.Equal(t, []string{"Auth"}, meta.Middleware)
}

func TestExtractMCPResourceMeta_FullAnnotations(t *testing.T) {
	t.Parallel()

	comments := strings.Join([]string{
		`@snapi.mcpresource("file:///{path}", "Files", "reads a file", "text/plain")`,
		`@snapi.usemiddleware(Auth)`,
	}, "\n")

	meta := extractMCPResourceMeta(comments)
	require.NotNil(t, meta)
	assert.Equal(t, "file:///{path}", meta.URI)
	assert.Equal(t, "Files", meta.Name)
	assert.Equal(t, "reads a file", meta.Description)
	assert.Equal(t, "text/plain", meta.MimeType)
	assert.Equal(t, []string{"Auth"}, meta.Middleware)
}

func TestExtractMCPPromptMeta_FullAnnotations(t *testing.T) {
	t.Parallel()

	comments := strings.Join([]string{
		`@snapi.mcpprompt("review", "review some code")`,
		`@snapi.mcppromptarg("code", "the code", "true")`,
		`@snapi.usemiddleware(Auth)`,
	}, "\n")

	meta := extractMCPPromptMeta(comments)
	require.NotNil(t, meta)
	assert.Equal(t, "review", meta.Name)
	assert.Equal(t, "review some code", meta.Description)
	require.Len(t, meta.Args, 1)
	assert.Equal(t, "code", meta.Args[0].Name)
	assert.True(t, meta.Args[0].Required)
	assert.Equal(t, []string{"Auth"}, meta.Middleware)
}

func TestExtractAnyHandlerMeta_MultipleFamiliesError(t *testing.T) {
	t.Parallel()

	comments := strings.Join([]string{
		`@snapi.GET("/greet")`,
		`@snapi.mcptool("greet", "greets someone")`,
	}, "\n")

	httpMeta, tool, resource, prompt, err := extractAnyHandlerMeta(comments, "file.go", "Greet")
	require.Error(t, err)
	assert.Nil(t, httpMeta)
	assert.Nil(t, tool)
	assert.Nil(t, resource)
	assert.Nil(t, prompt)
	var famErr *MultipleAnnotationFamiliesError
	require.ErrorAs(t, err, &famErr)
	assert.ElementsMatch(t, []string{"HTTP", "MCPTool"}, famErr.Families)
}

func TestParseHandler_MCPTool(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerProject(t, "tool.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @SnAPI.MCPTool("greet", "greets someone")
// @SnAPI.Request(api.GreetArgs)
func Greet(req runtime.Request, resp runtime.Response) {}
`)
	p, err := runHandler(t, dir)
	require.NoError(t, err)
	require.Len(t, p.project.HandlerFuncs, 1)
	h := p.project.HandlerFuncs[0]
	assert.Nil(t, h.Meta)
	require.NotNil(t, h.MCPTool)
	assert.Equal(t, "greet", h.MCPTool.Name)
	assert.Equal(t, "api.GreetArgs", h.MCPTool.InputModel)
}

func TestParseHandler_MCPToolAndHTTPConflict(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerProject(t, "tool.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

// @SnAPI.GET("/greet")
// @SnAPI.MCPTool("greet", "greets someone")
func Greet(req runtime.Request, resp runtime.Response) {}
`)
	_, err := runHandler(t, dir)
	require.Error(t, err)
	var famErr *MultipleAnnotationFamiliesError
	require.ErrorAs(t, err, &famErr)
}

func TestParseHandlerStruct_MCPResourceAndPrompt(t *testing.T) {
	t.Parallel()

	dir, _ := writeHandlerStructProject(t, "struct.go", `package api

import "github.com/StevenCyb/SnAPI/pkg/runtime"

type Files struct{}

// @SnAPI.MCPResource("file:///{path}", "Files", "reads a file", "text/plain")
func (f *Files) Read(req runtime.Request, resp runtime.Response) {}

// @SnAPI.MCPPrompt("review", "review some code")
// @SnAPI.MCPPromptArg("code", "the code", "true")
func (f *Files) Review(req runtime.Request, resp runtime.Response) {}
`)
	p, err := runHandlerStruct(t, dir)
	require.NoError(t, err)
	require.Len(t, p.project.HandlerStructs, 1)
	methods := p.project.HandlerStructs[0].Methods
	require.Len(t, methods, 2)

	var sawResource, sawPrompt bool
	for _, m := range methods {
		if m.MCPResource != nil {
			sawResource = true
			assert.Equal(t, "file:///{path}", m.MCPResource.URI)
		}
		if m.MCPPrompt != nil {
			sawPrompt = true
			require.Len(t, m.MCPPrompt.Args, 1)
		}
	}
	assert.True(t, sawResource)
	assert.True(t, sawPrompt)
}
