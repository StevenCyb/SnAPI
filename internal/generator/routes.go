package generator

import (
	_ "embed"
	"strings"
	"text/template"

	"github.com/StevenCyb/SnAPI/internal/generator/utils"
	"github.com/StevenCyb/SnAPI/internal/models"
)

//go:embed template/routes.tmpl
var routesTemplateString string

//nolint:gochecknoglobals // template is immutable and safe for reuse
var routesTemplate = template.Must(template.New("routes").Parse(routesTemplateString))

type routesData struct {
	Imports  []models.ProjectImport
	Handlers []handlerEntry
}

type handlerEntry struct {
	Method      string
	Path        string
	Package     string
	Name        string
	Middlewares []middlewareRef
}

type middlewareRef struct {
	Package string
	Name    string
}

// generateRoutes writes routes.go: the registerRoutes function that wires
// every discovered handler (and its middleware pipeline) into the mux.
func (g *Generator) generateRoutes() error {
	lookup := middlewareLookup(g.project.MiddlewareFuncs)

	handlers := make([]handlerEntry, 0, len(g.project.HandlerFuncs))
	for _, h := range g.project.HandlerFuncs {
		if h.Meta == nil {
			continue
		}
		refs, err := resolveMiddlewareRefs(h, lookup)
		if err != nil {
			return &RoutesGenerationError{Reason: "resolve middleware", Err: err}
		}
		handlers = append(handlers, handlerEntry{
			Method:      h.Meta.Method,
			Path:        h.Meta.Path,
			Package:     h.Package,
			Name:        h.Name,
			Middlewares: refs,
		})
	}

	imports := collectPackages(append(
		handlerPkgs(g.project.HandlerFuncs),
		middlewarePkgs(g.project.MiddlewareFuncs)...,
	))

	if err := utils.RenderToFile(g.dst, routesTemplate, "routes.go", routesData{
		Imports: imports, Handlers: handlers,
	}); err != nil {
		return &RoutesGenerationError{Reason: "render routes.go", Err: err}
	}
	return nil
}

// middlewareLookup indexes middlewares by both "pkg.Name" and "Name".
func middlewareLookup(mws []models.MiddlewareFunc) map[string]models.MiddlewareFunc {
	m := make(map[string]models.MiddlewareFunc, len(mws)*2)
	for _, mw := range mws {
		m[mw.Package+"."+mw.Name] = mw
		if _, ok := m[mw.Name]; !ok {
			m[mw.Name] = mw
		}
	}
	return m
}

// resolveMiddlewareRefs converts the names referenced by @snapi.usemiddleware
// into concrete package/func refs. Names may be qualified ("pkg.Name") or bare
// ("Name"). Bare names prefer the handler's own package, then fall back to any
// middleware with that name.
func resolveMiddlewareRefs(h models.HandlerFunc, lookup map[string]models.MiddlewareFunc) ([]middlewareRef, error) {
	if h.Meta == nil || len(h.Meta.Middleware) == 0 {
		return nil, nil
	}
	refs := make([]middlewareRef, 0, len(h.Meta.Middleware))
	for _, name := range h.Meta.Middleware {
		var (
			mw    models.MiddlewareFunc
			found bool
		)
		if strings.Contains(name, ".") {
			mw, found = lookup[name]
		} else {
			if mw, found = lookup[h.Package+"."+name]; !found {
				mw, found = lookup[name]
			}
		}
		if !found {
			return nil, &MiddlewareNotFoundError{Handler: h.Name, Name: name}
		}
		refs = append(refs, middlewareRef{Package: mw.Package, Name: mw.Name})
	}
	return refs, nil
}
