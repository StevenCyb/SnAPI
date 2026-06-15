package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeDotEnv writes content to a temp file and returns its path.
func writeDotEnv(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), ".env")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

// --- parseDotEnv ---

func TestParseDotEnv_BasicKeyValue(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "FOO=bar\nBAZ=qux\n")
	pairs, err := parseDotEnv(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"FOO=bar", "BAZ=qux"}, pairs)
}

func TestParseDotEnv_SkipsBlankLines(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "A=1\n\n\nB=2\n")
	pairs, err := parseDotEnv(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"A=1", "B=2"}, pairs)
}

func TestParseDotEnv_SkipsComments(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "# this is a comment\nKEY=value\n# another comment\n")
	pairs, err := parseDotEnv(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"KEY=value"}, pairs)
}

func TestParseDotEnv_ExportPrefix(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "export EXPORTED=yes\n")
	pairs, err := parseDotEnv(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"EXPORTED=yes"}, pairs)
}

func TestParseDotEnv_DoubleQuotedValue(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, `KEY="hello world"` + "\n")
	pairs, err := parseDotEnv(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"KEY=hello world"}, pairs)
}

func TestParseDotEnv_SingleQuotedValue(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "KEY='hello world'\n")
	pairs, err := parseDotEnv(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"KEY=hello world"}, pairs)
}

func TestParseDotEnv_MismatchedQuotesNotStripped(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, `KEY="mismatch'` + "\n")
	pairs, err := parseDotEnv(path)
	require.NoError(t, err)
	assert.Equal(t, []string{`KEY="mismatch'`}, pairs)
}

func TestParseDotEnv_ValueWithEqualsSign(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "TOKEN=abc=def==\n")
	pairs, err := parseDotEnv(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"TOKEN=abc=def=="}, pairs)
}

func TestParseDotEnv_EmptyValue(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "EMPTY=\n")
	pairs, err := parseDotEnv(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"EMPTY="}, pairs)
}

func TestParseDotEnv_SkipsLinesWithoutEquals(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "NOEQUALS\nGOOD=yes\n")
	pairs, err := parseDotEnv(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"GOOD=yes"}, pairs)
}

func TestParseDotEnv_SkipsEmptyKey(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "=value\nGOOD=yes\n")
	pairs, err := parseDotEnv(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"GOOD=yes"}, pairs)
}

func TestParseDotEnv_EmptyFile(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "")
	pairs, err := parseDotEnv(path)
	require.NoError(t, err)
	assert.Empty(t, pairs)
}

func TestParseDotEnv_OnlyCommentsAndBlanks(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "# comment\n\n# another\n")
	pairs, err := parseDotEnv(path)
	require.NoError(t, err)
	assert.Empty(t, pairs)
}

func TestParseDotEnv_FileNotFound(t *testing.T) {
	t.Parallel()
	_, err := parseDotEnv(filepath.Join(t.TempDir(), "nonexistent.env"))
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestParseDotEnv_WhitespaceAroundKeyAndValue(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "  KEY  =  value  \n")
	pairs, err := parseDotEnv(path)
	require.NoError(t, err)
	assert.Equal(t, []string{"KEY=value"}, pairs)
}

// --- injectDotEnv ---

func TestInjectDotEnv_EmptyPathReturnsEnvUnchanged(t *testing.T) {
	t.Parallel()
	env := []string{"EXISTING=yes"}
	result, err := injectDotEnv(env, "")
	require.NoError(t, err)
	assert.Equal(t, env, result)
}

func TestInjectDotEnv_AddsNewVars(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "NEW_VAR=hello\n")
	result, err := injectDotEnv([]string{"EXISTING=yes"}, path)
	require.NoError(t, err)
	assert.Contains(t, result, "EXISTING=yes")
	assert.Contains(t, result, "NEW_VAR=hello")
}

func TestInjectDotEnv_DoesNotOverrideExisting(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "KEY=from_dotenv\n")
	result, err := injectDotEnv([]string{"KEY=from_env"}, path)
	require.NoError(t, err)
	assert.Contains(t, result, "KEY=from_env")
	assert.NotContains(t, result, "KEY=from_dotenv")
}

func TestInjectDotEnv_EmptyDotEnvReturnsEnvUnchanged(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "# only comments\n")
	env := []string{"EXISTING=yes"}
	result, err := injectDotEnv(env, path)
	require.NoError(t, err)
	assert.Equal(t, env, result)
}

func TestInjectDotEnv_EmptyBaseEnv(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "A=1\nB=2\n")
	result, err := injectDotEnv(nil, path)
	require.NoError(t, err)
	assert.Equal(t, []string{"A=1", "B=2"}, result)
}

func TestInjectDotEnv_FileNotFoundReturnsError(t *testing.T) {
	t.Parallel()
	env := []string{"EXISTING=yes"}
	result, err := injectDotEnv(env, filepath.Join(t.TempDir(), "missing.env"))
	require.Error(t, err)
	assert.Equal(t, env, result)
}

func TestInjectDotEnv_MixedOverrideAndNew(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "KEEP=from_dotenv\nNEW=added\n")
	env := []string{"KEEP=original", "OTHER=untouched"}
	result, err := injectDotEnv(env, path)
	require.NoError(t, err)
	assert.Contains(t, result, "KEEP=original")
	assert.Contains(t, result, "OTHER=untouched")
	assert.Contains(t, result, "NEW=added")
	assert.NotContains(t, result, "KEEP=from_dotenv")
}

func TestInjectDotEnv_PreservesEnvOrder(t *testing.T) {
	t.Parallel()
	path := writeDotEnv(t, "C=3\n")
	env := []string{"A=1", "B=2"}
	result, err := injectDotEnv(env, path)
	require.NoError(t, err)
	// Original vars must come first.
	assert.Equal(t, "A=1", result[0])
	assert.Equal(t, "B=2", result[1])
	assert.Equal(t, "C=3", result[2])
}
