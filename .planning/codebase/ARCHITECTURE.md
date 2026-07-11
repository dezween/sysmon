# Architecture

**Analysis Date:** 2026-07-11

## Current State vs Target

**⚠️ Important:** Current implementation is flat (`main.go` in repo root). Target architecture is hexagonal (ports & adapters), documented in `docs/TASK.md` section 10. This section describes both.

## Current Architecture

### As-Is: Flat Monolithic

```
┌─────────────────────────────────────────────────┐
│           main.go (single file)                 │
│  ├─ Flag parsing (-interval, -quiet)            │
│  ├─ Signal handling (SIGINT, SIGTERM)           │
│  ├─ Ticker loop                                 │
│  └─ cgo nudge() call (CoreGraphics direct)      │
└─────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────┐
│    macOS CoreGraphics (cgo block at top)        │
│  CGEventCreate → CGEventGetLocation →           │
│  CGEventCreateMouseEvent → CGEventPost          │
└─────────────────────────────────────────────────┘
```

**Entry Point:** `main()` in `main.go:32`

### Target: Hexagonal (Ports & Adapters)

```
┌──────────────────────────────────────────────────────────┐
│            cmd/sysmon/main.go                            │
│  (Flag parsing, DI, signal handling, orchestration)      │
└──────────────────────────────────────────────────────────┘
               │
               ▼
┌──────────────────────────────────────────────────────────┐
│       internal/activity/service/keeper_service.go        │
│  (Use case: maintain activity with ticker + Pointer)     │
└──────────────────────────────────────────────────────────┘
       │          │
       │          ▼
       │   ┌────────────────────┐
       │   │ internal/activity/ │
       │   │ port/pointer.go    │
       │   │ (Nudge interface)  │
       │   └────────────────────┘
       │          ▲
       │          │
       ▼          ▼
┌──────────────────────────────────────┐
│ internal/activity/domain/ (types)    │
├──────────────────────────────────────┤
│ internal/activity/adapter/          │
│ cgpointer.go (CoreGraphics impl)     │
└──────────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility | File (Current) | File (Target) |
|-----------|---|---|---|
| Entry point | CLI flags, dependency injection, signal handling | `main.go:32` | `cmd/sysmon/main.go` |
| Use case (Keeper) | Maintain activity: ticker loop, call Pointer.Nudge() on interval | `main.go:51-65` | `internal/activity/service/keeper_service.go` |
| Port (Pointer) | Interface: abstract mouse movement | `main.go:10-19` (inline C code) | `internal/activity/port/pointer.go` |
| Domain | Interval, Offset value objects, validation rules | Not separate | `internal/activity/domain/` |
| Adapter (CGPointer) | Concrete: CoreGraphics implementation of Pointer | `main.go:3-20` (cgo block) | `internal/activity/adapter/cgpointer.go` |

## Data Flow

### Primary Request Path: Keep System Active

1. **Start** — `main()` parses `-interval` and `-quiet` flags (`main.go:32-35`)
2. **Setup** — Create ticker with parsed interval (`main.go:44`)
3. **Signal handling** — Subscribe to SIGINT/SIGTERM (`main.go:41-42`)
4. **Loop** — Select on ticker and signal channels (`main.go:51-65`)
5. **Tick action** — Call `C.nudge(1, 0)` → sleep 40ms → call `C.nudge(-1, 0)` (`main.go:56-58`)
6. **Tick detail** — Each nudge:
   - Get current mouse position via `CGEventCreate(NULL)` + `CGEventGetLocation()` (`main.go:11-12`)
   - Create synthetic mouse-moved event at `(pos.x + dx, pos.y + dy)` (`main.go:16`)
   - Post event via `CGEventPost(kCGHIDEventTap, move)` to trigger idle-timer reset (`main.go:17`)
   - Clean up with `CFRelease()` (`main.go:13, 18`)
7. **Termination** — SIGINT/SIGTERM stops ticker and exits with code 0 (`main.go:59-64`)

**State Management:** Single-threaded event loop via `select {}`. No global state except signal channel and ticker.

## Key Abstractions

**nudge(dx, dy) — System Activity Nudge:**
- Purpose: Move mouse by offset and post real MouseMoved event (critical to reset idle timer)
- Current: cgo C function in `main.go:10-19`
- Target: `internal/activity/port/Pointer.Nudge(dx, dy int) error` interface, implemented by `internal/activity/adapter/cgpointer.go`
- Why abstraction matters: Enables testing with mock Pointer; enables platform portability (Linux/Windows adapters)

**Interval Configuration:**
- Purpose: User-specified duration between nudges (default 10s)
- Current: Flag `-interval` parsed in `main()`, passed to `time.NewTicker()`
- Target: `internal/activity/domain/Interval` value object with validation

## Entry Points

**main() — CLI Entry:**
- Location: `main.go:32`
- Triggers: User runs `./sysmon [flags]`
- Responsibilities: Parse flags, create ticker, subscribe to signals, orchestrate loop

## Architectural Constraints

- **Threading:** Single-threaded event loop via `select {}`. No goroutines.
- **Global state:** None. All state is local to `main()` (interval, quiet flag, signal channel, ticker).
- **Circular imports:** None (flat structure currently; target has clear dependency direction: domain ← service ← adapter).
- **Platform binding:** Currently: cgo block in `main.go` tightly couples CoreGraphics API. Target: adapter pattern isolates platform code to `internal/activity/adapter/` with `//go:build darwin` tags.
- **Signal safety:** Signal handling via `signal.Notify()` into channel; safe, doesn't block.

## Architectural Gap (Current → Target)

**What's missing for target architecture:**

1. **Directory structure** — Must create `cmd/sysmon/`, `internal/activity/domain/`, `internal/activity/port/`, `internal/activity/service/`, `internal/activity/adapter/`.
2. **Port/interface extraction** — `nudge()` C function must become `Pointer` interface in `port/pointer.go`.
3. **Service layer** — `KeeperService` struct in `service/keeper_service.go` orchestrating ticker + Pointer, replacing inline loop in `main()`.
4. **Domain layer** — `Interval` and `Offset` value objects in `domain/`, with validation.
5. **Adapter implementation** — Move cgo block to `adapter/cgpointer.go` implementing `Pointer`.
6. **DI in main** — `cmd/sysmon/main.go` becomes thin: parse flags → create CGPointer → wire to KeeperService → run.
7. **Testing** — `KeeperService` can now be unit-tested with fake `Pointer` (no cgo, no macOS required).
8. **Build tags** — Mark platform-specific code with `//go:build darwin`.

**Impact if not refactored:**
- Cannot unit-test keeper logic without macOS/Accessibility permission.
- Platform portability blocked (Linux/Windows support requires code duplication or conditional compilation hacks).
- Domain logic tightly coupled to infrastructure (cgo calls embedded in main loop).

---

*Architecture analysis: 2026-07-11*
