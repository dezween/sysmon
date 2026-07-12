package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sysmon/internal/activity/domain"
)

// stubSource is a hand-injected deterministic domain.RandomSource -- NOT a
// mockgen mock, since this is a domain test with no port involved. It
// returns each value in seq in order, then repeats the last value forever
// (so a test only needs to specify as many draws as it cares about).
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

func TestPlanner_Plan_TargetWithinBoundsAndPathEndsAtTarget(t *testing.T) {
	t.Parallel()

	rnd := newStubSource(37, 12)
	p := domain.NewPlanner(rnd)

	path, err := p.Plan(0, 0, 100, 50)

	require.NoError(t, err)
	require.NotEmpty(t, path)

	last := path[len(path)-1]
	assert.Equal(t, 37, last.X)
	assert.Equal(t, 12, last.Y)

	for _, pt := range path {
		assert.GreaterOrEqual(t, pt.X, 0)
		assert.Less(t, pt.X, 100)
		assert.GreaterOrEqual(t, pt.Y, 0)
		assert.Less(t, pt.Y, 50)
	}
}

func TestPlanner_Plan_PathLengthInHumanRange(t *testing.T) {
	t.Parallel()

	rnd := newStubSource(80, 40)
	p := domain.NewPlanner(rnd)

	path, err := p.Plan(0, 0, 100, 100)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(path), 20)
	assert.LessOrEqual(t, len(path), 60)
}

func TestPlanner_Plan_PathIsContinuousNoTeleport(t *testing.T) {
	t.Parallel()

	rnd := newStubSource(999, 1)
	p := domain.NewPlanner(rnd)

	// Start far from the (clamped) target so the bounded-step property is
	// actually exercised over a large distance.
	path, err := p.Plan(0, 0, 1000, 2)

	require.NoError(t, err)
	require.NotEmpty(t, path)

	const maxStepMagnitude = 100 // generous bound: total distance / (stepCount/2)

	prevX, prevY := 0, 0
	for _, pt := range path {
		dx := pt.X - prevX
		if dx < 0 {
			dx = -dx
		}
		dy := pt.Y - prevY
		if dy < 0 {
			dy = -dy
		}
		assert.LessOrEqual(t, dx, maxStepMagnitude, "step in X exceeded bounded magnitude")
		assert.LessOrEqual(t, dy, maxStepMagnitude, "step in Y exceeded bounded magnitude")
		prevX, prevY = pt.X, pt.Y
	}
}

func TestPlanner_Plan_InvalidBounds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		width  int
		height int
	}{
		{name: "zero width", width: 0, height: 10},
		{name: "zero height", width: 10, height: 0},
		{name: "negative width", width: -5, height: 10},
		{name: "negative height", width: 10, height: -5},
		{name: "both zero", width: 0, height: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rnd := newStubSource(0, 0)
			p := domain.NewPlanner(rnd)

			path, err := p.Plan(0, 0, tt.width, tt.height)

			require.ErrorIs(t, err, domain.ErrInvalidBounds)
			assert.Nil(t, path)
		})
	}
}

// TestPlanner_Plan_EdgeCase_CursorAlreadyAtCorner covers the cursor starting
// exactly at a bounds corner: the path must still stay in-bounds and end at
// the (possibly distant) random target.
func TestPlanner_Plan_EdgeCase_CursorAlreadyAtCorner(t *testing.T) {
	t.Parallel()

	rnd := newStubSource(0, 0)
	p := domain.NewPlanner(rnd)

	// Cursor starts at the bottom-right corner of a 200x100 screen.
	path, err := p.Plan(199, 99, 200, 100)

	require.NoError(t, err)
	require.NotEmpty(t, path)

	last := path[len(path)-1]
	assert.Equal(t, 0, last.X)
	assert.Equal(t, 0, last.Y)

	for _, pt := range path {
		assert.GreaterOrEqual(t, pt.X, 0)
		assert.Less(t, pt.X, 200)
		assert.GreaterOrEqual(t, pt.Y, 0)
		assert.Less(t, pt.Y, 100)
	}
}

// TestPlanner_Plan_ClampsOutOfBoundsStart verifies that a starting position
// outside the display bounds -- as Position() can report on a multi-monitor
// setup -- is clamped, so no glide step lands off the main display. It covers
// negative, far-positive, and exact-boundary (curX == width) starts.
func TestPlanner_Plan_ClampsOutOfBoundsStart(t *testing.T) {
	t.Parallel()

	const w, h = 1920, 1080

	starts := []struct {
		name       string
		curX, curY int
	}{
		{"negative both", -500, -9000},
		{"far positive both", 999999, 5000},
		{"exact boundary equals size", w, h},
		{"mixed sign", -1, h + 1},
	}

	for _, s := range starts {
		t.Run(s.name, func(t *testing.T) {
			t.Parallel()

			p := domain.NewPlanner(newStubSource(100, 200))

			path, err := p.Plan(s.curX, s.curY, w, h)
			require.NoError(t, err)
			require.NotEmpty(t, path)

			for _, pt := range path {
				assert.GreaterOrEqual(t, pt.X, 0)
				assert.Less(t, pt.X, w)
				assert.GreaterOrEqual(t, pt.Y, 0)
				assert.Less(t, pt.Y, h)
			}
		})
	}
}

// TestPlanner_Plan_EdgeCase_OnePixelScreen covers a 1x1 screen: the only valid
// point is (0, 0), so every path point must be (0, 0).
func TestPlanner_Plan_EdgeCase_OnePixelScreen(t *testing.T) {
	t.Parallel()

	rnd := newStubSource(0, 0)
	p := domain.NewPlanner(rnd)

	path, err := p.Plan(0, 0, 1, 1)

	require.NoError(t, err)
	require.NotEmpty(t, path)

	for _, pt := range path {
		assert.Equal(t, domain.Point{X: 0, Y: 0}, pt)
	}
}

// TestPlanner_Plan_EdgeCase_TargetEqualsCurrent covers a random draw that
// lands exactly on the current position: the path must still be
// well-formed (right length, in-bounds) and every point equals the current
// position since there is nowhere to glide to.
func TestPlanner_Plan_EdgeCase_TargetEqualsCurrent(t *testing.T) {
	t.Parallel()

	rnd := newStubSource(42, 17)
	p := domain.NewPlanner(rnd)

	path, err := p.Plan(42, 17, 100, 100)

	require.NoError(t, err)
	require.NotEmpty(t, path)

	for _, pt := range path {
		assert.Equal(t, domain.Point{X: 42, Y: 17}, pt)
	}

	last := path[len(path)-1]
	assert.Equal(t, 42, last.X)
	assert.Equal(t, 17, last.Y)
}

// TestPlanner_Plan_DeterministicGivenSameSource asserts that two Planners
// fed the same sequence of draws produce byte-for-byte identical paths --
// the core determinism guarantee the injected RandomSource seam exists for.
func TestPlanner_Plan_DeterministicGivenSameSource(t *testing.T) {
	t.Parallel()

	p1 := domain.NewPlanner(newStubSource(55, 23))
	p2 := domain.NewPlanner(newStubSource(55, 23))

	path1, err1 := p1.Plan(10, 10, 200, 150)
	path2, err2 := p2.Plan(10, 10, 200, 150)

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, path1, path2)
}
