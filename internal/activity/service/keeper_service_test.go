package service_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"sysmon/internal/activity/domain"
	"sysmon/internal/activity/port/mock"
	"sysmon/internal/activity/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// errNudge is a local sentinel used to simulate a Pointer failure in these
// tests. It must never be adapter.ErrNudgeFailed: service tests stay
// cgo-free and must not import internal/activity/adapter.
var errNudge = errors.New("nudge sentinel failure")

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

// TestKeeperService_ForwardNudgeFailureSkipsBackNudge exercises nudgeCycle's
// early-return path: when the forward nudge fails, the cursor never moved,
// so the compensating back-nudge (-1, 0) must NOT be sent. The failing call
// also cancels ctx (via DoAndReturn) so Run observes ctx.Err() != nil right
// after nudgeCycle returns and stops without waiting for another tick.
func TestKeeperService_ForwardNudgeFailureSkipsBackNudge(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		ctx, cancel := context.WithCancel(t.Context())

		pointer.EXPECT().Nudge(1, 0).DoAndReturn(func(_, _ int) error {
			cancel()
			return errNudge
		})
		// No expectation for Nudge(-1, 0): gomock's controller will fail the
		// test if it is called, since only the forward nudge was recorded.

		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, time.Second), mustOffset(t, 1))

		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		time.Sleep(time.Second) // let exactly one tick fire
		synctest.Wait()

		require.NoError(t, <-done)
	})
}

// TestKeeperService_BackNudgeFailureIsTolerated asserts that a failed back
// nudge is logged but does not stop the loop: the second tick still runs
// both nudges normally.
func TestKeeperService_BackNudgeFailureIsTolerated(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		ctx, cancel := context.WithCancel(t.Context())

		gomock.InOrder(
			pointer.EXPECT().Nudge(1, 0).Return(nil),
			pointer.EXPECT().Nudge(-1, 0).Return(errNudge),
			pointer.EXPECT().Nudge(1, 0).Return(nil),
			pointer.EXPECT().Nudge(-1, 0).DoAndReturn(func(_, _ int) error {
				cancel()
				return nil
			}),
		)

		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, time.Second), mustOffset(t, 1))

		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		time.Sleep(2 * time.Second) // let two ticks fire
		synctest.Wait()

		require.NoError(t, <-done)
	})
}

// TestKeeperService_CustomOffset asserts that a non-default Offset value is
// forwarded verbatim to Pointer.Nudge in both directions.
func TestKeeperService_CustomOffset(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		ctx, cancel := context.WithCancel(t.Context())

		gomock.InOrder(
			pointer.EXPECT().Nudge(3, 0).Return(nil),
			pointer.EXPECT().Nudge(-3, 0).DoAndReturn(func(_, _ int) error {
				cancel()
				return nil
			}),
		)

		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, time.Second), mustOffset(t, 3))

		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		time.Sleep(time.Second)
		synctest.Wait()

		require.NoError(t, <-done)
	})
}

// capturingHandler is a minimal, race-safe slog.Handler that records every
// Handle call so tests can assert on emitted log records. It is not a
// hand-written fake of a domain dependency (which the test stack forbids) --
// it is a real slog.Handler implementation, the mechanism slog itself
// documents for testing loggers.
type capturingHandler struct {
	mu      *sync.Mutex
	records *[]slog.Record
}

func newCapturingHandler() *capturingHandler {
	return &capturingHandler{mu: &sync.Mutex{}, records: &[]slog.Record{}}
}

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.records = append(*h.records, r)
	return nil
}

func (h *capturingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *capturingHandler) WithGroup(string) slog.Handler { return h }

func (h *capturingHandler) snapshot() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]slog.Record, len(*h.records))
	copy(out, *h.records)
	return out
}

// TestKeeperService_LogsStartAndStop covers LOG-01: Run must log an Info
// "sysmon started" record carrying the configured interval as an attribute,
// then an Info "sysmon stopped" record once ctx is cancelled.
func TestKeeperService_LogsStartAndStop(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		const interval = time.Second

		ctx, cancel := context.WithCancel(t.Context())

		pointer.EXPECT().Nudge(1, 0).Return(nil)
		pointer.EXPECT().Nudge(-1, 0).DoAndReturn(func(_, _ int) error {
			cancel()
			return nil
		})

		handler := newCapturingHandler()
		logger := slog.New(handler)
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, interval), mustOffset(t, 1))

		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		time.Sleep(interval)
		synctest.Wait()

		require.NoError(t, <-done)

		records := handler.snapshot()
		require.NotEmpty(t, records)

		var sawStarted, sawStopped bool
		for _, r := range records {
			switch r.Message {
			case "sysmon started":
				sawStarted = true
				assert.Equal(t, slog.LevelInfo, r.Level)

				var sawIntervalAttr bool
				r.Attrs(func(a slog.Attr) bool {
					if a.Key == "interval" {
						sawIntervalAttr = true
					}
					return true
				})
				assert.True(t, sawIntervalAttr, "expected an interval attribute on the started record")
			case "sysmon stopped":
				sawStopped = true
				assert.Equal(t, slog.LevelInfo, r.Level)
			}
		}

		assert.True(t, sawStarted, "expected a sysmon started record")
		assert.True(t, sawStopped, "expected a sysmon stopped record")
	})
}
