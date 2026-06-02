package runner

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

const stopGracePeriod = 3 * time.Second

// WaitForSignalAndKill blocks until SIGINT/SIGTERM, then stops the process.
func WaitForSignalAndKill(c *exec.Cmd) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	StopProcess(c)
}

// StopProcess sends SIGTERM to the process group, waits up to stopGracePeriod
// for it to exit, then escalates to SIGKILL. It blocks until the process is
// reaped so the listening port is fully released before returning.
func StopProcess(c *exec.Cmd) {
	if c == nil || c.Process == nil {
		return
	}

	pid := strconv.Itoa(c.Process.Pid)
	ctx := context.Background()
	_ = exec.CommandContext(ctx, "pkill", "-TERM", "-P", pid).Run() //nolint:gosec // pid is controlled and not user input
	_ = c.Process.Signal(syscall.SIGTERM)

	done := make(chan struct{})
	go func() {
		_ = c.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(stopGracePeriod):
		_ = exec.CommandContext(ctx, "pkill", "-KILL", "-P", pid).Run() //nolint:gosec // pid is controlled and not user input
		_ = c.Process.Kill()
		<-done
	}
}

// IsTTY reports whether f is attached to a terminal and color output is
// allowed (i.e. NO_COLOR is unset).
func IsTTY(f *os.File) bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
