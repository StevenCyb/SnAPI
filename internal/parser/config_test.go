package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenCyb/SnAPI/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfigProject(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module github.com/example/app\n\ngo 1.21\n"), 0o600))
	pkgDir := filepath.Join(dir, "api")
	require.NoError(t, os.MkdirAll(pkgDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(pkgDir, "file.go"), []byte(content), 0o600))
	return dir
}

func runConfig(t *testing.T, dir string) (*Parser, error) {
	t.Helper()
	p := NewParser(dir)
	require.NoError(t, p.parseModule())
	return p, p.parseConfig()
}

func TestParseConfig_Empty(t *testing.T) {
	t.Parallel()
	dir := writeConfigProject(t, `package api
`)
	p, err := runConfig(t, dir)
	require.NoError(t, err)
	assert.Equal(t, models.ProjectConfig{}, p.project.Config)
}

func TestParseConfig_AllFields(t *testing.T) {
	t.Parallel()
	dir := writeConfigProject(t, `// @SnAPI.Title("Example API")
// @SnAPI.Description("This is an example API.")
// @SnAPI.Version("1.0.0")
package api
`)
	p, err := runConfig(t, dir)
	require.NoError(t, err)
	assert.Equal(t, models.ProjectConfig{
		Title:       "Example API",
		Description: "This is an example API.",
		Version:     "1.0.0",
	}, p.project.Config)
}

func TestParseConfig_Partial(t *testing.T) {
	t.Parallel()
	dir := writeConfigProject(t, `// @SnAPI.Title("Only Title")
package api
`)
	p, err := runConfig(t, dir)
	require.NoError(t, err)
	assert.Equal(t, models.ProjectConfig{Title: "Only Title"}, p.project.Config)
}
