package watcher

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockFSWatcher is a testify mock of FSWatcher.
type MockFSWatcher struct {
	mock.Mock
	eventsCh chan fsnotify.Event
	errorsCh chan error
}

func (m *MockFSWatcher) Add(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockFSWatcher) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockFSWatcher) GetEvents() <-chan fsnotify.Event {
	return m.eventsCh
}

func (m *MockFSWatcher) GetErrors() <-chan error {
	return m.errorsCh
}

func NewMockFSWatcher() *MockFSWatcher {
	return &MockFSWatcher{
		eventsCh: make(chan fsnotify.Event, 10),
		errorsCh: make(chan error, 10),
	}
}

func TestNewSuccess(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	w, err := New(buf)
	require.NoError(t, err)
	require.NotNil(t, w)
	assert.NotNil(t, w.fsw)
	assert.NotNil(t, w.stopCh)
	assert.Equal(t, buf, w.out)
}

func TestNewOutput(t *testing.T) {
	t.Parallel()
	w, err := New(io.Discard)
	require.NoError(t, err)
	require.NotNil(t, w)
}

func TestStop(t *testing.T) {
	t.Parallel()
	w, err := New(&bytes.Buffer{})
	require.NoError(t, err)

	w.Stop()
	select {
	case <-w.stopCh:
	default:
		t.Fatal("Stop did not close the stop channel")
	}
}

func TestShouldIgnoreDir(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		dirName  string
		expected bool
	}{
		{"git directory", ".git", true},
		{"node_modules directory", "node_modules", true},
		{"vendor directory", "vendor", true},
		{"snapi build directory", "snapi-build-123", true},
		{"snapi build another", "snapi-build-abc", true},
		{"normal directory", "src", false},
		{"another normal", "pkg", false},
		{"internal directory", "internal", false},
	}

	w, err := New(&bytes.Buffer{})
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := w.shouldIgnoreDir(tt.dirName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAddRecursiveSuccess(t *testing.T) {
	t.Parallel()
	mockFSW := NewMockFSWatcher()
	w := &Watcher{
		fsw:    mockFSW,
		out:    &bytes.Buffer{},
		stopCh: make(chan struct{}),
	}

	tmpDir := t.TempDir()
	subDir1 := filepath.Join(tmpDir, "sub1")
	subDir2 := filepath.Join(tmpDir, "sub2")
	ignoredDir := filepath.Join(tmpDir, ".git")

	require.NoError(t, os.Mkdir(subDir1, 0755))
	require.NoError(t, os.Mkdir(subDir2, 0755))
	require.NoError(t, os.Mkdir(ignoredDir, 0755))

	mockFSW.On("Add", tmpDir).Return(nil)
	mockFSW.On("Add", subDir1).Return(nil)
	mockFSW.On("Add", subDir2).Return(nil)
	mockFSW.On("Close").Return(nil)

	err := w.addRecursive(tmpDir)
	require.NoError(t, err)

	mockFSW.AssertCalled(t, "Add", tmpDir)
	mockFSW.AssertCalled(t, "Add", subDir1)
	mockFSW.AssertCalled(t, "Add", subDir2)
	mockFSW.AssertNotCalled(t, "Add", ignoredDir)
}

func TestAddRecursiveError(t *testing.T) {
	t.Parallel()
	mockFSW := NewMockFSWatcher()
	w := &Watcher{
		fsw:    mockFSW,
		out:    &bytes.Buffer{},
		stopCh: make(chan struct{}),
	}

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	require.NoError(t, os.Mkdir(subDir, 0755))

	mockFSW.On("Add", tmpDir).Return(nil)
	mockFSW.On("Add", subDir).Return(filepath.ErrBadPattern)

	err := w.addRecursive(tmpDir)
	require.Error(t, err)
	var watchErr *WatcherError
	require.ErrorAs(t, err, &watchErr)
	assert.Equal(t, subDir, watchErr.Path)
}

func TestWatchSuccess(t *testing.T) {
	t.Parallel()
	mockFSW := NewMockFSWatcher()
	w := &Watcher{
		fsw:    mockFSW,
		out:    &bytes.Buffer{},
		stopCh: make(chan struct{}),
	}

	tmpDir := t.TempDir()
	mockFSW.On("Add", mock.MatchedBy(func(path string) bool {
		return path == tmpDir
	})).Return(nil)
	mockFSW.On("Close").Return(nil)

	done := make(chan error, 1)
	go func() {
		done <- w.Watch(tmpDir, func() {})
	}()

	time.Sleep(100 * time.Millisecond)
	w.Stop()

	err := <-done
	require.NoError(t, err)
	mockFSW.AssertCalled(t, "Add", tmpDir)
}

func TestWatchInvalidPath(t *testing.T) {
	t.Parallel()
	mockFSW := NewMockFSWatcher()
	w := &Watcher{
		fsw:    mockFSW,
		out:    &bytes.Buffer{},
		stopCh: make(chan struct{}),
	}

	invalidPath := "/nonexistent\x00path"

	err := w.Watch(invalidPath, func() {})
	require.Error(t, err)
}

func TestWatchAddError(t *testing.T) {
	t.Parallel()
	mockFSW := NewMockFSWatcher()
	w := &Watcher{
		fsw:    mockFSW,
		out:    &bytes.Buffer{},
		stopCh: make(chan struct{}),
	}

	tmpDir := t.TempDir()
	mockFSW.On("Add", tmpDir).Return(filepath.ErrBadPattern)

	err := w.Watch(tmpDir, func() {})
	require.Error(t, err)
	var watchErr *WatcherError
	assert.ErrorAs(t, err, &watchErr)
}

func TestLoopGoFileChange(t *testing.T) {
	t.Parallel()
	mockFSW := NewMockFSWatcher()
	buf := &bytes.Buffer{}
	w := &Watcher{
		fsw:    mockFSW,
		out:    buf,
		stopCh: make(chan struct{}),
	}

	mockFSW.On("Close").Return(nil)
	callCount := 0
	onChange := func() {
		callCount++
	}

	go w.loop(onChange)

	mockFSW.eventsCh <- fsnotify.Event{
		Name: "/path/to/file.go",
		Op:   fsnotify.Write,
	}

	time.Sleep(debounceDelay + 50*time.Millisecond)
	w.Stop()

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, callCount)
}

