package protogen

import "fmt"

// CompileError wraps failures from compiling a .proto spec.
type CompileError struct {
	SpecPath string
	Err      error
}

func (e *CompileError) Error() string {
	return fmt.Sprintf("compile %s: %v", e.SpecPath, e.Err)
}

func (e *CompileError) Unwrap() error { return e.Err }

// ModuleNotFoundError indicates outputDir has no go.mod to read a module
// path from. `snapi proto` writes into an existing Go module; it does not
// scaffold one.
type ModuleNotFoundError struct {
	OutputDir string
}

func (e *ModuleNotFoundError) Error() string {
	return fmt.Sprintf("no go.mod found in %s (run `go mod init` first)", e.OutputDir)
}

// RenderError wraps failures rendering a generated Go file.
type RenderError struct {
	File string
	Err  error
}

func (e *RenderError) Error() string {
	return fmt.Sprintf("render %s: %v", e.File, e.Err)
}

func (e *RenderError) Unwrap() error { return e.Err }
