# Phase 1 — Validation Strategy

How each requirement is proven. Automated checks run in CI (build/lint/test on
macOS); a couple need a one-time manual check on the target Mac.

| REQ | Validation method | Type |
|-----|-------------------|------|
| ARCH-01 | Repo contains `cmd/sysmon/main.go` + `internal/activity/{domain,port,service,adapter}`; no root `main.go`. Checked by `go build ./...` + directory assertion. | build/structural |
| ARCH-02 | `Pointer` interface in `port`; `CGPointer` in `adapter` with `//go:build darwin`; `grep` confirms build tag + `CFRelease` present. | structural |
| ARCH-03 | `KeeperService` in `service` implements `Keeper`; imports show only `domain`+`port` (no `adapter`). Verified by import inspection / `go list -deps`. | structural |
| ARCH-04 | `domain` imports nothing from `port`/`service`/`adapter` and no cgo/os; `GOOS=linux go build ./internal/activity/dom/...` compiles (no cgo). | structural |
| ARCH-05 | No `main.go` at repo root (`test ! -f main.go`). | structural |
| TEST-01 | `go test -race ./...` passes a `KeeperService` unit test using a **mockgen-generated** `Pointer` mock (`go.uber.org/mock`), **testify** assertions, and `testing/synctest` for deterministic time; no real mouse, no Accessibility needed. | automated test |
| LOG-01 | `KeeperService` takes an injected `*slog.Logger`; no package-global logger (grep); `-quiet` raises level to warn. Unit test asserts via a capturing slog handler + testify. | automated test |
| BUILD-01 | `go build` succeeds; all Makefile targets work; `make build` builds `./cmd/sysmon`. | build/manual |
| BUILD-02 | Behavior preserved: `-interval`/`-quiet` flags, 10s default, real `kCGEventMouseMoved`, clean SIGINT/SIGTERM (exit 0). Flags/exit covered by test/manual; event delivery = manual on Mac. | automated + manual |

## Manual checks (one-time, on the target Mac, after merge)
- Grant Accessibility; run `make start`; confirm the screen does not sleep; `make stop` kills only sysmon. (BUILD-02 event delivery, from docs/TASK.md §8.)

## Notes
- Domain/service/port must be buildable and testable WITHOUT cgo/macOS so the
  test suite is portable and fast; only the `adapter` package requires darwin.
