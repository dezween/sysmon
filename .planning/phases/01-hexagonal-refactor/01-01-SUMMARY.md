---
phase: 01-hexagonal-refactor
plan: 01
subsystem: infra
tags: [go, cgo, hexagonal-architecture, slog, mockgen, testify, synctest, coregraphics]

# Dependency graph
requires: []
provides:
  - "cmd/sysmon entry point (flags, DI wiring, signal->context, no business logic)"
  - "internal/activity/domain: validated Interval and Offset value types (no cgo/os)"
  - "internal/activity/port: Pointer (driven) and Keeper (driving) interfaces + mockgen directive"
  - "internal/activity/port/mock: generated MockPointer (gomock)"
  - "internal/activity/service.KeeperService implementing Keeper, injected *slog.Logger"
  - "internal/activity/adapter.CGPointer: cgo/CoreGraphics Pointer impl behind //go:build darwin"
  - "Makefile build target pointed at ./cmd/sysmon"
affects: [any future phase touching activity behavior, adapters, or CLI wiring]

# Tech tracking
tech-stack:
  added: [github.com/stretchr/testify (test-only), go.uber.org/mock (test-only, mockgen)]
  patterns:
    - "Ports & adapters (hexagonal): domain -> port <- service, adapter implements port, cmd wires everything"
    - "Constructor-injected *slog.Logger and port.Pointer (no package globals) for testability"
    - "testing/synctest fake-clock bubble for deterministic ticker-loop unit tests"
    - "-quiet raises slog level threshold (LevelWarn) instead of redirecting output stream"
    - "os.Exit(run()) split so deferred cleanup always runs before process exit"

key-files:
  created:
    - cmd/sysmon/main.go
    - internal/activity/domain/activity.go
    - internal/activity/domain/activity_test.go
    - internal/activity/port/pointer.go
    - internal/activity/port/keeper.go
    - internal/activity/port/mock/mock_pointer.go
    - internal/activity/service/keeper_service.go
    - internal/activity/service/keeper_service_test.go
    - internal/activity/adapter/cgpointer.go
  modified:
    - Makefile
    - go.mod
    - go.sum
    - .golangci.yml
    - .gitignore

key-decisions:
  - "Offset modeled as a validated domain.Offset type (not a bare constant) for symmetry with Interval"
  - "nudgePause kept as a plain const in cmd/sysmon, passed through to the service constructor (no domain validation needed, per CONTEXT.md discretion)"
  - "mockgen invoked via the installed `mockgen` binary directly (not `go run go.uber.org/mock/mockgen`) to avoid requiring mockgen's own transitive tool deps in go.sum"
  - "Added formatters.settings.gofumpt.module-path: sysmon to .golangci.yml -- required once internal cross-package imports (sysmon/...) existed; without it golangci-lint's gofumpt formatter cannot distinguish local-module imports from third-party ones"
  - "Scoped .gitignore's binary-ignore rule from bare `sysmon` to `/sysmon` -- the bare pattern was also matching the new cmd/sysmon/ directory anywhere in the tree"

patterns-established:
  - "Domain packages have zero cgo/os import and must GOOS=linux build cleanly"
  - "Service layer depends only on domain+port, verified via `go list -deps` excluding adapter"
  - "cgo/platform code isolated behind //go:build darwin in adapter only"

requirements-completed: [ARCH-01, ARCH-02, ARCH-03, ARCH-04, ARCH-05, TEST-01, BUILD-01, BUILD-02, LOG-01]

