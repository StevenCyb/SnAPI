package protogen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantPath   string
		wantParams []string
	}{
		{name: "no params", input: "/v1/todos", wantPath: "/v1/todos", wantParams: nil},
		{name: "simple param", input: "/v1/todos/{id}", wantPath: "/v1/todos/{id}", wantParams: []string{"id"}},
		{
			name: "pattern suffix stripped", input: "/v1/{name=shelves/*/books/*}",
			wantPath: "/v1/{name}", wantParams: []string{"name"},
		},
		{
			name: "dotted nested field flattened", input: "/v1/todos/{todo.id}",
			wantPath: "/v1/todos/{todo}", wantParams: []string{"todo"},
		},
		{
			name: "multiple params", input: "/v1/{parent}/todos/{id}",
			wantPath: "/v1/{parent}/todos/{id}", wantParams: []string{"parent", "id"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path, params := normalizePath(tt.input)
			assert.Equal(t, tt.wantPath, path)
			assert.Equal(t, tt.wantParams, params)
		})
	}
}

func TestResolveRoute_ConnectFallback(t *testing.T) {
	t.Parallel()

	src := map[string]string{
		"test.proto": `
syntax = "proto3";
package todo.v1;

message PingRequest {}
message PingResponse {}

service PingService {
  rpc Ping(PingRequest) returns (PingResponse);
}
`,
	}
	fd := compileTestProto(t, src, "test.proto")
	md := fd.Services().Get(0).Methods().Get(0)

	route := resolveRoute(md, "todo.v1.PingService")
	assert.Equal(t, "POST", route.Method)
	assert.Equal(t, "/todo.v1.PingService/Ping", route.Path)
	assert.Equal(t, "*", route.BodyField)
	assert.Empty(t, route.PathParams)
	assert.Empty(t, route.QueryParams)
}

func TestResolveRoute_HTTPRule(t *testing.T) {
	t.Parallel()

	src := map[string]string{
		"test.proto": `
syntax = "proto3";
package todo.v1;
import "google/api/annotations.proto";

message GetTodoRequest {
  string id = 1;
}
message Todo {
  string id = 1;
}
message ListTodosRequest {
  int32 page_size = 1;
  string filter = 2;
}
message ListTodosResponse {}
message CreateTodoRequest {
  Todo todo = 1;
}

service TodoService {
  rpc GetTodo(GetTodoRequest) returns (Todo) {
    option (google.api.http) = { get: "/v1/todos/{id}" };
  }
  rpc ListTodos(ListTodosRequest) returns (ListTodosResponse) {
    option (google.api.http) = { get: "/v1/todos" };
  }
  rpc CreateTodo(CreateTodoRequest) returns (Todo) {
    option (google.api.http) = { post: "/v1/todos", body: "todo" };
  }
}
`,
	}
	fd := compileTestProto(t, src, "test.proto")
	methods := fd.Services().Get(0).Methods()

	get := resolveRoute(methods.Get(0), "todo.v1.TodoService")
	assert.Equal(t, Route{
		Method:     "GET",
		Path:       "/v1/todos/{id}",
		PathParams: []RouteParam{{Name: "id", Type: "string"}},
	}, get)

	list := resolveRoute(methods.Get(1), "todo.v1.TodoService")
	assert.Equal(t, "GET", list.Method)
	assert.Empty(t, list.PathParams)
	assert.ElementsMatch(t, []RouteParam{
		{Name: "page_size", Type: "int32"},
		{Name: "filter", Type: "string"},
	}, list.QueryParams)
	assert.Empty(t, list.BodyField)

	create := resolveRoute(methods.Get(2), "todo.v1.TodoService")
	assert.Equal(t, "POST", create.Method)
	assert.Equal(t, "todo", create.BodyField)
}
