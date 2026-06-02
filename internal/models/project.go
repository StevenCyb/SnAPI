package models

// Project represents the overall project structure, including the main module and any submodules.
type Project struct {
	MainModule      *Module
	Config          ProjectConfig
	SetupFuncs      []LifecycleFunc
	TeardownFuncs   []LifecycleFunc
	MiddlewareFuncs []MiddlewareFunc
	HandlerFuncs    []HandlerFunc

	// Generation-time fields populated by the generator before rendering.
	Addr    string
	Swagger bool
	Imports []ProjectImport
}

// ProjectConfig holds project-wide metadata parsed from package-level annotations.
type ProjectConfig struct {
	Title           string
	Description     string
	Version         string
	Servers         []ProjectServer
	SecuritySchemes []SecurityScheme
}

// ProjectServer is one entry in the OpenAPI `servers` list.
type ProjectServer struct {
	URL         string
	Description string
}

// SecurityScheme describes an entry under components.securitySchemes.
type SecurityScheme struct {
	Name         string // key in components.securitySchemes
	Type         string // http, apiKey, oauth2, openIdConnect
	Scheme       string // for http: bearer, basic, ...
	BearerFormat string // optional, e.g. JWT
	In           string // for apiKey: header, query, cookie
	ParamName    string // for apiKey: header/query/cookie parameter name
}

// ProjectImport is an aliased package reference used by a generated file.
type ProjectImport struct {
	Alias string
	Path  string
}

// Module represents all relevant go.mod data.
type Module struct {
	Dir       string
	Path      string
	GoVersion string
	Requires  []Require
	Replaces  []Replace
}

// Require represents a required dependency.
type Require struct {
	Path     string
	Version  string
	Indirect bool
}

// Replace represents a replace directive.
type Replace struct {
	OldPath    string
	OldVersion string
	NewPath    string
	NewVersion string
}

// LifecycleFunc represents a lifecycle (setup or teardown) function discovered in source files.
type LifecycleFunc struct {
	Package      string
	ImportPath   string
	Name         string
	ReturnsError bool
}

// MiddlewareFunc represents a middleware function discovered in source files.
type MiddlewareFunc struct {
	Package    string
	ImportPath string
	Name       string
}

// HandlerFunc represents an HTTP handler function discovered in source files.
type HandlerFunc struct {
	Package    string
	ImportPath string
	Name       string
	Meta       *HandlerMeta
	Services   []string
	Imports    map[string]string // alias -> import path
}

// HandlerMeta carries the parsed @snapi.* annotations of a handler.
type HandlerMeta struct {
	Method          string
	Path            string
	Summary         *string
	Description     *string
	OperationID     *string
	Middleware      []string
	Deprecated      bool
	Status          []HandlerStatus
	Tags            []string
	Security        []string
	Paths           []HandlerParam
	Queries         []HandlerParam
	Headers         []HandlerParam
	Cookies         []HandlerParam
	Requests        []HandlerRequest
	Responses       []HandlerResponse
	ResponseHeaders []HandlerResponseHeader
}

// HandlerParam describes a path / query / header / cookie parameter parsed from
// @snapi.path / @snapi.query / @snapi.header / @snapi.cookie annotations.
type HandlerParam struct {
	Name        string
	Type        string
	Description *string
	Required    bool
}

// HandlerResponseHeader describes a single @snapi.responseHeader entry.
type HandlerResponseHeader struct {
	Code        string
	Name        string
	Type        string
	Description *string
}

// HandlerQuery describes a @snapi.query annotation on a handler.
// Deprecated: use HandlerParam.
type HandlerQuery = HandlerParam

// HandlerStatus describes a single @snapi.status entry on a handler.
type HandlerStatus struct {
	Code        string
	Description *string
}

// HandlerRequest describes a @snapi.request annotation on a handler.
type HandlerRequest struct {
	ContentType string
	Model       string
}

// HandlerResponse describes a @snapi.response annotation, attaching a schema
// to a specific status code + content type. Description is sourced from the
// matching @snapi.status annotation.
type HandlerResponse struct {
	Code        string
	ContentType string
	Model       string
}
