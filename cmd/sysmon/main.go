// Command sysmon keeps macOS "active" by posting a real mouse-move event on a
// fixed interval, which resets the system idle timer so the screen does not
// sleep. The cursor is nudged one pixel and back, so it stays effectively in
// place.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sysmon/internal/activity/adapter"
	"sysmon/internal/activity/domain"
	"sysmon/internal/activity/service"
)

const (
	// defaultInterval is how often the cursor is nudged by default.
	defaultInterval = 10 * time.Second
	// nudgeOffset is the pixel shift applied then reversed on each nudge.
	nudgeOffset = 1
)

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
	interval := flag.Duration("interval", defaultInterval, "how often to nudge the cursor (e.g. 10s, 30s, 1m)")
	quiet := flag.Bool("quiet", false, "do not write logs to stdout")
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
	offset, err := domain.NewOffset(nudgeOffset)
	if err != nil {
		logger.Error("invalid offset", "err", err, "value", nudgeOffset)
		return exitUsage
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pointer := adapter.NewCGPointer()
	keeper := service.NewKeeperService(pointer, logger, validInterval, offset)

	if err := keeper.Run(ctx); err != nil {
		logger.Error("sysmon exited with error", "err", err)
		return exitFailure
	}

	return exitOK
}
