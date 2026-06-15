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

## See It In 5 Seconds

Write this in your Go module:

```go
package api

import (
	"net/http"

	"github.com/StevenCyb/SnAPI/pkg/runtime"
)

// @SnAPI.GET("/hello")
func Hello(r runtime.Request, w runtime.Response) {
	w.Html(http.StatusOK, "<h1>Hello</h1>")
}
```

Run:

```sh
snapi serve . --swagger /swagger
```

Get:

- `GET /hello` mapped and served.
- OpenAPI + Swagger UI mounted at `/swagger`.
- Auto-generated bootstrap (`main`, routing, middleware/lifecycle wiring).

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
go install github.com/StevenCyb/SnAPI/cmd/snapi@latest
```

The binary is called `snapi`.

## Quick start

```sh
# watch project, regenerate and restart on every .go change
snapi watch ./example

# generate + run once
snapi serve ./example

# generate + compile to a single binary
snapi build ./example ./bin/myapi
```

See [docs/cli.md](docs/cli.md) for the full command and flag reference.

## How it works

You point SnAPI at a Go module path. It:

1. Parses every `.go` file looking for `@SnAPI.*` / `@snapi.*` comments.
2. Builds an internal project model (handlers, middleware, lifecycle hooks,
   OpenAPI metadata).
3. Generates a fresh `main` package into a temp dir (or `output_path` for
   `build`) that wires `net/http`, registers all routes, mounts middleware,
   runs lifecycle hooks, and optionally serves a Swagger UI.
4. `serve` / `watch` then run the generated project for you.

## Documentation

| Document | What it covers |
| --- | --- |
| [docs/annotations.md](docs/annotations.md) | All `@SnAPI.*` and `@snapi.*` annotations — routing, middleware, lifecycle hooks, OpenAPI metadata |
| [docs/config.md](docs/config.md) | `runtime.LoadConfig`, `arg` tag syntax, `.env` file support |
| [docs/runtime.md](docs/runtime.md) | `Request` and `Response` interfaces available in handlers |
| [docs/cli.md](docs/cli.md) | CLI commands and flags |

## License

MIT — see [LICENSE](LICENSE).

