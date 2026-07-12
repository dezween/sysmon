package service_test

import (
	"context"
	"errors"
	"log/slog"
	mathrand "math/rand/v2"
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

// errPointer is a local sentinel used to simulate a Pointer failure in these
// tests. It must never be one of adapter's error sentinels: service tests
// stay cgo-free and must not import internal/activity/adapter.
var errPointer = errors.New("pointer sentinel failure")

// mustInterval builds a validated domain.Interval, failing the test on error.
func mustInterval(t *testing.T, d time.Duration) domain.Interval {
	t.Helper()
	iv, err := domain.NewInterval(d)
	require.NoError(t, err)
	return iv
}

// stubSource is a hand-injected deterministic domain.RandomSource so the
// planner (and therefore the expected MoveTo path) is exact and
// reproducible in these service tests.
type stubSource struct {
	seq []int
	i   int
}

func newStubSource(seq ...int) *stubSource {
	return &stubSource{seq: seq}
}

func (s *stubSource) IntN(n int) int {
	var v int
	if s.i < len(s.seq) {
		v = s.seq[s.i]
		s.i++
	} else if len(s.seq) > 0 {
		v = s.seq[len(s.seq)-1]
	}
	if v >= n {
		v = n - 1
	}
	if v < 0 {
		v = 0
	}
	return v
}

// mustPlanner builds a domain.Planner fed a stub source, so the path it
// produces is deterministic and known ahead of time by the test.
func mustPlanner(seq ...int) *domain.Planner {
	return domain.NewPlanner(newStubSource(seq...))
}

// inOrder is a typed wrapper around gomock.InOrder, which takes ...any: it
// lets call sites build a []*gomock.Call and spread it without a manual
// per-test conversion loop.
func inOrder(calls []*gomock.Call) {
	args := make([]any, len(calls))
	for i, c := range calls {
		args[i] = c
	}
	gomock.InOrder(args...)
}

// expectPath sets up ordered gomock expectations for Position, Bounds, and
// then a MoveTo call per point in path, returning nil errors throughout.
// The last MoveTo invokes onLastMove (if non-nil) so tests can trigger a
// side effect (e.g. cancel ctx) exactly once the whole path has been
// driven.
func expectPath(pointer *mock.MockPointer, curX, curY, w, h int, path []domain.Point, onLastMove func()) {
	calls := []*gomock.Call{
		pointer.EXPECT().Position().Return(curX, curY, nil),
		pointer.EXPECT().Bounds().Return(w, h, nil),
	}
	for i, pt := range path {
		if i == len(path)-1 && onLastMove != nil {
			calls = append(calls, pointer.EXPECT().MoveTo(pt.X, pt.Y).DoAndReturn(func(_, _ int) error {
				onLastMove()
				return nil
			}))
			continue
		}
		calls = append(calls, pointer.EXPECT().MoveTo(pt.X, pt.Y).Return(nil))
	}
	inOrder(calls)
}

func TestKeeperService_OneRoamPerTick_MoveToInOrder(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		const interval = 10 * time.Second

		// Two independent planners fed the identical draw sequence: one to
		// precompute the expected path for the mock expectations, one wired
		// into the service. They must be distinct instances -- calling Plan
		// on the same *domain.Planner twice would advance its stub source
		// twice and desync the service's actual draws from this precomputed
		// path.
		expectedPath, err := domain.NewPlanner(newStubSource(37, 12)).Plan(0, 0, 100, 50)
		require.NoError(t, err)
		planner := domain.NewPlanner(newStubSource(37, 12))

		ctx, cancel := context.WithCancel(t.Context())

		expectPath(pointer, 0, 0, 100, 50, expectedPath, cancel)

		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, interval), planner)

		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		time.Sleep(interval)
		synctest.Wait()

		require.NoError(t, <-done)
	})
}

func TestKeeperService_StopsOnContextCancel(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		planner := mustPlanner(1, 1)
		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, time.Second), planner)

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		synctest.Wait()
		cancel()
		synctest.Wait()

		require.NoError(t, <-done)
	})
}

