// Package service contains the KeeperService use case: the ticker loop that
// drives Pointer.Nudge on a fixed interval. It depends only on domain and
// port, never on adapter.
package service

import (
	"context"
	"log/slog"
	"time"

	"sysmon/internal/activity/port"
)

// KeeperService implements port.Keeper: it drives a ticker loop that nudges
// the pointer there and back on each tick, and stops cleanly when ctx is
// cancelled.
type KeeperService struct {
	pointer  port.Pointer
	logger   *slog.Logger
	interval time.Duration
	offset   int
	pause    time.Duration
}

// NewKeeperService constructs a KeeperService. pointer and logger are
// injected dependencies (never package globals), which is what makes this
// service unit-testable without a real mouse or macOS Accessibility.
func NewKeeperService(pointer port.Pointer, logger *slog.Logger, interval time.Duration, offset int, pause time.Duration) *KeeperService {
	return &KeeperService{
		pointer:  pointer,
		logger:   logger,
		interval: interval,
		offset:   offset,
		pause:    pause,
	}
}

// Run starts the nudge ticker loop and blocks until ctx is done. On each
// tick it nudges the pointer forward by offset, pauses, then nudges it back
// by the same offset (there-and-back), so the cursor stays effectively in
// place while a real move event is posted. Run returns nil on clean
// cancellation.
func (k *KeeperService) Run(ctx context.Context) error {
	ticker := time.NewTicker(k.interval)
	defer ticker.Stop()

	k.logger.Info("sysmon started", "interval", k.interval)

	for {
		select {
		case <-ticker.C:
			if err := k.pointer.Nudge(k.offset, 0); err != nil {
				k.logger.Error("nudge failed", "err", err)
				continue
			}
			time.Sleep(k.pause)
			if err := k.pointer.Nudge(-k.offset, 0); err != nil {
				k.logger.Error("nudge back failed", "err", err)
			}
		case <-ctx.Done():
			k.logger.Info("sysmon stopped")
			return nil
		}
	}
}
