package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/StevenCyb/SnAPI/internal/logger"
	"github.com/StevenCyb/SnAPI/internal/runner"

	"github.com/StevenCyb/GoCLI/pkg/cli"
)

const version = "v0.1.5"

var pathRegex = regexp.MustCompile(`^(?:(?:[a-zA-Z]:[\\/]|[\\/]{1,2})?[^<>:"|?*\r\n]+(?:[\\/][^<>:"|?*\r\n]+)*[\\/]?|[\\/])$`)

func main() {
	args := configureLoggingFromArgs(os.Args)

	c := cli.New(
		cli.Name("snapi"),
		cli.Banner(`   _____       ___    ____  ____
  / ___/____  /   |  / __ \/  _/
  \__ \/ __ \/ /| | / /_/ // /
 ___/ / / / / ___ |/ ____// /
/____/_/ /_/_/  |_/_/   /___/ `),
		cli.Description("Simple API framework generator for Go.\n\nGlobal flags:\n  --log-level, -l   debug|info|warn|error (default info)\n  --no-color        disable ANSI colors"),
		cli.Version(version),
		cli.Command(
			"build",
			cli.Description("Builds the project into a single binary."),
			cli.Argument(
				"project_path",
				cli.Description("Path to the root of the Go project."),
				cli.Validate(pathRegex),
				cli.Argument(
					"output_path",
					cli.Description("Path to output to (including filename)."),
					cli.Validate(pathRegex),
					cli.Option("tags", cli.Short('t'), cli.Default("")),
					cli.Option("swagger", cli.Short('s'), cli.Default(""),
						cli.Description("Mount Swagger UI at this path (empty = disabled).")),
					cli.Handler(handleBuild),
				),
			),
		),
		cli.Command(
			"serve",
			cli.Description("Builds and serves the project once."),
			cli.Argument(
				"project_path",
				cli.Description("Path to the root of the Go project."),
				cli.Validate(pathRegex),
				cli.Option("tags", cli.Short('t'), cli.Default("")),
				cli.Option("swagger", cli.Short('s'), cli.Default(""),
					cli.Description("Mount Swagger UI at this path (empty = disabled).")),
				cli.Option("dotenv", cli.Short('e'), cli.Default(""),
					cli.Description("Path to a .env file to inject into the server process (default: <project_path>/.env).")),
				cli.Handler(handleServe),
			),
		),
		cli.Command(
			"watch",
			cli.Description("Watch the project directory and restart the server on changes."),
			cli.Argument(
				"project_path",
				cli.Description("Path to the root of the Go project."),
				cli.Validate(pathRegex),
				cli.Option("tags", cli.Short('t'), cli.Default("")),
				cli.Option("swagger", cli.Short('s'), cli.Default(""),
					cli.Description("Mount Swagger UI at this path (empty = disabled).")),
				cli.Option("dotenv", cli.Short('e'), cli.Default(""),
					cli.Description("Path to a .env file to inject into the server process (default: <project_path>/.env).")),
				cli.Handler(handleWatch),
			),
		),
		cli.Command(
			"proto",
			cli.Description("Generate an annotated API package and DTOs from a .proto spec."),
			cli.Argument(
				"spec_path",
				cli.Description("Path to the .proto file."),
				cli.Validate(pathRegex),
				cli.Argument(
					"output_dir",
					cli.Description("Project directory to generate into (must already contain go.mod)."),
					cli.Validate(pathRegex),
					cli.Handler(handleProto),
				),
			),
		),
		cli.Command(
			"version",
			cli.Description("Get the version of the CLI"),
			cli.Handler(func(_ *cli.Context) error {
				fmt.Println(version)
				return nil
			}),
		),
	)

	if _, err := c.RunWith(args); err != nil {
		logger.Error("%v", err)
		c.PrintHelp()
		os.Exit(1)
	}
}

func handleBuild(ctx *cli.Context) error {
	src := *ctx.GetArgument("project_path")
	dst := *ctx.GetArgument("output_path")
	tags := *ctx.GetOption("tags")
	swagger := *ctx.GetOption("swagger")
	logger.Info("Build project from %s to %s (tags=%q, swagger=%q)", src, dst, tags, swagger)
	runner.WithTempDir(func(tmp string) {
		runner.Bootstrap(src, tmp, swagger)
		runner.Build(tmp, dst, tags)
	})
	return nil
}

func handleProto(ctx *cli.Context) error {
	spec := *ctx.GetArgument("spec_path")
	dst := *ctx.GetArgument("output_dir")
	logger.Info("Generate API from %s into %s", spec, dst)
	runner.GenerateProto(spec, dst)
	return nil
}

func handleServe(ctx *cli.Context) error {
	src := *ctx.GetArgument("project_path")
	tags := *ctx.GetOption("tags")
	swagger := *ctx.GetOption("swagger")
	dotenv := resolveDotEnv(*ctx.GetOption("dotenv"), src)
	logger.Info("Serve project at %s (tags=%q, swagger=%q)", src, tags, swagger)
	runner.WithTempDir(func(tmp string) {
		runner.Bootstrap(src, tmp, swagger)
		runner.Serve(tmp, tags, dotenv)
	})
	return nil
}

func handleWatch(ctx *cli.Context) error {
	src := *ctx.GetArgument("project_path")
	tags := *ctx.GetOption("tags")
	swagger := *ctx.GetOption("swagger")
	dotenv := resolveDotEnv(*ctx.GetOption("dotenv"), src)
	logger.Info("Watch project at %s (tags=%q, swagger=%q)", src, tags, swagger)
	runner.WithTempDir(func(tmp string) {
		runner.Bootstrap(src, tmp, swagger)
		runner.Watch(src, tmp, tags, swagger, dotenv)
	})
	return nil
}

// resolveDotEnv returns explicit if non-empty, otherwise falls back to
// <projectPath>/.env if that file exists, or empty string if neither is found.
func resolveDotEnv(explicit, projectPath string) string {
	if explicit != "" {
		return explicit
	}
	candidate := filepath.Join(projectPath, ".env")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

// configureLoggingFromArgs strips global logging flags from argv and applies
// them to the default logger. Recognized flags:
//
//	--log-level <lvl> | -l <lvl>   debug|info|warn|error
//	--no-color                     disable ANSI colors
//
// The SNAPI_LOG_LEVEL env var is honored as a fallback.
func configureLoggingFromArgs(in []string) []string {
	if lv := os.Getenv("SNAPI_LOG_LEVEL"); lv != "" {
		if parsed, ok := logger.ParseLevel(lv); ok {
			logger.SetLevel(parsed)
		}
	}

	out := make([]string, 0, len(in))
	out = append(out, in[0])
	for i := 1; i < len(in); i++ {
		switch in[i] {
		case "--no-color":
			logger.SetColor(false)
		case "--log-level", "-l":
			if i+1 < len(in) {
				if parsed, ok := logger.ParseLevel(in[i+1]); ok {
					logger.SetLevel(parsed)
				} else {
					logger.Warn("unknown log level %q, keeping default", in[i+1])
				}
				i++
			}
		default:
			out = append(out, in[i])
		}
	}
	return out
}
