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
  critical: 0
  warning: 0
  info: 2
  total: 2
status: issues_found
---

# Roaming Feature Review — Pass #2 (post review #1 fixes)

**Reviewed:** 2026-07-12
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found (info-only; no blockers, no warnings)

## Summary

All five review #1 findings were verified fixed, correctly, with no regressions:

- **CR-01 (clamp out-of-bounds start)** — fixed correctly. `Plan` now clamps
  `curX`/`curY` via a new `clamp(v, size)` helper (`planner.go:73-74,84-92`)
  before calling `interpolate`. Verified numerically for both directions
  (`curX=-500` -> `0`, `curX=5000` with `width=1920` -> `1919`) and confirmed
  that clamping both endpoints of a linear interpolation to `[0,size)` makes
  every intermediate point provably in-bounds (lerp between two in-bounds
  integers cannot overshoot either endpoint). Covered by the new
  `TestPlanner_Plan_ClampsOutOfBoundsStart` test, which asserts every path
  point stays in `[0,w) x [0,h)` starting from `(-500, 5000)` against a
  `1920x1080` bound. `go test -race` passes.
- **WR-01 (reuse one timer instead of `time.After` per step)** — fixed
  correctly. `drive()` now allocates a single `time.NewTimer(stepPause)`
  before the loop, calls `timer.Reset(stepPause)` after each `MoveTo`, and
  `defer timer.Stop()` on function exit (`keeper_service.go:107-126`).
  Verified this is correct under Go 1.26's timer semantics (this project's
  `go.mod` requires `go 1.26.5`, well past the Go 1.23 timer-channel
  redesign): `Reset` on an already-drained-by-select timer needs no manual
  drain, and `Reset` on a timer that hasn/t fired yet (the first-iteration
  case, since the timer is constructed already running) is likewise safe.
  Traced a live (non-synctest) 40-iteration run of the identical loop shape
  and confirmed no stale-fire drift (elapsed time tracked expected duration).
  All `synctest`-based service tests (`TestKeeperService_MidGlideCancelStopsPromptly`
  et al.) pass under `-race`, including the ctx-cancel-mid-glide path that
  exercises the timer/ctx.Done() select race. No goroutine or timer leak: the
  `defer timer.Stop()` runs on every return path (MoveTo error, ctx cancel,
  natural path completion).
- **WR-02 (rounding comment clarity)** — fixed; `roundingScale` doc comment
  now spells out the round-half-up algebra (`planner.go:12-16`). Nit-level,
  addressed as suggested.
- **IN-01 (bounds re-read per roam)** — addressed with an explanatory comment
  (`keeper_service.go:88-89`) noting this is intentional (picks up
  resolution/monitor changes). No behavior change requested or made; correct.
- **IN-02 (adapter `Bounds()` always-nil contract)** — addressed with an
  expanded doc comment (`cgpointer.go:96-100`) documenting the containment
  relationship with `domain.Planner`'s `ErrInvalidBounds` guard. Correct and
  matches the actual code path.

`go build ./...`, `go vet ./...`, `golangci-lint run ./...` (strict config,
0 issues), and `go test -race -count=1 ./...` all pass. `GOOS=linux go build`
succeeds for `domain`, `port`, and `service`. `gofmt -l` reports no files.
Layering remains correct: domain has no cgo/os import, service imports only
domain+port, cgo is confined to the adapter behind `//go:build darwin`, every
`CGEventCreate*` is still paired with `CFRelease`.

No new Critical or Warning-tier defects were introduced by the fix commit.
Two Info-level nits found, one of which is a genuine (if harmless) doc-comment
placement bug introduced by the fix itself.

## Info

### IN-01: Misplaced/orphaned doc comment left behind by the CR-01 test insertion

**File:** `internal/activity/domain/planner_test.go:166-170, 191`
**Issue:** The new `TestPlanner_Plan_ClampsOutOfBoundsStart` test (and its own
correct doc comment) was inserted *before* `TestPlanner_Plan_EdgeCase_OnePixelScreen`
in the file, but the original doc comment that belonged to
`TestPlanner_Plan_EdgeCase_OnePixelScreen` (lines 166-167: "TestPlanner_Plan_EdgeCase_OnePixelScreen
covers a 1x1 screen: the only valid point is (0, 0), so every path point must
be (0, 0).") was left in place above the new test instead of being moved down
with it. The result:
- Lines 166-170 now read as one merged, incorrect comment block: two lines
  that describe `TestPlanner_Plan_EdgeCase_OnePixelScreen` immediately
  followed by two lines that describe `TestPlanner_Plan_ClampsOutOfBoundsStart`
  -- all sitting directly above `func TestPlanner_Plan_ClampsOutOfBoundsStart`.
- `TestPlanner_Plan_EdgeCase_OnePixelScreen` itself (line 191) now has no doc
  comment at all.

This is purely cosmetic (does not affect test correctness, `go vet`, or
`golangci-lint`'s `godot` linter, which only checks comment punctuation, not
semantic placement) but it is a genuine, real defect: a reader of this file
will misread lines 166-167 as describing the function immediately below them.
**Fix:** Move the orphaned comment down to sit above its correct function:
```go
}

// TestPlanner_Plan_ClampsOutOfBoundsStart verifies that a starting position
// outside the display bounds -- as Position() can report on a multi-monitor
// setup -- is clamped, so no glide step lands off the main display.
func TestPlanner_Plan_ClampsOutOfBoundsStart(t *testing.T) {
	...
}

// TestPlanner_Plan_EdgeCase_OnePixelScreen covers a 1x1 screen: the only
// valid point is (0, 0), so every path point must be (0, 0).
func TestPlanner_Plan_EdgeCase_OnePixelScreen(t *testing.T) {
	...
}
```

### IN-02: No test exercises the exact upper-boundary clamp value (`curX == width`)

**File:** `internal/activity/domain/planner_test.go:171-189`
**Issue:** `TestPlanner_Plan_ClampsOutOfBoundsStart` covers a comfortably
out-of-bounds start (`curX=-500`, `curY=5000` against `w=1920,h=1080`), which
exercises both the `v < 0` and `v > size-1` branches of `clamp`, but not the
exact boundary value `v == size` (e.g. `curX = 1920` for `width = 1920`),
which is the smallest input that should still clamp to `size-1` via the
`v > size-1` branch rather than passing through unclamped. The current
implementation is correct at this boundary (verified numerically:
`clamp(1920, 1920) == 1919`), so this is a coverage gap, not a bug.
**Fix:** Add one boundary-value assertion, e.g. extend
`TestPlanner_Plan_ClampsOutOfBoundsStart` (or a small table test) with
`curX = width` and `curY = height` and assert the resulting path's first
point has `X < width` and `Y < height`.

---

_Reviewed: 2026-07-12_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
