# Requirements — sysmon

Scope of the current milestone: restructure the existing working utility into the
hexagonal architecture specified in `docs/TASK.md` §10, add tests, and keep all
current behavior intact. Existing runtime behavior is already Validated in
PROJECT.md; these requirements cover the restructure work.

## v1 Requirements

### Architecture (ARCH)

- [ ] **ARCH-01**: Code is organized as `cmd/sysmon/main.go` plus `internal/activity/` split into `domain`, `port`, `service`, and `adapter` packages
- [ ] **ARCH-02**: A driven `Pointer` port abstracts "nudge the pointer"; a `CGPointer` adapter implements it using cgo/CoreGraphics behind a `//go:build darwin` tag
- [ ] **ARCH-03**: A `Keeper` driving port and `KeeperService` run the interval loop, depending only on `domain` and `port` (never on `adapter`)
- [ ] **ARCH-04**: `domain` holds pure types/rules (e.g. interval validation) with no cgo/os/system imports; dependency direction points inward to the domain
- [ ] **ARCH-05**: The flat `main.go` is removed from the repo root; `cmd/sysmon/main.go` only wires flags, dependencies, and signal handling

### Testing (TEST)

- [ ] **TEST-01**: `KeeperService` is unit-tested with a fake `Pointer` — the test verifies nudges happen on the interval and never moves the real mouse, and runs without macOS Accessibility permission

### Build & Behavior (BUILD)

- [ ] **BUILD-01**: `go build` succeeds and all Makefile targets (build/run/start/stop/status/clean) work unchanged after the refactor
- [ ] **BUILD-02**: Runtime behavior is preserved — `-interval` and `-quiet` flags, 10s default, real mouse-move event, clean SIGINT/SIGTERM shutdown

## v2 Requirements (deferred)

- [ ] Autostart at login via LaunchAgent
- [ ] "Working hours" window (activity only during a configured time range)
- [ ] Alternative activity mode (key-press instead of mouse)

## Out of Scope

- Rootkit-level process hiding — detection evasion, deliberately excluded
- Non-macOS platforms — architecture leaves room via new adapters, but not built now
- Network / telemetry / persistence — not needed for a local utility

## Traceability

<!-- Filled by roadmap: REQ-ID → Phase. -->

| REQ-ID | Phase |
|--------|-------|
| (pending roadmap) | |
