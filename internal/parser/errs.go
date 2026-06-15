package parser

import (
	"errors"
	"fmt"
)

// ErrModulePathNotFound is returned when the module path cannot be found in go.mod.
var ErrModulePathNotFound = errors.New("module path not found in go.mod")

// AbsolutePathResolutionError represents an error that occurs when resolving an absolute path.
type AbsolutePathResolutionError struct {
	Path string
	Err  error
}

// Error as string.
func (e *AbsolutePathResolutionError) Error() string {
	return fmt.Sprintf("failed to resolve absolute path for %q: %v", e.Path, e.Err)
}

// Unwrap returns the underlying error.
func (e *AbsolutePathResolutionError) Unwrap() error { return e.Err }

// ReadGoModError wraps errors reading go.mod.
type ReadGoModError struct {
	Dir string
	Err error
}

// Error as string.
func (e *ReadGoModError) Error() string { return fmt.Sprintf("read go.mod in %q: %v", e.Dir, e.Err) }

// Unwrap returns the underlying error.
func (e *ReadGoModError) Unwrap() error { return e.Err }

// ParsingFileError wraps errors that occur while parsing a Go source file.
type ParsingFileError struct {
	FilePath string
	Err      error
}

// Error as string.
func (e *ParsingFileError) Error() string {
	return fmt.Sprintf("parse %q: %v", e.FilePath, e.Err)
}

// Unwrap returns the underlying error.
func (e *ParsingFileError) Unwrap() error { return e.Err }

// InvalidLifecycleFuncError represents an error when a lifecycle function is invalid.
type InvalidLifecycleFuncError struct {
	FilePath string
	FuncName string
	Reason   string
}

// Error returns a string representation of the error.
func (e *InvalidLifecycleFuncError) Error() string {
	return fmt.Sprintf("invalid lifecycle function %s in file %s: %s", e.FuncName, e.FilePath, e.Reason)
}

// InvalidMiddlewareFuncError represents an error when a middleware function is invalid.
type InvalidMiddlewareFuncError struct {
	FilePath string
	FuncName string
	Reason   string
}

// Error returns a string representation of the error.
func (e *InvalidMiddlewareFuncError) Error() string {
	return fmt.Sprintf("invalid middleware function %s in file %s: %s", e.FuncName, e.FilePath, e.Reason)
}

// Handler-related sentinel errors.
var (
	ErrExpectedAtLeast2Params    = errors.New("expected at least 2 params")
	ErrFirstParamMustBeRequest   = errors.New("first param must be runtime.Request")
	ErrSecondParamMustBeResponse = errors.New("second param must be runtime.Response")
)

// InvalidServiceParamError represents an error when a service parameter is invalid in a handler function.
type InvalidServiceParamError struct {
	FilePath  string
	FuncName  string
	ParamType string
	Reason    string
}

// Error returns a string representation of the error.
func (e *InvalidServiceParamError) Error() string {
	return fmt.Sprintf("invalid service parameter %q in handler %s in file %s: %s", e.ParamType, e.FuncName, e.FilePath, e.Reason)
}

// InvalidHandlerStructError represents an error when a struct-based handler group is invalid.
type InvalidHandlerStructError struct {
	FilePath string
	TypeName string
	FuncName string
	Reason   string
}

// Error returns a string representation of the error.
func (e *InvalidHandlerStructError) Error() string {
	if e.FuncName != "" {
		return fmt.Sprintf("invalid handler struct %s.%s in file %s: %s", e.TypeName, e.FuncName, e.FilePath, e.Reason)
	}
	return fmt.Sprintf("invalid handler struct %s: %s", e.TypeName, e.Reason)
}
