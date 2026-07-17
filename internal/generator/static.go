package generator

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/StevenCyb/SnAPI/internal/generator/utils"
)

//go:embed template/static.tmpl
var staticTemplateString string

//nolint:gochecknoglobals // template is immutable and safe for reuse
var staticTemplate = template.Must(template.New("static").Parse(staticTemplateString))

type staticEntry struct {
	MuxPattern string
	Strip      string
	Var        string
	SubVar     string
	EmbedDir   string
}

type staticData struct {
	StaticFiles []staticEntry
}

// generateStatic writes static.go: for each @SnAPI.StaticFile mapping it
// copies the referenced directory into the generated project and embeds it
// via go:embed, then wires an http.FileServerFS for it under its URL prefix.
//
// Embedding (rather than reading the directory at runtime via http.Dir) is
// required because `snapi serve`/`watch` run the generated binary from a
// temporary build directory that's unrelated to the source project, and
// `snapi build` promises a single, self-contained binary.
//
// Routes are registered as "GET <pattern>" rather than a bare pattern:
// http.ServeMux panics at registration time on an unresolvable conflict
// between a method-agnostic pattern and a method-specific one covering an
// overlapping path (e.g. an existing "GET /" handler) — going through GET
// keeps precedence unambiguous (longest path wins for the same method).
func (g *Generator) generateStatic() error {
	entries := make([]staticEntry, 0, len(g.project.Config.StaticFiles))
	for i, s := range g.project.Config.StaticFiles {
		srcDir := s.Dir
		if !filepath.IsAbs(srcDir) && g.project.MainModule != nil {
			srcDir = filepath.Join(g.project.MainModule.Dir, srcDir)
		}

		embedDir := fmt.Sprintf("static/%d", i)
		if err := copyDirRecursive(srcDir, filepath.Join(g.dst, embedDir)); err != nil {
			return &StaticGenerationError{Reason: fmt.Sprintf("copy %s", s.Dir), Err: err}
		}

		pattern := normalizeStaticPrefix(s.Prefix)
		entries = append(entries, staticEntry{
			MuxPattern: "GET " + pattern,
			Strip:      strings.TrimSuffix(pattern, "/"),
			Var:        fmt.Sprintf("staticFS%d", i),
			SubVar:     fmt.Sprintf("staticSub%d", i),
			EmbedDir:   embedDir,
		})
	}

	if err := utils.RenderToFile(g.dst, staticTemplate, "static.go", staticData{StaticFiles: entries}); err != nil {
		return &StaticGenerationError{Reason: "render static.go", Err: err}
	}
	return nil
}

// normalizeStaticPrefix ensures p is an absolute path ending in "/" so the
// generated http.ServeMux pattern matches the whole subtree.
func normalizeStaticPrefix(p string) string {
	if p == "" {
		p = "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// copyDirRecursive copies the directory tree rooted at src into dst,
// preserving structure.
func copyDirRecursive(src, dst string) error {
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	return os.CopyFS(dst, os.DirFS(absSrc))
}