func TestLoopIgnoresTestFiles(t *testing.T) {
	t.Parallel()
	mockFSW := NewMockFSWatcher()
	w := &Watcher{
		fsw:    mockFSW,
		out:    &bytes.Buffer{},
		stopCh: make(chan struct{}),
	}

	mockFSW.On("Close").Return(nil)
	callCount := 0
	onChange := func() {
		callCount++
	}

	go w.loop(onChange)

	mockFSW.eventsCh <- fsnotify.Event{
		Name: "/path/to/file_test.go",
		Op:   fsnotify.Write,
	}

	time.Sleep(100 * time.Millisecond)
	w.Stop()

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, callCount)
}

func TestLoopIgnoresNonGoFiles(t *testing.T) {
	t.Parallel()
	mockFSW := NewMockFSWatcher()
	w := &Watcher{
		fsw:    mockFSW,
		out:    &bytes.Buffer{},
		stopCh: make(chan struct{}),
	}

	mockFSW.On("Close").Return(nil)
	callCount := 0
	onChange := func() {
		callCount++
	}

	go w.loop(onChange)

	mockFSW.eventsCh <- fsnotify.Event{
		Name: "/path/to/file.txt",
		Op:   fsnotify.Write,
	}
	mockFSW.eventsCh <- fsnotify.Event{
		Name: "/path/to/file.md",
		Op:   fsnotify.Write,
	}

	time.Sleep(100 * time.Millisecond)
	w.Stop()

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, callCount)
}

func TestLoopDebouncing(t *testing.T) {
	t.Parallel()
	mockFSW := NewMockFSWatcher()
	w := &Watcher{
		fsw:    mockFSW,
		out:    &bytes.Buffer{},
		stopCh: make(chan struct{}),
	}

	mockFSW.On("Close").Return(nil)
	callCount := 0
	onChange := func() {
		callCount++
	}

	go w.loop(onChange)

	for range 5 {
		mockFSW.eventsCh <- fsnotify.Event{
			Name: "/path/to/file.go",
			Op:   fsnotify.Write,
		}
		time.Sleep(50 * time.Millisecond)
	}

	time.Sleep(debounceDelay + 50*time.Millisecond)
	w.Stop()

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, callCount)
}

