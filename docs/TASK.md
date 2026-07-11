# Spec: sysmon -- a macOS activity-keeper utility

## 1. Goal

A small console utility in Go for macOS that keeps the system from going idle:
at a configurable interval it posts a real mouse-move event. This resets the
system idle timer -- the screen does not dim, the screensaver does not kick in,
and status in messengers/task-trackers does not flip to "away". The cursor stays
effectively in place (a 1px nudge there and back).

The utility is launched from the terminal, runs quietly in the background, and
has no window of its own.

## 2. Context and motivation

- Target machine -- a Mac on Apple Silicon (arm64), macOS 26.
- The original program was lost (left on another computer in another country);
  the task is to recreate it from scratch and keep it under version control on
  GitHub.
- The repository is private: `git@github.com:dezween/sysmon.git`.

## 3. Functional requirements

1. **Mouse movement on an interval.** By default -- once every 10 seconds. The
   interval is configurable via the `-interval` flag (accepts a Go duration:
   `10s`, `30s`, `1m`).
2. **A real event, not just a cursor reposition.** It must post
   `kCGEventMouseMoved` via `CGEventPost(kCGHIDEventTap, …)` -- only that resets
   the idle timer. A plain `CGWarpMouseCursorPosition` moves the cursor but does
   not touch the idle timer, so it is not suitable.
3. **Minimal offset.** Shift by +1px and immediately back by -1px so the cursor
   does not visibly "drift".
4. **Quiet mode.** The `-quiet` flag suppresses logs to stdout -- for background
   runs.
5. **Clean shutdown.** Handle `SIGINT`/`SIGTERM`: stop the ticker properly and
   exit with code 0.
6. **Background run with no window.** An ordinary console program; background via
   `nohup … &` or via `make start`.

## 4. Non-functional requirements

- **No external dependencies.** Only the Go standard library + cgo with the
  system `ApplicationServices`/`CoreGraphics` framework. No third-party Go
  modules (robotgo and the like) -- so the build stays light and reproducible.
- **One-command build** via the `Makefile`.
- **Readability.** Code with comments at a reasonable level; a README with
  instructions.

## 5. Explicit non-goals (out of scope)

> Worth stating explicitly to avoid misunderstanding.

- ❌ **No rootkit-level hiding of the process from `ps`/Activity Monitor**
  (intercepting system calls, faking kernel output, LD_PRELOAD tricks, etc.).
  The utility is an honest process in the list. "Invisibility" is limited to the
  absence of a GUI window and the quiet background mode.
- ❌ No autostart at login (LaunchAgent) -- by requirement. May be added as a
  separate task later.
- ❌ No network activity, telemetry, or data logging.

## 6. Technical solution

- **Language/build:** Go 1.26, `cgo` enabled (Xcode Command Line Tools required).
- **Mouse-movement API:**
  - current cursor position: `CGEventCreate(NULL)` -> `CGEventGetLocation`;
  - event: `CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, point, …)`;
  - posting: `CGEventPost(kCGHIDEventTap, event)`;
  - mandatory `CFRelease` for created objects (no CF-memory leaks).
- **Loop:** `time.NewTicker(interval)` + `select` over the ticker and the signal
  channel.
- **Repository layout** (target, hexagonal -- see section 10):
  ```
  sysmon/
  ├── cmd/
  │   └── sysmon/
  │       └── main.go            # entry point: flags, DI wiring, signals
  ├── internal/
  │   └── activity/              # the "activity keeping" module
  │       ├── domain/            # domain entities and rules
  │       ├── port/              # interfaces (ports)
  │       ├── service/           # application logic (use cases)
  │       └── adapter/           # port implementations (macOS/CoreGraphics)
  ├── pkg/                       # reusable code (if needed)
  ├── go.mod                     # module sysmon, no external dependencies
  ├── Makefile                   # build / run / start / stop / status / clean
  ├── .gitignore                 # binary, .DS_Store, .idea/
  ├── README.md                  # user instructions
  └── docs/
      └── TASK.md                # this document
  ```

## 7. Permissions (macOS TCC)

To deliver synthetic mouse events, the system needs **Accessibility** permission
for the terminal that `sysmon` is launched from (Terminal.app / iTerm):

**System Settings -> Privacy & Security -> Accessibility.**

Without permission the program starts and does not crash, but the mouse event is
not delivered (the idle timer is not reset). This is expected OS behavior, not a
bug.

## 8. Acceptance criteria

- [x] `go build -o sysmon .` compiles without errors and without third-party
      dependencies.
- [x] The binary is Mach-O arm64.
- [x] Running with `-interval 300ms` works, and `SIGTERM` exits the process with
      code 0.
- [x] `make start` brings up a background process, `make status` shows it, and
      `make stop` kills **only it** (via the PID file).
