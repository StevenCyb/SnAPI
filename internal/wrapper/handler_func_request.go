package wrapper

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

// Request wraps an http.Request and provides convenient helper methods
// for accessing request data in a handler function.
type Request struct {
	R *http.Request
}

// Raw returns the underlying *http.Request.
func (r *Request) Raw() *http.Request {
	return r.R
}

// Context returns the request context used for cancellation and deadlines.
func (r *Request) Context() context.Context {
	return r.R.Context()
}

// PathValue returns the value of a path parameter by key.
func (r *Request) PathValue(key string) string {
	return r.R.PathValue(key)
}

// QueryValue returns the value of a query parameter by key.
func (r *Request) QueryValue(key string) string {
	return r.R.URL.Query().Get(key)
}

// QueryValueOrDefault returns the value of a query parameter by key, or the provided default if not present.
func (r *Request) QueryValueOrDefault(key, defaultValue string) string {
	value := r.R.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Header returns the value of a header by key.
func (r *Request) Header(key string) string {
	return r.R.Header.Get(key)
}

// HeaderOrDefault returns the value of a header by key, or the provided default if not present.
func (r *Request) HeaderOrDefault(key, defaultValue string) string {
	value := r.R.Header.Get(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Body returns the request body as an io.ReadCloser.
func (r *Request) Body() io.ReadCloser {
	return r.R.Body
}

// FromJsonBody decodes the JSON request body into the provided object.
func (r *Request) FromJsonBody(obj any) error {
	return json.NewDecoder(r.R.Body).Decode(obj)
}

// Form returns the value of a form field by key.
func (r *Request) Form(key string) string {
	return r.R.FormValue(key)
}

// FormOrDefault returns the value of a form field by key, or the provided default if not present.
func (r *Request) FormOrDefault(key, defaultValue string) string {
	value := r.R.FormValue(key)
	if value == "" {
		return defaultValue
	}
	return value
}
