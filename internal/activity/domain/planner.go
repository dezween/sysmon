package domain

import "errors"

// Movement planning constants. stepCount is fixed rather than configurable:
// it is an implementation detail of what "looks like a human glide" (see
// docs/TASK-roaming.md), not a user-facing setting.
const (
	// stepCount is the number of intermediate points a glide path is split
	// into, chosen to land in the human-like ~20-60 step range.
	stepCount = 40
	// roundingScale doubles both the numerator and denominator of the
	// interpolation division. That lets us add `total` (which equals half of
	// the doubled denominator) before dividing, implementing round-half-up to
	// the nearest pixel instead of truncation toward zero.
	roundingScale = 2
)

// ErrInvalidBounds is returned when Plan is given a non-positive width or
// height, since a zero or negative screen dimension has no valid target
// point.
var ErrInvalidBounds = errors.New("bounds must be positive")

// RandomSource is the minimal seam through which Planner draws randomness.
// It is satisfied by *math/rand/v2.Rand (via its IntN method), so production
// code can inject real runtime randomness while tests inject a deterministic
// stub. The interface only lists the method Planner actually calls, keeping
// it minimal.
type RandomSource interface {
	// IntN returns a pseudo-random int in [0, n). It must not be called with
	// n <= 0.
	IntN(n int) int
}

// Point is a single (x, y) coordinate of a glide path, in screen pixels.
type Point struct {
	X int
	Y int
}

// Planner produces glide paths for the cursor-roaming use case: given a
// current position and screen bounds, it plans a sequence of small steps
// ending at a fresh uniformly-random target. Planner is pure and
// deterministic given its injected RandomSource, has no cgo or os
// dependency, and builds under GOOS=linux.
type Planner struct {
	rnd RandomSource
}

// NewPlanner constructs a Planner using rnd as its source of randomness.
func NewPlanner(rnd RandomSource) *Planner {
	return &Planner{rnd: rnd}
}

// Plan returns an ordered glide path from (curX, curY) to a fresh uniformly
// random target inside [0, width) x [0, height), clamped to those bounds.
// The path is continuous (no teleporting step) and its last element is
// always exactly the target. width and height must be positive, or Plan
// returns ErrInvalidBounds. When the screen is 1x1, the only valid point is
// (0, 0) and Plan returns a path that stays there. When the random target
// equals the current position, Plan still returns a well-formed path ending
// at that same point.
func (p *Planner) Plan(curX, curY, width, height int) ([]Point, error) {
	if width <= 0 || height <= 0 {
		return nil, ErrInvalidBounds
	}

	// Clamp the starting position into this display's bounds before planning.
	// Position() can report global desktop coordinates -- negative, or beyond
	// the main display -- when the cursor sits on a secondary monitor.
	// Interpolating from an out-of-bounds start would send the first glide
	// steps off-screen, violating the "never off-screen" guarantee.
	curX = clamp(curX, width)
	curY = clamp(curY, height)

	targetX := p.rnd.IntN(width)
	targetY := p.rnd.IntN(height)

	return interpolate(curX, curY, targetX, targetY), nil
}

// clamp constrains v to the valid pixel range [0, size-1] for an axis of the
// given size. size is guaranteed positive by the caller.
func clamp(v, size int) int {
	if v < 0 {
		return 0
	}
	if v > size-1 {
		return size - 1
	}
	return v
}

// interpolate builds the ordered intermediate points from (fromX, fromY) to
// (toX, toY) over stepCount steps, with the final element exactly equal to
// the target. Using integer division with rounding at each step keeps every
// step's delta bounded and avoids cumulative drift, since the last step
// always lands exactly on the target rather than an accumulated
// approximation.
func interpolate(fromX, fromY, toX, toY int) []Point {
	path := make([]Point, 0, stepCount)

	for i := 1; i <= stepCount; i++ {
		path = append(path, lerpPoint(fromX, fromY, toX, toY, i, stepCount))
	}

	// Guarantee exact arrival regardless of integer rounding above.
	path[len(path)-1] = Point{X: toX, Y: toY}

	return path
}

// lerpPoint linearly interpolates between (fromX, fromY) and (toX, toY) at
// step/total of the way there, rounding to the nearest integer pixel.
func lerpPoint(fromX, fromY, toX, toY, step, total int) Point {
	return Point{
		X: lerpAxis(fromX, toX, step, total),
		Y: lerpAxis(fromY, toY, step, total),
	}
}

// lerpAxis linearly interpolates a single axis between from and to at
// step/total of the way there, rounding to the nearest integer.
func lerpAxis(from, to, step, total int) int {
	delta := to - from
	// Round-to-nearest via adding half the denominator before truncating
	// division, handling negative delta correctly since total is always
	// positive.
	numerator := delta*step*roundingScale + total
	return from + floorDiv(numerator, total*roundingScale)
}

// floorDiv returns the floor of a/b for a positive b, matching mathematical
// rounding for both positive and negative a (unlike Go's truncating "/").
func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}