// TestKeeperService_MidGlideCancelStopsPromptly cancels ctx after exactly N
// MoveTo calls have landed and asserts no further MoveTo calls are made --
// the core "interruptible mid-glide" guarantee.
func TestKeeperService_MidGlideCancelStopsPromptly(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		path, err := domain.NewPlanner(newStubSource(80, 40)).Plan(0, 0, 100, 100)
		require.NoError(t, err)
		require.Greater(t, len(path), 5, "test needs a path long enough to cancel partway through")
		planner := domain.NewPlanner(newStubSource(80, 40))

		const cancelAfter = 3

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		calls := []*gomock.Call{
			pointer.EXPECT().Position().Return(0, 0, nil),
			pointer.EXPECT().Bounds().Return(100, 100, nil),
		}
		for i := range cancelAfter {
			pt := path[i]
			if i == cancelAfter-1 {
				calls = append(calls, pointer.EXPECT().MoveTo(pt.X, pt.Y).DoAndReturn(func(_, _ int) error {
					cancel()
					return nil
				}))
				continue
			}
			calls = append(calls, pointer.EXPECT().MoveTo(pt.X, pt.Y).Return(nil))
		}
		inOrder(calls)
		// No expectations for path[cancelAfter:] -- gomock's controller fails
		// the test if any of those MoveTo calls are made.

		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, time.Second), planner)

		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		time.Sleep(time.Second)
		synctest.Wait()

		require.NoError(t, <-done)
	})
}

// TestKeeperService_PositionErrorAbandonsTickWithoutCrash asserts a
// Position failure is logged and the tick is abandoned (no Bounds/MoveTo
// calls), without crashing the loop.
func TestKeeperService_PositionErrorAbandonsTickWithoutCrash(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		ctx, cancel := context.WithCancel(t.Context())

		pointer.EXPECT().Position().DoAndReturn(func() (int, int, error) {
			cancel()
			return 0, 0, errPointer
		})
		// No Bounds/MoveTo expectations: gomock fails the test if called.

		planner := mustPlanner(1, 1)
		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, time.Second), planner)

		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		time.Sleep(time.Second)
		synctest.Wait()

		require.NoError(t, <-done)
	})
}

// TestKeeperService_BoundsErrorAbandonsTickWithoutCrash asserts a Bounds
// failure is logged and the tick is abandoned (no MoveTo calls), without
// crashing the loop.
func TestKeeperService_BoundsErrorAbandonsTickWithoutCrash(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		ctx, cancel := context.WithCancel(t.Context())

		pointer.EXPECT().Position().Return(5, 5, nil)
		pointer.EXPECT().Bounds().DoAndReturn(func() (int, int, error) {
			cancel()
			return 0, 0, errPointer
		})
		// No MoveTo expectations: gomock fails the test if called.

		planner := mustPlanner(1, 1)
		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, time.Second), planner)

		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		time.Sleep(time.Second)
		synctest.Wait()

		require.NoError(t, <-done)
	})
}

// TestKeeperService_MoveErrorAbandonsRoamAndContinues asserts a MoveTo
// failure partway through a path is logged and stops that roam early, but
// the loop continues to the next tick and roams normally there.
func TestKeeperService_MoveErrorAbandonsRoamAndContinues(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		// Draw sequence covers both ticks: (1,1) is the first tick's target,
		// (7,3) is the second's. The planner wired into the service draws
		// both in order across its two Plan calls (one per tick), so a
		// second, independently-seeded planner with the SAME sequence is
		// used here purely to precompute the expected paths -- reusing the
		// service's own planner instance would double-consume the sequence.
		firstPath, err := domain.NewPlanner(newStubSource(1, 1, 7, 3)).Plan(0, 0, 10, 10)
		require.NoError(t, err)
		planner := domain.NewPlanner(newStubSource(1, 1, 7, 3))

		// First tick: Position/Bounds succeed, first MoveTo fails -> roam
		// abandoned, no further MoveTo calls this tick.
		gomock.InOrder(
			pointer.EXPECT().Position().Return(0, 0, nil),
			pointer.EXPECT().Bounds().Return(10, 10, nil),
			pointer.EXPECT().MoveTo(firstPath[0].X, firstPath[0].Y).Return(errPointer),
		)

		// Second tick: full roam succeeds normally; last MoveTo cancels ctx.
		secondPath, err := domain.NewPlanner(newStubSource(7, 3)).Plan(2, 2, 10, 10)
		require.NoError(t, err)

		secondCalls := []*gomock.Call{
			pointer.EXPECT().Position().Return(2, 2, nil),
			pointer.EXPECT().Bounds().Return(10, 10, nil),
		}
		for i, pt := range secondPath {
			if i == len(secondPath)-1 {
				secondCalls = append(secondCalls, pointer.EXPECT().MoveTo(pt.X, pt.Y).DoAndReturn(func(_, _ int) error {
					cancel()
					return nil
				}))
				continue
			}
			secondCalls = append(secondCalls, pointer.EXPECT().MoveTo(pt.X, pt.Y).Return(nil))
		}
		inOrder(secondCalls)

		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, time.Second), planner)

		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		time.Sleep(2 * time.Second)
		synctest.Wait()

		require.NoError(t, <-done)
	})
}

