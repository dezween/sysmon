// Package service contains the KeeperService use case: the ticker loop that
// drives a roam (a planned glide path) through Pointer on a fixed interval.
// It depends only on domain and port, never on adapter.
package service

import (
	"context"
	"log/slog"
	"time"

	"sysmon/internal/activity/domain"
	"sysmon/internal/activity/port"
)

// stepPause is the delay between successive MoveTo calls within a single
// roam's glide path. It is an implementation detail of what makes the
// motion look human (a brief burst of small moves), not a user-facing
// setting, so it is a package constant rather than a parameter.
const stepPause = 8 * time.Millisecond

// KeeperService implements port.Keeper: it drives a ticker loop that, on
// each tick, roams the pointer to a fresh random on-screen target via a
// planned glide path, and stops cleanly when ctx is cancelled.
type KeeperService struct {
	pointer  port.Pointer
	logger   *slog.Logger
	interval domain.Interval
	planner  *domain.Planner
}

// NewKeeperService constructs a KeeperService. pointer and logger are
// injected dependencies (never package globals), which is what makes this
// service unit-testable without a real mouse or macOS Accessibility.
// interval is a validated domain value object, so the ticker can never
// receive a non-positive duration. planner is the pure domain component
// that turns a current position and screen bounds into a glide path.
func NewKeeperService(pointer port.Pointer, logger *slog.Logger, interval domain.Interval, planner *domain.Planner) *KeeperService {
	return &KeeperService{
		pointer:  pointer,
		logger:   logger,
		interval: interval,
		planner:  planner,
	}
}

// Run starts the roam ticker loop and blocks until ctx is done. On each tick
// it plans and drives exactly one glide path to a fresh random on-screen
// target. Run returns nil on clean cancellation, including cancellation that
// lands in the middle of a glide.
func (k *KeeperService) Run(ctx context.Context) error {
	ticker := time.NewTicker(k.interval.Duration())
	defer ticker.Stop()

	k.logger.Info("sysmon started", "interval", k.interval.Duration())

	for {
		select {
		case <-ticker.C:
			k.roam(ctx)
			// roam may have returned early because ctx was cancelled
			// mid-glide. If so, stop here instead of waiting for the next
			// tick.
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

// roam performs exactly one roam: read the current position and screen
// bounds, ask the planner for a glide path, then drive the pointer through
// that path in order with a short pause between steps. ctx is checked
// between every step so a cancellation stops the glide promptly, without
// making the remaining MoveTo calls. Errors from Position, Bounds, or
// MoveTo are logged and abandon the current roam without crashing the
// loop.
func (k *KeeperService) roam(ctx context.Context) {
	x, y, err := k.pointer.Position()
	if err != nil {
		k.logger.Error("read position failed", "err", err)
		return
	}

	// Bounds are read on every roam (not cached) so a resolution change or a
	// monitor being plugged/unplugged is picked up without restarting.
	w, h, err := k.pointer.Bounds()
	if err != nil {
		k.logger.Error("read bounds failed", "err", err)
		return
	}

	path, err := k.planner.Plan(x, y, w, h)
	if err != nil {
		k.logger.Error("plan glide path failed", "err", err)
		return
	}

	k.drive(ctx, path)
}

// drive walks path in order, calling MoveTo for each point with a short
// inter-step pause, stopping immediately if ctx is cancelled between steps.
func (k *KeeperService) drive(ctx context.Context, path []domain.Point) {
	// Reuse one timer across the whole glide instead of allocating a fresh
	// one per step (time.After), keeping the burst lightweight.
	timer := time.NewTimer(stepPause)
	defer timer.Stop()

	for _, pt := range path {
		if err := k.pointer.MoveTo(pt.X, pt.Y); err != nil {
			k.logger.Error("move failed", "err", err)
			return
		}

		timer.Reset(stepPause)
		select {
		case <-timer.C:
		case <-ctx.Done():
			return
		}
	}
}
