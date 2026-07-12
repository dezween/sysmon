# Phase 1: Hexagonal Refactor - Research

**Researched:** 2026-07-12
**Domain:** Go stdlib (hexagonal restructure of a tiny cgo utility): deterministic ticker testing, port fakes, `log/slog` injection, cgo build-tag isolation, behavior preservation.
**Confidence:** HIGH

## Summary

This is a small, fully-specified refactor with zero new libraries. The one genuinely non-obvious decision is how to make `KeeperService.Run(ctx)` (a `time.Ticker`-driven loop) unit-testable without real-time flakiness. The Go toolchain installed on this machine is **1.26.5**, which means `testing/synctest` — stable since Go 1.25 — is fully available and is the correct idiomatic answer: it gives a fake clock and deterministic goroutine scheduling inside an isolated "bubble," eliminating both real-time sleeps in tests and any real ticker jitter. This was verified directly against the local toolchain via `go doc testing/synctest`, not just from memory or web search.

Everything else in this phase is standard, low-risk stdlib usage: a hand-written fake for the `Pointer` port (no mock library — zero deps is the whole point), constructor-injected `*slog.Logger` with `slog.DiscardHandler` for silent tests, a `//go:build darwin` tag isolating the cgo adapter so `domain`/`port`/`service` compile and test on any OS, and `os/signal.NotifyContext` as a cleaner idiomatic replacement for the current manual `signal.Notify` + channel + `select` wiring in `main()`.

**Primary recommendation:** Use `testing/synctest.Test` + a hand-written fake `Pointer` to test `KeeperService.Run(ctx)` deterministically; inject `*slog.Logger` via constructor; keep the cgo `nudge` C block essentially verbatim inside `adapter/cgpointer.go` behind `//go:build darwin`; replace the manual signal-channel plumbing in `main()` with `signal.NotifyContext`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Flag parsing, DI wiring, signal→context | `cmd/sysmon/main.go` (entry point) | — | Per `docs/TASK.md` §10, `cmd` wires everything; no business logic lives here |
| Interval/offset validation | `domain` | — | Pure value objects/rules, zero external deps, importable and testable on any OS |
| "Nudge the pointer" abstraction | `port` (interface) | — | Decouples service from concrete mouse-movement mechanism |
| Ticker loop / use-case orchestration | `service` | `port` (calls it) | Owns the `for { select }` loop and calls `Pointer.Nudge` — pure Go, no cgo |
| CoreGraphics mouse-move implementation | `adapter` (macOS-only) | — | Only place cgo/CoreGraphics may appear; behind `//go:build darwin` |
| Structured logging | `service` (uses injected `*slog.Logger`) | `cmd` (constructs it) | Logger built in `main`, injected into `KeeperService` constructor — never a package global |

This is a single-module (`internal/activity`), single-tier-set (no browser/CDN/DB — this is a local CLI) application; the map above documents *intra-hexagon* responsibility rather than multi-tier web architecture.

## Standard Stack

### Core

No new libraries — 100% Go standard library, matching the locked decision and `CLAUDE.md` constraints.

| Package | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `time` | stdlib (go1.26.5) | `time.Ticker` for the nudge loop | Already used; unchanged |
| `context` | stdlib | Cancellation signal into `Keeper.Run(ctx)` | Idiomatic Go cancellation propagation |
| `os/signal` | stdlib | `SIGINT`/`SIGTERM` → context cancellation | `signal.NotifyContext` (Go 1.16+) is the modern idiom, replaces manual `signal.Notify`+channel+select `[VERIFIED: local go doc]` |
| `log/slog` | stdlib (go1.21+) | Structured logging, injected `*slog.Logger` | Locked decision (LOG-01); zero third-party logger |
| `testing/synctest` | stdlib (go1.25+, **stable**) | Deterministic ticker-loop unit test | Verified stable (not experimental) and available on the installed 1.26.5 toolchain via `go doc testing/synctest` `[VERIFIED: local go doc + go.dev docs]` |
| `flag` | stdlib | `-interval`, `-quiet` flags | Unchanged from current `main.go` |

### Supporting

