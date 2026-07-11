package service_test

import (
	"context"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"sysmon/internal/activity/domain"
	"sysmon/internal/activity/port/mock"
	"sysmon/internal/activity/service"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// mustInterval builds a validated domain.Interval, failing the test on error.
func mustInterval(t *testing.T, d time.Duration) domain.Interval {
	t.Helper()
	iv, err := domain.NewInterval(d)
	require.NoError(t, err)
	return iv
}

// mustOffset builds a validated domain.Offset, failing the test on error.
func mustOffset(t *testing.T, px int) domain.Offset {
	t.Helper()
	off, err := domain.NewOffset(px)
	require.NoError(t, err)
	return off
}

func TestKeeperService_NudgesPerTick(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		const (
			interval = 10 * time.Second
			ticks    = 3
		)

		gomock.InOrder(
			pointer.EXPECT().Nudge(1, 0).Return(nil),
			pointer.EXPECT().Nudge(-1, 0).Return(nil),
			pointer.EXPECT().Nudge(1, 0).Return(nil),
			pointer.EXPECT().Nudge(-1, 0).Return(nil),
			pointer.EXPECT().Nudge(1, 0).Return(nil),
			pointer.EXPECT().Nudge(-1, 0).Return(nil),
		)

		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, interval), mustOffset(t, 1))

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		time.Sleep(ticks * interval)
		synctest.Wait()

		cancel()
		synctest.Wait()

		require.NoError(t, <-done)
	})
}

// TestKeeperService_CompensatesOnCancelDuringPause exercises the exact path
// added by review #1: cancellation landing strictly inside the inter-nudge
// pause. The forward nudge triggers cancel(), so the pause select takes the
// ctx.Done arm, and the compensating back-nudge must still fire before Run
// returns.
func TestKeeperService_CompensatesOnCancelDuringPause(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		ctx, cancel := context.WithCancel(t.Context())

		gomock.InOrder(
			pointer.EXPECT().Nudge(1, 0).DoAndReturn(func(_, _ int) error {
				cancel() // cancel while we are about to enter the pause
				return nil
			}),
			pointer.EXPECT().Nudge(-1, 0).Return(nil), // must still compensate
		)

		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, time.Second), mustOffset(t, 1))

		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		time.Sleep(time.Second) // let exactly one tick fire
		synctest.Wait()

		require.NoError(t, <-done)
	})
}

func TestKeeperService_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, time.Second), mustOffset(t, 1))

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		synctest.Wait()
		cancel()
		synctest.Wait()

		require.NoError(t, <-done)
	})
}
