package wrapper

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
)

// MCPKind identifies which MCP result shape an MCPResponse builds, since
// tools, resources and prompts each have a different result envelope.
type MCPKind int

const (
	MCPKindTool MCPKind = iota
	MCPKindResource
	MCPKindPrompt
)

// contentTypeText is the MCP "text" content type marker, and doubles as the
// map key holding the text itself ({"type": "text", "text": "..."}).
const contentTypeText = "text"

// ProtocolError is a JSON-RPC-level error, as opposed to a tool execution
// error (reported via Content+IsError). Resources and prompts have no
// isError result field, so their handler errors always become one of these.
type ProtocolError struct {
	Code    int
	Message string
}

// MCPResponse implements runtime.Response for an MCP tool/resource/prompt
// call. Unlike the HTTP Response, it does not write to a live
// http.ResponseWriter: it buffers the handler's output so the MCP
// dispatcher can shape it into the correct JSON-RPC result after the
// handler (and its middleware chain) returns. Raw() returns nil - there is
// no live http.ResponseWriter for a single MCP call within a POST that may
// serve others; handlers/middleware relying on Raw() do not work for MCP.
type MCPResponse struct {
	Kind MCPKind

	// ResourceURI/ResourceMimeType seed the "uri"/"mimeType" fields of a
	// resources/read content entry - the resource's own declared values,
	// since Text/Json only supply the body.
	ResourceURI      string
	ResourceMimeType string

	StatusCode int

	// Content/StructuredContent/IsError are populated by Json/Text/Html and
	// shape a tools/call result. Resource/prompt calls only ever populate
	// Content (a single contents/messages entry).
	Content           []map[string]any
	StructuredContent any
	IsError           bool

	// ProtocolErr, when set, means the dispatcher must return a real
	// JSON-RPC error response instead of a result.
	ProtocolErr *ProtocolError
}

// NewMCPResponse builds an MCPResponse for the given result family.
func NewMCPResponse(kind MCPKind) *MCPResponse {
	return &MCPResponse{Kind: kind, StatusCode: http.StatusOK}
}

// Raw returns nil - MCP calls have no live http.ResponseWriter.
func (r *MCPResponse) Raw() http.ResponseWriter {
	return nil
}

// Status sets the notional status code used to decide tool isError.
func (r *MCPResponse) Status(statusCode int) {
	r.StatusCode = statusCode
}

// Header is a no-op - MCP results carry no HTTP headers of their own.
func (r *MCPResponse) Header(_, _ string) {}

// Json sends the JSON-encoded data as the response body.
func (r *MCPResponse) Json(statusCode int, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	r.setBody(statusCode, string(raw), raw)
	return nil
}

// Text sends text as the response body.
func (r *MCPResponse) Text(statusCode int, text string) {
	r.setBody(statusCode, text, nil)
}

// Html sends html as the response body.
func (r *MCPResponse) Html(statusCode int, html string) {
	r.setBody(statusCode, html, nil)
}

// Template renders tmpl and sends it as the response body.
func (r *MCPResponse) Template(statusCode int, tmpl string, data any) error {
	t, err := template.New("response").Parse(tmpl)
	if err != nil {
		return err
	}
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return err
	}
	r.setBody(statusCode, buf.String(), nil)
	return nil
}

// RedirectTemporary is a no-op - MCP results are not HTTP redirects.
func (r *MCPResponse) RedirectTemporary(_ *http.Request, _ string) {}

// RedirectPermanent is a no-op - MCP results are not HTTP redirects.
func (r *MCPResponse) RedirectPermanent(_ *http.Request, _ string) {}

// Error reports a handler-side error. For a tool this is a tool execution
// error (isError: true, still a successful JSON-RPC result, per spec so the
// model can see and self-correct). For a resource/prompt - which have no
// isError result field - this becomes a real JSON-RPC error (-32602).
func (r *MCPResponse) Error(statusCode int, errorMsg string) {
	if r.Kind == MCPKindTool {
		r.StatusCode = statusCode
		r.IsError = true
		r.Content = []map[string]any{{"type": contentTypeText, "text": errorMsg}}
		return
	}
	r.ProtocolErr = &ProtocolError{Code: -32602, Message: errorMsg}
}

func (r *MCPResponse) setBody(statusCode int, text string, structured json.RawMessage) {
	r.StatusCode = statusCode
	if statusCode >= http.StatusBadRequest {
		if r.Kind == MCPKindTool {
			r.IsError = true
		} else {
			r.ProtocolErr = &ProtocolError{Code: -32603, Message: text}
			return
		}
	}

	switch r.Kind {
	case MCPKindTool:
		r.Content = []map[string]any{{"type": contentTypeText, "text": text}}
		if len(structured) > 0 {
			var sc any
			if json.Unmarshal(structured, &sc) == nil {
				r.StructuredContent = sc
			}
		}
	case MCPKindResource:
		r.Content = []map[string]any{{
			"uri":      r.ResourceURI,
			"mimeType": r.ResourceMimeType,
			"text":     text,
		}}
	case MCPKindPrompt:
		r.Content = []map[string]any{{
			"role":    "user",
			"content": map[string]any{"type": contentTypeText, "text": text},
		}}
	}
}
