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
	testDepPath = "github.com/one/dep"
)

func writeGoMod(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o600))
	return dir
}

func parseModuleAt(t *testing.T, dir string) (*models.Module, error) {
	t.Helper()
	p := NewParser(dir)
	err := p.parseModule()
	if err != nil {
		return nil, err
	}
	return p.project.MainModule, nil
}

func TestParseModule_Valid(t *testing.T) {
	t.Parallel()

	dir := writeGoMod(t, "module github.com/example/app\n\ngo 1.21\n")

	mod, err := parseModuleAt(t, dir)
	require.NoError(t, err)
	assert.Equal(t, "github.com/example/app", mod.Path)
	assert.Equal(t, "1.21", mod.GoVersion)
	// post-processing appends a self-require and self-replace
	assert.Equal(t, []models.Require{{Path: "github.com/example/app", Version: "v0.0.0"}}, mod.Requires)
	assert.Equal(t, []models.Replace{{OldPath: "github.com/example/app", NewPath: dir}}, mod.Replaces)
	assert.Equal(t, dir, mod.Dir)
}

func TestParseModule_WithDependencies(t *testing.T) {
	t.Parallel()

	dir := writeGoMod(t,
		"module github.com/user/project\n\ngo 1.21\n\nrequire github.com/some/dep v1.0.0\n",
	)

	mod, err := parseModuleAt(t, dir)
	require.NoError(t, err)
	assert.Equal(t, "github.com/user/project", mod.Path)
	assert.Equal(t, "1.21", mod.GoVersion)
	require.Len(t, mod.Requires, 2)
	// non-self, non-SnAPI deps are marked indirect by post-processing
	assert.Equal(t, models.Require{Path: "github.com/some/dep", Version: "v1.0.0", Indirect: true}, mod.Requires[0])
	assert.Equal(t, models.Require{Path: "github.com/user/project", Version: "v0.0.0"}, mod.Requires[1])
	assert.Equal(t, []models.Replace{{OldPath: "github.com/user/project", NewPath: dir}}, mod.Replaces)
	assert.Equal(t, dir, mod.Dir)
}

func TestParseModule_NoGoMod(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := parseModuleAt(t, dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read go.mod")
}

func TestParseModule_MissingModuleLine(t *testing.T) {
	t.Parallel()

	dir := writeGoMod(t, "go 1.21\n")
	_, err := parseModuleAt(t, dir)
	require.ErrorIs(t, err, ErrModulePathNotFound)
}

func TestParseModule_EmptyFile(t *testing.T) {
	t.Parallel()

	dir := writeGoMod(t, "")
	_, err := parseModuleAt(t, dir)
	require.ErrorIs(t, err, ErrModulePathNotFound)
}

func TestParseModule_WithReplaceAndIndirect(t *testing.T) {
	t.Parallel()

	content := "module github.com/foo/bar\n" +
		"go 1.21\n\n" +
		"require (\n" +
		"\t" + testDepPath + " v1.2.3\n" +
		"\tgithub.com/two/dep v2.3.4 // indirect\n" +
		")\n\n" +
		"replace (\n" +
		"\t" + testDepPath + " => " + testDepPath + " v1.2.4\n" +
		"\tgithub.com/old/dep v0.1.0 => github.com/new/dep v0.2.0\n" +
		")\n"
	dir := writeGoMod(t, content)

	mod, err := parseModuleAt(t, dir)
	require.NoError(t, err)
	assert.Equal(t, "github.com/foo/bar", mod.Path)
	assert.Equal(t, "1.21", mod.GoVersion)
	require.Len(t, mod.Requires, 3)
	assert.Equal(t, models.Require{Path: testDepPath, Version: "v1.2.3", Indirect: true}, mod.Requires[0])
	assert.Equal(t, models.Require{Path: "github.com/two/dep", Version: "v2.3.4", Indirect: true}, mod.Requires[1])
	assert.Equal(t, models.Require{Path: "github.com/foo/bar", Version: "v0.0.0"}, mod.Requires[2])
	require.Len(t, mod.Replaces, 3)
	assert.Equal(t, models.Replace{OldPath: testDepPath, NewPath: testDepPath, NewVersion: "v1.2.4"}, mod.Replaces[0])
	assert.Equal(t, models.Replace{OldPath: "github.com/old/dep", OldVersion: "v0.1.0", NewPath: "github.com/new/dep", NewVersion: "v0.2.0"}, mod.Replaces[1])
	assert.Equal(t, models.Replace{OldPath: "github.com/foo/bar", NewPath: dir}, mod.Replaces[2])
	assert.Equal(t, dir, mod.Dir)
}

func TestParseModule_SingleLineReplace(t *testing.T) {
	t.Parallel()

	content := "module github.com/foo/bar\n" +
		"go 1.21\n\n" +
		"require " + testDepPath + " v1.2.3\n" +
		"replace " + testDepPath + " => " + testDepPath + " v1.2.4\n"
	dir := writeGoMod(t, content)

	mod, err := parseModuleAt(t, dir)
	require.NoError(t, err)
	assert.Equal(t, "github.com/foo/bar", mod.Path)
	assert.Equal(t, "1.21", mod.GoVersion)
	require.Len(t, mod.Requires, 2)
	assert.Equal(t, models.Require{Path: testDepPath, Version: "v1.2.3", Indirect: true}, mod.Requires[0])
	assert.Equal(t, models.Require{Path: "github.com/foo/bar", Version: "v0.0.0"}, mod.Requires[1])
	require.Len(t, mod.Replaces, 2)
	assert.Equal(t, models.Replace{OldPath: testDepPath, NewPath: testDepPath, NewVersion: "v1.2.4"}, mod.Replaces[0])
	assert.Equal(t, models.Replace{OldPath: "github.com/foo/bar", NewPath: dir}, mod.Replaces[1])
	assert.Equal(t, dir, mod.Dir)
}

func TestParseModule_SnAPIDirectDep(t *testing.T) {
	t.Parallel()

	content := "module github.com/foo/bar\n\ngo 1.21\n\nrequire github.com/StevenCyb/SnAPI v0.1.0\n"
	dir := writeGoMod(t, content)

	mod, err := parseModuleAt(t, dir)
	require.NoError(t, err)
	require.Len(t, mod.Requires, 2)
	// SnAPI itself stays direct
	assert.Equal(t, models.Require{Path: "github.com/StevenCyb/SnAPI", Version: "v0.1.0"}, mod.Requires[0])
}

func TestParser_Parse_InvokesModule(t *testing.T) {
	t.Parallel()

	dir := writeGoMod(t, "module github.com/example/app\n\ngo 1.21\n")
	proj, err := NewParser(dir).Parse()
	require.NoError(t, err)
	require.NotNil(t, proj.MainModule)
	assert.Equal(t, "github.com/example/app", proj.MainModule.Path)
}

func TestParser_Parse_PropagatesError(t *testing.T) {
	t.Parallel()

	_, err := NewParser(t.TempDir()).Parse()
	require.Error(t, err)
}
