package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExists(t *testing.T) {
	t.Parallel()

	f, err := os.CreateTemp(t.TempDir(), "file-*.txt")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	assert.True(t, Exists(f.Name()))
	assert.False(t, Exists(filepath.Join(os.TempDir(), "does-not-exist")))
}

func TestCopyFile(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()

	src := filepath.Join(tmp, "src.txt")
	dst := filepath.Join(tmp, "dst.txt")

	require.NoError(t, os.WriteFile(src, []byte("hello world"), 0600))

	err := CopyFile(src, dst)
	require.NoError(t, err)

	data, err := os.ReadFile(dst)
	require.NoError(t, err)

	assert.Equal(t, "hello world", string(data))
}

func TestCopyFileRejectsRelative(t *testing.T) {
	err := CopyFile("src.txt", "/tmp/dst.txt")
	require.Error(t, err)

	err = CopyFile("/tmp/src.txt", "dst.txt")
	require.Error(t, err)
}
