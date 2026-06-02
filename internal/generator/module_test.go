package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/StevenCyb/SnAPI/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newModuleGenerator(t *testing.T, mod *models.Module) (*Generator, string) {
	t.Helper()
	dst := t.TempDir()
	g := NewGenerator(&models.Project{MainModule: mod}, dst, Config{})
	return g, dst
}

func TestGenerateModule_WritesGoMod(t *testing.T) {
	t.Parallel()

	g, dst := newModuleGenerator(t, &models.Module{
		Path:      "github.com/example/app",
		GoVersion: "1.21",
		Requires: []models.Require{
			{Path: "github.com/foo/bar", Version: "v1.2.3"},
			{Path: "github.com/baz/qux", Version: "v0.1.0", Indirect: true},
		},
		Replaces: []models.Replace{
			{OldPath: "github.com/example/app", NewPath: "/tmp/local"},
		},
	})

	require.NoError(t, g.generateModule())

	data, err := os.ReadFile(filepath.Join(dst, "go.mod"))
	require.NoError(t, err)
	content := string(data)
	assert.Contains(t, content, "module bootstrapped")
	assert.Contains(t, content, "go 1.21")
	assert.Contains(t, content, "github.com/foo/bar v1.2.3")
	assert.Contains(t, content, "github.com/baz/qux v0.1.0 // indirect")
	assert.Contains(t, content, "github.com/example/app => /tmp/local")
}

func TestGenerateModule_CopiesGoSum(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "go.sum"), []byte("sum-data"), 0o600))

	g, dst := newModuleGenerator(t, &models.Module{
		Path:      "github.com/example/app",
		GoVersion: "1.21",
		Dir:       srcDir,
	})

	require.NoError(t, g.generateModule())

	data, err := os.ReadFile(filepath.Join(dst, "go.sum"))
	require.NoError(t, err)
	assert.Equal(t, "sum-data", string(data))
}

func TestGenerateModule_NoGoSumIsOK(t *testing.T) {
	t.Parallel()

	g, dst := newModuleGenerator(t, &models.Module{
		Path:      "github.com/example/app",
		GoVersion: "1.21",
		Dir:       t.TempDir(),
	})

	require.NoError(t, g.generateModule())
	_, err := os.Stat(filepath.Join(dst, "go.sum"))
	assert.True(t, os.IsNotExist(err))
}

func TestGenerateModule_MissingMainModule(t *testing.T) {
	t.Parallel()

	g := NewGenerator(&models.Project{}, t.TempDir(), Config{})
	err := g.generateModule()
	require.Error(t, err)
	var modErr *ModuleGenerationError
	require.ErrorAs(t, err, &modErr)
	assert.Equal(t, "main module missing", modErr.Reason)
}

func TestGenerate_RunsModuleStep(t *testing.T) {
	t.Parallel()

	dst := t.TempDir()
	g := NewGenerator(&models.Project{MainModule: &models.Module{
		Path: "github.com/example/app", GoVersion: "1.21",
	}}, dst, Config{})

	require.NoError(t, g.Generate())
	_, err := os.Stat(filepath.Join(dst, "go.mod"))
	require.NoError(t, err)
}
