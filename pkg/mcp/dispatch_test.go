package mcp_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/StevenCyb/SnAPI/pkg/mcp"
	"github.com/StevenCyb/SnAPI/pkg/runtime"
)

const protocolVersionHeader = "Mcp-Protocol-Version"

func newTestServer() *mcp.Server {
	srv := mcp.NewServer(mcp.ServerInfo{Name: "test-server", Version: "1.0.0", Instructions: "be nice"})
	srv.AddTool(mcp.Tool{
		Name:        "greet",
		Description: "Greets someone",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`),
		Handler: func(req runtime.Request, resp runtime.Response) {
			var args struct {
				Name string `json:"name"`
			}
			_ = req.FromJsonBody(&args)
			_ = resp.Json(http.StatusOK, map[string]string{"greeting": "hello " + args.Name})
		},
	})
	srv.AddResource(mcp.Resource{
		URI: "file:///{path}", Name: "files", MimeType: "text/plain",
		Handler: func(req runtime.Request, resp runtime.Response) {
			resp.Text(http.StatusOK, "content of "+req.PathValue("path"))
		},
	})
	srv.AddPrompt(mcp.Prompt{
		Name: "review", Description: "review some code",
		Args: []mcp.PromptArg{{Name: "code", Required: true}},
		Handler: func(req runtime.Request, resp runtime.Response) {
			resp.Text(http.StatusOK, "please review: "+req.QueryValue("code"))
		},
	})
	return srv
}

func newRequest(t *testing.T, method, url string, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, strings.NewReader(body))
	require.NoError(t, err)
	return req
}

func post(t *testing.T, ts *httptest.Server, method string, params map[string]any, headers map[string]string) *http.Response {
	t.Helper()
	body := map[string]any{"jsonrpc": "2.0", "id": 1, "method": method}
	if params != nil {
		body["params"] = params
	}
	raw, err := json.Marshal(body)
	require.NoError(t, err)

	req := newRequest(t, http.MethodPost, ts.URL, string(raw))
	req.Header.Set("Content-Type", "application/json")
	if _, ok := headers[protocolVersionHeader]; !ok {
		req.Header.Set(protocolVersionHeader, mcp.ProtocolVersion)
	}
	if _, ok := headers["Mcp-Method"]; !ok {
		req.Header.Set("Mcp-Method", method)
	}
	for k, v := range headers {
		if v == "" {
			req.Header.Del(k)
			continue
		}
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func decodeResult(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var out struct {
		Result map[string]any `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	if out.Error != nil {
		t.Fatalf("unexpected JSON-RPC error: %d %s", out.Error.Code, out.Error.Message)
	}
	return out.Result
}

// decodeErrorCode reads a JSON-RPC error's code from resp, closing the body.
func decodeErrorCode(t *testing.T, resp *http.Response) int {
	t.Helper()
	defer resp.Body.Close()
	var out struct {
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	return out.Error.Code
}

func TestServerDiscover(t *testing.T) {
	ts := httptest.NewServer(newTestServer())
	defer ts.Close()

	resp := post(t, ts, "server/discover", nil, nil)
	defer resp.Body.Close()
	result := decodeResult(t, resp)
	assert.Equal(t, "complete", result["resultType"])
	assert.Contains(t, result["supportedVersions"], mcp.ProtocolVersion)
	caps, _ := result["capabilities"].(map[string]any)
	assert.Contains(t, caps, "tools")
	assert.Contains(t, caps, "resources")
	assert.Contains(t, caps, "prompts")
}

func TestToolsListAndCall(t *testing.T) {
	ts := httptest.NewServer(newTestServer())
	defer ts.Close()

	listResp := post(t, ts, "tools/list", nil, nil)
	defer listResp.Body.Close()
	listResult := decodeResult(t, listResp)
	tools, _ := listResult["tools"].([]any)
	require.Len(t, tools, 1)

	callResp := post(t, ts, "tools/call",
		map[string]any{"name": "greet", "arguments": map[string]any{"name": "Bob"}},
		map[string]string{"Mcp-Name": "greet"})
	defer callResp.Body.Close()
	callResult := decodeResult(t, callResp)
	assert.Equal(t, false, callResult["isError"])
	content, _ := callResult["content"].([]any)
	require.Len(t, content, 1)
	entry, _ := content[0].(map[string]any)
	assert.Contains(t, entry["text"], "hello Bob")
}

func TestToolsCallUnknownTool(t *testing.T) {
	ts := httptest.NewServer(newTestServer())
	defer ts.Close()

	resp := post(t, ts, "tools/call", map[string]any{"name": "nope", "arguments": map[string]any{}},
		map[string]string{"Mcp-Name": "nope"})
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, mcp.CodeInvalidParams, decodeErrorCode(t, resp))
}

func TestResourceTemplateRead(t *testing.T) {
	ts := httptest.NewServer(newTestServer())
	defer ts.Close()

	listResp := post(t, ts, "resources/templates/list", nil, nil)
	defer listResp.Body.Close()
	tmplResult := decodeResult(t, listResp)
	templates, _ := tmplResult["resourceTemplates"].([]any)
	require.Len(t, templates, 1)

	readResp := post(t, ts, "resources/read",
		map[string]any{"uri": "file:///foo.txt"},
		map[string]string{"Mcp-Name": "file:///foo.txt"})
	defer readResp.Body.Close()
	readResult := decodeResult(t, readResp)
	contents, _ := readResult["contents"].([]any)
	require.Len(t, contents, 1)
	entry, _ := contents[0].(map[string]any)
	assert.Equal(t, "content of foo.txt", entry["text"])
}

func TestPromptsGet(t *testing.T) {
	ts := httptest.NewServer(newTestServer())
	defer ts.Close()

	resp := post(t, ts, "prompts/get",
		map[string]any{"name": "review", "arguments": map[string]any{"code": "x=1"}},
		map[string]string{"Mcp-Name": "review"})
	defer resp.Body.Close()
	result := decodeResult(t, resp)
	messages, _ := result["messages"].([]any)
	require.Len(t, messages, 1)
}

func TestMethodNotFound(t *testing.T) {
	ts := httptest.NewServer(newTestServer())
	defer ts.Close()

	resp := post(t, ts, "totally/unknown", nil, nil)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, mcp.CodeMethodNotFound, decodeErrorCode(t, resp))
}

func TestMissingProtocolVersionHeader(t *testing.T) {
	ts := httptest.NewServer(newTestServer())
	defer ts.Close()

	resp := post(t, ts, "tools/list", nil, map[string]string{protocolVersionHeader: ""})
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, mcp.CodeHeaderMismatch, decodeErrorCode(t, resp))
}

