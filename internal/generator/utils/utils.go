package utils

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"
)

// Exists reports whether the given path exists on disk.
func Exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	return !errors.Is(err, fs.ErrNotExist)
}

// CopyFile copies the file at src to dst. Both paths must be absolute.
func CopyFile(src, dst string) error {
	if !filepath.IsAbs(src) || !filepath.IsAbs(dst) {
		return errors.New("only absolute paths allowed")
	}

	// #nosec G304 G703 -- src and dst are required to be absolute paths
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// #nosec G304 G703 -- dst is validated as absolute path
	return os.WriteFile(dst, data, 0600)
}

// RenderToFile resolves dst, ensures the directory exists, executes tmpl with
// data and writes the result to <dst>/<name>.
func RenderToFile(dst string, tmpl *template.Template, name string, data any) error {
	abs, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(abs, name), buf.Bytes(), 0600)
}
