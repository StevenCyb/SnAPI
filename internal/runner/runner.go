// Package runner contains the high-level operations the snapi CLI exposes:
// bootstrapping the project, building it, serving it, and watching for
// changes. Each function is callable on its own and handles its own logging.
package runner

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	"github.com/StevenCyb/SnAPI/internal/cmd"
	"github.com/StevenCyb/SnAPI/internal/generator"
	"github.com/StevenCyb/SnAPI/internal/logger"
	"github.com/StevenCyb/SnAPI/internal/models"
	"github.com/StevenCyb/SnAPI/internal/parser"
	"github.com/StevenCyb/SnAPI/internal/watcher"
)

// WithTempDir creates a temporary build directory, calls fn, then removes it.
// The cleanup also runs if logger.Fatal is called from inside fn.
// On creation failure it logs and exits the process.
func WithTempDir(fn func(dir string)) {
	out, err := os.MkdirTemp("", "snapi-build-")
	if err != nil {
		logger.Fatal("create temp dir: %v", err)
	}
	cleanup := func() { _ = os.RemoveAll(out) }
	cancel := logger.OnFatal(cleanup)
	defer func() {
		cancel()
		cleanup()
	}()
	fn(out)
}

// Bootstrap parses src and generates a runnable Go project into dst.
// If swaggerPath is non-empty, the generated server exposes Swagger UI
// at that path. Exits the process on failure.
func Bootstrap(src, dst, swaggerPath string) {
	log := logger.Scope("gen")
	log.Info("Parsing source from %s", src)
	project, err := parser.NewParser(src).Parse()
	if err != nil {
		log.Fatal("failed to parse source: %v", err)
	}

	log.Info("Generating output to %s", dst)
	if err := generate(project, dst, swaggerPath); err != nil {
		log.Fatal("failed to generate output: %v", err)
	}
	log.Debug("generation done")
}

// Build runs `go build` against the generated project at src and writes the
// binary to dst. Exits the process on failure.
func Build(src, dst, tags string) {
	log := logger.Scope("build")
	log.Info("compiling...")
	c := cmd.Build(src, dst, tags)

	output, err := c.CombinedOutput()
	if err != nil {
		log.Fatal("build failed: %v\n%s", err, output)
	}
	log.Info("binary ready at %s", dst)
}

// Serve runs the generated project from dst and streams its output back to
// the user. Blocks until SIGINT/SIGTERM.
func Serve(dst, tags string) {
	log := logger.Scope("serve")
	c := cmd.Run(dst, tags)

	stdoutW := logger.NewPrefixWriter(os.Stdout, "app", IsTTY(os.Stdout))
	stderrW := logger.NewPrefixWriter(os.Stderr, "app", IsTTY(os.Stderr))
	c.Stdout = stdoutW
	c.Stderr = stderrW

	if err := c.Start(); err != nil {
		log.Fatal("start server: %v", err)
	}
	log.Info("started (pid %d)", c.Process.Pid)

	WaitForSignalAndKill(c)
	stdoutW.Flush()
	stderrW.Flush()
	log.Info("stopped")
}

// Watch regenerates, rebuilds and (re)starts the server whenever a .go file
// under src changes. Blocks until SIGINT/SIGTERM.
func Watch(src, dst, tags, swaggerPath string) {
	log := logger.Scope("watch")
	var (
		mu      sync.Mutex
		current *exec.Cmd
	)

	restart := func() {
		mu.Lock()
		defer mu.Unlock()

		StopProcess(current)
		current = nil

		log.Info("regenerating...")
		if err := safeBootstrap(src, dst, swaggerPath); err != nil {
			log.Error("regen failed: %v", err)
			return
		}
		if err := safeBuild(dst, dst, tags); err != nil {
			log.Error("build failed: %v", err)
			return
		}

		c := cmd.Run(dst, tags)
		c.Stdout = logger.NewPrefixWriter(os.Stdout, "app", IsTTY(os.Stdout))
		c.Stderr = logger.NewPrefixWriter(os.Stderr, "app", IsTTY(os.Stderr))
		if err := c.Start(); err != nil {
			log.Error("start failed: %v", err)
			return
		}
		log.Info("started (pid %d)", c.Process.Pid)
		current = c
	}

	restart()

	w, err := watcher.New(os.Stderr)
	if err != nil {
		log.Fatal("create watcher: %v", err)
	}

	go func() {
		if err := w.Watch(src, restart); err != nil {
			log.Error("%v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Info("shutting down...")
	w.Stop()
	mu.Lock()
	StopProcess(current)
	mu.Unlock()
}

func generate(project *models.Project, dst, swaggerPath string) error {
	cfg := generator.Config{Addr: ":8080"}
	if swaggerPath != "" {
		cfg.Swagger = &generator.SwaggerConfig{Path: swaggerPath}
	}
	return generator.NewGenerator(project, dst, cfg).Generate()
}

func safeBootstrap(src, dst, swaggerPath string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	project, err := parser.NewParser(src).Parse()
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	return generate(project, dst, swaggerPath)
}

func safeBuild(src, dst, tags string) error {
	c := cmd.Build(src, dst, tags)
	output, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, output)
	}
	return nil
}
