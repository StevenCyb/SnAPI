package watcher

// PathResolverError is an error that occurs when resolving a
// path to an absolute path.
type PathResolverError struct {
	Path string
	Err  error
}

// Error as string.
func (e *PathResolverError) Error() string { return "resolve path " + e.Path + ": " + e.Err.Error() }

// Unwrap returns the underlying error.
func (e *PathResolverError) Unwrap() error { return e.Err }

// WatcherError is an error that occurs when adding a path
// to the watcher.
type WatcherError struct {
	Path string
	Err  error
}

// Error as string.
func (e *WatcherError) Error() string { return "watch path " + e.Path + ": " + e.Err.Error() }

// Unwrap returns the underlying error.
func (e *WatcherError) Unwrap() error { return e.Err }
