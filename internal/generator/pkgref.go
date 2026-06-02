package generator

import (
	"sort"

	"github.com/StevenCyb/SnAPI/internal/models"
)

// collectPackages deduplicates packages by import path. The first package name
// observed per path is used as the alias.
func collectPackages(refs []models.ProjectImport) []models.ProjectImport {
	seen := make(map[string]string, len(refs))
	for _, r := range refs {
		if _, ok := seen[r.Path]; !ok {
			seen[r.Path] = r.Alias
		}
	}
	out := make([]models.ProjectImport, 0, len(seen))
	for path, alias := range seen {
		out = append(out, models.ProjectImport{Alias: alias, Path: path})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func lifecyclePkgs(funcs []models.LifecycleFunc) []models.ProjectImport {
	refs := make([]models.ProjectImport, 0, len(funcs))
	for _, f := range funcs {
		refs = append(refs, models.ProjectImport{Alias: f.Package, Path: f.ImportPath})
	}
	return refs
}

func handlerPkgs(funcs []models.HandlerFunc) []models.ProjectImport {
	refs := make([]models.ProjectImport, 0, len(funcs))
	for _, f := range funcs {
		refs = append(refs, models.ProjectImport{Alias: f.Package, Path: f.ImportPath})
	}
	return refs
}

func middlewarePkgs(funcs []models.MiddlewareFunc) []models.ProjectImport {
	refs := make([]models.ProjectImport, 0, len(funcs))
	for _, f := range funcs {
		refs = append(refs, models.ProjectImport{Alias: f.Package, Path: f.ImportPath})
	}
	return refs
}
