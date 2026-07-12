---
phase: quick-260712-roaming
reviewed: 2026-07-12T00:00:00Z
depth: standard
files_reviewed: 9
files_reviewed_list:
  - internal/activity/domain/planner.go
  - internal/activity/domain/planner_test.go
  - internal/activity/domain/activity.go
  - internal/activity/domain/activity_test.go
  - internal/activity/port/pointer.go
  - internal/activity/service/keeper_service.go
  - internal/activity/service/keeper_service_test.go
  - internal/activity/adapter/cgpointer.go
  - internal/activity/adapter/cgpointer_test.go
  - cmd/sysmon/main.go
findings:
  critical: 1
  warning: 2
  info: 2
  total: 5
status: issues_found
---

# Roaming Feature Review — Pass #1

**Reviewed:** 2026-07-12
**Depth:** standard
**Files Reviewed:** 9 (+ cmd/sysmon/main_test.go read for context)
**Status:** issues_found

## Summary

This is a well-built, small, hexagonally-clean feature. `go build ./...`,
`golangci-lint run ./...` (0 issues), and `go test -race ./...` all pass;
domain/port/service build under `GOOS=linux`. The `RandomSource` seam is
minimal and correctly injected; planner tests are genuinely deterministic
(verified `TestPlanner_Plan_DeterministicGivenSameSource`); the interpolation
math was traced numerically (monotonic, bounded step, exact final-target
arrival, no drift, correct for both positive and negative deltas via
`floorDiv`) and is sound. cgo memory management in the adapter is correct:
every `CGEventCreate*` is paired with `CFRelease`, `CGDisplayBounds`/
`CGMainDisplayID` create no CF object (nothing to release), no `CFRelease(NULL)`
path exists, `//go:build darwin` is present, and `*CGPointer` still satisfies
`port.Pointer`. Layering is correct: domain has no cgo/os imports and builds
under `GOOS=linux`; service imports only `domain` + `port`, never `adapter`.
Ctx cancellation is checked between every `MoveTo` in `drive()`, and the
mid-glide-cancel test proves no further `MoveTo` calls happen after cancel.

One real correctness gap found against the acceptance criteria: the planner
never clamps or validates the *current* position it interpolates from, only
the random *target*. On a real multi-monitor Mac, `Position()` can legitimately
return coordinates outside the main display's `[0,width)x[0,height)` (e.g. a
secondary monitor placed to the left has negative global x). The domain layer
silently interpolates from that out-of-bounds start, so the first several
glide steps of a roam can be off the main display — directly contradicting the
"never off-screen" acceptance criterion in a realistic, not contrived,
scenario. See CR-01.

## Critical Issues

### CR-01: Glide path is not bounds-clamped when the current position is outside the main display

**File:** `internal/activity/domain/planner.go:62-71`
**Issue:** `Plan` validates `width`/`height` and draws a target uniformly in
`[0,width) x [0,height)`, but `curX`/`curY` (the position `interpolate` glides
*from*) are used as-is, with no clamping and no test coverage for the case
where they fall outside `[0,width)x[0,height)`. On a real Mac with a
secondary display, `CGEventGetLocation` (via `adapter.CGPointer.Position()`)
returns coordinates in the *global* desktop coordinate space, not the main
display's local space — a monitor placed to the left of the main display
produces negative x, a monitor placed above produces negative y, and a
monitor placed to the right/below produces x/y beyond the main display's
width/height. Since `interpolate` linearly walks from `(curX, curY)` to the
clamped target, the earliest steps of the glide remain off the main display
(and can even be at negative coordinates) until the interpolation catches up.
This is a direct violation of the spec's "whole screen, bounded... never
off-screen" acceptance criterion (`docs/TASK-roaming.md:28-30`) and of
`PLAN.md`'s truth "The cursor never leaves the main display bounds ... no
cumulative drift". Verified numerically: `Plan`-equivalent interpolation from
`curX=-500` to a target of `960` inside a `1920`-wide main display yields
intermediate steps of `-463`, `-317`, ... — genuinely off-display coordinates
posted via `MoveTo`.
**Fix:** Clamp `curX`/`curY` into `[0,width) x [0,height)` before calling
`interpolate`, so the glide always starts and stays inside the main display's
bounds even if the OS-reported current position is elsewhere:
```go
func (p *Planner) Plan(curX, curY, width, height int) ([]Point, error) {
	if width <= 0 || height <= 0 {
		return nil, ErrInvalidBounds
	}

	curX = clamp(curX, width)
	curY = clamp(curY, height)

	targetX := p.rnd.IntN(width)
	targetY := p.rnd.IntN(height)

	return interpolate(curX, curY, targetX, targetY), nil
}

// clamp confines v to [0, n).
func clamp(v, n int) int {
	if v < 0 {
		return 0
	}
	if v >= n {
		return n - 1
	}
	return v
}
```
Add a planner test with `curX`/`curY` outside `[0,width)x[0,height)` (both
negative and >= bound) asserting every path point, including the first,
stays within `[0,width)x[0,height)`.

