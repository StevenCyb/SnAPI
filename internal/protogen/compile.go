package protogen

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// compileProto compiles specPath (and any local .proto files it imports)
// into linked descriptors. It returns the descriptor for specPath itself,
// plus the transitive closure of its local imports -- i.e. imports that are
// neither a "google/protobuf/*" well-known type nor a "google/api/*"
// googleapis annotation file -- so their messages can be rendered into
// their own dedicated DTO files too.
func compileProto(specPath string) (protoreflect.FileDescriptor, []protoreflect.FileDescriptor, error) {
	dir := filepath.Dir(specPath)
	name := filepath.Base(specPath)

	resolver := protocompile.WithStandardImports(protocompile.CompositeResolver{
		&protocompile.SourceResolver{ImportPaths: []string{dir}},
		protocompile.ResolverFunc(globalRegistryResolver),
	})

	compiler := protocompile.Compiler{Resolver: resolver}

	files, err := compiler.Compile(context.Background(), name)
	if err != nil {
		return nil, nil, &CompileError{SpecPath: specPath, Err: err}
	}

	main := protoreflect.FileDescriptor(files[0])

	var local []protoreflect.FileDescriptor
	seen := map[string]bool{main.Path(): true}
	collectLocalImports(main, &local, seen)

	return main, local, nil
}

// collectLocalImports recursively walks fd's imports, collecting every
// non-well-known file exactly once.
func collectLocalImports(fd protoreflect.FileDescriptor, out *[]protoreflect.FileDescriptor, seen map[string]bool) {
	imports := fd.Imports()
	for i := 0; i < imports.Len(); i++ {
		imp := imports.Get(i).FileDescriptor
		if imp == nil || seen[imp.Path()] || isWellKnownImportPath(imp.Path()) {
			continue
		}
		seen[imp.Path()] = true
		*out = append(*out, imp)
		collectLocalImports(imp, out, seen)
	}
}

func isWellKnownImportPath(path string) bool {
	return strings.HasPrefix(path, "google/protobuf/") || strings.HasPrefix(path, "google/api/")
}

// globalRegistryResolver answers "google/api/http.proto" and
// "google/api/annotations.proto" (and anything else already registered
// globally, e.g. by a blank-imported generated package) from
// protoregistry.GlobalFiles.
func globalRegistryResolver(path string) (protocompile.SearchResult, error) {
	fd, err := protoregistry.GlobalFiles.FindFileByPath(path)
	if err != nil {
		return protocompile.SearchResult{}, err
	}
	return protocompile.SearchResult{Desc: fd}, nil
}
