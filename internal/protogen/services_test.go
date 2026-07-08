package protogen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildServices_FuncVsStructLayout(t *testing.T) {
	t.Parallel()

	src := map[string]string{
		"test.proto": `
syntax = "proto3";
package todo.v1;
import "google/protobuf/empty.proto";

message PingRequest {}

service PingService {
  rpc Ping(PingRequest) returns (google.protobuf.Empty);
}

message GetTodoRequest { string id = 1; }
message Todo { string id = 1; }
message ListTodosRequest {}
message ListTodosResponse {}

service TodoService {
  rpc GetTodo(GetTodoRequest) returns (Todo);
  rpc ListTodos(ListTodosRequest) returns (ListTodosResponse);
}
`,
	}
	fd := compileTestProto(t, src, "test.proto")
	services := buildServices(fd)

	byName := map[string]Service{}
	for _, s := range services {
		byName[s.Name] = s
	}

	ping := toServiceView("example.com/mod/model", byName["PingService"])
	assert.False(t, ping.IsStruct)
	assert.Len(t, ping.Methods, 1)
	assert.Equal(t, "PingServicePing", ping.Methods[0].FuncName)
	assert.Equal(t, "204", ping.Methods[0].SuccessStatus)
	// PingRequest is a real (if field-less) message, not google.protobuf.Empty,
	// so the Connect-fallback convention still sends it as the whole body.
	assert.Equal(t, "PingRequest", ping.Methods[0].RequestType)
	assert.Empty(t, ping.Methods[0].ResponseType)

	todo := toServiceView("example.com/mod/model", byName["TodoService"])
	assert.True(t, todo.IsStruct)
	assert.Len(t, todo.Methods, 2)
	assert.Equal(t, "GetTodo", todo.Methods[0].FuncName)
	assert.True(t, todo.NeedsModel)
}

func TestBuildServices_SkipsStreaming(t *testing.T) {
	t.Parallel()

	src := map[string]string{
		"test.proto": `
syntax = "proto3";
package test;

message Chunk {}

service ChatService {
  rpc Stream(stream Chunk) returns (stream Chunk);
  rpc Send(Chunk) returns (Chunk);
}
`,
	}
	fd := compileTestProto(t, src, "test.proto")
	services := buildServices(fd)

	assert.Len(t, services, 1)
	assert.Equal(t, []string{"Stream"}, services[0].SkippedStreaming)
	assert.Len(t, services[0].Methods, 1)
	assert.Equal(t, "Send", services[0].Methods[0].Name)
}

func TestRequestBodyType(t *testing.T) {
	t.Parallel()

	src := map[string]string{
		"test.proto": `
syntax = "proto3";
package test;

message Todo { string id = 1; }
message CreateTodoRequest {
  Todo todo = 1;
  string parent = 2;
}
`,
	}
	fd := compileTestProto(t, src, "test.proto")
	input := fd.Messages().ByName("CreateTodoRequest")

	assert.Empty(t, requestBodyType(input, ""))
	assert.Equal(t, "CreateTodoRequest", requestBodyType(input, "*"))
	assert.Equal(t, "Todo", requestBodyType(input, "todo"))
	// "parent" is a scalar, not representable as its own model type -> falls
	// back to the whole request message rather than silently dropping it.
	assert.Equal(t, "CreateTodoRequest", requestBodyType(input, "parent"))
}