coverage:
  - id: D1
    description: "domain package: validated Interval/Offset value types, GOOS=linux buildable, no cgo/os"
    requirement: "ARCH-04"
    verification:
      - kind: unit
        ref: "internal/activity/domain/activity_test.go#TestNewInterval,TestNewOffset"
        status: pass
      - kind: other
        ref: "GOOS=linux GOARCH=amd64 go build ./internal/activity/domain/..."
        status: pass
    human_judgment: false
  - id: D2
    description: "port package: Pointer/Keeper interfaces + committed mockgen //go:generate directive"
    requirement: "ARCH-02, ARCH-03"
    verification:
      - kind: other
        ref: "go build ./internal/activity/port/... && grep go:generate pointer.go"
        status: pass
    human_judgment: false
  - id: D3
    description: "generated MockPointer (mockgen, DO NOT EDIT header) plus testify/go.uber.org/mock as test-only go.mod deps"
    requirement: "TEST-01"
    verification:
      - kind: other
        ref: "test -f internal/activity/port/mock/mock_pointer.go && head -3 shows DO NOT EDIT"
        status: pass
    human_judgment: false
  - id: D4
    description: "KeeperService implements Keeper with injected *slog.Logger; nudges-per-tick and stop-on-cancel proven via synctest+gomock+testify, no real mouse/Accessibility"
    requirement: "ARCH-03, TEST-01, LOG-01"
    verification:
      - kind: unit
        ref: "internal/activity/service/keeper_service_test.go#TestKeeperService_NudgesPerTick"
        status: pass
      - kind: unit
        ref: "internal/activity/service/keeper_service_test.go#TestKeeperService_StopsOnContextCancel"
        status: pass
      - kind: other
        ref: "go list -deps sysmon/internal/activity/service | grep -v adapter"
        status: pass
    human_judgment: false
  - id: D5
    description: "CGPointer cgo/CoreGraphics adapter behind //go:build darwin, both CFRelease calls preserved verbatim"
    requirement: "ARCH-02"
    verification:
      - kind: other
        ref: "grep -c CFRelease cgpointer.go == 2; head -1 == //go:build darwin; diff of nudge() C body vs original main.go byte-identical"
        status: pass
    human_judgment: false
  - id: D6
    description: "cmd/sysmon wiring: flags, slog level via -quiet, signal.NotifyContext, manual DI, root main.go removed"
    requirement: "ARCH-01, ARCH-05, BUILD-02, LOG-01"
    verification:
      - kind: other
        ref: "go build ./cmd/sysmon; grep NotifyContext/LevelWarn; ! grep SetOutput; test ! -f main.go"
        status: pass
      - kind: manual_procedural
        ref: "manual smoke: ./sysmon -interval 500ms then SIGTERM -> logs started/stopped, exit code 0; -quiet -> no stdout output"
        status: pass
    human_judgment: false
  - id: D7
    description: "Makefile build target -> ./cmd/sysmon; full gate (go build, golangci-lint, go test -race, make build) passes clean"
    requirement: "BUILD-01"
    verification:
      - kind: other
        ref: "go build ./... && golangci-lint run ./... (0 issues) && go test -race ./... && make build && test -x ./sysmon"
        status: pass
    human_judgment: false
  - id: D8
    description: "Real cursor nudge, screen-awake, -quiet silence on real hardware, and Makefile start/status/stop/clean cycle (Task 8)"
    verification: []
    human_judgment: true
    rationale: "Requires a real Mac with Accessibility permission granted to observe actual cursor movement and confirm the screen does not sleep -- automated tests explicitly cannot and must not touch the darwin adapter (per CONTEXT.md/RESEARCH.md). Pending human verification (Task 8 checkpoint, not yet run)."

# Metrics
duration: 9min
completed: 2026-07-12
status: complete
---

# Phase 1 Plan 1: Hexagonal Refactor Summary

**Restructured the flat root `main.go` into a hexagonal ports-&-adapters layout (cmd/sysmon + internal/activity/{domain,port,service,adapter}), added a mockgen+testify+synctest deterministic unit test for the ticker loop, and wired in injected `log/slog` logging — with zero observable runtime behavior change (verified by manual smoke test).**

## Performance

- **Duration:** 9 min (first commit 02:10:31 -> last commit 02:19:06, local time)
- **Started:** 2026-07-12T02:10:31+03:00
- **Completed:** 2026-07-12T02:19:06+03:00 (Tasks 1-7; Task 8 human-verify checkpoint still pending)
- **Tasks:** 7 of 8 completed (Task 8 is a blocking human-verify checkpoint, not executable by the agent)
- **Files modified:** 14 (9 created, 5 modified)

## Accomplishments
- Full hexagon in place: `domain` (pure, GOOS=linux-buildable), `port` (Pointer/Keeper interfaces + committed mockgen directive), `service` (KeeperService depending only on domain+port), `adapter` (cgo CoreGraphics behind `//go:build darwin`), `cmd/sysmon` (wiring-only entry point). Root `main.go` removed.
- `KeeperService` is now unit-testable without a real mouse or macOS Accessibility: `go test -race ./internal/activity/service/...` proves nudges-per-tick and stop-on-context-cancel using `testing/synctest`'s fake clock, a mockgen-generated `MockPointer`, and testify assertions.
- `log/slog` injected into `KeeperService` via constructor (never a package global); `-quiet` raises the handler's level threshold to `LevelWarn` instead of redirecting the output stream, matching the locked LOG-01 design.
- `go build ./...`, `golangci-lint run ./...` (0 issues), `go test -race ./...`, and `make build` all pass clean on the target Mac.

## Task Commits

Each task was committed atomically:

1. **Task 1: Create the pure domain package** - `628acc6` (feat)
2. **Task 2: Define the driven and driving ports** - `0985f67` (feat)
3. **Task 3: Add test-only deps and generate the Pointer mock** - `8445690` (test)
4. **Task 4: Implement KeeperService with injected slog logger + synctest test** - `35e1bfe` (feat)
5. **Task 5: Relocate the cgo CoreGraphics adapter** - `61dd982` (feat)
6. **Task 6: Create cmd/sysmon/main.go wiring, remove root main.go** - `b33f14c` (feat)
7. **Task 7: Update Makefile build target, run full gate, fix lint findings** - `547ae3f` (build)