| Package | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `errors` | stdlib | Wrapping/returning `Pointer.Nudge` errors if adapter ever fails | Only if `CGEventPost`/CF calls need error propagation (currently they're void C calls — see Pitfalls) |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `testing/synctest` fake clock | Injected `<-chan time.Time` (test drives ticks manually) | Also deterministic and zero-dependency; slightly more boilerplate (needs a `TickSource`/`Ticker` interface port) but works on Go versions without synctest. Since this repo is pinned to 1.26.5, prefer synctest — less code, and it also correctly fake-advances any `time.Sleep`/pause logic without a custom abstraction. |
| `testing/synctest` fake clock | Very short real interval (e.g. 5ms) + bounded `time.Sleep`/polling wait | Simplest to write, but reintroduces real-time flakiness under CI load — explicitly what CONTEXT.md says to avoid. Not recommended. |
| Hand-written fake `Pointer` | A mocking library (e.g. gomock, testify/mock) | Locked decision is zero deps; hand-written fake is trivially simple for a 1-method interface — no library needed or wanted. |
| `signal.NotifyContext` | Manual `signal.Notify(ch, ...)` + `select` (current code) | Manual version works but is more code and less idiomatic since Go 1.16 introduced the combinator; `NotifyContext` collapses signal-channel wiring directly into the `context.Context` the rest of the app already uses. |

**Installation:**
```bash
# No installation needed — everything above is Go stdlib, already available
# with go 1.26.5 (confirmed: go version -> go1.26.5 darwin/arm64)
```

**Version verification:** `go version` on the target machine reports `go1.26.5 darwin/arm64`. All packages referenced (`log/slog`, `testing/synctest`, `os/signal.NotifyContext`) were confirmed present and documented via `go doc <pkg>` run directly against this toolchain — the strongest possible verification (local, authoritative, zero network dependency). No `go.mod` changes are needed; `go 1.26.5` in `go.mod` already satisfies the `go1.25` minimum for `testing/synctest`.

## Package Legitimacy Audit

Not applicable — this phase introduces zero external packages (npm/PyPI/crates or Go modules). All recommended packages are Go standard library, shipped with the toolchain itself. No `go.mod` `require` entries change.

## Architecture Patterns

### System Architecture Diagram

```
                     SIGINT/SIGTERM
                           │
                           ▼
┌──────────────────────────────────────────────────────────────┐
│  cmd/sysmon/main.go                                          │
│  1. flag.Parse() -> interval, quiet                           │
│  2. slog.New(handler w/ level threshold from -quiet)          │
│  3. ctx, stop := signal.NotifyContext(context.Background(),  │
│                     syscall.SIGINT, syscall.SIGTERM)          │
│  4. pointer := adapter.NewCGPointer()                         │
│  5. keeper := service.NewKeeperService(pointer, logger, ...)  │
│  6. err := keeper.Run(ctx)   -- blocks until ctx.Done()       │
└───────────────────────────┬────────────────────────────────────┘
                             │ calls Run(ctx)
                             ▼
┌──────────────────────────────────────────────────────────────┐
│  internal/activity/service/keeper_service.go                 │
│  KeeperService.Run(ctx context.Context) error {               │
│    ticker := time.NewTicker(interval); defer ticker.Stop()    │
│    logger.Info("started", "interval", interval)               │
│    for {                                                       │
│      select {                                                  │
│      case <-ticker.C:                                          │
│         pointer.Nudge(+1, 0)  -- via port.Pointer interface    │
│         time.Sleep(nudgePause)                                 │
│         pointer.Nudge(-1, 0)                                   │
│      case <-ctx.Done():                                        │
│         logger.Info("stopped")                                 │
│         return nil                                             │
│      }                                                          │
│    }                                                             │
│  }                                                               │
└───────────┬─────────────────────────────────────────────┬───────┘
            │ depends on (interface)                       │ depends on (types)
            ▼                                              ▼
┌───────────────────────────┐              ┌──────────────────────────┐
│ internal/activity/port/    │              │ internal/activity/domain/│
│ Pointer { Nudge(dx,dy) err}│              │ Interval, Offset (rules) │
│ Keeper  { Run(ctx) error } │              │ no cgo/os/time-in        │
└─────────────┬──────────────┘              └──────────────────────────┘
              │ implemented by
              ▼
┌──────────────────────────────────────────────────────────────┐
│ internal/activity/adapter/cgpointer.go   //go:build darwin    │
│ CGPointer.Nudge(dx, dy) error {                                │
│   cur := C.CGEventCreate(nil); defer C.CFRelease(cur)          │
│   p := C.CGEventGetLocation(cur)                                │
│   move := C.CGEventCreateMouseEvent(..., kCGEventMouseMoved,..) │
│   C.CGEventPost(C.kCGHIDEventTap, move)                         │
│   C.CFRelease(move)                                             │
│ }                                                                │
└──────────────────────────────────────────────────────────────┘

Test path (no adapter, no macOS, no real time):
┌──────────────────────────────────────────────────────────────┐
│ keeper_service_test.go                                        │
│ synctest.Test(t, func(t *testing.T) {                          │
│   fake := &fakePointer{}                                        │
│   svc := NewKeeperService(fake, slog.New(slog.DiscardHandler),  │
│              interval, offset, pause)                          │
│   ctx, cancel := context.WithCancel(t.Context())                │
│   go func() { errCh <- svc.Run(ctx) }()                         │
│   time.Sleep(3 * interval); synctest.Wait()                     │
│   cancel(); synctest.Wait()                                      │
│   assert len(fake.calls) == expected pairs of (+1,0)/(-1,0)      │
│ })                                                                │
└──────────────────────────────────────────────────────────────┘
```

### Recommended Project Structure
```
cmd/
└── sysmon/
    └── main.go                     # flags, slog setup, DI, signal->context, run
internal/
└── activity/
    ├── domain/
    │   └── activity.go             # Interval, Offset value types + validation
    ├── port/
    │   ├── pointer.go              # driven port: Pointer interface
    │   └── keeper.go                # driving port: Keeper interface
    ├── service/
    │   ├── keeper_service.go        # KeeperService (implements Keeper)
    │   └── keeper_service_test.go   # synctest-based unit test, fake Pointer
    └── adapter/
        └── cgpointer.go             # //go:build darwin, cgo CoreGraphics impl
```

### Pattern 1: Constructor-injected dependencies, no globals
**What:** `KeeperService` takes `port.Pointer`, `*slog.Logger`, and interval/offset/pause values via `NewKeeperService(...)`, never reaching for package-level state.
**When to use:** Always, for both the logger and the pointer — this is what makes `TEST-01` possible at all.
**Example:**
```go
// internal/activity/service/keeper_service.go
package service

import (
	"context"
	"log/slog"
	"time"

	"sysmon/internal/activity/port"
)

type KeeperService struct {
	pointer  port.Pointer
	logger   *slog.Logger
	interval time.Duration
	offset   int
	pause    time.Duration
}

func NewKeeperService(pointer port.Pointer, logger *slog.Logger, interval time.Duration, offset int, pause time.Duration) *KeeperService {
	return &KeeperService{pointer: pointer, logger: logger, interval: interval, offset: offset, pause: pause}
}

func (k *KeeperService) Run(ctx context.Context) error {
	ticker := time.NewTicker(k.interval)
	defer ticker.Stop()

	k.logger.Info("sysmon started", "interval", k.interval)

	for {
		select {
		case <-ticker.C:
			if err := k.pointer.Nudge(k.offset, 0); err != nil {
				k.logger.Error("nudge failed", "err", err)
				continue
			}
			time.Sleep(k.pause)
			if err := k.pointer.Nudge(-k.offset, 0); err != nil {
				k.logger.Error("nudge back failed", "err", err)
			}
		case <-ctx.Done():
			k.logger.Info("sysmon stopped")
			return nil
		}
	}
}
```

### Pattern 2: Deterministic ticker test via `testing/synctest`
**What:** Run the service inside a synctest "bubble" so the fake clock advances instantly and deterministically instead of sleeping in real time.
**When to use:** Any test exercising `time.Ticker`/`time.Sleep`-driven code — exactly `KeeperService.Run`.
**Example:**
```go
// internal/activity/service/keeper_service_test.go
package service_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"sysmon/internal/activity/service"
)

type fakePointer struct {
	mu    sync.Mutex
	calls [][2]int
}

func (f *fakePointer) Nudge(dx, dy int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, [2]int{dx, dy})
	return nil
}

func (f *fakePointer) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func TestKeeperService_NudgesOnEachTick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := &fakePointer{}
		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(fake, logger, 10*time.Second, 1, 40*time.Millisecond)

		ctx, cancel := context.WithCancel(t.Context())

		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		// Let 3 full ticks elapse -- instantly, on the bubble's fake clock.
		time.Sleep(35 * time.Second)
		synctest.Wait() // ensure all pending goroutine work (the +1/-1 nudge pairs) has settled

		if got := fake.callCount(); got != 6 { // 3 ticks * 2 calls (+1, -1)
			t.Fatalf("callCount = %d, want 6", got)
		}

		cancel()
		synctest.Wait()

		if err := <-done; err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	})
}

func TestKeeperService_StopsOnContextCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := &fakePointer{}
		logger := slog.New(slog.DiscardHandler)
		svc := service.NewKeeperService(fake, logger, time.Second, 1, time.Millisecond)

		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- svc.Run(ctx) }()

		synctest.Wait() // let Run reach its select{} and block
		cancel()

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Run returned error: %v", err)
			}
		default:
			synctest.Wait() // drain until Run's goroutine exits
			<-done
		}
	})
}
```
**Source:** pattern verified against `go doc testing/synctest` output and go.dev/blog/testing-time — see Sources.

### Anti-Patterns to Avoid
- **Sleeping in real time inside the test (`time.Sleep(3 * time.Second)` in a normal, non-synctest test):** Reintroduces flakiness under CI load and slows the suite. Avoid entirely now that `testing/synctest` is stable on this toolchain.
- **A package-level `var logger = slog.Default()` in `service`:** Breaks testability (LOG-01 explicitly requires injection) and prevents `-quiet` from being scoped per-instance.
- **Leaving a goroutine running past the end of a `synctest.Test` closure:** The bubble's root goroutine returning stops the fake clock; if a child goroutine is still blocked waiting for time to advance when the bubble ends, the test panics with a deadlock diagnostic. Always `cancel()` and `synctest.Wait()` before the closure returns (or before `<-done`).
- **Testing `CGPointer` directly:** Never construct or call the darwin adapter from tests — it requires Accessibility permission and moves the real cursor. Tests operate only on the `port.Pointer` interface via the fake.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Deterministic time-based test | A custom fake ticker/clock abstraction (`type Clock interface{ Now() time.Time; Tick() <-chan time.Time }`) injected everywhere | `testing/synctest.Test` + real `time.Ticker` inside the bubble | Stdlib as of Go 1.25 (this repo runs 1.26.5); zero extra abstraction layers in production code, and it correctly fake-advances `time.Sleep` too (the 40ms nudge pause) without a second injection point. |
| Mock generation for `Pointer` | A mock-generation tool/library (gomock, testify/mock, moq) | A ~10-line hand-written `fakePointer` struct | One-method interface; locked decision is zero third-party deps; a generated mock adds a dependency and codegen step for no benefit here. |
| Signal → cancellation plumbing | Manual `signal.Notify(ch, ...)` + a `select` arm converting to context cancel | `signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)` | Stdlib since Go 1.16; directly returns a `context.Context` + `stop func()`, removing one manual translation layer from `main.go`. |

**Key insight:** For a utility this size, the temptation is to add abstraction layers "for testability" (custom clock interfaces, DI containers). The stdlib as of Go 1.25/1.26 already solves the one hard problem (deterministic time) natively — reach for `testing/synctest` before reaching for a hand-rolled clock port.

## Runtime State Inventory

> Skipped — this is not a rename/refactor-of-identity phase (no renamed strings, IDs, or externally-registered names). It is a structural code reorganization of a stateless CLI. No databases, external services, OS-registered schedulers, secrets, or build artifacts reference the old flat-file layout by name. The Makefile's `go build -o $(BINARY) .` target is the one thing that must be updated to point at `./cmd/sysmon` — this is a code/build-config edit, not a runtime-state migration, and is called out explicitly under Common Pitfalls below.

## Common Pitfalls

### Pitfall 1: Makefile silently breaks after `main.go` moves
**What goes wrong:** `Makefile`'s `build` target runs `go build -o $(BINARY) .`, which builds the package in the current directory (repo root). After `main.go` moves to `cmd/sysmon/main.go` and the root `package main` file is deleted, `go build -o sysmon .` at the root will fail (no Go files) or silently build nothing useful.
**Why it happens:** BUILD-01 requires "Makefile targets keep working unchanged" — but the build target's *argument* must change even though its *interface* (`make build`) stays the same.
**How to avoid:** Update the `build` target to `go build -o $(BINARY) ./cmd/sysmon`. Verify with `make build && make run && make start && make status && make stop && make clean` end-to-end after the refactor — this satisfies BUILD-01 literally, not just structurally.
**Warning signs:** `make build` reports "no Go files in ..." or builds an empty/stale binary from `go.mod`'s module cache.

### Pitfall 2: cgo adapter breaking `go test ./...` on non-darwin or in domain/service packages
**What goes wrong:** If the `//go:build darwin` tag is omitted or misplaced (e.g., only on the `.go` file but the package also has a non-tagged file, or the tag is a `// +build` old-style comment with a typo), `go vet`/`go test ./...` can fail to compile the adapter package on CI, or — worse — cgo directives leak into a file that `domain`/`service` accidentally imports transitively.
**Why it happens:** Build constraints are easy to get subtly wrong (blank line requirements, `//go:build` vs `// +build` syntax, package-level vs file-level placement).
**How to avoid:** Put `//go:build darwin` as the very first line of `cgpointer.go`, followed by a blank line, then `package adapter`. Confirm `domain` and `port` and `service` packages have zero cgo imports and zero references to `adapter` (import direction is inward only — `adapter` depends on `port`, never the reverse). Run `GOOS=linux go build ./internal/activity/domain/... ./internal/activity/port/... ./internal/activity/service/...` as a sanity check — it should succeed even though `adapter` (and thus the full `cmd/sysmon` binary) only builds on darwin. This is expected and acceptable per CONTEXT.md ("adapter package won't compile off-darwin — acceptable since CI is macOS").
**Warning signs:** `go test ./...` from repo root fails with cgo/framework linker errors when run on a non-darwin CI runner or in a sandboxed environment without Xcode Command Line Tools — should only ever affect the `adapter` package, never `domain`/`port`/`service`.

### Pitfall 3: `testing/synctest` bubble deadlock from a goroutine that never exits
**What goes wrong:** If the test spawns `go func() { svc.Run(ctx) }()` but never cancels `ctx` and never lets that goroutine return before the `synctest.Test` closure ends, Go panics with a bubble deadlock diagnostic (time cannot advance because the root goroutine returned while another goroutine in the bubble is still durably blocked).
**Why it happens:** `synctest.Test`'s fake clock stops advancing when the bubble's root goroutine exits; any child goroutine still waiting on that clock is now stuck forever.
**How to avoid:** Always pair the goroutine spawn with an explicit `cancel()` + drain of the result channel (`<-done`) before the `synctest.Test` closure returns, as shown in the Pattern 2 code sketch above.
**Warning signs:** Test panics with a message referencing "deadlock" or "goroutines are stuck" pointing at `testing/synctest` internals.

### Pitfall 4: Forgetting `CFRelease` on both CGEventRefs when relocating the cgo block
**What goes wrong:** The current `main.go` calls `CFRelease` on both `cur` (from `CGEventCreate`) and `move` (from `CGEventCreateMouseEvent`) — two separate CoreFoundation objects per nudge. When relocating this C code into `cgpointer.go`, it's easy to accidentally drop one `CFRelease` (e.g., during a refactor that "cleans up" the C block) and introduce a slow CF-object leak that only shows up after hours of runtime (this is a long-running background utility per the project's core value).
**Why it happens:** The two `CFRelease` calls aren't adjacent to their allocations in a way that makes the pairing visually obvious once the code is reformatted or the function is split.
**How to avoid:** Per CONTEXT.md's Specific Ideas, keep the cgo C `nudge` helper "essentially as-is" — copy-paste the existing C block into `cgpointer.go` verbatim rather than rewriting it, changing only the surrounding Go glue (method receiver, error return). Diff the moved C code against `main.go`'s original block to confirm byte-for-byte equivalence of the CF lifecycle calls.
**Warning signs:** None observable in a short test run — this is the kind of bug that only appears after hours/days of production use (memory growth). Preserve the original C code exactly to avoid re-introducing a fixed issue.

