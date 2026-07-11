# Phase 1 — Hexagonal Refactor — CONTEXT

**Source:** Synthesized from `docs/TASK.md` §10, PROJECT.md Key Decisions, and the
`log/slog` decision (design already locked earlier; interactive discuss-phase skipped
as redundant).

## Phase Boundary

Restructure the existing, working flat `main.go` into a hexagonal (ports &
adapters) layout, add a unit test, and wire in structured logging — **without
changing observable runtime behavior**. This is a pure refactor + tests +
logging. No new features.

In scope: ARCH-01..05, TEST-01, BUILD-01, BUILD-02, LOG-01.
Out of scope: autostart/LaunchAgent, working-hours, key-press mode, non-macOS
adapters (all v2/deferred).

## Implementation Decisions (locked)

### Structure
- Layout exactly per `docs/TASK.md` §10:
  - `cmd/sysmon/main.go` — entry point: flag parsing (`-interval`, `-quiet`),
    dependency wiring (manual DI), signal handling, startup. No business logic.
  - `internal/activity/domain/` — pure Go value objects + rules (e.g. an
    `Interval` that validates `> 0`, an `Offset`). Imports nothing from
    `port`/`service`/`adapter`; no cgo/os/time-from-outside.
  - `internal/activity/port/` — interfaces: driven port `Pointer` with
    `Nudge(dx, dy int) error`; driving port `Keeper` with `Run(ctx) error`.
  - `internal/activity/service/` — `KeeperService` implements `Keeper`: ticker
    loop, on each tick `Pointer.Nudge(+1,0)` → pause → `Nudge(-1,0)`; stops
    cleanly on `ctx` cancellation. Depends only on `domain` + `port`.
  - `internal/activity/adapter/` — `CGPointer` implements `Pointer` via
    cgo/CoreGraphics (`CGEventCreateMouseEvent`/`CGEventPost`), behind
    `//go:build darwin`. All CoreFoundation objects released with `CFRelease`.
- Remove the flat `main.go` from the repo root.
- Module path stays `sysmon`; internal import paths are `sysmon/internal/activity/...`.

### Behavior preservation (must not change)
- Flags `-interval` (default 10s) and `-quiet`; real `kCGEventMouseMoved` event
  via `CGEventPost(kCGHIDEventTap, …)`; 1px nudge there-and-back with the ~40ms
  pause; clean exit on SIGINT/SIGTERM (code 0); `time.Ticker` sleep between ticks
  (near-zero idle CPU — no busy-wait).
- Makefile targets (build/run/start/stop/status/clean) keep working unchanged.

### Logging (LOG-01)
- Use stdlib `log/slog`. Inject a `*slog.Logger` into `KeeperService` (constructor
  param) — no package-global logger, so tests stay clean.
- `-quiet` raises the level threshold (e.g. warn+ only) rather than redirecting
  output. Lifecycle events (start with interval, stop) logged with structure.
- Zero third-party logger — this is the whole reason slog was chosen.

### Testing (TEST-01)
- Unit-test `KeeperService` with a **fake `Pointer`** (records Nudge calls). The
  test asserts nudges occur per tick and that the loop stops on context cancel.
- Test must NOT move the real mouse and must NOT require macOS Accessibility
  permission — i.e. it never touches the `CGPointer` adapter.
- Prefer an injectable interval / short interval so the test is fast and
  deterministic; avoid real-time flakiness (consider a channel-driven or
  injected tick, or a very short interval with a bounded wait).
- `go test -race ./...` must pass (CI runs it).

## Claude's Discretion
- Exact domain type shape (`Interval`/`Offset` naming, whether Offset is a
  domain type or a constant), constructor signatures, and how the tick source is
  made testable (injected clock/ticker vs short interval) — pick the cleanest
  idiomatic Go. Keep it small; do not over-engineer a tiny utility.
- Whether `Keeper.Run` takes the interval/offset via constructor or method.

## Canonical References
- `docs/TASK.md` §10 — the authoritative architecture spec.
- `.planning/codebase/ARCHITECTURE.md` — current→target mapping.
- Current `main.go` — the behavior to preserve (cgo nudge, flags, signals, loop).

## Specific Ideas
- Keep the cgo C `nudge` helper essentially as-is, just relocated into the
  `CGPointer` adapter with the `//go:build darwin` tag.
- `cmd/sysmon/main.go` builds a `slog.Logger`, constructs `CGPointer`, injects
  both into `KeeperService`, wires SIGINT/SIGTERM to a `context.CancelFunc`, and
  calls `keeper.Run(ctx)`.

## Deferred Ideas
- LaunchAgent autostart, working-hours window, key-press mode, Linux/Windows
  adapters — all out of scope for this phase (tracked in REQUIREMENTS.md v2).
