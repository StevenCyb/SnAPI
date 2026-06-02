package generator

import (
	_ "embed"
	"text/template"

	"github.com/StevenCyb/SnAPI/internal/generator/utils"
)

//go:embed template/main.tmpl
var serveTemplateString string

//nolint:gochecknoglobals // template is immutable and safe for reuse
var serveTemplate = template.Must(template.New("main").Parse(serveTemplateString))

// generateServe writes main.go: server bootstrap, signal handling, lifecycle
// hooks and the calls into the registerRoutes / registerSwaggerHandlers helpers.
func (g *Generator) generateServe() error {
	g.project.Addr = g.config.addr()
	g.project.Swagger = g.config.Swagger != nil
	g.project.Imports = collectPackages(append(
		lifecyclePkgs(g.project.SetupFuncs),
		lifecyclePkgs(g.project.TeardownFuncs)...,
	))

	if err := utils.RenderToFile(g.dst, serveTemplate, "main.go", g.project); err != nil {
		return &ServeGenerationError{Reason: "render main.go", Err: err}
	}
	return nil
}
