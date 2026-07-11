# Roadmap — sysmon

Milestone: v1 — Hexagonal Refactor

## Phases

- [ ] **Phase 1: Hexagonal Refactor** - Restructure the flat `main.go` into `cmd/sysmon` + `internal/activity` (domain/port/service/adapter), with `KeeperService` unit-tested via a fake `Pointer`, and all existing runtime behavior preserved.

## Phase Details

### Phase 1: Hexagonal Refactor
**Goal**: The codebase is organized as ports & adapters (mirroring `minitok.go`'s pattern) so that the activity-keeping logic is unit-testable and platform code is isolated, while every existing runtime behavior (flags, mouse-move event, clean shutdown, Makefile targets) keeps working exactly as before.
**Depends on**: Nothing (first phase)
**Requirements**: ARCH-01, ARCH-02, ARCH-03, ARCH-04, ARCH-05, TEST-01, BUILD-01, BUILD-02
**Success Criteria** (what must be TRUE):
  1. Repo has `cmd/sysmon/main.go` plus `internal/activity/{domain,port,service,adapter}`; no flat `main.go` remains at repo root
  2. `internal/activity/domain` imports nothing from `port`, `service`, `adapter`, or cgo/os — it is pure Go with interval/offset validation rules
  3. `KeeperService` (in `service`) depends only on `domain` and `port` types, never imports `adapter`, and runs the tick loop via the `Keeper`/`Pointer` interfaces
  4. `CGPointer` adapter implements `Pointer` using cgo/CoreGraphics and is isolated behind a `//go:build darwin` tag
  5. `go build` succeeds, all Makefile targets (build/run/start/stop/status/clean) work unchanged, and `go test ./...` passes a `KeeperService` unit test using a fake `Pointer` that verifies nudges fire on the interval without moving the real mouse and without requiring macOS Accessibility permission
**Plans**: TBD

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Hexagonal Refactor | 0/? | Not started | - |
