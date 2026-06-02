package parser

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sync/errgroup"
)

// fileCtx is the per-file context passed to every extractor.
type fileCtx struct {
	Path       string
	ImportPath string
	File       *ast.File
	FSet       *token.FileSet
}

// extractor inspects a single parsed Go file and mutates p.project.
// Extractors run concurrently across files, so any mutation of p.project
// must happen under p.mu.
type extractor func(fc fileCtx) error

// walkAndExtract walks every non-test .go file under the main module directory,
// parses each one, and then runs every extractor against each parsed file in
// parallel. Files in package "main" are skipped.
func (p *Parser) walkAndExtract(extractors ...extractor) error {
	if p.project.MainModule == nil {
		return ErrModulePathNotFound
	}
	modDir := p.project.MainModule.Dir
	modPath := p.project.MainModule.Path

	files, err := collectGoFiles(modDir)
	if err != nil {
		return err
	}

	g := new(errgroup.Group)
	g.SetLimit(runtime.GOMAXPROCS(0))
	for _, path := range files {
		path := path
		g.Go(func() error {
			fSet := token.NewFileSet()
			node, err := goparser.ParseFile(fSet, path, nil, goparser.ParseComments)
			if err != nil {
				return &ParsingFileError{FilePath: path, Err: err}
			}
			if node.Name.Name == "main" {
				return nil
			}
			fc := fileCtx{
				Path:       path,
				ImportPath: resolveImportPath(path, modDir, modPath),
				File:       node,
				FSet:       fSet,
			}
			for _, ex := range extractors {
				if err := ex(fc); err != nil {
					return err
				}
			}
			return nil
		})
	}
	return g.Wait()
}

// collectGoFiles returns all non-test .go file paths under dir.
func collectGoFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

// resolveImportPath returns the import path of the package containing path.
func resolveImportPath(path, modDir, modPath string) string {
	if modPath == "" || modDir == "" {
		return ""
	}
	rel, err := filepath.Rel(modDir, filepath.Dir(path))
	if err != nil || rel == "." {
		return modPath
	}
	return modPath + "/" + filepath.ToSlash(rel)
}

// commentText returns the doc comment lines with comment markers stripped,
// joined by newlines for consumption by utils.ExtractAnnotation.
func commentText(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	lines := make([]string, 0, len(doc.List))
	for _, c := range doc.List {
		text := c.Text
		switch {
		case strings.HasPrefix(text, "//"):
			lines = append(lines, strings.TrimPrefix(text, "//"))
		case strings.HasPrefix(text, "/*"):
			text = strings.TrimPrefix(text, "/*")
			text = strings.TrimSuffix(text, "*/")
			lines = append(lines, strings.Split(text, "\n")...)
		}
	}
	return strings.Join(lines, "\n")
}
