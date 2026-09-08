package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenCyb/SnAPI/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasMCPFeature(t *testing.T) {
	t.Parallel()

	assert.False(t, hasMCPFeature(sampleProject()))

	withTool := sampleProject()
	withTool.HandlerFuncs = append(withTool.HandlerFuncs, models.HandlerFunc{
		Package: "api", Name: "Greet",
		MCPTool: &models.MCPToolMeta{Name: "greet", Description: "greets"},
	})
	assert.True(t, hasMCPFeature(withTool))

	withStructPrompt := sampleProject()
	withStructPrompt.HandlerStructs = []models.HandlerStruct{{
		Package: "api", Name: "Files", VarName: "files",
		Methods: []models.HandlerFunc{{
			Package: "api", Name: "Review",
			MCPPrompt: &models.MCPPromptMeta{Name: "review"},
		}},
	}}
	assert.True(t, hasMCPFeature(withStructPrompt))
}

// writeMCPModelProject builds a temp module on disk with a package that
// declares GreetArgs/GreetResult, so schema resolution (which reads real
// Go source via go/parser) has something to resolve against.
func writeMCPModelProject(t *testing.T) *models.Project {
	t.Helper()
	root := t.TempDir()
	apiDir := filepath.Join(root, "api")
	require.NoError(t, os.MkdirAll(apiDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(apiDir, "types.go"), []byte(`package api

type GreetArgs struct {
	Name string `+"`json:\"name\"`"+`
}

type GreetResult struct {
	Greeting string `+"`json:\"greeting\"`"+`
}
`), 0o600))

	proj := &models.Project{
		MainModule: &models.Module{Path: "github.com/example/app", Dir: root, GoVersion: "1.21"},
		HandlerFuncs: []models.HandlerFunc{{
			Package: "api", ImportPath: "github.com/example/app/api", Name: "Greet",
			MCPTool: &models.MCPToolMeta{
				Name: "greet", Description: "greets someone",
				InputModel: "api.GreetArgs", OutputModel: new("api.GreetResult"),
			},
			Imports: map[string]string{"api": "github.com/example/app/api"},
		}},
	}
	return proj
}

func TestGenerateMCP_Tool(t *testing.T) {
	t.Parallel()
	proj := writeMCPModelProject(t)
	dst := t.TempDir()
	g := NewGenerator(proj, dst, Config{})

	require.NoError(t, g.generateMCP())

	data, err := os.ReadFile(filepath.Join(dst, "mcp.go"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, `mcp.NewServer(mcp.ServerInfo{`)
	assert.Contains(t, content, `srv.AddTool(mcp.Tool{`)
	assert.Contains(t, content, `Name:        "greet"`)
	assert.Contains(t, content, `Description: "greets someone"`)
	assert.Contains(t, content, `api.Greet(req, resp)`)
	assert.Contains(t, content, `\"name\":`)     // input schema property
	assert.Contains(t, content, `\"greeting\":`) // output schema property
	assert.Contains(t, content, `mux.Handle("/mcp", srv)`)
}

func TestGenerateMCP_CustomEndpoint(t *testing.T) {
	t.Parallel()
	proj := writeMCPModelProject(t)
	proj.Config.MCPEndpoint = "/agent"
	proj.Config.MCPInstructions = new("be nice")
	proj.Config.MCPAllowedOrigins = []string{"https://example.com"}
	dst := t.TempDir()
	g := NewGenerator(proj, dst, Config{})

	require.NoError(t, g.generateMCP())

	data, _ := os.ReadFile(filepath.Join(dst, "mcp.go"))
	content := string(data)
	assert.Contains(t, content, `mux.Handle("/agent", srv)`)
	assert.Contains(t, content, `Instructions: "be nice"`)
	assert.Contains(t, content, `mcp.WithAllowedOrigins("https://example.com")`)
}

func TestGenerateMCP_ResourceAndPromptOnStruct(t *testing.T) {
	t.Parallel()
	proj := sampleProject()
	proj.HandlerStructs = []models.HandlerStruct{{
		Package: "api", ImportPath: "github.com/example/app/api",
		Name: "Files", VarName: "files",
		Methods: []models.HandlerFunc{
			{
				Package: "api", Name: "Read",
				MCPResource: &models.MCPResourceMeta{URI: "file:///{path}", Name: "Files", MimeType: "text/plain"},
			},
			{
				Package: "api", Name: "Review",
				MCPPrompt: &models.MCPPromptMeta{
					Name: "review", Description: "review code",
					Args: []models.HandlerParam{{Name: "code", Required: true}},
				},
			},
		},
	}}
	dst := t.TempDir()
	g := NewGenerator(proj, dst, Config{})

	require.NoError(t, g.generateMCP())

	data, err := os.ReadFile(filepath.Join(dst, "mcp.go"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, `srv.AddResource(mcp.Resource{`)
	assert.Contains(t, content, `URI:         "file:///{path}"`)
	assert.Contains(t, content, `files.Read(req, resp)`)
	assert.Contains(t, content, `srv.AddPrompt(mcp.Prompt{`)
	assert.Contains(t, content, `Name: "code", Description: "", Required: true`)
	assert.Contains(t, content, `files.Review(req, resp)`)
	assert.NotContains(t, content, `"encoding/json"`) // no tool => schema import not needed
}

func TestGenerate_NoMCPFeatureSkipsMCPFile(t *testing.T) {
	t.Parallel()
	dst := t.TempDir()
	require.NoError(t, NewGenerator(sampleProject(), dst, Config{}).Generate())

	_, err := os.Stat(filepath.Join(dst, "mcp.go"))
	assert.True(t, os.IsNotExist(err))

	data, rerr := os.ReadFile(filepath.Join(dst, "main.go"))
	require.NoError(t, rerr)
	assert.NotContains(t, string(data), "registerMCPServer(mux)")
}

func TestGenerate_WithMCPFeatureCallsRegister(t *testing.T) {
	t.Parallel()
	proj := writeMCPModelProject(t)
	dst := t.TempDir()
	require.NoError(t, NewGenerator(proj, dst, Config{}).Generate())

	data, err := os.ReadFile(filepath.Join(dst, "main.go"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "registerMCPServer(mux)")

	_, err = os.Stat(filepath.Join(dst, "mcp.go"))
	assert.NoError(t, err)
}