func TestLoopErrorHandling(t *testing.T) {
	t.Parallel()
	mockFSW := NewMockFSWatcher()
	w := &Watcher{
		fsw:    mockFSW,
		out:    &bytes.Buffer{},
		stopCh: make(chan struct{}),
	}

	mockFSW.On("Close").Return(nil)
	onChange := func() {}

	go w.loop(onChange)

	mockFSW.errorsCh <- filepath.ErrBadPattern

	time.Sleep(100 * time.Millisecond)
	w.Stop()

	time.Sleep(100 * time.Millisecond)
}

func TestLoopStopsOnClosedEventChannel(t *testing.T) {
	t.Parallel()
	mockFSW := NewMockFSWatcher()
	w := &Watcher{
		fsw:    mockFSW,
		out:    &bytes.Buffer{},
		stopCh: make(chan struct{}),
	}

	loopDone := make(chan struct{})
	onChange := func() {}

	go func() {
		w.loop(onChange)
		close(loopDone)
	}()

	close(mockFSW.eventsCh)

	select {
	case <-loopDone:
	case <-time.After(1 * time.Second):
		t.Fatal("loop did not exit when events channel closed")
	}
}

func TestLoopStopsOnClosedErrorChannel(t *testing.T) {
	t.Parallel()
	mockFSW := NewMockFSWatcher()
	w := &Watcher{
		fsw:    mockFSW,
		out:    &bytes.Buffer{},
		stopCh: make(chan struct{}),
	}

	loopDone := make(chan struct{})
	onChange := func() {}

	go func() {
		w.loop(onChange)
		close(loopDone)
	}()

	close(mockFSW.errorsCh)

	select {
	case <-loopDone:
	case <-time.After(1 * time.Second):
		t.Fatal("loop did not exit when errors channel closed")
	}
}

func TestLoopTimerCancellation(t *testing.T) {
	t.Parallel()
	mockFSW := NewMockFSWatcher()
	w := &Watcher{
		fsw:    mockFSW,
		out:    &bytes.Buffer{},
		stopCh: make(chan struct{}),
	}

	mockFSW.On("Close").Return(nil)
	callCount := 0
	onChange := func() {
		callCount++
	}

	go w.loop(onChange)

	mockFSW.eventsCh <- fsnotify.Event{
		Name: "/path/to/file1.go",
		Op:   fsnotify.Write,
	}

	time.Sleep(100 * time.Millisecond)

	mockFSW.eventsCh <- fsnotify.Event{
		Name: "/path/to/file2.go",
		Op:   fsnotify.Write,
	}

	time.Sleep(debounceDelay + 50*time.Millisecond)
	w.Stop()

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, callCount)
}

func TestIntegrationWithRealFSWatcher(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	w, err := New(io.Discard)
	require.NoError(t, err)
	defer w.Stop()

	callCount := 0
	onChange := func() {
		callCount++
	}

	done := make(chan error, 1)
	go func() {
		done <- w.Watch(tmpDir, onChange)
	}()

	time.Sleep(200 * time.Millisecond)

	testFile := filepath.Join(tmpDir, "test.go")
	err = os.WriteFile(testFile, []byte("package main"), 0644)
	require.NoError(t, err)

	// Wait for debounce
	time.Sleep(debounceDelay + 100*time.Millisecond)

	w.Stop()
	<-done

	assert.Positive(t, callCount)
}

func TestNewFsnotifyError(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	failingFactory := func() (FSWatcher, error) {
		return nil, filepath.ErrBadPattern
	}

	w, err := NewWithFactory(buf, failingFactory)
	require.Error(t, err)
	require.Nil(t, w)
	assert.ErrorIs(t, err, filepath.ErrBadPattern)
}

func TestAddRecursiveSkipsFiles(t *testing.T) {
	t.Parallel()
	mockFSW := NewMockFSWatcher()
	w := &Watcher{
		fsw:    mockFSW,
		out:    &bytes.Buffer{},
		stopCh: make(chan struct{}),
	}

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	testFile := filepath.Join(tmpDir, "test.go")

	require.NoError(t, os.Mkdir(subDir, 0755))
	require.NoError(t, os.WriteFile(testFile, []byte("package main"), 0644))

	mockFSW.On("Add", tmpDir).Return(nil)
	mockFSW.On("Add", subDir).Return(nil)
	mockFSW.On("Close").Return(nil)

	err := w.addRecursive(tmpDir)
	require.NoError(t, err)

	mockFSW.AssertCalled(t, "Add", tmpDir)
	mockFSW.AssertCalled(t, "Add", subDir)
	mockFSW.AssertNotCalled(t, "Add", testFile)
}
