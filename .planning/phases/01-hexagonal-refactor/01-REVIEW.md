---
phase: 01-hexagonal-refactor
reviewed: 2026-07-12T00:00:00Z
depth: standard
files_reviewed: 10
files_reviewed_list:
  - cmd/sysmon/main.go
  - internal/activity/domain/activity.go
  - internal/activity/domain/activity_test.go
  - internal/activity/port/pointer.go
  - internal/activity/port/keeper.go
  - internal/activity/service/keeper_service.go
  - internal/activity/service/keeper_service_test.go
  - internal/activity/adapter/cgpointer.go
  - Makefile
  - .golangci.yml
findings:
  critical: 1
  warning: 2
  info: 2
  total: 5
status: issues_found
---

# Phase 1: Code Review Report

**Reviewed:** 2026-07-12
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

The hexagonal split itself is clean: `domain` has zero cgo/os/port/service/adapter
imports and builds under `GOOS=linux` (verified), `service` depends only on
`domain`... except it doesn't, see CR-01 below... and `port`, never `adapter`
(verified via `go list -deps`), `adapter` is the sole cgo package, correctly
tagged `//go:build darwin`, and both `CFRelease` calls are preserved verbatim
from the original `main.go`. `go vet`, `go build ./...`, and `golangci-lint run
./...` are all clean, and `go test -race ./...` passes.

