# sysmon

## What This Is

A small local command-line utility written in Go for macOS. It keeps the machine
"active" by posting a real mouse-move event every N seconds, resetting the system
idle timer so the screen does not sleep and the user's presence status does not
flip to idle. The cursor stays effectively in place (a 1px nudge and back). Built
for a single user on their own Mac.

## Core Value

The machine reliably stays "active" — a real mouse event is delivered on a fixed
interval and the OS idle timer is reset — while staying invisible (no window) and
nearly free in resource use.

## Requirements

### Validated

<!-- Inferred from existing working code (flat main.go). -->

- ✓ Posts a real `kCGEventMouseMoved` event via CoreGraphics every N seconds — existing
- ✓ Interval configurable via `-interval` flag (default 10s) — existing
- ✓ Quiet background mode via `-quiet` flag — existing
- ✓ Clean shutdown on SIGINT/SIGTERM (exit 0, ticker stopped) — existing
- ✓ Builds with zero third-party Go dependencies (stdlib + cgo/CoreGraphics) — existing
- ✓ Makefile with build/run/start/stop/status/clean (safe pidfile-based control) — existing

### Active

<!-- Building toward these. Hypotheses until shipped. -->

- [ ] Refactor to hexagonal architecture (cmd/sysmon + internal/activity with domain/port/service/adapter) per docs/TASK.md §10
- [ ] `KeeperService` unit-tested with a fake `Pointer` (no real mouse movement in tests)
- [ ] Platform build tag (`//go:build darwin`) isolates the macOS/cgo adapter
- [ ] Structured logging via stdlib `log/slog` (injected, level-controlled by `-quiet`) replaces ad-hoc `log.Printf`

### Out of Scope

- Rootkit-level process hiding (hiding from `ps`/Activity Monitor) — deliberately excluded; the tool is an honest process. "Invisibility" means no GUI window only.
- Autostart at login (LaunchAgent) — deferred to a future milestone, not v1.
- Network activity, telemetry, data persistence — none needed; single local utility.
- Distributed-systems concerns (scale, messaging, k8s) — irrelevant for a single-machine CLI.
- Non-macOS platforms (Linux/Windows) — out of scope now; architecture leaves room via new adapters later.

## Context

- Recreation of a utility the user previously had on another machine (now abroad).
- Repo is private on GitHub (`dezween/sysmon`), kept separate from the machine's other git setups via a dedicated SSH key and local git identity.
- Current code is a working but flat `main.go` in the repo root; the hexagonal layout exists only as a spec in `docs/TASK.md` §10.
- Requires macOS Accessibility (TCC) permission for the controlling terminal, or synthetic events are silently dropped.
- `.planning/` is kept local-only (git-ignored); a tailored code-review prompt lives in `docs/prompts/code-review.md`.

## Constraints

- **Tech stack**: Go 1.26 + cgo, macOS only (arm64) — uses CoreGraphics/ApplicationServices C API.
- **Dependencies**: runtime/production is standard-library only (zero third-party in the binary — reason `log/slog` was chosen). Test-only deps are allowed: `testify` (assertions), `go.uber.org/mock`/mockgen (mocks from interfaces), `testcontainers` (integration tests only).
- **Platform**: requires Xcode Command Line Tools and macOS Accessibility permission.
- **Resource use**: must stay lightweight — near-zero idle CPU, tiny memory footprint, battery-friendly (runs for hours in background).

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Real mouse-move event (CGEventPost), not cursor warp | Only a real event resets the idle timer | ✓ Good |
| Zero third-party deps (cgo + CoreGraphics) | Lightweight, reproducible build | ✓ Good |
| Hexagonal architecture (like a larger Go monorepo) | Testability, portability, consistency | — Pending |
| pidfile-based start/stop in Makefile | Avoids broad `pkill` matching unrelated procs (e.g. `sysmond`) | ✓ Good |
| No rootkit process hiding | Detection-evasion out of scope; honest process | ✓ Good |
| Structured logging via stdlib log/slog | Levels/structure without breaking the zero-dep constraint (vs zerolog/zap) | — Pending |
| CI on macos-latest (build/lint/test) | cgo + CoreGraphics only compiles on macOS | ✓ Good |
| golangci-lint v2 (strict golden config, trimmed) | Consistent strict linting, minus project-specific/web linters | ✓ Good |
| Test stack: testify + mockgen (from interfaces) + testcontainers | Standard Go test tooling; mocks generated, not hand-written (test-only deps, binary stays zero-dep) | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-07-12 after initialization*
