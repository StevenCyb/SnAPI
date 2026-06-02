package generator

import "fmt"

// ModuleGenerationError wraps errors that occur while generating the go.mod file.
type ModuleGenerationError struct {
	Reason string
	Err    error
}

// Error returns a string representation of the error.
func (e *ModuleGenerationError) Error() string {
	return fmt.Sprintf("module generation: %s: %v", e.Reason, e.Err)
}

// Unwrap returns the underlying error.
func (e *ModuleGenerationError) Unwrap() error { return e.Err }

// ServeGenerationError wraps errors that occur while generating main.go.
type ServeGenerationError struct {
	Reason string
	Err    error
}

func (e *ServeGenerationError) Error() string {
	return fmt.Sprintf("serve generation: %s: %v", e.Reason, e.Err)
}
func (e *ServeGenerationError) Unwrap() error { return e.Err }

// RoutesGenerationError wraps errors that occur while generating routes.go.
type RoutesGenerationError struct {
	Reason string
	Err    error
}

func (e *RoutesGenerationError) Error() string {
	return fmt.Sprintf("routes generation: %s: %v", e.Reason, e.Err)
}
func (e *RoutesGenerationError) Unwrap() error { return e.Err }

// SwaggerGenerationError wraps errors that occur while generating swagger.go.
type SwaggerGenerationError struct {
	Reason string
	Err    error
}

func (e *SwaggerGenerationError) Error() string {
	return fmt.Sprintf("swagger generation: %s: %v", e.Reason, e.Err)
}
func (e *SwaggerGenerationError) Unwrap() error { return e.Err }

// MiddlewareNotFoundError indicates a handler referenced a middleware that wasn't discovered.
type MiddlewareNotFoundError struct {
	Handler string
	Name    string
}

func (e *MiddlewareNotFoundError) Error() string {
	return fmt.Sprintf("handler %q references unknown middleware %q", e.Handler, e.Name)
}
