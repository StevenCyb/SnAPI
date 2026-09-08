package runtime

import (
	"context"
	"io"
	"net/http"
	"slices"

	"github.com/StevenCyb/SnAPI/internal/wrapper"
)

// Request provides convenient helper methods for accessing request data in a handler function.
type Request interface {
	// Raw returns the underlying *http.Request.
	Raw() *http.Request
	// Context returns the request context used for cancellation and deadlines.
	Context() context.Context
	// PathValue returns the value of a path parameter by key.
	PathValue(key string) string
	// QueryValue returns the value of a query parameter by key.
	QueryValue(key string) string
	// QueryValueOrDefault returns the value of a query parameter by key, or the provided default if not present.
	QueryValueOrDefault(key, defaultValue string) string
	// Header returns the value of a header by key.
	Header(key string) string
	// HeaderOrDefault returns the value of a header by key, or the provided default if not present.
	HeaderOrDefault(key, defaultValue string) string
	// Body returns the request body as an io.ReadCloser.
	Body() io.ReadCloser
	// FromJsonBody decodes the JSON request body into the provided object.
	FromJsonBody(obj any) error
	// Form returns the value of a form field by key.
	Form(key string) string
	// FormOrDefault returns the value of a form field by key, or the provided default if not present.
	FormOrDefault(key, defaultValue string) string
}

// Response provides convenient helper methods for sending responses in a handler function.
type Response interface {
	// Raw returns the underlying http.ResponseWriter.
	Raw() http.ResponseWriter
	// Status sets the HTTP status code for the response.
	Status(statusCode int)
	// Header sets a header key-value pair in the response.
	Header(key, value string)
	// Json sends a JSON response with the given status code and data.
	Json(statusCode int, data any) error
	// Text sends a plain text response with the given status code and text.
	Text(statusCode int, text string)
	// Html sends an HTML response with the given status code and HTML content.
	Html(statusCode int, html string)
	// Template renders and sends an HTML template with the given status code and data.
	Template(statusCode int, tmpl string, data any) error
	// RedirectTemporary sends a 307 Temporary Redirect to the specified URL.
	RedirectTemporary(req *http.Request, url string)
	// RedirectPermanent sends a 308 Permanent Redirect to the specified URL.
	RedirectPermanent(req *http.Request, url string)
	// Error sends an HTTP error response with the given status code and error message.
	Error(statusCode int, errorMsg string)
}

type HandlerFunc func(req Request, resp Response)

type MiddlewareFunc func(req Request, resp Response, next HandlerFunc)

func WrapHandlerFunc(h HandlerFunc, middlewares ...MiddlewareFunc) http.HandlerFunc {
	chain := Chain(h, middlewares...)
	return func(w http.ResponseWriter, r *http.Request) {
		req := &wrapper.Request{R: r}
		resp := &wrapper.Response{W: w}
		chain(req, resp)
	}
}

// Chain folds middlewares (outermost first) around h into a single
// HandlerFunc. Transport-specific callers (net/http, MCP, ...) invoke the
// result directly against their own Request/Response implementations.
func Chain(h HandlerFunc, middlewares ...MiddlewareFunc) HandlerFunc {
	chain := h
	for _, v := range slices.Backward(middlewares) {
		mw := v
		next := chain
		chain = func(req Request, resp Response) {
			mw(req, resp, next)
		}
	}
	return chain
}
