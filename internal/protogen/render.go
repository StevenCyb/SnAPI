package protogen

import (
	"bytes"
	_ "embed"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	genutils "github.com/StevenCyb/SnAPI/internal/generator/utils"
)

//go:embed template/model.tmpl
var modelTemplateSource string

//go:embed template/handler_func.tmpl
var handlerFuncTemplateSource string

//go:embed template/handler_struct.tmpl
var handlerStructTemplateSource string

//nolint:gochecknoglobals // templates are immutable and safe for reuse
var (
	modelTemplate         = template.Must(template.New("model").Parse(modelTemplateSource))
	handlerFuncTemplate   = template.Must(template.New("handler_func").Parse(handlerFuncTemplateSource))
	handlerStructTemplate = template.Must(template.New("handler_struct").Parse(handlerStructTemplateSource))
)

// ServiceView is the template data for one generated <service>.go file.
type ServiceView struct {
	PackageName        string // the file's own `package` clause, e.g. "api"
	ModelImportPath    string // full import path of the sibling model package
	PackageServiceName string
	IsStruct           bool
	NeedsModel         bool
	SkippedStreaming   []string
	Methods            []MethodView
}

// MethodView is the template data for one generated handler.
type MethodView struct {
	FuncName          string // free-function name (func layout) or method name (struct layout)
	RPCName           string
	HTTPMethod        string
	Path              string
	OperationID       string
	PathParams        []RouteParam
	QueryParams       []RouteParam
	RequestType       string
	ResponseType      string
	SuccessStatus     string
	SuccessDesc       string
	ExtraBindingsNote string
}

// toServiceView converts the proto IR into template data.
func toServiceView(modelImportPath string, svc Service) ServiceView {
	view := ServiceView{
		ModelImportPath:    modelImportPath,
		PackageServiceName: svc.Name,
		IsStruct:           len(svc.Methods) > 1,
		SkippedStreaming:   svc.SkippedStreaming,
	}

	for _, m := range svc.Methods {
		mv := MethodView{
			FuncName:     m.Name,
			RPCName:      m.Name,
			HTTPMethod:   m.Route.Method,
			Path:         m.Route.Path,
			OperationID:  svc.FQName + "." + m.Name,
			PathParams:   m.Route.PathParams,
			QueryParams:  m.Route.QueryParams,
			RequestType:  m.RequestType,
			ResponseType: m.ResponseType,
		}
		if !view.IsStruct {
			mv.FuncName = svc.Name + m.Name
		}
		if m.ResponseType == "" {
			mv.SuccessStatus, mv.SuccessDesc = "204", "No Content"
		} else {
			mv.SuccessStatus, mv.SuccessDesc = "200", "OK"
		}
		if m.Route.ExtraBindings > 0 {
			mv.ExtraBindingsNote = fmt.Sprintf(
				"Note: %d additional google.api.http binding(s) on this RPC were ignored (only the primary binding is used).",
				m.Route.ExtraBindings)
		}
		if m.RequestType != "" || m.ResponseType != "" {
			view.NeedsModel = true
		}
		view.Methods = append(view.Methods, mv)
	}
	return view
}

// renderModelFile always (re)writes <outputDir>/model/<file>.go -- it's pure
// derived data, never hand-edited.
func renderModelFile(outputDir string, pf ProtoFile) error {
	dir := filepath.Join(outputDir, "model")
	return renderGoFile(dir, snakeCase(pf.Name)+".go", modelTemplate, pf)
}

// renderServiceFile writes <targetDir>/<service>.go, but only if it doesn't
// already exist -- re-running `snapi proto` must never clobber hand-written
// business logic in a previously generated handler file.
func renderServiceFile(targetDir, pkgName string, sv ServiceView) error {
	sv.PackageName = pkgName
	name := snakeCase(sv.PackageServiceName) + ".go"
	if genutils.Exists(filepath.Join(targetDir, name)) {
		return nil
	}
	tmpl := handlerFuncTemplate
	if sv.IsStruct {
		tmpl = handlerStructTemplate
	}
	return renderGoFile(targetDir, name, tmpl, sv)
}

func renderGoFile(dir, name string, tmpl *template.Template, data any) error {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return &RenderError{File: name, Err: err}
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return &RenderError{File: name, Err: err}
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return &RenderError{File: name, Err: err}
	}
	return os.WriteFile(filepath.Join(dir, name), formatted, 0600)
}

// snakeCase converts a PascalCase Go identifier to snake_case for filenames.
func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