**Plan metadata:** this SUMMARY.md commit (pending)

Task 8 (human-verify checkpoint) is NOT executed by this run — it requires a human on the target Mac. See "Next Phase Readiness" below.

## Files Created/Modified
- `cmd/sysmon/main.go` - New entry point: flags, slog setup (level via -quiet), signal.NotifyContext, manual DI, `os.Exit(run())` split for clean deferred cleanup
- `internal/activity/domain/activity.go` - `Interval` and `Offset` validated value types (reject non-positive)
- `internal/activity/domain/activity_test.go` - Table-driven tests (testify) for both constructors
- `internal/activity/port/pointer.go` - `Pointer` interface (`Nudge(dx, dy int) error`) + `//go:generate mockgen` directive
- `internal/activity/port/keeper.go` - `Keeper` interface (`Run(ctx context.Context) error`)
- `internal/activity/port/mock/mock_pointer.go` - Generated `MockPointer` (mockgen, DO NOT EDIT header, lint-exempt)
- `internal/activity/service/keeper_service.go` - `KeeperService` implementing `Keeper`, ticker loop, injected logger/pointer
- `internal/activity/service/keeper_service_test.go` - `synctest` + gomock + testify tests for nudge-per-tick and stop-on-cancel
- `internal/activity/adapter/cgpointer.go` - `CGPointer` cgo/CoreGraphics implementation, `//go:build darwin`, both `CFRelease` calls preserved verbatim
- `Makefile` - `build` target now points at `./cmd/sysmon`
- `go.mod` / `go.sum` - Added test-only deps `github.com/stretchr/testify`, `go.uber.org/mock` (never linked into the production binary)
- `.golangci.yml` - Added `formatters.settings.gofumpt.module-path: sysmon`
- `.gitignore` - Scoped the binary-ignore rule from bare `sysmon` to `/sysmon`
- `main.go` (root) - **Deleted**

## Decisions Made
- `Offset` modeled as a validated domain type (`domain.Offset`, `NewOffset`) rather than a bare constant, for symmetry with `Interval` and because CONTEXT.md left this to discretion with either option acceptable.
- `nudgePause` (40ms) kept as a plain `time.Duration` const in `cmd/sysmon`, not promoted to a domain type — it has no user-facing flag or validation rule, per CONTEXT.md's "keep it small" guidance.
- The `//go:generate` directive invokes the installed `mockgen` binary directly rather than `go run go.uber.org/mock/mockgen`, since the latter requires mockgen's full transitive tool dependency graph (`golang.org/x/tools`, `golang.org/x/mod`) in `go.sum` for no benefit in this project.
- `main()` split into `os.Exit(run())` plus a `run() int` function so `defer stop()` (the signal-notify context cleanup) always executes before the process exits — `os.Exit` called directly inside `main()` would have skipped it.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking issue] Pulled testify's go.mod dependency forward from Task 3 into Task 1**
- **Found during:** Task 1 (domain package tests)
- **Issue:** Task 1's own `<verify>` block requires `go test ./internal/activity/domain/...` to pass, but the test file uses `testify` assertions, and testify is only formally added to `go.mod` in Task 3. Without it, Task 1 cannot be verified as written.
- **Fix:** Ran `go get github.com/stretchr/testify` and `go mod tidy` during Task 1 so its own test could compile and pass; Task 3 then added `go.uber.org/mock` and generated the mock on top of the already-present testify dependency.
- **Files modified:** `go.mod`, `go.sum`
- **Verification:** `go test ./internal/activity/domain/...` passes; `go mod tidy` run again in Task 3 confirms consistency.
- **Committed in:** `628acc6` (Task 1 commit)

**2. [Rule 3 - Blocking issue] `.gitignore`'s bare `sysmon` pattern silently blocked `cmd/sysmon/`**
- **Found during:** Task 6 (creating `cmd/sysmon/main.go`)
- **Issue:** `.gitignore` had both `/sysmon` (root binary only) and a bare `sysmon` line (intended as a belt-and-suspenders duplicate), but the bare pattern matches any file or directory named `sysmon` anywhere in the tree — including the new `cmd/sysmon/` package directory, which `git add` then silently refused to stage.
- **Fix:** Removed the redundant bare `sysmon` line, keeping only `/sysmon` (root-level binary).
- **Files modified:** `.gitignore`
- **Verification:** `git check-ignore -v cmd/sysmon/main.go` now exits 1 (not ignored); `cmd/sysmon/main.go` stages and commits normally.
- **Committed in:** `b33f14c` (Task 6 commit)

