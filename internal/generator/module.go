package generator

import (
	_ "embed"
	"path/filepath"
	"text/template"

	"github.com/StevenCyb/SnAPI/internal/generator/utils"
)

//go:embed template/go.mod.tmpl
var goModTemplateString string

//nolint:gochecknoglobals // template is immutable and safe for reuse
var goModTemplate = template.Must(template.New("go.mod").Parse(goModTemplateString))

// generateModule emits go.mod (and copies go.sum if available) for the project's main module.
func (g *Generator) generateModule() error {
	mod := g.project.MainModule
	if mod == nil {
		return &ModuleGenerationError{Reason: "main module missing", Err: nil}
	}

	if err := utils.RenderToFile(g.dst, goModTemplate, "go.mod", mod); err != nil {
		return &ModuleGenerationError{Reason: "render go.mod", Err: err}
	}

	if mod.Dir != "" {
		dst, err := filepath.Abs(g.dst)
		if err != nil {
			return &ModuleGenerationError{Reason: "resolve dst", Err: err}
		}
		src := filepath.Join(mod.Dir, "go.sum")
		if utils.Exists(src) {
			if err := utils.CopyFile(src, filepath.Join(dst, "go.sum")); err != nil {
				return &ModuleGenerationError{Reason: "copy go.sum", Err: err}
			}
		}
	}

	return nil
}