### Pitfall 5: `-quiet` implemented as output redirection instead of a level threshold
**What goes wrong:** The current `main.go` implements `-quiet` by doing `log.SetOutput(os.Stderr)` — redirecting rather than filtering. LOG-01 and CONTEXT.md explicitly require the *refactored* version to raise the level threshold (e.g., warn+) instead. If the refactor naively ports the old redirect-based logic into an `slog.Handler` wrapper, it technically "logs less to stdout" but violates the locked design (structured level-based filtering) and makes the two lifecycle log lines (start/stop) — which are Info-level — silently disappear or reappear inconsistently depending on which stream a test asserts against.
**Why it happens:** Redirection and level-filtering both "look like" they satisfy "-quiet suppresses logs to stdout" from a black-box behavior view, but only one matches the locked slog-based design.
**How to avoid:** In `main.go`, choose `slog.LevelWarn` (or higher) as the handler's minimum level when `-quiet` is set, `slog.LevelInfo` otherwise — do not touch the output stream at all. Example:
```go
level := slog.LevelInfo
if *quiet {
    level = slog.LevelWarn
}
handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
logger := slog.New(handler)
```
Since the current lifecycle logs (start, stop) are informational, under `-quiet` they simply won't meet the Warn threshold and won't print — matching the observed behavior ("no logs to stdout when quiet") through a different, more idiomatic mechanism.
**Warning signs:** A test or manual check expecting zero stdout output under `-quiet` still sees output because the handler's level wasn't actually raised (e.g., `HandlerOptions.Level` left `nil`, which defaults to Info).

