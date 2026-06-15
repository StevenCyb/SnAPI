# Config loading

## runtime.LoadConfig

`runtime.LoadConfig[T]()` builds a `T` from defaults, environment variables
and CLI flags. The result is cached per type; subsequent calls return the same
value without re-reading the environment.

```go
type Config struct {
    Addr    string        `arg:"addr,default=:8080"`
    Timeout time.Duration `arg:"timeout,env=APP_TIMEOUT,default=30s"`
    Token   string        `arg:"token,env=API_TOKEN,required"`
}

// @snapi.setup
func LoadConfig() error {
    cfg, err := runtime.LoadConfig[Config]()
    if err != nil {
        return err
    }
    // use cfg ...
    return nil
}
```

### Tag syntax

Fields are controlled with the `arg` struct tag.

| Tag                              | Meaning                                           |
| -------------------------------- | ------------------------------------------------- |
| `arg:"name"`                     | flag `--name` / env var `NAME`                    |
| `arg:"name,env=ENV_VAR"`         | custom env var name                               |
| `arg:"name,default=value"`       | fallback when neither env nor flag is set         |
| `arg:"name,required"`            | error if no value is supplied from any source     |
| `arg:"-"`                        | skip this field entirely                          |

**Precedence** (later wins): `default` → env var → CLI flag.

### Supported field types

`string`, `bool`, `int`/`int8`/…/`int64`, `uint`/…/`uint64`,
`float32`/`float64`, `time.Duration`, pointer variants of all of the above,
and `[]T` slices (comma-separated input).

---

## .env file support

`serve` and `watch` automatically inject variables from a `.env` file into the
server process before startup. This lets `runtime.LoadConfig` see values that
live in the project source tree even though the server binary runs from a
temporary build directory.

### Auto-detection

When `--dotenv` is not passed, SnAPI looks for `.env` in the project root
(`<project_path>/.env`) and loads it silently when found.

### Explicit path

Pass `--dotenv`/`-e` to point at any file:

```sh
snapi serve ./myproject --dotenv ./myproject/.env.local
snapi watch ./myproject -e ./myproject/.env.staging
```

### Precedence

Variables already present in the shell environment always win over `.env`
values. The file acts as a set of defaults, not overrides.

### Supported syntax

```dotenv
# comment lines are ignored
KEY=value
export EXPORTED=yes        # leading "export" is stripped
QUOTED="hello world"       # outer double quotes are stripped
SINGLE='hello world'       # outer single quotes are stripped
WITH_EQUALS=base64==       # value may contain = signs
```
