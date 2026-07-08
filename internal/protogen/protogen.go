// Package protogen generates an annotated SnAPI `api` package (handler
// stubs + DTOs) from a .proto spec. It is the reverse of internal/parser +
// internal/generator: instead of turning annotated Go into a server, it
// turns a .proto file into annotated Go that the existing snapi
// build/serve/watch pipeline then consumes unchanged.
package protogen

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// Generate compiles specPath and writes annotated handler(s) plus DTOs.
// Where they land is decided by the spec itself:
//
//   - If the main .proto file declares `option go_package = "path;name";`
//     (the standard protobuf mechanism for this -- see resolveGoPackage),
//     handler files go to <path> (resolved against the enclosing module) as
//     package <name>.
//   - Otherwise, handler files go directly into outputDir -- exactly the
//     directory given, no implicit subdirectory -- as package "api".
//
// Either way, DTOs always go into a "model" subdirectory next to the
// handlers and are always regenerated (pure derived data). Handler files are
// written only if they don't already exist yet, so hand-filled business
// logic survives re-running this against an updated spec.
//
// outputDir doesn't need to contain go.mod itself -- it's located by
// searching outputDir and its ancestors, exactly like the `go` tool, so
// `snapi proto spec.proto .` works with go.mod at the module root regardless
// of where go_package ultimately places the handlers.
func Generate(specPath, outputDir string) error {
	modulePath, moduleDir, err := findModule(outputDir)
	if err != nil {
		return err
	}

	main, localImports, err := compileProto(specPath)
	if err != nil {
		return err
	}

	absOut, err := filepath.Abs(outputDir)
	if err != nil {
		return err
	}
	targetDir, pkgName := resolveTarget(main, modulePath, moduleDir, absOut)

	relDir, err := filepath.Rel(moduleDir, targetDir)
	if err != nil {
		return err
	}
	modelImportPath := path.Join(modulePath, filepath.ToSlash(relDir), "model")

	files := append([]protoreflect.FileDescriptor{main}, localImports...)
	for _, fd := range files {
		if err := renderModelFile(targetDir, buildProtoFile(fd)); err != nil {
			return err
		}
	}

	for _, svc := range buildServices(main) {
		if len(svc.Methods) == 0 {
			continue // every RPC on this service is streaming; nothing routable to generate
		}
		if err := renderServiceFile(targetDir, pkgName, toServiceView(modelImportPath, svc)); err != nil {
			return err
		}
	}
	return nil
}

// findModule searches dir and its ancestors for a go.mod, like the `go`
// tool's own module resolution. Returns the module path it declares and the
// directory it was found in.
func findModule(dir string) (modulePath, moduleDir string, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", err
	}
	for d := abs; ; {
		if mp, ok := readModulePath(filepath.Join(d, "go.mod")); ok {
			return mp, d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", "", &ModuleNotFoundError{OutputDir: dir}
		}
		d = parent
	}
}

// readModulePath reads the `module` directive out of the go.mod at path.
// Returns ok=false if path doesn't exist or has no module directive.
func readModulePath(path string) (modulePath string, ok bool) {
	f, err := os.Open(path) //nolint:gosec // path is derived from a CLI argument
	if err != nil {
		return "", false
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if after, cut := strings.CutPrefix(line, "module "); cut {
			return strings.TrimSpace(after), true
		}
	}
	return "", false
}