## Code Examples

### Domain value objects (Interval, Offset)
```go
// internal/activity/domain/activity.go
package domain

import (
	"errors"
	"time"
)

// Interval is a validated positive duration between nudges.
type Interval struct {
	d time.Duration
}

func NewInterval(d time.Duration) (Interval, error) {
	if d <= 0 {
		return Interval{}, errors.New("interval must be greater than zero")
	}
	return Interval{d: d}, nil
}

func (i Interval) Duration() time.Duration { return i.d }

// Offset is the pixel shift magnitude applied then reversed on each nudge.
type Offset int

func NewOffset(px int) (Offset, error) {
	if px <= 0 {
		return 0, errors.New("offset must be greater than zero")
	}
	return Offset(px), nil
}
```
Note: per CONTEXT.md "Claude's Discretion," the exact shape (whether `Offset` is a distinct type or a plain constant) is left open — the above is one clean option; a bare `const defaultOffset = 1` in `service` is equally acceptable if the team prefers less ceremony for a single hard-coded value that TASK.md never asks to be user-configurable.

### Ports
```go
// internal/activity/port/pointer.go
package port

// Pointer abstracts moving the system pointer by a relative offset and
// posting a synthetic mouse-moved event, which resets the OS idle timer.
type Pointer interface {
	Nudge(dx, dy int) error
}
```
```go
// internal/activity/port/keeper.go
package port

import "context"

// Keeper runs the activity-keeping loop until ctx is done.
type Keeper interface {
	Run(ctx context.Context) error
}
```