## Warnings

### WR-01: `time.After` allocates a new timer every glide step instead of reusing one

**File:** `internal/activity/service/keeper_service.go:112-116`
**Issue:** `drive` calls `time.After(stepPause)` inside the per-point loop.
Each call allocates a new `time.Timer` that is not eligible for reuse and,
if `ctx.Done()` fires first, is never explicitly stopped (it is left to be
GC'd once it fires ~8ms later). With `stepCount = 40`, a single roam
allocates up to 40 timers. This is not a leak in the classic sense (they do
get GC'd), but it is unidiomatic Go for a hot loop and is exactly the pattern
`time.After`'s own godoc warns against reusing in tight loops. Given this
project's stated efficiency goals (near-zero idle CPU / lightweight
footprint), a single reused timer is the more correct pattern.
**Fix:** Use one `time.NewTimer`, `Reset` it each iteration, and `Stop` it on
early exit:
```go
func (k *KeeperService) drive(ctx context.Context, path []domain.Point) {
	timer := time.NewTimer(stepPause)
	defer timer.Stop()

	for _, pt := range path {
		if err := k.pointer.MoveTo(pt.X, pt.Y); err != nil {
			k.logger.Error("move failed", "err", err)
			return
		}

		select {
		case <-timer.C:
			timer.Reset(stepPause)
		case <-ctx.Done():
			return
		}
	}
}
```

### WR-02: `roundingScale` name/doc slightly obscures the actual purpose (round-half-up bias, not "doubling the denominator" as an end in itself)

**File:** `internal/activity/domain/planner.go:13-15, 103-110`
**Issue:** The comment says "roundingScale doubles the interpolation
denominator so integer division can round to the nearest pixel instead of
truncating toward zero" — accurate, but the name `roundingScale` combined
with the `lerpAxis` comment ("Round-to-nearest via adding half the
denominator before truncating division") requires the reader to reconstruct
that `roundingScale = 2` exists purely to let `+ total` represent "add half
of `total*roundingScale`" without needing a fractional `total/2`. This is a
classic round-half-up-via-integer-arithmetic trick; it works (verified
numerically, monotonic and drift-free for a wide sweep of positive/negative
deltas), but a reader unfamiliar with the trick has to work harder than
necessary. Not a correctness issue — purely a readability nit.
**Fix:** Consider renaming to `roundHalfScale` or adding one more line to the
`lerpAxis` doc comment spelling out the algebra
(`numerator/ (total*2) == round(delta*step/total)`), or extracting the
`+ total` term into a named `halfStep := total * roundingScale / 2` — wait,
that reintroduces truncation; simplest is just a slightly more explicit
comment. Optional; low priority.

## Info

### IN-01: `Bounds()` is re-queried every tick even though the main display's dimensions essentially never change mid-session

**File:** `internal/activity/service/keeper_service.go:88-92`
**Issue:** Every `roam()` call re-reads `Bounds()` via cgo. This is not a
performance defect in the sense that's in scope for this review (a single
cgo call per 10s tick is trivially cheap), but it is worth noting for anyone
later trying to reduce cgo call surface — genuinely out of v1 scope per the
review lens, included here only as a nit.
**Fix:** None required; no action needed unless display-bounds caching
becomes desirable for other reasons (e.g. detecting external-display
attach/detach mid-run, which is arguably a *feature*, not a bug).

### IN-02: `getBounds`'s "always succeeds" comment is not fully defensive against a 0x0 or negative bounds return

**File:** `internal/activity/adapter/cgpointer.go:27-34, 96-101`
**Issue:** `Bounds()` trusts `CGDisplayBounds(CGMainDisplayID())` and never
returns an error — reasonable per Apple's API contract (a running macOS
session always has a main display with positive dimensions), but if this
ever returned a degenerate rect (e.g. transient state during
display-configuration change / sleep-wake), `int(w)`/`int(h)` could yield 0,
and the resulting `Bounds()` values would flow straight into
`domain.Planner.Plan`, which already returns `ErrInvalidBounds` for
`width<=0 || height<=0` and is handled/logged by `roam()` without crashing.
So the failure mode is already contained by the domain+service boundary;
this is purely a note that the containment is implicit (relies on Plan's
guard) rather than validated at the adapter boundary itself. No fix required
given the existing containment; flagging for awareness only.

---

_Reviewed: 2026-07-12_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
