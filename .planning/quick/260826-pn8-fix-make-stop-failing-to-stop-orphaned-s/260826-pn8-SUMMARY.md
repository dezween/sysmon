---
phase: quick-260826-pn8
plan: 01
subsystem: infra
tags: [makefile, process-management, pgrep, macos]

# Dependency graph
requires: []
provides:
  - "PGREP helper variable as single liveness source of truth in Makefile"
  - "make stop hardened to kill via live-scan when pidfile is missing/stale"
  - "make start refuses double-launch (tracked or untracked orphan)"
  - "make status reports 4 distinct liveness states"
  - "make clean chained to stop, cannot strand a running process"
affects: [makefile, process-lifecycle]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "pgrep -x -U $(id -u) BINARY as exact-name, current-user liveness scan (never -f, never pkill)"
    - "pidfile trusted only when its PID is a member of the live-scan output (whole-line literal grep -qxF match), defeating PID recycling"

key-files:
  created: []
  modified:
    - Makefile

key-decisions:
  - "SUPERSEDES STATE.md decision \"pidfile-based start/stop in Makefile retained as-is (not touched by this refactor)\": the pidfile is kept as a convenience label (still written by start, still read for display), but liveness is now determined solely by an exact-name, current-user pgrep scan; the pidfile's PID is only ever acted on when it is corroborated by that scan."
  - "Used pgrep -x (exact process-name match) instead of -f (full command-line match) because -f matches the unrelated macOS daemon /usr/libexec/sysmond -- verified live on this machine."
  - "stop always signals the full live-scan PID list, not just the tracked PID, so untracked orphans are cleared alongside a tracked instance in one call."
  - "stop confirms termination with a bounded 10 x 0.2s poll loop before removing the pidfile; a failed kill leaves the pidfile in place and exits 1, so clean (which now depends on stop) aborts before deleting anything."

patterns-established:
  - "Pattern 1: liveness source of truth is a live-process scan (PGREP), not file state; the pidfile is demoted to a corroborated label."

requirements-completed: [QUICK-260826-pn8]

coverage:
  - id: D1
    description: "make stop kills a live sysmon whether or not the pidfile exists or is accurate, reporting which path (tracked vs live-scan) it used"
    requirement: "QUICK-260826-pn8"
    verification:
      - kind: manual_procedural
        ref: "make -s build; make -s start; rm -f /tmp/sysmon.pid; make -s stop (task 1 automated verify)"
        status: pass
    human_judgment: false
  - id: D2
    description: "make clean runs stop first and cannot strand a running process"
    requirement: "QUICK-260826-pn8"
    verification:
      - kind: manual_procedural
        ref: "make -s start; make -s clean; pgrep -x -U $(id -u) sysmon (task 1 automated verify)"
        status: pass
    human_judgment: false
  - id: D3
    description: "make start refuses with non-zero exit when a live sysmon (tracked or untracked) already exists, instead of orphaning it"
    requirement: "QUICK-260826-pn8"
    verification:
      - kind: manual_procedural
        ref: "make -s start twice in a row must fail on the second call (task 2 automated verify)"
        status: pass
    human_judgment: false
  - id: D4
    description: "make status distinguishes not-running, stale-pidfile, running-tracked, and running-UNTRACKED states"
    requirement: "QUICK-260826-pn8"
    verification:
      - kind: manual_procedural
        ref: "task 2 automated verify: greps for 'not running', 'running, tracked', 'running, UNTRACKED'"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-08-26
status: complete
---

# Quick Task 260826-pn8: Fix make stop failing to stop orphaned sysmon Summary

**Rebuilt Makefile process lifecycle (start/stop/status/clean) around an exact-name, current-user `pgrep` liveness scan, demoting the pidfile from source-of-truth to a corroborated label.**

## Performance

- **Duration:** 12 min
- **Started:** 2026-08-26T15:25:00Z (approx)
- **Completed:** 2026-08-26T15:37:44Z
- **Tasks:** 2
- **Files modified:** 1

## Accomplishments
- `make stop` now kills any live `sysmon` process even when `/tmp/sysmon.pid` is missing, stale, or holds a recycled PID -- fixing the reported bug where an orphaned process (PID 70263) survived `make stop` while it printed "sysmon is not running".
- `make start` refuses to launch a second instance (exit 1, message to stderr) whether the existing instance is tracked by the pidfile or is an untracked orphan discovered by the live scan.
- `make status` now reports four distinct, mutually exclusive states: not running, not running with a stale pidfile, running+tracked (with a bonus warning line if untracked orphans coexist), and running+UNTRACKED.
- `make clean` now depends on `stop`, so it can never delete files while leaving a process running; a failed stop aborts clean before any deletion.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add the live-scan helper, rebuild stop around it, and chain clean to stop** - `0ab0a9d` (fix)
2. **Task 2: Guard start against double-launch and make status report real liveness** - `9582fa7` (fix)

**Plan metadata:** committed separately by the orchestrator (docs commit not made by this executor per constraints).

## Files Created/Modified
- `Makefile` - Added `PGREP` helper variable; rewrote `stop` to scan-then-kill-then-confirm; guarded `start` against double-launch; rewrote `status` to report 4 liveness states; chained `clean: stop`.

## Decisions Made
- **Supersedes STATE.md decision.** The STATE.md line "pidfile-based start/stop in Makefile retained as-is (not touched by this refactor)" is superseded by this plan. The pidfile is retained as a convenience label (still written on start, still read and displayed), but it is no longer the sole or authoritative liveness signal. Liveness is now determined by `pgrep -x -U $(id -u) sysmon` (exact process name, current user only), and the pidfile's PID is honoured only when it appears in that live scan's output -- defeating both staleness (deleted/overwritten pidfile) and PID recycling (a stale pidfile pointing at an unrelated, later-spawned process with the same PID).
- Used `pgrep -x` (exact name match) rather than `-f` (full command-line match): `-f` matches the unrelated macOS daemon `/usr/libexec/sysmond`, verified live on this machine during planning and re-confirmed during execution (`pgrep -fl sysmon` -> `92862 /usr/libexec/sysmond`; `pgrep -x -U 501 sysmon` -> nothing).
- Never used `pkill` -- `stop` needs to capture and report the exact PID list it acted on, which requires the scan-then-kill pattern rather than a fire-and-forget pattern match.

## Deviations from Plan

None - plan executed exactly as written. The only deliberate implementation choice within the plan's stated flexibility was using a `for i in 1 2 3 4 5 6 7 8 9 10; do ... done` loop (rather than a `while` counter) for the 10x0.2s termination-confirmation retry, per the plan's explicit "for loop does not create a subshell" guidance.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Bug is fixed and verified: both task-level automated verify commands print PASS, and the human-check sequence in the plan's `<verification>` block was independently exercised via the automated verify commands (idle stop, double-start refusal, orphan detection, orphan stop, clean-after-start).
- No leftover `sysmon` process or `/tmp/sysmon.pid` file remains on the host after execution (`pgrep -x -U $(id -u) sysmon` returns nothing, exit 1).
- No further work identified; this is a standalone quick-task fix.

---
*Phase: quick-260826-pn8*
*Completed: 2026-08-26*

## Self-Check: PASSED

- FOUND: Makefile
- FOUND: .planning/quick/260826-pn8-fix-make-stop-failing-to-stop-orphaned-s/260826-pn8-SUMMARY.md
- FOUND: 0ab0a9d (task 1 commit)
- FOUND: 9582fa7 (task 2 commit)
