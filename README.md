# SnAPI

```
   _____       ___    ____  ____
  / ___/____  /   |  / __ \/  _/
  \__ \/ __ \/ /| | / /_/ // /
 ___/ / / / / ___ |/ ____// /
/____/_/ /_/_/  |_/_/   /___/
```

Annotation-driven HTTP API generator for Go. Write plain handler functions,
sprinkle `@SnAPI.*` comments on top, and SnAPI generates a runnable `main`
package plus an OpenAPI/Swagger spec for it.

> **Disclaimer.** This is a personal *just-for-fun* project. It is **not
> production ready**, has rough edges, and the API surface will change
> without notice. Use it for hacking, learning, or as inspiration —
> not for anything you care about.

## Motivation

I've written a lot of microservices and kept wondering why I always end up
writing the same boilerplate around them. Other languages have elegant
frameworks that strip the code down to the essentials — handlers and
services, nothing else. I wanted to see how close I could get to that in Go.
The language's design imposes some limits, but the approach is still an
interesting one to explore.

## Install

```sh
go install github.com/StevenCyb/SnAPI/cmd/cli@latest
```

The binary is called `snapi`.

## Quick start

```sh
# generate + run an example project, restart on every .go change
snapi watch ./example

# generate + run once (no file watching)
snapi serve ./example

# generate + compile to a single binary
snapi build ./example ./bin/myapi
```

Global flags (work on every command):

| flag                 | description                              |
| -------------------- | ---------------------------------------- |
| `-l`, `--log-level`  | `debug`, `info`, `warn`, `error`         |
| `--no-color`         | disable ANSI colors                      |
| `SNAPI_LOG_LEVEL`    | env-var fallback for `--log-level`       |

Per-command flags on `build` / `serve` / `watch`:

| flag             | description                                                     |
| ---------------- | --------------------------------------------------------------- |
| `-t`, `--tags`   | build tags to forward to `go build` / `go run`                  |
| `-s`, `--swagger`| mount path for Swagger UI + OpenAPI spec (empty = disabled)     |

CLI logs and the spawned application's logs are colored and tagged
distinctly so you can tell them apart at a glance.

## How it works

You point SnAPI at a Go module path. It:

1. Parses every `.go` file looking for `@SnAPI.*` / `@snapi.*` comments.
2. Builds an internal project model (handlers, middleware, lifecycle hooks,
   config types, OpenAPI metadata).
3. Generates a fresh `main` package into a temp dir (or `output_path` for
   `build`) that wires `net/http`, registers all routes, mounts middleware,
   runs lifecycle hooks, and (when `--swagger <path>` is given) serves a
   Swagger UI at that path.
4. `serve`/`watch` then `go run`s the generated project for you.

See [docs/annotations.md](docs/annotations.md) for the full annotation
reference.

## License

MIT — see [LICENSE](LICENSE).
