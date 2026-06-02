package wrapper

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponse_Raw(t *testing.T) {
	t.Parallel()
	rw := httptest.NewRecorder()
	resp := &Response{W: rw}
	assert.Equal(t, rw, resp.Raw())
}

func TestResponse_Status(t *testing.T) {
	t.Parallel()
	rw := httptest.NewRecorder()
	resp := &Response{W: rw}
	resp.Status(http.StatusTeapot)
	assert.Equal(t, http.StatusTeapot, rw.Code)
}

func TestResponse_Header(t *testing.T) {
	t.Parallel()
	rw := httptest.NewRecorder()
	resp := &Response{W: rw}
	resp.Header("X-Test", "value")
	assert.Equal(t, "value", rw.Header().Get("X-Test"))
}

func TestResponse_Json(t *testing.T) {
	t.Parallel()
	rw := httptest.NewRecorder()
	resp := &Response{W: rw}
	data := map[string]string{"foo": "bar"}
	err := resp.Json(http.StatusOK, data)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "application/json; charset=utf-8", rw.Header().Get("Content-Type"))
	var out map[string]string
	require.NoError(t, json.NewDecoder(rw.Body).Decode(&out))
	assert.Equal(t, data, out)
}

func TestResponse_Text(t *testing.T) {
	t.Parallel()
	rw := httptest.NewRecorder()
	resp := &Response{W: rw}
	resp.Text(http.StatusOK, "hello")
	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "text/plain; charset=utf-8", rw.Header().Get("Content-Type"))
	assert.Equal(t, "hello", rw.Body.String())
}

func TestResponse_Html(t *testing.T) {
	t.Parallel()
	rw := httptest.NewRecorder()
	resp := &Response{W: rw}
	resp.Html(http.StatusOK, "<b>hi</b>")
	assert.Equal(t, http.StatusOK, rw.Code)
	assert.Equal(t, "text/html; charset=utf-8", rw.Header().Get("Content-Type"))
	assert.Equal(t, "<b>hi</b>", rw.Body.String())
}

func TestResponse_Template(t *testing.T) {
	t.Parallel()
	rw := httptest.NewRecorder()
	resp := &Response{W: rw}
	tmpl := "Hello, {{.Name}}!"
	data := map[string]string{"Name": "World"}
	err := resp.Template(http.StatusOK, tmpl, data)
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", rw.Body.String())
}

func TestResponse_Template_Error(t *testing.T) {
	t.Parallel()
	rw := httptest.NewRecorder()
	resp := &Response{W: rw}
	err := resp.Template(http.StatusOK, "{{.Bad", nil)
	require.Error(t, err)
}

func TestResponse_RedirectTemporary(t *testing.T) {
	t.Parallel()
	rw := httptest.NewRecorder()
	resp := &Response{W: rw}
	req := httptest.NewRequestWithContext(t.Context(), "GET", "/", nil)
	resp.RedirectTemporary(req, "/tmp")
	assert.Equal(t, http.StatusTemporaryRedirect, rw.Code)
	assert.Equal(t, "/tmp", rw.Header().Get("Location"))
}

func TestResponse_RedirectPermanent(t *testing.T) {
	t.Parallel()
	rw := httptest.NewRecorder()
	resp := &Response{W: rw}
	req := httptest.NewRequestWithContext(t.Context(), "GET", "/", nil)
	resp.RedirectPermanent(req, "/perm")
	assert.Equal(t, http.StatusPermanentRedirect, rw.Code)
	assert.Equal(t, "/perm", rw.Header().Get("Location"))
}

func TestResponse_Error(t *testing.T) {
	t.Parallel()
	rw := httptest.NewRecorder()
	resp := &Response{W: rw}
	resp.Error(http.StatusBadRequest, "bad req")
	assert.Equal(t, http.StatusBadRequest, rw.Code)
	assert.Contains(t, rw.Body.String(), "bad req")
}
