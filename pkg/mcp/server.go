package mcp

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"

	"github.com/StevenCyb/SnAPI/pkg/runtime"
)

// ServerInfo identifies the server in a server/discover result.
type ServerInfo struct {
	Name         string
	Version      string
	Instructions string
}

// PromptArg describes one prompts/get argument.
type PromptArg struct {
	Name        string
	Description string
	Required    bool
}

// Tool registers one MCP tool.
type Tool struct {
	Name         string
	Description  string
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
	Handler      runtime.HandlerFunc
	Middlewares  []runtime.MiddlewareFunc
}

// Resource registers one MCP resource. A URI containing a "{param}" segment
// is treated as a resource template (listed under resources/templates/list
// and matched against incoming resources/read URIs) rather than a static
// resource. Only single-segment "{name}" placeholders are supported - not
// the full RFC 6570 template grammar.
type Resource struct {
	URI         string
	Name        string
	Description string
	MimeType    string
	Handler     runtime.HandlerFunc
	Middlewares []runtime.MiddlewareFunc
}

// Prompt registers one MCP prompt.
type Prompt struct {
	Name        string
	Description string
	Args        []PromptArg
	Handler     runtime.HandlerFunc
	Middlewares []runtime.MiddlewareFunc
}

// Option configures a Server at construction time.
type Option func(*Server)

// WithAllowedOrigins sets the Origin allowlist used to reject cross-origin
// requests (DNS-rebinding protection). A request whose Origin header is
// present but not in this list is rejected with 403. Requests without an
// Origin header (typical for non-browser MCP clients) are never rejected on
// this basis, per spec.
func WithAllowedOrigins(origins ...string) Option {
	return func(s *Server) { s.allowedOrigins = append(s.allowedOrigins, origins...) }
}

// Server implements the MCP Streamable HTTP transport as an http.Handler.
type Server struct {
	info ServerInfo

	tools      []Tool
	toolByName map[string]Tool

	resources         []Resource
	resourceTemplates []Resource
	resourceByURI     map[string]Resource

	prompts      []Prompt
	promptByName map[string]Prompt

	allowedOrigins []string
}

// NewServer creates an MCP server with the given identity.
func NewServer(info ServerInfo, opts ...Option) *Server {
	s := &Server{
		info:          info,
		toolByName:    map[string]Tool{},
		resourceByURI: map[string]Resource{},
		promptByName:  map[string]Prompt{},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// AddTool registers a tool.
func (s *Server) AddTool(t Tool) {
	s.tools = append(s.tools, t)
	s.toolByName[t.Name] = t
}

// AddResource registers a resource or resource template.
func (s *Server) AddResource(r Resource) {
	if isTemplateURI(r.URI) {
		s.resourceTemplates = append(s.resourceTemplates, r)
		return
	}
	s.resources = append(s.resources, r)
	s.resourceByURI[r.URI] = r
}

// AddPrompt registers a prompt.
func (s *Server) AddPrompt(p Prompt) {
	s.prompts = append(s.prompts, p)
	s.promptByName[p.Name] = p
}

func (s *Server) originAllowed(origin string) bool {
	return slices.Contains(s.allowedOrigins, origin)
}

var templateParamRegex = regexp.MustCompile(`\{([^/{}]+)\}`)

func isTemplateURI(uri string) bool {
	return templateParamRegex.MatchString(uri)
}

// matchTemplate matches uri against a "{param}" template (single-segment
// placeholders only) and returns the extracted values.
func matchTemplate(tmpl, uri string) (map[string]string, bool) {
	names := templateParamRegex.FindAllStringSubmatch(tmpl, -1)
	pattern := "^" + regexp.QuoteMeta(tmpl) + "$"
	for _, m := range names {
		pattern = strings.Replace(pattern, regexp.QuoteMeta("{"+m[1]+"}"), `([^/]+)`, 1)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, false
	}
	match := re.FindStringSubmatch(uri)
	if match == nil {
		return nil, false
	}
	values := make(map[string]string, len(names))
	for i, m := range names {
		values[m[1]] = match[i+1]
	}
	return values, true
}
