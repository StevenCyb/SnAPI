package wrapper

import (
	"encoding/json"
	"html/template"
	"net/http"
)

// Response wraps an http.ResponseWriter and provides convenient helper methods
// for sending responses in a handler function.
type Response struct {
	W http.ResponseWriter
}

// Raw returns the underlying http.ResponseWriter.
func (r *Response) Raw() http.ResponseWriter {
	return r.W
}

// Status sets the HTTP status code for the response.
func (r *Response) Status(statusCode int) {
	r.W.WriteHeader(statusCode)
}

// Header sets a header key-value pair in the response.
func (r *Response) Header(key, value string) {
	r.W.Header().Set(key, value)
}

// Json sends a JSON response with the given status code and data.
func (r *Response) Json(statusCode int, data any) error {
	r.W.Header().Set("Content-Type", "application/json; charset=utf-8")
	r.W.WriteHeader(statusCode)
	return json.NewEncoder(r.W).Encode(data)
}

// Text sends a plain text response with the given status code and text.
func (r *Response) Text(statusCode int, text string) {
	r.W.Header().Set("Content-Type", "text/plain; charset=utf-8")
	r.W.WriteHeader(statusCode)
	_, _ = r.W.Write([]byte(text))
}

// Html sends an HTML response with the given status code and HTML content.
func (r *Response) Html(statusCode int, html string) {
	r.W.Header().Set("Content-Type", "text/html; charset=utf-8")
	r.W.WriteHeader(statusCode)
	_, _ = r.W.Write([]byte(html))
}

// Template renders and sends an HTML template with the given status code and data.
func (r *Response) Template(statusCode int, tmpl string, data any) error {
	r.W.Header().Set("Content-Type", "text/html; charset=utf-8")
	r.W.WriteHeader(statusCode)
	t, err := template.New("response").Parse(tmpl)
	if err != nil {
		return err
	}
	return t.Execute(r.W, data)
}

// RedirectTemporary sends a 307 Temporary Redirect to the specified URL.
func (r *Response) RedirectTemporary(req *http.Request, url string) {
	http.Redirect(r.W, req, url, http.StatusTemporaryRedirect)
}

// RedirectPermanent sends a 308 Permanent Redirect to the specified URL.
func (r *Response) RedirectPermanent(req *http.Request, url string) {
	http.Redirect(r.W, req, url, http.StatusPermanentRedirect)
}

// Error sends an HTTP error response with the given status code and error message.
func (r *Response) Error(statusCode int, errorMsg string) {
	http.Error(r.W, errorMsg, statusCode)
}
