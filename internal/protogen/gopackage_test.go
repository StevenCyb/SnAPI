package protogen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveGoPackage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		goPackageOpt string
		wantPath     string
		wantName     string
		wantOK       bool
	}{
		{name: "unset", goPackageOpt: "", wantOK: false},
		{name: "path;name form", goPackageOpt: "github.com/org/repo/pkg/api;api", wantPath: "github.com/org/repo/pkg/api", wantName: "api", wantOK: true},
		{name: "path only, name from last segment", goPackageOpt: "github.com/org/repo/pkg/api", wantPath: "github.com/org/repo/pkg/api", wantName: "api", wantOK: true},
		{name: "bare relative path", goPackageOpt: "pkg/api;api", wantPath: "pkg/api", wantName: "api", wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opt := ""
			if tt.goPackageOpt != "" {
				opt = `option go_package = "` + tt.goPackageOpt + `";` + "\n"
			}
			src := map[string]string{
				"test.proto": "syntax = \"proto3\";\npackage test;\n" + opt + "message Foo {}\n",
			}
			fd := compileTestProto(t, src, "test.proto")

			path, name, ok := resolveGoPackage(fd)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantPath, path)
				assert.Equal(t, tt.wantName, name)
			}
		})
	}
}

func TestResolveTarget(t *testing.T) {
	t.Parallel()

	withGoPackage := map[string]string{
		"test.proto": `
syntax = "proto3";
package test;
option go_package = "github.com/org/repo/pkg/api;api";
message Foo {}
`,
	}
	withoutGoPackage := map[string]string{
		"test.proto": `
syntax = "proto3";
package test;
message Foo {}
`,
	}

	t.Run("go_package prefixed by module path", func(t *testing.T) {
		t.Parallel()
		fd := compileTestProto(t, withGoPackage, "test.proto")
		dir, pkgName := resolveTarget(fd, "github.com/org/repo", "/module/root", "/module/root/somewhere")
		assert.Equal(t, "/module/root/pkg/api", dir)
		assert.Equal(t, "api", pkgName)
	})

	t.Run("go_package equal to module path", func(t *testing.T) {
		t.Parallel()
		src := map[string]string{
			"test.proto": `
syntax = "proto3";
package test;
option go_package = "github.com/org/repo;repo";
message Foo {}
`,
		}
		fd := compileTestProto(t, src, "test.proto")
		dir, pkgName := resolveTarget(fd, "github.com/org/repo", "/module/root", "/module/root/somewhere")
		assert.Equal(t, "/module/root", dir)
		assert.Equal(t, "repo", pkgName)
	})

	t.Run("no go_package falls back to outputDir and package api", func(t *testing.T) {
		t.Parallel()
		fd := compileTestProto(t, withoutGoPackage, "test.proto")
		dir, pkgName := resolveTarget(fd, "github.com/org/repo", "/module/root", "/module/root/api")
		assert.Equal(t, "/module/root/api", dir)
		assert.Equal(t, "api", pkgName)
	})
}
