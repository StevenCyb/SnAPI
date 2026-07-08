package protogen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestGoFieldName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "single word", input: "id", expected: "Id"},
		{name: "snake case", input: "user_id", expected: "UserId"},
		{name: "multi segment", input: "created_at_utc", expected: "CreatedAtUtc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, goFieldName(protoreflect.Name(tt.input)))
		})
	}
}

func TestSnakeCase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "single word", input: "Todo", expected: "todo"},
		{name: "two words", input: "TodoService", expected: "todo_service"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, snakeCase(tt.input))
		})
	}
}

func TestBuildProtoFile_TypeMapping(t *testing.T) {
	t.Parallel()

	src := map[string]string{
		"test.proto": `
syntax = "proto3";
package test;
import "google/protobuf/timestamp.proto";

enum Status {
  STATUS_UNKNOWN = 0;
  STATUS_ACTIVE = 1;
}

message Nested {
  string value = 1;
}

message Todo {
  string id = 1;
  int32 count = 2;
  repeated string tags = 3;
  map<string, int32> counters = 4;
  Status status = 5;
  Nested nested = 6;
  google.protobuf.Timestamp created_at = 7;
  oneof kind {
    string a = 8;
    string b = 9;
  }
}
`,
	}
	fd := compileTestProto(t, src, "test.proto")
	pf := buildProtoFile(fd)

	assert.True(t, pf.NeedsTime)
	assert.Len(t, pf.Enums, 1)
	assert.Equal(t, "Status", pf.Enums[0].Name)
	assert.Equal(t, []EnumValue{
		{Name: "Status_STATUS_UNKNOWN", Number: 0},
		{Name: "Status_STATUS_ACTIVE", Number: 1},
	}, pf.Enums[0].Values)

	var todo *Message
	for i := range pf.Messages {
		if pf.Messages[i].Name == "Todo" {
			todo = &pf.Messages[i]
		}
	}
	if todo == nil {
		t.Fatal("Todo message not found")
	}

	byName := map[string]Field{}
	for _, f := range todo.Fields {
		byName[f.GoName] = f
	}

	assert.Equal(t, "string", byName["Id"].GoType)
	assert.Equal(t, "int32", byName["Count"].GoType)
	assert.Equal(t, "[]string", byName["Tags"].GoType)
	assert.Equal(t, "map[string]int32", byName["Counters"].GoType)
	assert.Equal(t, "Status", byName["Status"].GoType)
	assert.Equal(t, "*Nested", byName["Nested"].GoType)
	assert.Equal(t, "*time.Time", byName["CreatedAt"].GoType)
	assert.Equal(t, "kind", byName["A"].OneofGroup)
	assert.Equal(t, "kind", byName["B"].OneofGroup)
	assert.Empty(t, byName["Id"].OneofGroup)
}

func TestBuildProtoFile_NestedMessageFlattening(t *testing.T) {
	t.Parallel()

	src := map[string]string{
		"test.proto": `
syntax = "proto3";
package test;

message Order {
  message Item {
    string sku = 1;
  }
  repeated Item items = 1;
}
`,
	}
	fd := compileTestProto(t, src, "test.proto")
	pf := buildProtoFile(fd)

	names := make([]string, len(pf.Messages))
	for i, m := range pf.Messages {
		names[i] = m.Name
	}
	assert.Contains(t, names, "Order")
	assert.Contains(t, names, "Order_Item")

	for _, m := range pf.Messages {
		if m.Name == "Order" {
			assert.Equal(t, "[]*Order_Item", m.Fields[0].GoType)
		}
	}
}

func TestScalarGoType_WrapperAndWellKnown(t *testing.T) {
	t.Parallel()

	src := map[string]string{
		"test.proto": `
syntax = "proto3";
package test;
import "google/protobuf/wrappers.proto";
import "google/protobuf/duration.proto";

message Todo {
  google.protobuf.StringValue nickname = 1;
  google.protobuf.Duration ttl = 2;
  optional string note = 3;
}
`,
	}
	fd := compileTestProto(t, src, "test.proto")
	pf := buildProtoFile(fd)

	byName := map[string]Field{}
	for _, f := range pf.Messages[0].Fields {
		byName[f.GoName] = f
	}
	assert.Equal(t, "*string", byName["Nickname"].GoType)
	assert.Equal(t, "*time.Duration", byName["Ttl"].GoType)
	assert.Equal(t, "*string", byName["Note"].GoType)
}