### cgo adapter (relocated, essentially unchanged)
```go
//go:build darwin

// internal/activity/adapter/cgpointer.go
package adapter

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreGraphics
#include <ApplicationServices/ApplicationServices.h>

static void nudge(int dx, int dy) {
    CGEventRef cur = CGEventCreate(NULL);
    CGPoint p = CGEventGetLocation(cur);
    CFRelease(cur);

    CGPoint np = CGPointMake(p.x + dx, p.y + dy);
    CGEventRef move = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, np, kCGMouseButtonLeft);
    CGEventPost(kCGHIDEventTap, move);
    CFRelease(move);
}
*/
import "C"

// CGPointer implements port.Pointer using CoreGraphics on macOS.
type CGPointer struct{}

func NewCGPointer() *CGPointer { return &CGPointer{} }

func (c *CGPointer) Nudge(dx, dy int) error {
	C.nudge(C.int(dx), C.int(dy))
	return nil
}
```

### `cmd/sysmon/main.go` wiring
```go
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"syscall"
	"time"

	"sysmon/internal/activity/adapter"
	"sysmon/internal/activity/service"
)

const (
	defaultInterval = 10 * time.Second
	nudgeOffset     = 1
	nudgePause      = 40 * time.Millisecond
)

func main() {
	interval := flag.Duration("interval", defaultInterval, "how often to nudge the cursor (e.g. 10s, 30s, 1m)")
	quiet := flag.Bool("quiet", false, "do not write logs to stdout")
	flag.Parse()

	level := slog.LevelInfo
	if *quiet {
		level = slog.LevelWarn
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pointer := adapter.NewCGPointer()
	keeper := service.NewKeeperService(pointer, logger, *interval, nudgeOffset, nudgePause)

	if err := keeper.Run(ctx); err != nil {
		logger.Error("sysmon exited with error", "err", err)
		os.Exit(1)
	}
}
```
(Import `"os/signal"` alongside the others above — omitted from the snippet's import block only for brevity; include it in the actual file.)

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `signal.Notify(ch, sig...)` + manual `select` arm to detect shutdown | `signal.NotifyContext(ctx, sig...)` returning a ready-made `context.Context` | Go 1.16 (2021) | Collapses signal handling directly into the context-cancellation idiom the rest of the app already uses for `Keeper.Run(ctx)`. |
| Hand-rolled fake clock/ticker interfaces injected for testability | `testing/synctest` fake-clock "bubble" | Go 1.24 experimental (2024) -> **stable in Go 1.25** (Aug 2025); `Run` renamed to `Test` at stabilization | Removes the need for a custom `Clock`/`Ticker` port purely for test determinism; real `time.Ticker`/`time.Sleep` "just work" deterministically inside `synctest.Test`. This repo's `go 1.26.5` toolchain has it natively. |
| `log.Printf`/`log.Println` with `log.SetOutput` for quiet mode | `log/slog` with `HandlerOptions.Level` threshold, injected `*slog.Logger` | slog added Go 1.21 (2023); this is a project-local behavior change (LOG-01) | Structured, leveled logs; `-quiet` becomes a level filter rather than a stream redirect; testable without touching global output. |

**Deprecated/outdated:**
- `testing/synctest.Run` (the pre-stabilization experimental name): deprecated in favor of `Test` as of Go 1.25; do not use `Run` in new code — verified via `go doc testing/synctest`, which shows only `Test` and `Wait` as exported functions on this toolchain.
- `GOEXPERIMENT=synctest`: no longer required; the package is a normal stdlib import on Go 1.25+ (this repo is on 1.26.5).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `HandlerOptions.Level` set to `slog.LevelWarn` under `-quiet` is an acceptable interpretation of "raise the level threshold" from CONTEXT.md, and that lifecycle Info logs (start/stop) are the only logs currently emitted | Common Pitfalls (Pitfall 5), Code Examples | Low — if future log statements are added at Info level that must still show under `-quiet`, the threshold choice may need revisiting; easy to adjust (single constant). |
| A2 | `Offset` as a validated domain type (vs. a bare constant) is the preferred shape | Code Examples (domain) | Low — CONTEXT.md explicitly leaves this to Claude's discretion; either shape satisfies ARCH-04. |

**All other claims in this research were verified directly against the installed Go 1.26.5 toolchain (`go doc`) or cross-checked against go.dev official documentation/blog posts** — no user confirmation needed for the `testing/synctest` recommendation, the `slog.DiscardHandler` mechanism, or `signal.NotifyContext`'s availability.

## Open Questions

1. **Should `KeeperService.Run` accept interval/offset/pause via constructor (as sketched above) or via `Run` method parameters?**
   - What we know: CONTEXT.md explicitly defers this to Claude's discretion ("Whether `Keeper.Run` takes the interval/offset via constructor or method").
   - What's unclear: No strong technical reason to prefer one over the other for a single-shot CLI process.
   - Recommendation: Constructor injection (as shown) — `Run(ctx)` matches the `port.Keeper` interface signature exactly (`Run(ctx) error`), keeping the port interface simple and making `main.go`'s wiring linear (build once, run once). This also matches the pattern already used for `Pointer` and `logger` injection, for consistency.

2. **Does `nudgePause` (40ms) belong in `domain` as a validated type, or as a plain `time.Duration` constant in `service`/`cmd`?**
   - What we know: TASK.md and CONTEXT.md only call out `Interval` and `Offset` explicitly as domain candidates; the pause is described as "hard-coded for reproducibility" in the existing CLAUDE.md conventions notes.
   - What's unclear: Whether promoting it to a domain type adds meaningful validation value (it's never user-configurable, unlike interval).
   - Recommendation: Keep `nudgePause` as a plain constant (in `cmd/sysmon/main.go`, passed through to the service constructor) — do not over-engineer a value that has no validation rule and no user-facing flag, consistent with CONTEXT.md's "keep it small; do not over-engineer" guidance.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Entire build/test | Yes | go1.26.5 darwin/arm64 (confirmed via `go version`) | — |
