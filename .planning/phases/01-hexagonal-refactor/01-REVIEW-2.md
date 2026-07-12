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
  critical: 0
  warning: 1
  info: 1
  total: 2
status: issues_found
---

# Phase 1: Code Review Report (second pass, post-fix)

**Reviewed:** 2026-07-12
**Depth:** standard
**Files Reviewed:** 10
**Status:** issues_found

## Summary

All three findings from review #1 are genuinely and correctly resolved, with
no regressions introduced.

- **CR-01 (domain validation never wired)** — fixed. `run()` now calls
  `domain.NewInterval(*interval)` and `domain.NewOffset(nudgeOffset)`
  (`main.go:56-65`) and exits with code 2 and a clear `slog` error on failure
  instead of reaching `time.NewTicker`. Verified by reproduction:
  `./sysmon -interval=-5s` and `-interval=0s` both now print
  `level=ERROR msg="invalid interval" err="interval must be greater than zero"`
  and exit 2, no panic. `KeeperService` was retyped to accept
  `domain.Interval`/`domain.Offset` directly (`keeper_service.go:35`), so the
  raw-primitive bypass this finding described is closed at the type level, not
  just at the one call site in `main.go`.
- **WR-02 (positional-argument-order footgun)** — fixed as a side effect of the
  above. `NewKeeperService(pointer port.Pointer, logger *slog.Logger, interval
  domain.Interval, offset domain.Offset)` no longer has two adjacent
  `time.Duration`-typed parameters; `nudgePause` was demoted from a
  constructor parameter to an unexported package constant
  (`keeper_service.go:18`), so there is nothing left to swap by accident.
- **WR-01 (uninterruptible `time.Sleep(pause)` delaying shutdown)** — fixed
  correctly. The pause is now `select { case <-time.After(nudgePause):
  case <-ctx.Done(): }` inside the new `nudgeCycle` helper
  (`keeper_service.go:84-87`), and `Run`'s ticker branch checks `ctx.Err()`
  immediately after `nudgeCycle` returns and exits promptly
  (`keeper_service.go:58-65`) rather than looping back to `select` and waiting
  for the next tick. Confirmed no double "sysmon stopped" log is possible:
  once the ticker branch returns, the `ctx.Done()` branch is never reached.
- Layering, cgo, and build hygiene are all still clean: `domain` has zero
  port/service/adapter imports, `service` doesn't import `adapter` (verified
  via `go list -deps`), both `CFRelease` calls are intact in `cgpointer.go`,
  `//go:build darwin` is present, and `go build`, `go vet`,
  `golangci-lint run ./...` (0 issues), and `go test -race ./...` are all
  clean.

One design choice in the WR-01 fix introduces a new, narrower gap that wasn't
present in review #1's assessment (since the interruptible pause didn't exist
yet): `nudgeCycle` still fires the compensating back-`Nudge` even when
`ctx.Done()` is what woke it from the pause, and this specific interleaving
(cancel arriving strictly between the forward and backward nudge) is not
exercised by either test. It's a reasonable behavior choice (matches the
pre-refactor original's "always complete the pair" semantics more closely
than an early return would), but it's undocumented and untested, so it's
filed as a Warning rather than accepted silently.

## Warnings

### WR-01: New interruptible-pause branch (nudgeCycle mid-pause cancellation) is untested

**File:** `internal/activity/service/keeper_service.go:83-91`
**Issue:** `nudgeCycle`'s pause is now raced against `ctx.Done()`:
```go
select {
case <-time.After(nudgePause):
case <-ctx.Done():
}
if err := k.pointer.Nudge(-offset, 0); err != nil {
    k.logger.Error("nudge back failed", "err", err)
}
```
Regardless of which branch fires, the backward `Nudge` still executes
unconditionally afterward. This is a defensible choice (it completes the
there-and-back pair rather than leaving the cursor 1px offset, matching how
the pre-refactor `main.go` always completed both `C.nudge` calls before ever
checking the signal channel), but neither existing test exercises this
interleaving:
- `TestKeeperService_StopsOnContextCancel` cancels before any tick ever
  fires (1s interval, cancelled immediately after the first `synctest.Wait()`),
  so it only exercises `Run`'s outer `<-ctx.Done()` select branch, never
  `nudgeCycle`'s internal one.
- `TestKeeperService_NudgesPerTick` cancels only after all 3 expected ticks'
  nudge pairs have fully completed (`time.Sleep(ticks*interval)` then
  `synctest.Wait()` before `cancel()`), so the pause's `ctx.Done()` arm is
  never selected either.

As a result, the exact code path review #1's WR-01 asked to add (interruptible
pause) is present but has zero test coverage of its cancellation arm, and the
doc comment ("the pause between the two nudges is interruptible so shutdown is
not delayed by up to nudgePause", `keeper_service.go:74`) slightly overstates
promptness: shutdown latency is still bounded by however long the second
`Nudge` call takes to return (unbounded in principle for a future `Pointer`
implementation, though negligible for `CGPointer`), since it's called
unconditionally after the race resolves either way.
**Fix:** Add a synctest case that cancels ctx strictly inside the 40ms pause
window (e.g. schedule a tick, let `nudgeCycle` start, `time.Sleep(20*time.
Millisecond)` then `cancel()` before the pause's `time.After(nudgePause)`
fires) and assert the backward `Nudge` still occurs and `Run` returns
promptly. If the "always complete the pair" behavior is intentional, say so
explicitly in the doc comment on `nudgeCycle` rather than implying early
return; if early return (skip the back-nudge on cancel) is actually preferred,
change the code to `return` in the `ctx.Done()` case instead.

## Info

### IN-01: `nudgeCycle` doc comment overstates what "interruptible" guarantees

**File:** `internal/activity/service/keeper_service.go:73-74`
**Issue:** The comment "The pause between the two nudges is interruptible so
shutdown is not delayed by up to nudgePause" is true only for the pause
itself; it does not mention that the backward `Nudge` call still runs after
an interrupted pause, which is the behavior that actually determines total
shutdown latency in that branch. A future reader (or the next reviewer) could
reasonably assume cancellation short-circuits the entire cycle.
**Fix:** Tighten the comment, e.g.: "The pause between the two nudges races
against ctx cancellation so it doesn't block for the full nudgePause on
shutdown; the compensating back-nudge still runs afterward either way, to
avoid leaving the cursor offset."

---

_Reviewed: 2026-07-12_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
