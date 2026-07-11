// Package service contains the KeeperService use case: the ticker loop that
// drives Pointer.Nudge on a fixed interval. It depends only on domain and
// port, never on adapter.
package service

import (
	"context"
	"log/slog"
	"time"

	"sysmon/internal/activity/domain"
	"sysmon/internal/activity/port"
)

// nudgePause is the delay between the forward and backward nudge of a single
// cycle. It is an implementation detail of the there-and-back move, not a
// user-facing setting, so it is a package constant rather than a parameter.
const nudgePause = 40 * time.Millisecond

// KeeperService implements port.Keeper: it drives a ticker loop that nudges
// the pointer there and back on each tick, and stops cleanly when ctx is
// cancelled.
type KeeperService struct {
	pointer  port.Pointer
	logger   *slog.Logger
	interval domain.Interval
	offset   domain.Offset
}

// NewKeeperService constructs a KeeperService. pointer and logger are injected
// dependencies (never package globals), which is what makes this service
// unit-testable without a real mouse or macOS Accessibility. interval and
// offset are validated domain value objects, so the ticker can never receive a
// non-positive duration and the two settings cannot be swapped by accident.
func NewKeeperService(pointer port.Pointer, logger *slog.Logger, interval domain.Interval, offset domain.Offset) *KeeperService {
	return &KeeperService{
		pointer:  pointer,
		logger:   logger,
		interval: interval,
		offset:   offset,
	}
}

// Run starts the nudge ticker loop and blocks until ctx is done. On each tick
// it nudges the pointer forward by offset, pauses, then nudges it back by the
// same offset (there-and-back), so the cursor stays effectively in place while
// a real move event is posted. Run returns nil on clean cancellation.
func (k *KeeperService) Run(ctx context.Context) error {
	ticker := time.NewTicker(k.interval.Duration())
	defer ticker.Stop()

	offset := k.offset.Int()
	k.logger.Info("sysmon started", "interval", k.interval.Duration())

	for {
		select {
		case <-ticker.C:
			k.nudgeCycle(ctx, offset)
			// nudgeCycle has already run its compensating back-nudge (even if
			// ctx was cancelled during the pause). If ctx is now done, stop
			// here instead of waiting for the next tick.
			if ctx.Err() != nil {
				k.logger.Info("sysmon stopped")
				return nil
			}
		case <-ctx.Done():
			k.logger.Info("sysmon stopped")
			return nil
		}
	}
}

// nudgeCycle performs one there-and-back nudge. The pause between the two
// nudges is interruptible: if ctx is cancelled during it, the pause ends early
// but the compensating back-nudge STILL runs, so the cursor returns to its
// original position before shutdown. The only path that skips the back-nudge
// is a failed forward nudge (the cursor never moved).
func (k *KeeperService) nudgeCycle(ctx context.Context, offset int) {
	if err := k.pointer.Nudge(offset, 0); err != nil {
		// The forward nudge failed, so the cursor never moved -- do NOT send
		// the compensating back-nudge, which would leave it offset the other
		// way. Skipping it keeps the cursor where it was.
		k.logger.Error("nudge failed", "err", err)
		return
	}

	select {
	case <-time.After(nudgePause):
	case <-ctx.Done():
	}

	if err := k.pointer.Nudge(-offset, 0); err != nil {
		k.logger.Error("nudge back failed", "err", err)
	}
}