| `testing/synctest` (stdlib) | Deterministic ticker test (TEST-01) | Yes | Stable since go1.25; present in go1.26.5 (confirmed via `go doc testing/synctest`) | If ever downgraded below go1.25, fall back to an injected `<-chan time.Time` tick source in `KeeperService` |
| Xcode Command Line Tools / CoreGraphics frameworks | `adapter.CGPointer` build (cgo) | Assumed present (per CLAUDE.md platform requirements; existing `main.go` already builds with cgo on this machine) | — | None needed — `domain`/`port`/`service` compile and test without cgo; only the darwin adapter and full binary require it |
| macOS Accessibility permission | Runtime delivery of the real mouse event (not build/test) | Out of scope for this phase's automated tests — TEST-01 explicitly requires the test to run without it | — | N/A (manual verification only, already tracked in TASK.md §8 acceptance criteria) |

**Missing dependencies with no fallback:** None identified.

**Missing dependencies with fallback:** None currently missing; the `testing/synctest` fallback above is documented preemptively in case of a future Go downgrade.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `testing/synctest` (stable, go1.25+) |
| Config file | none — no test framework config needed for stdlib `go test` |
| Quick run command | `go test ./internal/... -run TestKeeperService -v` |
| Full suite command | `go test -race ./...` (required by CONTEXT.md; note this only exercises `domain`/`port`/`service` meaningfully in CI unless run on a darwin runner with cgo enabled for the `adapter` package and `cmd/sysmon`) |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| TEST-01 | `KeeperService` nudges on each tick via fake `Pointer`, no real mouse | unit | `go test ./internal/activity/service/... -run TestKeeperService_NudgesOnEachTick -race` | Wave 0 |
| TEST-01 | `KeeperService.Run` stops cleanly on context cancellation | unit | `go test ./internal/activity/service/... -run TestKeeperService_StopsOnContextCancel -race` | Wave 0 |
| ARCH-04 | `domain` package has no cgo/os/system imports | build check | `GOOS=linux GOARCH=amd64 go build ./internal/activity/domain/... ./internal/activity/port/... ./internal/activity/service/...` (should succeed without cgo/darwin) | Wave 0 |
| BUILD-01 | All Makefile targets still work post-refactor | manual/smoke | `make build && make run` (Ctrl+C) `&& make start && make status && make stop && make clean` | Wave 0 (Makefile edit) |
| BUILD-02 | `-interval`/`-quiet` flags, default 10s, clean SIGINT/SIGTERM exit code 0 | manual smoke (already Validated per PROJECT.md for existing behavior; re-verify post-move) | `./sysmon -interval 300ms` then `kill -TERM <pid>`; `echo $?` | Wave 0 |
| LOG-01 | `-quiet` raises level threshold; lifecycle events logged with structure | unit (handler level assertion) or manual | `go test ./cmd/... -run TestQuietRaisesLevel` (optional) or manual `./sysmon -quiet` stdout check | Wave 0 (optional unit test) |

