package protogen

import (
	"context"
	"testing"

	"github.com/bufbuild/protocompile"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// compileTestProto compiles an in-memory set of .proto sources (keyed by
// import path, e.g. "test.proto") and returns the descriptor for mainPath.
// Mirrors compileProto's resolver setup but reads from memory instead of
// disk, so tests stay hermetic and fast.
func compileTestProto(t *testing.T, srcs map[string]string, mainPath string) protoreflect.FileDescriptor {
	t.Helper()

	resolver := protocompile.WithStandardImports(protocompile.CompositeResolver{
		&protocompile.SourceResolver{Accessor: protocompile.SourceAccessorFromMap(srcs)},
		protocompile.ResolverFunc(globalRegistryResolver),
	})
	compiler := protocompile.Compiler{Resolver: resolver}

	files, err := compiler.Compile(context.Background(), mainPath)
	require.NoError(t, err)
	require.Len(t, files, 1)

	return protoreflect.FileDescriptor(files[0])
}
