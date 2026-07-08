# Generating from a .proto spec

`snapi proto <spec_path> <output_dir>` turns a `.proto` file into an
annotated Go package plus DTOs — the reverse of everything else SnAPI does.
Point it at a spec, get back plain Go with `@SnAPI.*` comments that
`snapi build`/`serve`/`watch` then consume exactly like hand-written code.

```sh
snapi proto ./todo.proto .
```

`output_dir` doesn't need to contain `go.mod` itself — it's located by
searching `output_dir` and its ancestors, exactly like the `go` tool. See
[`example_proto/`](../example_proto) for a full worked example built this way.

To make regeneration part of your normal workflow, drop a `//go:generate`
directive in the target module instead of re-running the command by hand
(see [`example_proto/generate.go`](../example_proto/generate.go)):

```go
//go:generate go run github.com/StevenCyb/SnAPI/cmd/snapi proto todo.proto .
```

then `go generate ./...` reproduces the same output.

---

## Where files land

By default, handler files go **directly into `output_dir`** — exactly the
directory given, no implicit subdirectory — as package `api`, and DTOs go
into `output_dir/model` as package `model`.

To put them somewhere else, declare the standard protobuf
`option go_package` in the spec (not the same thing as the `package`
statement — see below):

```protobuf
option go_package = "github.com/you/yourmodule/pkg/api";
```

- The import path is resolved against the enclosing module: matched against
  `output_dir`'s module path, and the remainder becomes the target directory
  (`pkg/api` above → `<module root>/pkg/api`).
- The package name is the import path's last segment (`api` above) unless
  overridden with a `;name` suffix (`"...pkg/api;myapi"`) — only needed when
  the desired package name differs from the last path segment.
- DTOs still go into a `model` subdirectory next to wherever the handlers
  land (`pkg/api/model` in this example).

| Path                              | Regenerated?                 | Contents                                    |
| ---------------------------------- | ------------------------------ | -------------------------------------------- |
| `<target>/model/<spec>.go`         | Always                          | One DTO struct per message, one enum type per proto enum |
| `<target>/<service>.go`            | Only if it doesn't exist yet    | Annotated handler(s) for one `service`       |

Handler files are scaffolding, not final output: bodies are
`http.StatusNotImplemented` stubs you fill in yourself — pre-decoded into a
typed `req` variable when the RPC has a request body, and returning a typed
zero-value `resp` when it has a response, so the stub compiles as-is and you
just fill in the logic in between. Since business logic lives in the same
function as the routing annotations, re-running `snapi proto` after an
updated spec **never overwrites an existing handler file** — only the
`model` subdirectory (pure derived data, safe to regenerate) is always
rewritten. Delete a handler file yourself if you want it regenerated from
scratch.

A service with a single RPC generates one free function (mirrors
[`example/api_funcs.go`](../example/api_funcs.go)); a service with two or
more RPCs generates a handler struct (mirrors
[`example/api_struct.go`](../example/api_struct.go)).

---

## HTTP routing

Each RPC's route is resolved in this order:

1. **A `google.api.http` option**, if present — mapped to a real REST route.
2. Otherwise, the **Connect-RPC fallback**: `POST /<package>.<Service>/<Method>`,
   the whole request message as the JSON body, the whole response message as
   the JSON body.

### `google.api.http` mapping

```protobuf
rpc GetTodo(GetTodoRequest) returns (Todo) {
  option (google.api.http) = { get: "/v1/todos/{id}" };
}
```

becomes

```go
// @SnAPI.GET("/v1/todos/{id}")
// @SnAPI.Path("id", "string")
// @SnAPI.Status(200, "OK")
// @SnAPI.Response(200, "application/json", model.Todo)
func (s *TodoService) GetTodo(r runtime.Request, w runtime.Response) { ... }
```

- `get`/`put`/`post`/`delete`/`patch` → the matching `@SnAPI.<METHOD>(path)`.
- `{field}` path template placeholders map 1:1 to SnAPI's own `{field}`
  syntax and become `@SnAPI.Path(field, type)`. A pattern suffix
  (`{name=shelves/*}`) is stripped down to `{name}`; a dotted nested field
  path (`{a.b}`) is flattened to its top-level field `{a}`.
