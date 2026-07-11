//go:build darwin

package main_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncBuffer is a mutex-guarded byte buffer used to capture a child
// process's combined output while it is still running, so a test can poll
// for a marker line (e.g. "sysmon started") instead of racing a fixed
// sleep against the process's own startup latency.
type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// waitForOutput polls buf until substr appears or timeout elapses, failing
// the test in the latter case. It never sleeps longer than needed once the
// marker appears, so it does not slow down a fast machine or flake on a
// slow one.
func waitForOutput(t *testing.T, buf *syncBuffer, substr string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), substr) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q in output; got:\n%s", substr, buf.String())
}

// buildOnce ensures the sysmon binary is compiled exactly once for the
// whole test binary run, however many tests in this package request it.
// Building it separately per parallel test wastes wall-clock time and, on
// a loaded machine, can push the child process's startup so late that a
// fixed-delay signal in another test fires before the binary has even
// produced its first log line.
var (
	buildOnce   sync.Once
	builtBin    string
	errBinBuild error
)

// binPath returns the path to a compiled sysmon binary shared across all
// tests in this package. Building is skipped entirely (via t.Skip) when
// testing.Short() is set, since compiling a fresh binary is too slow for a
// quick unit-test run.
func binPath(t *testing.T) string {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping cmd/sysmon integration test in -short mode")
	}

	buildOnce.Do(func() {
		_, thisFile, _, ok := runtime.Caller(0)
		if !ok {
			errBinBuild = os.ErrInvalid
			return
		}
		repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

		dir, err := os.MkdirTemp("", "sysmon-bin-*")
		if err != nil {
			errBinBuild = err
			return
		}
		bin := filepath.Join(dir, "sysmon")

		// #nosec G204 -- fixed argv (go build -o <tmp> ./cmd/sysmon) built
		// from this test's own repo root, not from external/user input.
		cmd := exec.CommandContext(context.Background(), "go", "build", "-o", bin, "./cmd/sysmon")
		cmd.Dir = repoRoot
		var out []byte
		out, errBinBuild = cmd.CombinedOutput()
		if errBinBuild != nil {
			errBinBuild = fmt.Errorf("go build failed: %s: %w", out, errBinBuild)
			return
		}
		builtBin = bin
	})

	require.NoError(t, errBinBuild)
	return builtBin
}

// TestMain_InvalidInterval covers both flag-validation failure exit paths:
// a negative interval and a zero interval must both fail fast with exit
// code 2 (exitUsage) and a message identifying the problem.
func TestMain_InvalidInterval(t *testing.T) {
	t.Parallel()

	bin := binPath(t)

	tests := []struct {
		name     string
		arg      string
		wantText string
	}{
		{name: "negative interval", arg: "-interval=-5s", wantText: "invalid interval"},
		{name: "zero interval", arg: "-interval=0s", wantText: "invalid interval"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// #nosec G204 -- bin is our own freshly built test binary path,
			// not external/user input.
			cmd := exec.CommandContext(t.Context(), bin, tt.arg)
			out, err := cmd.CombinedOutput()

			var exitErr *exec.ExitError
			require.ErrorAs(t, err, &exitErr, "expected the process to exit non-zero")
			assert.Equal(t, 2, exitErr.ExitCode())
			assert.Contains(t, string(out), tt.wantText)
		})
	}
}

// TestMain_Lifecycle starts sysmon with a short interval, lets it run for a
// bit, then sends SIGTERM and waits for a clean exit. It asserts exit code 0
// and that both the startup and shutdown log lines were emitted. No fixed
// sleep races the process: shutdown is awaited via the Wait() error channel
// with a generous timeout.
func TestMain_Lifecycle(t *testing.T) {
	t.Parallel()

	bin := binPath(t)

	// #nosec G204 -- bin is our own freshly built test binary path, not
	// external/user input.
	cmd := exec.CommandContext(t.Context(), bin, "-interval=50ms")
	out := &syncBuffer{}
	cmd.Stdout = out
	cmd.Stderr = out

	require.NoError(t, cmd.Start())

	// Wait for the process to actually reach the "started" log line rather
	// than sleeping a fixed duration: on a loaded machine, go build/link
	// contention from sibling parallel tests can push process startup out
	// far enough that a fixed sleep would race it.
	waitForOutput(t, out, "sysmon started", 5*time.Second)

	require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))

	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	select {
	case err := <-waitDone:
		require.NoError(t, err, "process should exit cleanly on SIGTERM; output:\n%s", out.String())
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("process did not exit within timeout after SIGTERM; output:\n%s", out.String())
	}

	output := out.String()
	assert.Contains(t, output, "sysmon started", "expected startup log line, got:\n%s", output)
	assert.Contains(t, output, "sysmon stopped", "expected shutdown log line, got:\n%s", output)
}

// TestMain_ExitCodeZeroOnCleanShutdown is a narrower companion to
// TestMain_Lifecycle: it isolates the exit-code assertion using
// os.Interrupt (SIGINT), which the process also handles via
// signal.NotifyContext. It runs without -quiet (unlike a plain smoke test)
// so waitForOutput has a reliable marker to poll for before signaling.
func TestMain_ExitCodeZeroOnCleanShutdown(t *testing.T) {
	t.Parallel()

	bin := binPath(t)

	// #nosec G204 -- bin is our own freshly built test binary path, not
	// external/user input.
	cmd := exec.CommandContext(t.Context(), bin, "-interval=50ms")
	out := &syncBuffer{}
	cmd.Stdout = out
	cmd.Stderr = out

	require.NoError(t, cmd.Start())

	waitForOutput(t, out, "sysmon started", 5*time.Second)
	require.NoError(t, cmd.Process.Signal(os.Interrupt))

	waitDone := make(chan error, 1)
	go func() { waitDone <- cmd.Wait() }()

	select {
	case err := <-waitDone:
		require.NoError(t, err, "process should exit 0 on SIGINT; output:\n%s", out.String())
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("process did not exit within timeout after SIGINT; output:\n%s", out.String())
	}
}