// TestKeeperService_InvalidBoundsAbandonsTickWithoutCrash asserts that when
// Bounds() returns a non-positive dimension, the real domain.Planner rejects
// it with domain.ErrInvalidBounds, roam logs the error and abandons the
// tick, and no MoveTo call is ever made. A real planner (backed by a PCG
// source) is used here rather than the stub, since the point under test is
// Plan's own bounds validation, not a specific stubbed draw.
func TestKeeperService_InvalidBoundsAbandonsTickWithoutCrash(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		ctx, cancel := context.WithCancel(t.Context())

		pointer.EXPECT().Position().Return(0, 0, nil)
		pointer.EXPECT().Bounds().DoAndReturn(func() (int, int, error) {
			cancel()
			return 0, 0, nil
		})
		// No MoveTo expectations: gomock fails the test if called, since
		// Plan must reject (0, 0) bounds with domain.ErrInvalidBounds before
		// any glide step is produced.

		planner := domain.NewPlanner(mathrand.New(mathrand.NewPCG(1, 2))) //nolint:gosec // test randomness, not security
		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, time.Second), planner)

		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		time.Sleep(time.Second)
		synctest.Wait()

		require.NoError(t, <-done)
	})
}

// TestKeeperService_MoveErrorOnFirstStepAbandonsGlideImmediately asserts
// that when the very first MoveTo of a roam fails, drive stops immediately:
// no subsequent MoveTo call is made for the rest of that path, and Run still
// returns nil since the failure is only logged, not propagated.
func TestKeeperService_MoveErrorOnFirstStepAbandonsGlideImmediately(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		ctx, cancel := context.WithCancel(t.Context())

		path, err := domain.NewPlanner(newStubSource(80, 40)).Plan(0, 0, 100, 100)
		require.NoError(t, err)
		require.Greater(t, len(path), 1, "test needs at least one further step that must NOT be called")
		planner := domain.NewPlanner(newStubSource(80, 40))

		pointer.EXPECT().Position().Return(0, 0, nil)
		pointer.EXPECT().Bounds().Return(100, 100, nil)
		pointer.EXPECT().MoveTo(path[0].X, path[0].Y).DoAndReturn(func(_, _ int) error {
			cancel()
			return errPointer
		})
		// No further MoveTo expectations: gomock fails the test if any of
		// path[1:] is ever called.

		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, time.Second), planner)

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

		path, err := domain.NewPlanner(newStubSource(1, 1)).Plan(0, 0, 10, 10)
		require.NoError(t, err)
		planner := domain.NewPlanner(newStubSource(1, 1))

		expectPath(pointer, 0, 0, 10, 10, path, cancel)

		handler := newCapturingHandler()
		logger := slog.New(handler)
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, interval), planner)

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

// TestKeeperService_RoamStaysInBoundsFromOutOfBoundsStart asserts the
// service-to-adapter "never off-screen" contract at that seam: even when
// Position() reports an out-of-bounds start (as a multi-monitor Mac does with
// global desktop coordinates), every MoveTo the service issues lands inside
// [0, w) x [0, h).
func TestKeeperService_RoamStaysInBoundsFromOutOfBoundsStart(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		const w, h = 1920, 1080

		ctrl := gomock.NewController(t)
		pointer := mock.NewMockPointer(ctrl)

		ctx, cancel := context.WithCancel(t.Context())

		pointer.EXPECT().Position().Return(-500, 5000, nil)
		pointer.EXPECT().Bounds().Return(w, h, nil)
		pointer.EXPECT().MoveTo(gomock.Any(), gomock.Any()).DoAndReturn(func(x, y int) error {
			assert.GreaterOrEqual(t, x, 0)
			assert.Less(t, x, w)
			assert.GreaterOrEqual(t, y, 0)
			assert.Less(t, y, h)
			return nil
		}).MinTimes(1)

		logger := slog.New(slog.DiscardHandler)
		planner := domain.NewPlanner(mathrand.New(mathrand.NewPCG(1, 2))) //nolint:gosec // test randomness, not security
		svc := service.NewKeeperService(pointer, logger, mustInterval(t, time.Second), planner)

		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		time.Sleep(time.Second) // let exactly one roam fire
		synctest.Wait()

		cancel()
		synctest.Wait()

		require.NoError(t, <-done)
	})
}