However, tracing the actual data flow from `cmd/sysmon/main.go` into
`service.NewKeeperService` turns up one real, provable regression: the new
`domain.Interval`/`domain.Offset` validation is never invoked anywhere outside
its own test file. `main.go` passes the raw `*flag.Duration` and an untyped
`int` constant straight into the service, bypassing the domain package
entirely. This isn't just dead code (Info-tier) — because `flag.Duration`
accepts negative values and `time.NewTicker` panics on non-positive durations,
a user running `sysmon -interval=-5s` (or `-interval=0s`) now crashes with an
unhandled panic, which the pre-refactor `main.go` also would have crashed on,
but the entire point of adding `domain.Interval` in this phase was to prevent
exactly that class of bug ("preventing a busy-loop or a panic from a
non-positive time.Ticker" per its own doc comment). The validation was built
but never wired in, so the phase's own stated goal (ARCH-04) is unmet in
practice despite the unit tests for `domain` passing in isolation.

Two lower-severity findings: the ticker loop's `time.Sleep(k.pause)` between
the two nudges is not selected against `ctx.Done()`, so shutdown latency can
be delayed by up to `pause` (bounded, small, but avoidable); and the second
`Nudge` (the "nudge back") on tick has no compensating action if it fails,
same as before, which is fine but the resulting drift is now unbounded
un-nudged-back state that's harder to reason about than the pre-refactor
inline code.

## Critical Issues

### CR-01: domain.Interval/domain.Offset validation exists but is never called from production code — negative/zero interval panics

**File:** `cmd/sysmon/main.go:37,51`
**Issue:** `flag.Duration("interval", ...)` accepts any `time.Duration`, including
zero and negative values, with no validation. `run()` passes `*interval`
directly to `service.NewKeeperService` (`keeper_service.go:28`), which stores
it as a raw `time.Duration` and later calls `time.NewTicker(k.interval)`
(`keeper_service.go:44`). `time.NewTicker` panics if its argument is `<= 0`.
`domain.NewInterval` was built in this phase specifically to reject `d <= 0`
(`activity.go:28-33`, doc comment: "preventing a busy-loop or a panic from a
non-positive time.Ticker"), but neither `main.go` nor
`service.KeeperService` ever calls `domain.NewInterval` or
`domain.NewOffset` — confirmed by `grep -rl "internal/activity/domain"
--include="*.go" | grep -v _test.go` returning zero non-test files, and by
reproduction: `./sysmon -interval=-5s` panics with
`panic: non-positive interval for NewTicker`.
This is a regression versus even the flat pre-refactor `main.go` in spirit:
the phase explicitly added this guard rail (ARCH-04, "Interval that validates
> 0") and it is inert. `KeeperService.interval`/`offset` are plain
`time.Duration`/`int` fields, not `domain.Interval`/`domain.Offset`, so there
is no type-level enforcement either.
**Fix:** Have `run()` construct a `domain.Interval` from the flag value and
handle the error before constructing the service (print a usage error and
`return 1` rather than panicking), and change `KeeperService`'s constructor
signature to accept `domain.Interval`/`domain.Offset` instead of raw
primitives so this can't regress silently again:
```go
// cmd/sysmon/main.go
iv, err := domain.NewInterval(*interval)
if err != nil {
    logger.Error("invalid -interval", "err", err)
    return 1
}
off, err := domain.NewOffset(nudgeOffset)
if err != nil {
    logger.Error("invalid nudge offset", "err", err)
    return 1
}
keeper := service.NewKeeperService(pointer, logger, iv, off, nudgePause)
```
```go
// internal/activity/service/keeper_service.go
func NewKeeperService(pointer port.Pointer, logger *slog.Logger, interval domain.Interval, offset domain.Offset, pause time.Duration) *KeeperService {
    return &KeeperService{
        pointer:  pointer,
        logger:   logger,
        interval: interval.Duration(),
        offset:   offset.Int(),
        pause:    pause,
    }
}
```
Update `keeper_service_test.go` call sites accordingly (they currently pass
raw `int`/`time.Duration` literals too, so they'd need `domain.NewInterval`/
`domain.NewOffset` helpers or exported test constructors).

## Warnings

### WR-01: Shutdown can be delayed up to `pause` because `time.Sleep` isn't selected against `ctx.Done()`

**File:** `internal/activity/service/keeper_service.go:56`
**Issue:** Inside the `case <-ticker.C:` branch, after the first `Nudge`
succeeds, the code calls `time.Sleep(k.pause)` unconditionally before issuing
the second `Nudge`. If `ctx` is cancelled during that sleep (e.g. SIGTERM
arrives 1ms into a 40ms pause), the cancellation is not observed until the
sleep finishes and the loop re-enters `select`. This is bounded by `pause`
(40ms default) so it's not severe, but it's a real, avoidable deviation from
"stops cleanly on ctx cancellation" as stated in the package doc comment
(`keeper_service.go:15-16`), and it silently degrades the "clean SIGINT/SIGTERM
exit" behavior preserved from the original by up to `pause` on every run that
happens to be cancelled mid-nudge.
**Fix:** Race the sleep against `ctx.Done()`:
```go
select {
case <-time.After(k.pause):
case <-ctx.Done():
    k.logger.Info("sysmon stopped")
    return nil
}
```
Given `pause` is only 40ms, this is low-impact and could reasonably be
accepted as-is, but it should be a conscious tradeoff, not an oversight.

### WR-02: KeeperService constructor takes 5 positional same-typed-adjacent params, inviting argument-order mistakes

**File:** `internal/activity/service/keeper_service.go:28`
**Issue:** `NewKeeperService(pointer port.Pointer, logger *slog.Logger,
interval time.Duration, offset int, pause time.Duration)` has two
`time.Duration` parameters (`interval`, `pause`) separated only by an `int`.
Nothing at the call site or type system prevents swapping `interval` and
`pause` (both compile fine either order) — e.g.
`NewKeeperService(pointer, logger, pause, offset, interval)` would silently
tick every 40ms and pause for 10s between nudges. This is exactly the class of
bug the locked design's domain types (`Interval`) were meant to close off
(see CR-01) — once `domain.Interval` is actually threaded through as
recommended in CR-01's fix, this concern is resolved as a side effect since
`domain.Interval` and a raw `pause time.Duration` are no longer
interchangeable by accident... but `pause` itself is still a bare
`time.Duration` type-identical to nothing else, so this is a narrower residual
concern once CR-01 lands. Filed separately because it stands on its own even
if CR-01's fix is deferred.
**Fix:** Adopt CR-01's fix (typed `domain.Interval`/`domain.Offset`
parameters), which also makes `interval`/`pause` non-interchangeable at
the type level since only `interval` becomes `domain.Interval`. If `pause`
stays a raw `time.Duration`, consider a small named type or a functional-options
constructor to remove positional ambiguity entirely.

## Info

### IN-01: `nudgeOffset` and `nudgePause` are untyped/unvalidated constants duplicated between `main.go` and tests

**File:** `cmd/sysmon/main.go:24,26`
**Issue:** `nudgeOffset = 1` and `nudgePause = 40 * time.Millisecond` are
plain package-level consts in `cmd/sysmon`, matching CONTEXT.md's explicit
discretion note ("no domain validation needed"). This is acceptable per the
locked design, but worth flagging alongside CR-01: since `domain.Offset`
exists and is unused, there's now redundant modeling (a validated `Offset`
type nobody constructs, and a bare `int` constant that's actually used) that
reads as unfinished rather than intentional. Once CR-01's fix is applied this
tension disappears since `nudgeOffset` would flow through `domain.NewOffset`.
**Fix:** No standalone action needed if CR-01 is fixed; otherwise, remove
`domain.Offset`/`NewOffset` to avoid presenting unused validation as if it
were live.

### IN-02: `KeeperService.Run` continues the loop after a failed forward `Nudge`, silently skipping the compensating back-nudge

**File:** `internal/activity/service/keeper_service.go:52-55`
**Issue:** If the first `Nudge(k.offset, 0)` returns an error, the code logs
and `continue`s — correctly skipping `time.Sleep` and the second `Nudge` since
there's nothing to compensate for. This matches the original's implicit
behavior (the original never checked errors at all) and is not a bug, but the
asymmetry is worth documenting: if the forward nudge partially succeeds at the
OS level but still returns an error (unlikely with the current `CGPointer`,
which never returns non-nil, but possible for a future adapter), the cursor
could drift by `offset` px per such event with no recovery path. Not
actionable now since `CGPointer.Nudge` always returns `nil` (`cgpointer.go:38-41`),
but flagging for future adapters.
**Fix:** No change required today. If a future `Pointer` implementation can
fail after partially moving the cursor, consider always attempting the
compensating `Nudge(-k.offset, 0)` even when the forward call errors.

---

_Reviewed: 2026-07-12_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