### Sampling Rate
- **Per task commit:** `go test ./internal/activity/... -race`
- **Per wave merge:** `go test -race ./...` (on darwin, cgo-enabled — this is the only environment where the full module including `adapter`/`cmd` compiles)
- **Phase gate:** Full suite green (`go test -race ./...` on darwin) before `/gsd-verify-work`, plus the manual Makefile smoke sequence above (BUILD-01/BUILD-02 are behavior-preservation requirements that automated tests can only partially cover — e.g., "real mouse doesn't visibly drift" and "screen doesn't sleep" remain manual per TASK.md §8).

### Wave 0 Gaps
- [ ] `internal/activity/service/keeper_service_test.go` — covers TEST-01 (both sub-behaviors: nudge-per-tick, stop-on-cancel)
- [ ] `Makefile` edit — `build` target must point at `./cmd/sysmon` instead of `.` (see Pitfall 1) — required for BUILD-01
- [ ] No new test framework install needed — `testing`/`testing/synctest` are stdlib, already available on go1.26.5

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-------------------|
| V2 Authentication | No | N/A — single-user local CLI, no auth boundary |
| V3 Session Management | No | N/A |
| V4 Access Control | No | N/A — relies on macOS TCC (Accessibility permission), an OS-level control already documented in TASK.md §7, unchanged by this refactor |
| V5 Input Validation | Yes (minimal) | `domain.NewInterval` rejects `<= 0` durations; `flag.Duration` already rejects unparseable duration strings at the `flag` package level before reaching domain code |
| V6 Cryptography | No | N/A — no cryptographic operations in this utility |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|----------------------|
| Unreleased CoreFoundation objects (`CGEventRef`) causing unbounded memory growth in a long-running background process | Denial of Service (resource exhaustion) | Preserve both `CFRelease` calls exactly as in the original `nudge` C function (see Pitfall 4) — this is the only "security-adjacent" concern in an otherwise trust-boundary-free local utility. |
| Negative/zero interval causing a busy-loop (`time.NewTicker` panics on `d <= 0`) | Denial of Service (local resource exhaustion / crash) | `domain.NewInterval` validation rejects non-positive durations before a `time.Ticker` is ever constructed with them (ARCH-04's stated purpose). |

This phase has no network exposure, no authentication boundary, and no data persistence — the security surface is limited to the two items above, both already addressed by the locked design (domain validation + CFRelease preservation).

## Sources

### Primary (HIGH confidence — verified directly against installed toolchain)
- `go doc testing/synctest` (run locally against go1.26.5) — confirmed `Test`/`Wait` signatures, fake-clock semantics, deadlock-on-early-return behavior
- `go doc log/slog.DiscardHandler`, `go doc log/slog.HandlerOptions` (run locally) — confirmed `DiscardHandler` and `HandlerOptions.Level` mechanics
- `go doc os/signal.NotifyContext` (run locally) — confirmed signature and cancellation semantics
- `go version` (run locally) — confirmed `go1.26.5 darwin/arm64`, satisfying `testing/synctest`'s go1.25 minimum

### Secondary (MEDIUM confidence — official docs/blog, cross-checked)
- https://pkg.go.dev/testing/synctest — package doc, `Test`/`Wait` API, fake-clock behavior
- https://go.dev/blog/synctest — introduction of the experimental `Run`-based API in Go 1.24 (superseded)
- https://go.dev/blog/testing-time — stabilization details, `Run`→`Test` rename, root-goroutine-exit-stops-clock behavior
- https://go.dev/doc/go1.25 — Go 1.25 release notes confirming synctest graduated from experiment to GA

### Tertiary (LOW confidence — community sources, used only for corroboration)
- https://boyter.org/posts/golang-slog-disable-tests/ — `slog.SetDefault(slog.New(slog.DiscardHandler))` pattern for test suites (corroborated by local `go doc` output, not relied on alone)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — 100% stdlib, versions confirmed against the actual installed toolchain via `go doc`/`go version`, not memory
- Architecture: HIGH — directly specified by `docs/TASK.md` §10 and `01-CONTEXT.md`; no open architectural questions
- Pitfalls: HIGH for cgo/CFRelease/Makefile items (grounded in reading the actual `main.go` and `Makefile`); MEDIUM for the synctest deadlock pitfall (grounded in official docs, not yet reproduced in this repo's own test suite)

**Research date:** 2026-07-12
**Valid until:** 2026-10-10 (approx. 90 days — stdlib-only research on a pinned Go version is unusually stable; revisit only if `go.mod`'s Go version changes)
</content>
