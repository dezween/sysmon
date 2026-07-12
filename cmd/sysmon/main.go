// Command sysmon keeps macOS "active" by posting a real mouse-move event on a
// fixed interval, which resets the system idle timer so the screen does not
// sleep. On each interval the cursor glides to a fresh random on-screen
// point over several small steps, so the motion looks like a person actually
// moving the mouse.
package main

import (
	"context"
	"flag"
	"log/slog"
	"math/rand/v2"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sysmon/internal/activity/adapter"
	"sysmon/internal/activity/domain"
	"sysmon/internal/activity/service"
)

// defaultInterval is how often the cursor roams by default.
const defaultInterval = 10 * time.Second

// Process exit codes.
const (
	exitOK      = 0
	exitFailure = 1
	exitUsage   = 2
)

func main() {
	os.Exit(run())
}

// run wires the dependencies and drives the keeper to completion, returning
// the process exit code. It is separated from main so that deferred cleanup
// (stopping the signal-notify context) always runs before the process exits.
func run() int {
	interval := flag.Duration("interval", defaultInterval, "how often to roam the cursor to a new random point (e.g. 10s, 30s, 1m)")
	quiet := flag.Bool("quiet", false, "suppress routine logs; still print warnings and errors")
	flag.Parse()

	level := slog.LevelInfo
	if *quiet {
		level = slog.LevelWarn
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	// Validate the settings through the domain before starting, so a bad
	// -interval (e.g. zero or negative) fails fast with a clear message
	// instead of panicking inside time.NewTicker.
	validInterval, err := domain.NewInterval(*interval)
	if err != nil {
		logger.Error("invalid interval", "err", err, "value", interval.String())
		return exitUsage
	}

	// Preflight: without Accessibility permission macOS silently drops the
	// synthetic mouse events, so warn once at startup rather than looking like
	// a no-op. This is logged at Warn so it shows even under -quiet.
	if !adapter.AccessibilityTrusted() {
		logger.Warn("accessibility permission not granted; mouse events will be ignored until you enable sysmon's terminal in System Settings -> Privacy & Security -> Accessibility")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pointer := adapter.NewCGPointer()
	// rand.NewPCG is seeded from the package-level generator, which
	// math/rand/v2 auto-seeds from a nondeterministic runtime source. This
	// gives the planner real runtime randomness while staying stdlib-only.
	// Cursor-roaming targets are not security-sensitive, so a
	// non-cryptographic PRNG is the right, stdlib-only tool here.
	rnd := rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())) //nolint:gosec // roaming targets are not security-sensitive
	planner := domain.NewPlanner(rnd)
	keeper := service.NewKeeperService(pointer, logger, validInterval, planner)

	if err := keeper.Run(ctx); err != nil {
		logger.Error("sysmon exited with error", "err", err)
		return exitFailure
	}

	return exitOK
}
