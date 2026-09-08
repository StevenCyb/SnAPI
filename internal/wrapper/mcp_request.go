package wrapper

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// MCPRequest implements runtime.Request for an MCP tool/resource/prompt
// call. Raw/Context/Header/Form delegate to the real incoming HTTP POST to
// the MCP endpoint. Body/FromJsonBody decode the call's arguments (not the
// outer JSON-RPC envelope), so a tool handler reads its input exactly like
// an HTTP POST handler reads a JSON body. PathValue/QueryValue read from a
// small string map, since an MCP call has no mux path or query string:
// resource template segments and prompt arguments are exposed through it.
type MCPRequest struct {
	R      *http.Request
	Values map[string]string
	Args   []byte
}

// NewMCPRequest builds an MCPRequest. args is the raw JSON of the call's
// arguments (tool arguments; nil for resources/prompts, which have no JSON
// body to decode). values holds resource-template segments or prompt
// arguments, looked up via PathValue/QueryValue.
func NewMCPRequest(r *http.Request, args []byte, values map[string]string) *MCPRequest {
	if values == nil {
		values = map[string]string{}
	}
	return &MCPRequest{R: r, Args: args, Values: values}
}

// Raw returns the underlying *http.Request (the POST to the MCP endpoint).
func (r *MCPRequest) Raw() *http.Request {
	return r.R
}

// Context returns the request context used for cancellation and deadlines.
func (r *MCPRequest) Context() context.Context {
	if r.R == nil {
		return context.Background()
	}
	return r.R.Context()
}

// PathValue returns a resource-template segment by name.
func (r *MCPRequest) PathValue(key string) string {
	return r.Values[key]
}

// QueryValue returns a prompt argument by name.
func (r *MCPRequest) QueryValue(key string) string {
	return r.Values[key]
}

// QueryValueOrDefault returns a prompt argument by name, or defaultValue if not present.
func (r *MCPRequest) QueryValueOrDefault(key, defaultValue string) string {
	if v, ok := r.Values[key]; ok && v != "" {
		return v
	}
	return defaultValue
}

// Header returns the value of a header on the underlying HTTP POST.
func (r *MCPRequest) Header(key string) string {
	if r.R == nil {
		return ""
	}
	return r.R.Header.Get(key)
}

// HeaderOrDefault returns a header value, or defaultValue if not present.
func (r *MCPRequest) HeaderOrDefault(key, defaultValue string) string {
	if v := r.Header(key); v != "" {
		return v
	}
	return defaultValue
}

// Body returns the call's arguments as an io.ReadCloser.
func (r *MCPRequest) Body() io.ReadCloser {
	return io.NopCloser(bytes.NewReader(r.Args))
}

// FromJsonBody decodes the call's arguments into the provided object.
func (r *MCPRequest) FromJsonBody(obj any) error {
	if len(r.Args) == 0 {
		return nil
	}
	return json.Unmarshal(r.Args, obj)
}

// Form returns a prompt argument by name (same values as QueryValue).
func (r *MCPRequest) Form(key string) string {
	return r.Values[key]
}

// FormOrDefault returns a prompt argument by name, or defaultValue if not present.
func (r *MCPRequest) FormOrDefault(key, defaultValue string) string {
	return r.QueryValueOrDefault(key, defaultValue)
}
