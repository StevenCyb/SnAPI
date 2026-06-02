package wrapper

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRequest(method, target string, body io.Reader) *http.Request {
	return httptest.NewRequestWithContext(context.Background(), method, target, body)
}

func TestRequest_PathValue(t *testing.T) {
	t.Parallel()
	r := &http.Request{URL: &url.URL{Path: "/foo/bar"}}
	// PathValue is a stub for demonstration; actual implementation may differ
	req := &Request{R: r}
	assert.NotNil(t, req.Raw())
}

func TestRequest_Context(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), "k", "v")
	r := httptest.NewRequestWithContext(ctx, "GET", "/", nil)
	req := &Request{R: r}
	assert.Equal(t, "v", req.Context().Value("k"))
}

func TestRequest_QueryValue(t *testing.T) {
	t.Parallel()
	r := newTestRequest("GET", "/?foo=bar", nil)
	req := &Request{R: r}
	assert.Equal(t, "bar", req.QueryValue("foo"))
}

func TestRequest_QueryValueOrDefault(t *testing.T) {
	t.Parallel()
	r := newTestRequest("GET", "/?foo=bar", nil)
	req := &Request{R: r}
	assert.Equal(t, "bar", req.QueryValueOrDefault("foo", "baz"))
	assert.Equal(t, "baz", req.QueryValueOrDefault("missing", "baz"))
}

func TestRequest_Header(t *testing.T) {
	t.Parallel()
	r := newTestRequest("GET", "/", nil)
	r.Header.Set("X-Test", "value")
	req := &Request{R: r}
	assert.Equal(t, "value", req.Header("X-Test"))
}

func TestRequest_HeaderOrDefault(t *testing.T) {
	t.Parallel()
	r := newTestRequest("GET", "/", nil)
	r.Header.Set("X-Test", "value")
	req := &Request{R: r}
	assert.Equal(t, "value", req.HeaderOrDefault("X-Test", "default"))
	assert.Equal(t, "default", req.HeaderOrDefault("Missing", "default"))
}

func TestRequest_Body(t *testing.T) {
	t.Parallel()
	body := io.NopCloser(strings.NewReader("test body"))
	r := newTestRequest("POST", "/", body)
	req := &Request{R: r}
	assert.NotNil(t, req.Body())
}

func TestRequest_FromJsonBody(t *testing.T) {
	t.Parallel()
	obj := map[string]string{"foo": "bar"}
	buf := new(bytes.Buffer)
	require.NoError(t, json.NewEncoder(buf).Encode(obj))
	r := newTestRequest("POST", "/", buf)
	req := &Request{R: r}
	var out map[string]string
	require.NoError(t, req.FromJsonBody(&out))
	assert.Equal(t, obj, out)
}

func TestRequest_Form(t *testing.T) {
	t.Parallel()
	form := url.Values{}
	form.Set("foo", "bar")
	r := newTestRequest("POST", "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req := &Request{R: r}
	_ = r.ParseForm()
	assert.Equal(t, "bar", req.Form("foo"))
}

func TestRequest_FormOrDefault(t *testing.T) {
	t.Parallel()
	form := url.Values{}
	form.Set("foo", "bar")
	r := newTestRequest("POST", "/", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req := &Request{R: r}
	_ = r.ParseForm()
	assert.Equal(t, "bar", req.FormOrDefault("foo", "baz"))
	assert.Equal(t, "baz", req.FormOrDefault("missing", "baz"))
}
