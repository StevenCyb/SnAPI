package protogen

import (
	"regexp"
	"slices"
	"strings"

	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Route is the resolved HTTP shape of a single RPC.
type Route struct {
	Method        string
	Path          string
	PathParams    []RouteParam
	QueryParams   []RouteParam
	BodyField     string // "" = no body, "*" = whole input message, else a specific field name
	ExtraBindings int    // count of dropped additional_bindings entries
}

// RouteParam is a single @SnAPI.Path/@SnAPI.Query annotation's (name, type).
type RouteParam struct {
	Name string
	Type string
}

// httpRuleExt identifies the `google.api.http` extension. Reading it through
// plain protoreflect (rather than proto.GetExtension's typed *HttpRule cast)
// avoids a dynamicpb/generated-type mismatch: protocompile links extension
// option values against the HttpRule descriptor it resolved on its own, which
// is a distinct Go value from the annotations package's generated type even
// though it describes the same proto message.
var httpRuleExt = annotations.E_Http.TypeDescriptor()

// resolveRoute determines how method md is reachable over HTTP: a
// `google.api.http` option if present, otherwise the Connect-RPC fallback
// convention (POST /<fqService>/<Method>, whole request as JSON body).
func resolveRoute(md protoreflect.MethodDescriptor, fqService string) Route {
	opts := md.Options().ProtoReflect()
	if opts.IsValid() && opts.Has(httpRuleExt) {
		if rule := opts.Get(httpRuleExt).Message(); rule.IsValid() {
			return routeFromHTTPRule(rule, md.Input())
		}
	}
	return Route{
		Method:    "POST",
		Path:      "/" + fqService + "/" + string(md.Name()),
		BodyField: "*",
	}
}

func routeFromHTTPRule(rule protoreflect.Message, input protoreflect.MessageDescriptor) Route {
	httpMethod, rawPath := httpPattern(rule)
	path, pathParamNames := normalizePath(rawPath)

	route := Route{
		Method:        httpMethod,
		Path:          path,
		ExtraBindings: httpRuleField(rule, "additional_bindings").List().Len(),
	}
	for _, name := range pathParamNames {
		route.PathParams = append(route.PathParams, RouteParam{Name: name, Type: paramType(input, name)})
	}

	body := httpRuleField(rule, "body").String()
	switch {
	case body == "*":
		route.BodyField = "*"
	case body != "":
		route.BodyField = body
	default:
		fields := input.Fields()
		for i := 0; i < fields.Len(); i++ {
			name := string(fields.Get(i).Name())
			if slices.Contains(pathParamNames, name) {
				continue
			}
			route.QueryParams = append(route.QueryParams, RouteParam{Name: name, Type: paramType(input, name)})
		}
	}
	return route
}

// httpRuleField reads a field of the HttpRule message by name, generically
// via its own descriptor -- works regardless of which concrete Go type
// backs the message (dynamicpb or the generated annotations.HttpRule).
func httpRuleField(rule protoreflect.Message, name string) protoreflect.Value {
	fd := rule.Descriptor().Fields().ByName(protoreflect.Name(name))
	if fd == nil {
		return protoreflect.Value{}
	}
	return rule.Get(fd)
}

func httpPattern(rule protoreflect.Message) (method, path string) {
	switch {
	case httpRuleField(rule, "get").String() != "":
		return "GET", httpRuleField(rule, "get").String()
	case httpRuleField(rule, "put").String() != "":
		return "PUT", httpRuleField(rule, "put").String()
	case httpRuleField(rule, "post").String() != "":
		return "POST", httpRuleField(rule, "post").String()
	case httpRuleField(rule, "delete").String() != "":
		return "DELETE", httpRuleField(rule, "delete").String()
	case httpRuleField(rule, "patch").String() != "":
		return "PATCH", httpRuleField(rule, "patch").String()
	default:
		return "POST", "/"
	}
}

var pathParamRe = regexp.MustCompile(`\{([a-zA-Z_][a-zA-Z0-9_.]*)(?:=[^}]*)?\}`)

// normalizePath rewrites a google.api.http path template into SnAPI's own
// `{param}` syntax: `{name=pattern/*}` becomes `{name}`, and a dotted nested
// field path `{a.b}` is flattened to its top-level field `{a}` (SnAPI has no
// concept of a nested-field path parameter -- known v1 limitation).
func normalizePath(raw string) (path string, params []string) {
	seen := map[string]bool{}
	path = pathParamRe.ReplaceAllStringFunc(raw, func(m string) string {
		name := pathParamRe.FindStringSubmatch(m)[1]
		if idx := strings.Index(name, "."); idx >= 0 {
			name = name[:idx]
		}
		if !seen[name] {
			seen[name] = true
			params = append(params, name)
		}
		return "{" + name + "}"
	})
	return path, params
}

// paramType returns the annotation "type" string (e.g. "string", "int32")
// for a top-level field of input named name. Message/enum-kind fields (rare
// as path/query params) fall back to "string".
func paramType(input protoreflect.MessageDescriptor, name string) string {
	f := input.Fields().ByName(protoreflect.Name(name))
	if f == nil {
		return "string"
	}
	switch f.Kind() {
	case protoreflect.EnumKind, protoreflect.MessageKind, protoreflect.GroupKind:
		return "string"
	default:
		return scalarGoType(f.Kind())
	}
}
