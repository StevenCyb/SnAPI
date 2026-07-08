package protogen

import (
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// resolveGoPackage reads the `option go_package = "path;name";` file option,
// the standard protobuf mechanism for declaring where generated Go code
// belongs -- unlike the proto `package` statement (a dotted namespace for
// fully-qualified names, not a filesystem path). Returns ok=false if unset.
//
// Read generically via protoreflect (like the google.api.http extension in
// httprule.go) rather than a typed *descriptorpb.FileOptions cast, so it
// works regardless of which concrete Go type backs the compiled options.
func resolveGoPackage(fd protoreflect.FileDescriptor) (importPath, pkgName string, ok bool) {
	opts := fd.Options().ProtoReflect()
	field := opts.Descriptor().Fields().ByName("go_package")
	if field == nil || !opts.Has(field) {
		return "", "", false
	}

	value := opts.Get(field).String()
	if value == "" {
		return "", "", false
	}

	importPath = value
	if idx := strings.LastIndex(value, ";"); idx >= 0 {
		importPath, pkgName = value[:idx], value[idx+1:]
	}
	if pkgName == "" {
		pkgName = importPath
		if idx := strings.LastIndex(importPath, "/"); idx >= 0 {
			pkgName = importPath[idx+1:]
		}
	}
	return importPath, pkgName, true
}

// resolveTarget determines the directory to generate handler files into and
// the Go package name to give them.
//
//   - If main declares `option go_package`, its import path is resolved
//     against modulePath/moduleDir (stripping the module prefix if present,
//     otherwise treated as already module-relative) and its `;name` suffix
//     becomes the package name.
//   - Otherwise, outputDir is used as-is with the fixed package name "api".
func resolveTarget(main protoreflect.FileDescriptor, modulePath, moduleDir, outputDir string) (dir, pkgName string) {
	importPath, name, ok := resolveGoPackage(main)
	if !ok {
		return outputDir, "api"
	}

	rel := importPath
	switch {
	case importPath == modulePath:
		rel = ""
	default:
		if trimmed, cut := strings.CutPrefix(importPath, modulePath+"/"); cut {
			rel = trimmed
		}
	}
	return filepath.Join(moduleDir, filepath.FromSlash(rel)), name
}