- `body: "*"` → the whole request message becomes `@SnAPI.Request(...)`.
- `body: "field"` → only that (message-typed) field becomes
  `@SnAPI.Request(...)`, referencing *its* type — not the wrapping request
  message.
- `body` unset (typically `get`/`delete`) → every request field not used as
  a path param becomes `@SnAPI.Query(field, type)` instead.
- `google.protobuf.Empty` as the response type → `@SnAPI.Status(204, "No Content")`
  with no `@SnAPI.Response`. As the request type → no `@SnAPI.Request` at all.
- Only the primary binding is used; extra `additional_bindings` entries are
  dropped with a comment noting how many were ignored (no silent cap).

### Connect-RPC fallback

```protobuf
service PingService {
  rpc Ping(google.protobuf.Empty) returns (PingResponse);
}
```

becomes

```go
// @SnAPI.POST("/todo.v1.PingService/Ping")
// @SnAPI.Status(200, "OK")
// @SnAPI.Response(200, "application/json", model.PingResponse)
func PingServicePing(r runtime.Request, w runtime.Response) { ... }
```

---

## Type mapping

| Proto                                            | Go                                     |
| ------------------------------------------------- | --------------------------------------- |
| `double` / `float`                                | `float64` / `float32`                   |
| `int32`/`sint32`/`sfixed32`, `int64`/`sint64`/`sfixed64` | `int32`, `int64`                 |
| `uint32`/`fixed32`, `uint64`/`fixed64`            | `uint32`, `uint64`                      |
| `bool`, `string`, `bytes`                         | `bool`, `string`, `[]byte`              |
| `enum`                                            | generated `int32`-backed named type     |
| message (defined in the spec)                     | `*<Message>`                            |
| `repeated X`                                      | `[]X`                                   |
| `map<K, V>`                                       | `map[K]V`                               |
| proto3 `optional` scalar                          | pointer                                 |
| `google.protobuf.Timestamp`                       | `*time.Time`                            |
| `google.protobuf.Duration`                        | `*time.Duration` — JSON round-trips as a plain number of nanoseconds, not protobuf's canonical `"3.5s"` string |
| `google.protobuf.{String,Int32,Int64,Bool,UInt32,UInt64,Float,Double}Value` | `*string`/`*int32`/... |
| `google.protobuf.Any` / `Struct` / `Value`        | `map[string]interface{}` (generic fallback) |
| `google.protobuf.ListValue`                       | `[]interface{}`                          |
| `google.protobuf.FieldMask`                       | `[]string`                               |
| `oneof` members                                   | each field keeps its normal type; a comment above the group names the mutually-exclusive fields |

Field names become `PascalCase` Go field names (`user_id` → `UserId`) with a
`json` tag set to the proto field's JSON name (proto3's default
lowerCamelCase, or an explicit `json_name` override) — the same mapping
Connect's own JSON codec uses. Nested proto messages are flattened to
top-level Go structs (`Parent_Child`, protoc-gen-go style) since Go has no
nested named types.

---

## Known limitations

- **No streaming.** Client/server/bidi-streaming RPCs aren't representable
  as a single request/response handler; they're skipped with a comment in
  the generated file.
- **One `model` package.** DTOs from every proto file (the main spec plus
  its local imports) land in the same Go package, one file per source
  `.proto` file. Two proto files defining a message with the same name will
  collide as Go types. Only the main spec's `go_package` is consulted for
  placement — imported files' own `go_package` (if any) is ignored, their
  messages join the main spec's `model` package regardless.
- **`package` vs `go_package`.** The proto `package` statement (e.g.
  `todo.v1`) is just a dotted namespace used for fully-qualified names like
  `todo.v1.TodoService` (and the Connect-RPC fallback path) — it has no
  effect on where generated files land. Only `option go_package` does.
- **Local imports only.** `import`s to other `.proto` files are resolved
  relative to the spec file's own directory. `google/protobuf/*.proto`
  well-known types and `google/api/{http,annotations}.proto` resolve without
  any extra files.
- **RPC input/output must be messages you define** (or `google.protobuf.Empty`
  for a bodyless response) — using a well-known wrapper type directly as an
  RPC's request or response is unsupported.
- Comments in the `.proto` file aren't carried over into
  `@SnAPI.Summary`/`@SnAPI.Description`.