**3. [Rule 1 - Bug] `gocritic` exitAfterDefer: `os.Exit(1)` inside `main()` skipped `defer stop()`**
- **Found during:** Task 7 (full quality gate)
- **Issue:** `golangci-lint run ./...` flagged `cmd/sysmon/main.go`: calling `os.Exit(1)` directly inside `main()` on a `keeper.Run` error bypasses the deferred `stop()` call for the `signal.NotifyContext`-returned cancel function — a real (if low-impact, since the process is exiting anyway) resource-cleanup-order bug.
- **Fix:** Split `main()` into a thin `os.Exit(run())` wrapper and a `run() int` function that returns an exit code; all `defer`s inside `run()` now execute before the process exits.
- **Files modified:** `cmd/sysmon/main.go`
- **Verification:** `golangci-lint run ./...` no longer flags `exitAfterDefer`; manual smoke test confirms SIGTERM still yields a clean exit code 0 and the "started"/"stopped" logs still print.
- **Committed in:** `547ae3f` (Task 7 commit)

**4. [Rule 3 - Blocking issue] `golangci-lint`'s gofumpt formatter misformatted files with local (`sysmon/...`) imports**
- **Found during:** Task 7 (full quality gate)
- **Issue:** `golangci-lint run ./...` flagged 4 files with "File is not properly formatted (gofumpt)" pointing at import lines, even though the standalone `gofumpt` binary and `golangci-lint fmt ./...` both reported the same files as already correctly formatted. Root cause (traced into golangci-lint v2.8.0's vendored `mvdan.cc/gofumpt` source): `formatters.settings.gofumpt.module-path` was unset in `.golangci.yml`, so gofumpt's import-grouping logic could not recognize `sysmon/...` imports as belonging to the local module and treated them inconsistently between its own direct invocation and golangci-lint's wrapped analyzer path. This only started mattering once this refactor introduced internal cross-package imports (`sysmon/internal/activity/...`) for the first time — the original single-file `main.go` had no local imports to trigger it.
- **Fix:** Added `formatters.settings.gofumpt.module-path: sysmon` to `.golangci.yml`. Also normalized import group order (local packages before third-party) in `keeper_service_test.go` to match `goimports`'s `local-prefixes: [sysmon]` setting, applied via `golangci-lint fmt ./...`.
- **Files modified:** `.golangci.yml`, `internal/activity/service/keeper_service_test.go`
- **Verification:** `golangci-lint run ./...` reports `0 issues` after the fix.
- **Committed in:** `547ae3f` (Task 7 commit)

---

**Total deviations:** 4 auto-fixed (2 Rule 3 blocking-issue fixes, 1 Rule 1 bug fix, 1 Rule 3 tooling-config fix)
**Impact on plan:** All four were necessary to make the plan's own verification gates pass as written; none changed the locked architecture, ports, or observable CLI behavior. No scope creep.

## Issues Encountered
None beyond the deviations documented above.

## User Setup Required
None - no external service configuration required. Xcode Command Line Tools and macOS Accessibility permission (already granted per the task's stated environment) remain the only platform prerequisites, unchanged from before this refactor.

## Next Phase Readiness

**Task 8 (human-verify checkpoint) is PENDING and was intentionally not attempted by this execution.** All automated gates required before Task 8 are green:
- `go build ./...` — pass
- `golangci-lint run ./...` — 0 issues
- `go test -race ./...` — pass (including both KeeperService sub-tests and domain tests)
- `GOOS=linux GOARCH=amd64 go build ./internal/activity/domain/... ./internal/activity/port/... ./internal/activity/service/...` — pass
- `make build` — pass, produces `./sysmon`
- Manual smoke (informal, not a substitute for Task 8): `./sysmon -interval 500ms` then SIGTERM logs "started"/"stopped" and exits 0; `-quiet` produces zero stdout output.

Remaining before this plan is fully done: a human must run the Task 8 checklist on the target Mac (real cursor nudge visible, screen stays awake, `-quiet` silent, SIGINT/SIGTERM exit 0, and the full `make start && make status && make stop && make clean` background cycle), then respond with "approved" (or describe any misbehavior) to close out Phase 1 Plan 1.

---
*Phase: 01-hexagonal-refactor*
*Completed: 2026-07-12 (Tasks 1-7; Task 8 pending human verification)*

## Self-Check: PASSED

All 14 claimed files verified present on disk (including confirming root `main.go` is absent), and all 7 claimed task commit hashes (`628acc6`, `0985f67`, `8445690`, `35e1bfe`, `61dd982`, `b33f14c`, `547ae3f`) verified present in `git log`.