- [x] `make stop` does not touch the system daemon `/usr/libexec/sysmond` or
      other unrelated processes (the `-f "./sysmon"` regex bug is fixed).
- [x] The repository is private, commits are signed with a personal hidden email
      (`dezween@users.noreply.github.com`), and no personal-work identity leaks.
- [ ] Accessibility permission is granted on the target machine and it has been
      verified that the screen really does not go to sleep while `sysmon` is
      running (manual check).
- [ ] The project is brought to the hexagonal structure (section 10):
      `cmd/sysmon`, an `internal/activity` module with `domain/port/service/adapter`;
      dependency direction is respected (`domain` depends on nothing).
- [ ] `KeeperService` is covered by a unit test with a fake `Pointer` (no real
      mouse).

## 9. Possible future improvements

- Optional autostart via a LaunchAgent (`~/Library/LaunchAgents`).
- A "working hours" flag (activity only, say, 09:00-19:00).
- A key-press simulation mode (e.g. Shift) as an alternative to the mouse.

## 10. Project architecture (hexagonal ports & adapters)

The project is built on a hexagonal architecture (ports & adapters): business
logic at the center, infrastructure at the edges, wired together through
interfaces. Even though the utility is small, the structure is kept for
consistency and easy extensibility.

### 10.1. Directory layout

- **`cmd/<binary>/main.go`** -- entry points. Only flag parsing, dependency
  wiring (manual dependency injection), signal handling, and startup here. No
  business logic.
- **`internal/<module>/`** -- a module (bounded context). Each module is split
  internally into exactly four packages:
  - **`domain/`** -- domain entities, value objects, and rules. Pure Go, **with
    no external or system dependencies** (no cgo, no os, no ambient time -- those
    come in through ports). Imports nothing from `port`, `service`, or `adapter`.
  - **`port/`** -- interfaces (ports). Two kinds:
    - *driven (output) ports* -- what the domain/service needs from the outside
      world (for example, "move the pointer");
    - *driving (input) ports* -- how the system is invoked from outside (for
      example, "run the activity-keeping loop").
    Ports know nothing about concrete implementations.
  - **`service/`** -- the application layer (use cases). Orchestrates the domain
    and calls driven ports. Depends **only on `domain` and `port`**, not on
    `adapter`.
  - **`adapter/`** -- concrete port implementations, all the infrastructure.
    This is where the cgo/CoreGraphics code, the OS work, the clock, etc. live.
    Adapters implement the interfaces from `port`.
- **`pkg/`** -- reusable code not tied to a specific module (if needed). May be
  empty by default.

### 10.2. Dependency direction

```
cmd  -->  service  -->  port  <--  adapter
                 |        ^
                 +-->  domain
```

Rule: dependencies always point **inward**, toward the domain. `domain` depends
on nothing. `service` depends on `domain` and `port`. `adapter` depends on `port`
(implements it) and on `domain` (for types). `cmd` knows about everything and
wires adapters to services through ports.

### 10.3. The `activity` module -- concrete layout for sysmon

The only module is `internal/activity`. Suggested contents:

- **`domain/`**
  - `activity.go` -- domain types: e.g. `Interval` (a validated interval),
    `Offset` (pointer shift magnitude) and rules (interval > 0, etc.).
- **`port/`**
  - `pointer.go` -- the driven port `Pointer` with a method like
    `Nudge(dx, dy int) error` (an abstraction over "wiggle the pointer").
  - `keeper.go` -- the driving port `Keeper` with a method `Run(ctx) error`
    (start the activity-keeping loop).
- **`service/`**
  - `keeper_service.go` -- the `Keeper` implementation: a ticker over `Interval`,
    on each tick it calls `Pointer.Nudge(+1,0)` -> pause -> `Nudge(-1,0)`; exits
    cleanly on `ctx` cancellation.
- **`adapter/`**
  - `cgpointer.go` -- the `Pointer` implementation via cgo + CoreGraphics
    (`CGEventCreateMouseEvent` / `CGEventPost`), with a `//go:build darwin` build
    tag.

The entry point **`cmd/sysmon/main.go`** reads the flags (`-interval`, `-quiet`),
creates `adapter.CGPointer`, passes it into `service.NewKeeperService(...)`,
subscribes to `SIGINT/SIGTERM` via `context`, and runs `keeper.Run(ctx)`.

### 10.4. Benefits of this split for sysmon

- **Testability.** `KeeperService` is tested with a fake `Pointer` (mock),
  without real mouse movement and without macOS.
- **Portability.** To support Linux/Windows it is enough to add a new `Pointer`
  adapter for that OS with a build tag -- the domain and service do not change.
- **Consistency.** A familiar structure that is easier to maintain.

> Note: the current implementation in the repo is still flat (`main.go` at the
> root). Refactoring into the structure described here is a separate task under
> this spec.
