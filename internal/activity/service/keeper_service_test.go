package service_test

import (
	"context"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"sysmon/internal/activity/port/mock"
	"sysmon/internal/activity/service"
)

func TestKeeperService_NudgesPerTick(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		const (
			interval = 10 * time.Second
			offset   = 1
			pause    = 40 * time.Millisecond
			ticks    = 3
		)

		gomock.InOrder(
			pointer.EXPECT().Nudge(offset, 0).Return(nil),
			pointer.EXPECT().Nudge(-offset, 0).Return(nil),
			pointer.EXPECT().Nudge(offset, 0).Return(nil),
			pointer.EXPECT().Nudge(-offset, 0).Return(nil),
			pointer.EXPECT().Nudge(offset, 0).Return(nil),
			pointer.EXPECT().Nudge(-offset, 0).Return(nil),
		)

		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(pointer, logger, interval, offset, pause)

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

func TestKeeperService_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(pointer, logger, time.Second, 1, time.Millisecond)

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		synctest.Wait()
		cancel()
		synctest.Wait()

		require.NoError(t, <-done)
	})
}
