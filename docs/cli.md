# CLI reference

## Commands

### `snapi build <project_path> <output_path>`

Parses and generates the project, then compiles it to a single binary at
`output_path`.

### `snapi serve <project_path>`

Generates, compiles and runs the project once. Blocks until `SIGINT`/`SIGTERM`.

### `snapi watch <project_path>`

Watches `project_path` for `.go` file changes. On every change it regenerates,
recompiles and restarts the server automatically. Blocks until
`SIGINT`/`SIGTERM`.

### `snapi proto <spec_path> <output_dir>`

Generates annotated handler(s) plus DTOs from a `.proto` spec, into
`output_dir` (or wherever the spec's `option go_package` points, if set).
See [proto.md](proto.md) for the full mapping reference.

### `snapi version`

Prints the current version string.

---

## Global flags

These work on every command.

| Flag                  | Description                                      |
| --------------------- | ------------------------------------------------ |
| `-l`, `--log-level`   | `debug`, `info`, `warn`, `error` (default `info`)|
| `--no-color`          | disable ANSI color output                        |

`SNAPI_LOG_LEVEL` is an env-var fallback for `--log-level`.

SnAPI's own log lines and the spawned application's output are colored and
tagged distinctly so you can tell them apart at a glance.

---

## Flags for `build` / `serve` / `watch`

| Flag              | Short | Description                                                   |
| ----------------- | ----- | ------------------------------------------------------------- |
| `--tags`          | `-t`  | build tags forwarded to `go build` / `go run`                 |
| `--swagger`       | `-s`  | mount path for Swagger UI + OpenAPI spec (empty = disabled)   |

---

## Flags for `serve` / `watch`

| Flag        | Short | Description                                                                      |
| ----------- | ----- | -------------------------------------------------------------------------------- |
| `--dotenv`  | `-e`  | path to a `.env` file injected into the server process at startup                |

When `--dotenv` is omitted, SnAPI automatically looks for `<project_path>/.env`
and loads it if it exists. See [config.md](config.md#env-file-support) for
details on precedence and supported syntax.
