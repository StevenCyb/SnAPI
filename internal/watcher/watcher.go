package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/StevenCyb/SnAPI/internal/logger"
	"github.com/fsnotify/fsnotify"
)

const debounceDelay = 500 * time.Millisecond

// FSWatcher is an interface for mocking file system watcher behavior.
type FSWatcher interface {
	Add(name string) error
	Close() error
	GetEvents() <-chan fsnotify.Event
	GetErrors() <-chan error
}

// fsnotifyAdapter adapts fsnotify.Watcher to the FSWatcher interface.
type fsnotifyAdapter struct {
	w *fsnotify.Watcher
}

func (a *fsnotifyAdapter) Add(name string) error {
	return a.w.Add(name)
}

func (a *fsnotifyAdapter) Close() error {
	return a.w.Close()
}

func (a *fsnotifyAdapter) GetEvents() <-chan fsnotify.Event {
	return a.w.Events
}

func (a *fsnotifyAdapter) GetErrors() <-chan error {
	return a.w.Errors
}

// Watcher watches a directory tree and calls onChange after debouncing
// file-system events on .go files.
type Watcher struct {
	fsw    FSWatcher
	log    *logger.Logger
	stopCh chan struct{}
}

// NewWithFactory creates a Watcher using the given logger.
// It accepts a factory function for creating the FSWatcher (mainly for testing).
func NewWithFactory(log *logger.Logger, factory func() (FSWatcher, error)) (*Watcher, error) {
	fsw, err := factory()
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}

	return &Watcher{fsw: fsw, log: log, stopCh: make(chan struct{})}, nil
}

// New creates a Watcher using the given logger.
func New(log *logger.Logger) (*Watcher, error) {
	return NewWithFactory(log, func() (FSWatcher, error) {
		fsw, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, err
		}
		return &fsnotifyAdapter{w: fsw}, nil
	})
}

// Watch starts watching dir recursively and calls onChange after each debounce
// window. Watch blocks until Stop is called or a fatal watcher error occurs.
func (w *Watcher) Watch(dir string, onChange func()) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return &PathResolverError{Path: dir, Err: err}
	}

	if err = w.addRecursive(absDir); err != nil {
		return err
	}

	w.loop(onChange)

	return nil
}

// Stop signals the watcher to shut down.
func (w *Watcher) Stop() {
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
	}
	_ = w.fsw.Close()
}

// addRecursive walks dir and adds every non-ignored subdirectory to the watcher.
func (w *Watcher) addRecursive(dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		if w.shouldIgnoreDir(d.Name()) {
			return filepath.SkipDir
		}

		if watchErr := w.fsw.Add(path); watchErr != nil {
			return &WatcherError{Path: path, Err: watchErr}
		}

		return nil
	})
}

// shouldIgnoreDir returns true if the directory should be skipped entirely.
func (*Watcher) shouldIgnoreDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor":
		return true
	default:
		return strings.HasPrefix(name, "snapi-build-")
	}
}

// loop is the main event-processing loop with debouncing.
func (w *Watcher) loop(onChange func()) {
	var timer *time.Timer

	for {
		select {
		case <-w.stopCh:
			return
		case event, ok := <-w.fsw.GetEvents():
			if !ok {
				return
			}
			if !strings.HasSuffix(event.Name, ".go") || strings.HasSuffix(event.Name, "_test.go") {
				continue
			}
			w.log.Info("%s changed", filepath.Base(event.Name))
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounceDelay, onChange)
		case err, ok := <-w.fsw.GetErrors():
			if !ok {
				return
			}
			w.log.Error("%v", err)
		}
	}
}