func TestMismatchedMcpMethodHeader(t *testing.T) {
	ts := httptest.NewServer(newTestServer())
	defer ts.Close()

	resp := post(t, ts, "tools/list", nil, map[string]string{"Mcp-Method": "prompts/list"})
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetAndDeleteNotAllowed(t *testing.T) {
	ts := httptest.NewServer(newTestServer())
	defer ts.Close()

	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		req := newRequest(t, method, ts.URL, "")
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode, method)
	}
}

func TestNotificationAccepted(t *testing.T) {
	ts := httptest.NewServer(newTestServer())
	defer ts.Close()

	req := newRequest(t, http.MethodPost, ts.URL, `{"jsonrpc":"2.0","method":"notifications/whatever"}`)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}

func TestOriginAllowlist(t *testing.T) {
	srv := mcp.NewServer(mcp.ServerInfo{Name: "s", Version: "1"}, mcp.WithAllowedOrigins("https://good.example"))
	ts := httptest.NewServer(srv)
	defer ts.Close()

	discoverBody := `{"jsonrpc":"2.0","id":1,"method":"server/discover"}`

	req := newRequest(t, http.MethodPost, ts.URL, discoverBody)
	req.Header.Set(protocolVersionHeader, mcp.ProtocolVersion)
	req.Header.Set("Mcp-Method", "server/discover")
	req.Header.Set("Origin", "https://evil.example")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)

	req2 := newRequest(t, http.MethodPost, ts.URL, discoverBody)
	req2.Header.Set(protocolVersionHeader, mcp.ProtocolVersion)
	req2.Header.Set("Mcp-Method", "server/discover")
	req2.Header.Set("Origin", "https://good.example")
	resp2, err := http.DefaultClient.Do(req2)
	require.NoError(t, err)
	resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
}
