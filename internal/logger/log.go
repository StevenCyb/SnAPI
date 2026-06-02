package logger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Level represents the severity of a log message.
type Level int

const (
	// LevelDebug shows every message, including verbose internal traces.
	LevelDebug Level = iota
	// LevelInfo is the default level and shows operational messages.
	LevelInfo
	// LevelWarn shows recoverable problems.
	LevelWarn
	// LevelError shows errors that typically stop the current operation.
	LevelError
)

// ParseLevel converts a string to a Level. Unknown values fall back to Info.
func ParseLevel(s string) (Level, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return LevelInfo, true
	case "debug", "trace", "verbose":
		return LevelDebug, true
	case "warn", "warning":
		return LevelWarn, true
	case "error", "err", "fatal":
		return LevelError, true
	}
	return LevelInfo, false
}

// ANSI escape codes used for colorizing output.
const (
	ansiReset  = "\033[0m"
	ansiDim    = "\033[2m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiBlue   = "\033[34m"
	ansiMag    = "\033[35m"
	ansiCyan   = "\033[36m"
	ansiGray   = "\033[90m"
)

// Logger writes leveled, optionally colored messages with an optional scope tag.
type Logger struct {
	mu     sync.Mutex
	out    io.Writer
	err    io.Writer
	level  Level
	color  bool
	scope  string
	scopeC string // cached colorized scope tag
}

// Default is the package-level logger used by the convenience functions.
var Default = New(os.Stdout, os.Stderr)

// New creates a Logger with auto-detected color support.
func New(out, errOut io.Writer) *Logger {
	l := &Logger{
		out:   out,
		err:   errOut,
		level: LevelInfo,
		color: shouldUseColor(out),
	}
	return l
}

// SetLevel changes the minimum level that will be emitted.
func (l *Logger) SetLevel(lv Level) { l.mu.Lock(); l.level = lv; l.mu.Unlock() }

// SetColor forces color output on or off.
func (l *Logger) SetColor(on bool) {
	l.mu.Lock()
	l.color = on
	l.scopeC = l.renderScope()
	l.mu.Unlock()
}

// Level returns the current minimum log level.
func (l *Logger) Level() Level { l.mu.Lock(); defer l.mu.Unlock(); return l.level }

// Scope returns a child logger that tags every message with the given name.
func (l *Logger) Scope(name string) *Logger {
	c := &Logger{
		out:   l.out,
		err:   l.err,
		level: l.level,
		color: l.color,
		scope: name,
	}
	c.scopeC = c.renderScope()
	return c
}

func (l *Logger) renderScope() string {
	if l.scope == "" {
		return ""
	}
	if l.color {
		return ansiMag + "[" + l.scope + "]" + ansiReset + " "
	}
	return "[" + l.scope + "] "
}

// Debug logs at debug level.
func (l *Logger) Debug(format string, args ...any) { l.emit(LevelDebug, format, args...) }

// Info logs at info level.
func (l *Logger) Info(format string, args ...any) { l.emit(LevelInfo, format, args...) }

// Warn logs at warn level.
func (l *Logger) Warn(format string, args ...any) { l.emit(LevelWarn, format, args...) }

// Error logs at error level.
func (l *Logger) Error(format string, args ...any) { l.emit(LevelError, format, args...) }

// Fatal logs at error level, runs all registered fatal hooks (LIFO),
// then exits with status 1.
func (l *Logger) Fatal(format string, args ...any) {
	l.emit(LevelError, format, args...)
	runFatalHooks()
	os.Exit(1)
}

//nolint:gochecknoglobals // process-wide cleanup registry
var (
	fatalMu    sync.Mutex
	fatalHooks []*func()
)

// OnFatal registers fn to run before Fatal calls os.Exit. The returned
// cancel function removes the hook; callers should defer it when the
// scope that owns the cleanup is about to return normally.
func OnFatal(fn func()) (cancel func()) {
	fatalMu.Lock()
	defer fatalMu.Unlock()
	h := &fn
	fatalHooks = append(fatalHooks, h)
	return func() {
		fatalMu.Lock()
		defer fatalMu.Unlock()
		for i, e := range fatalHooks {
			if e == h {
				fatalHooks = append(fatalHooks[:i], fatalHooks[i+1:]...)
				return
			}
		}
	}
}

func runFatalHooks() {
	fatalMu.Lock()
	hooks := fatalHooks
	fatalHooks = nil
	fatalMu.Unlock()
	for i := len(hooks) - 1; i >= 0; i-- {
		func() {
			defer func() { _ = recover() }()
			(*hooks[i])()
		}()
	}
}

func (l *Logger) emit(lv Level, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if lv < l.level {
		return
	}

	w := l.out
	if lv >= LevelWarn {
		w = l.err
	}

	ts := time.Now().Format("15:04:05")
	tag, color := levelTag(lv)
	msg := fmt.Sprintf(format, args...)
	msg = strings.TrimRight(msg, "\n")

	if l.color {
		fmt.Fprintf(w, "%s%s%s %s%-5s%s %s%s\n",
			ansiGray, ts, ansiReset,
			color, tag, ansiReset,
			l.scopeC, msg,
		)
	} else {
		fmt.Fprintf(w, "%s %-5s %s%s\n", ts, tag, l.scopeC, msg)
	}
}

func levelTag(lv Level) (string, string) {
	switch lv {
	case LevelDebug:
		return "DEBUG", ansiGray
	case LevelInfo:
		return "INFO", ansiCyan
	case LevelWarn:
		return "WARN", ansiYellow
	case LevelError:
		return "ERROR", ansiRed
	}
	return "INFO", ansiCyan
}

// shouldUseColor returns true if w is a TTY and NO_COLOR is not set.
func shouldUseColor(w io.Writer) bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Convenience wrappers that forward to Default.

func SetLevel(lv Level)         { Default.SetLevel(lv) }
func SetColor(on bool)          { Default.SetColor(on) }
func Scope(name string) *Logger { return Default.Scope(name) }
func Debug(f string, a ...any)  { Default.Debug(f, a...) }
func Info(f string, a ...any)   { Default.Info(f, a...) }
func Warn(f string, a ...any)   { Default.Warn(f, a...) }
func Error(f string, a ...any)  { Default.Error(f, a...) }
func Fatal(f string, a ...any)  { Default.Fatal(f, a...) }

// PrefixWriter wraps an io.Writer and prepends a colored prefix to every line.
// It is line-buffered: partial lines are held until a newline arrives.
type PrefixWriter struct {
	mu     sync.Mutex
	out    io.Writer
	prefix string
	color  bool
	buf    bytes.Buffer
}

// NewPrefixWriter returns a writer that emits each line of input to out with
// the given tag, colored to differentiate it from CLI logs. Each line is
// prefixed with a fresh timestamp so child-process output aligns with the
// CLI's own log lines.
func NewPrefixWriter(out io.Writer, tag string, color bool) *PrefixWriter {
	prefix := "│ " + tag + " │ "
	if color {
		prefix = ansiBlue + "│" + ansiReset + " " +
			ansiBold + ansiGreen + tag + ansiReset + " " +
			ansiBlue + "│" + ansiReset + " "
	}
	return &PrefixWriter{out: out, prefix: prefix, color: color}
}

// stdlibLogPrefix matches the default `log` package prefix `YYYY/MM/DD HH:MM:SS `
// so we can strip it before re-prefixing, avoiding double timestamps.
var stdlibLogPrefix = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)? `)

func (p *PrefixWriter) writeLine(line []byte) error {
	clean := stdlibLogPrefix.ReplaceAll(line, nil)
	ts := time.Now().Format("15:04:05")
	if p.color {
		_, err := fmt.Fprintf(p.out, "%s%s%s %s%s\n", ansiGray, ts, ansiReset, p.prefix, clean)
		return err
	}
	_, err := fmt.Fprintf(p.out, "%s %s%s\n", ts, p.prefix, clean)
	return err
}

// Write implements io.Writer.
func (p *PrefixWriter) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	n := len(b)
	p.buf.Write(b)

	for {
		data := p.buf.Bytes()
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			break
		}
		line := data[:idx]
		if err := p.writeLine(line); err != nil {
			return n, err
		}
		p.buf.Next(idx + 1)
	}

	return n, nil
}

// Flush writes any buffered partial line to the underlying writer.
func (p *PrefixWriter) Flush() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.buf.Len() == 0 {
		return
	}
	_ = p.writeLine(p.buf.Bytes())
	p.buf.Reset()
}
