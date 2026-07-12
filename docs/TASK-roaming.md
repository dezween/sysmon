# Task: Realistic random cursor roaming

## Goal

Change sysmon's activity behavior from the current invisible 1px nudge-and-back
to **realistic, random cursor roaming across the whole screen**. On each
interval the cursor should glide to a new random on-screen point in a way that
looks like a person actually moving the mouse -- a smooth multi-step path, not a
teleport. Every interval picks a fresh random destination.

This supersedes the Phase 1 nudge behavior. It still serves the same purpose
(keep the machine active / idle timer reset) but the movement is now visible and
human-like by design, which is what the user actually wants.

## Motivation

The 1px nudge kept the system awake but was imperceptible; the user wants the
cursor to visibly roam as if in active use.

## Functional requirements

1. **Random destination each interval.** On every tick, choose a random target
   point uniformly within the current screen bounds. Always a new random point.
2. **Realistic glide.** Move to the target over several small steps with short
   pauses between them, so the motion looks continuous and human, not an instant
   jump. (Reasonable defaults: on the order of ~20-60 steps spread over a few
   hundred milliseconds; exact numbers are implementation detail.)
3. **Whole screen, bounded.** The cursor may go anywhere on the main display but
   never off-screen. No cumulative drift (targets are absolute, clamped to
   screen bounds).
4. **Interval.** Reuse the existing `-interval` flag (default 10s). The interval
   is the gap between the *start* of successive roams.
5. **Interruptible.** SIGINT/SIGTERM must stop the loop promptly, including mid
   glide (do not force-finish a long glide). Clean exit code 0.
6. **Quiet mode.** `-quiet` keeps raising the log level (unchanged).

## Non-functional requirements

- **Runtime stays zero-dependency.** Use only the standard library
  (`math/rand/v2` is fine) plus the existing cgo/CoreGraphics adapter. No new
  third-party runtime deps.
- **Lightweight.** Between roams the process sleeps on the ticker (near-zero idle
  CPU). The glide is a brief burst of small moves, then idle again.
- **Hexagonal architecture preserved** (see docs/TASK.md section 10).

## Design (hexagonal placement)

- **domain** -- a pure, deterministic movement planner. Given the current cursor
  position, the screen bounds, and an injected random source, it produces the
  sequence of intermediate points (the glide path) toward a random target.
  - Inject the randomness (e.g. an interface or a `*rand.Rand`) so tests are
    deterministic. The domain must stay free of cgo/os and build under
    `GOOS=linux`.
  - Targets are clamped to `[0, width) x [0, height)`.
- **port** -- extend the driven side so the service can:
  - read the current pointer position,
  - read the screen bounds,
  - move the pointer to an absolute (x, y).
  Keep the port minimal and cohesive. The existing `Pointer.Nudge` may be
  replaced by `MoveTo(x, y int) error` plus `Position() (x, y int, err error)`
  and a screen-bounds accessor (either on the same port or a small `Screen`
  port). Regenerate the mockgen mock from the updated interface(s).
- **service** -- `KeeperService` (or a renamed keeper) on each tick asks the
  planner for a glide path from the current position, then drives the adapter
  through the path with the small inter-step pauses, honoring ctx cancellation
  between steps.
- **adapter** -- `CGPointer` gains the CoreGraphics implementations: current
  position (`CGEventGetLocation`), screen bounds
  (`CGDisplayBounds(CGMainDisplayID())`), and absolute move + posted
  MouseMoved event. Keep it behind `//go:build darwin`, release every
  CoreFoundation object.

## Testing (locked stack -- see docs/prompts and testing conventions)

- **testify** for assertions; **mockgen** (`go.uber.org/mock`) for mocks
  generated from the updated port interface(s); **testing/synctest** for
  deterministic timing. No hand-written fakes. No testcontainers.
- Planner tests use an injected deterministic random source and assert:
  targets are within bounds, path is continuous (steps bounded), path ends at
  the target, edge cases (cursor already at a corner, 1x1 screen, target equals
  current position).
- Service tests (synctest + mock) assert: one roam per tick, the path is driven
  through `MoveTo` in order, prompt stop on ctx cancel mid-glide, and quiet/log
  behavior.
- Domain + service stay cgo-free (`GOOS=linux go build`).

## Acceptance criteria

- [ ] On each interval the cursor visibly glides to a new random on-screen point,
      smoothly (multiple steps), never off-screen, no drift.
- [ ] `-interval` still controls the gap; `-quiet` still raises the log level.
- [ ] SIGINT/SIGTERM stops promptly, including mid-glide; exit code 0.
- [ ] Runtime binary has zero third-party dependencies; test-only deps unchanged.
- [ ] Hexagonal layering intact: domain pure and cgo-free; service depends only
      on domain + port; cgo only in adapter behind `//go:build darwin`.
- [ ] `go build ./...`, `golangci-lint run ./...` (0 issues), `go test -race ./...`
      all pass; domain + service build under `GOOS=linux`.
- [ ] Planner and service covered by tests incl. edge cases.

## Out of scope

- Non-macOS adapters, LaunchAgent autostart, working-hours window (still v2).
- Configurable glide speed/step count as user flags (may be a later follow-up;
  sensible fixed defaults are enough here).
