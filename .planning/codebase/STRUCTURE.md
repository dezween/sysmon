# Codebase Structure

**Analysis Date:** 2026-07-11

## Directory Layout

```
sysmon/
├── main.go           # Single-file app (current flat structure)
├── go.mod            # Module declaration (sysmon, no external deps)
├── Makefile          # Build automation
├── README.md         # User instructions
├── .gitignore        # Exclude: sysmon binary, .DS_Store, .idea/
└── docs/
    └── TASK.md       # Technical specification & target architecture (section 10)
```

## Directory Purposes

**Project Root:**
- Contains: Single application binary in development; configuration files
- Key files: `main.go` (current monolithic structure), `go.mod` (zero external deps)

**docs/:**
- Purpose: Technical documentation and requirements
- Key files: `TASK.md` (full spec including target hexagonal architecture, acceptance criteria, future refactoring)

## Key File Locations

**Entry Points:**
- `main.go:32` — `func main()`: CLI flag parsing, ticker setup, signal handling, keep-alive loop

**Configuration:**
- Command-line flags only:
  - `-interval string` (default: `10s`) — Time between mouse-nudge events
  - `-quiet bool` (default: `false`) — Suppress stdout logs

**Core Logic:**
- `main.go:10-19` — `nudge(dx, dy)` cgo C function: get cursor position, post synthetic mouse-moved event
- `main.go:51-65` — Main event loop: ticker select block calling `C.nudge()`
- `main.go:3-20` — cgo block: C imports (`#cgo LDFLAGS`, `#include <ApplicationServices/ApplicationServices.h>`)

**Testing:**
- None currently (see CONCERNS.md)

## Naming Conventions

**Files:**
- Lowercase single-word names: `main.go`, `go.mod`
- Documentation in `docs/` subdirectory

**Functions (Go):**
- PascalCase for exported: `main()` is special case (entry point)
- Comments in Russian (matching project's primary language)

**Imports:**
- stdlib only: no external Go modules

## Where to Add New Code

### Before Refactoring (Current State)

**Bug fixes / Minor features:**
- Edit `main.go` directly
- Keep `nudge()` C function at top of file (in cgo block)
- Keep `main()` loop logic inline

### After Refactoring (Target Hexagonal Structure)

**New features (post-refactor):**
1. **Domain logic** → `internal/activity/domain/` (value objects, rules, no I/O)
2. **Use cases** → `internal/activity/service/` (orchestration, business logic)
3. **Port interfaces** → `internal/activity/port/` (abstraction contracts)
4. **Platform code** → `internal/activity/adapter/` (cgo, OS calls, infrastructure)
5. **Entry point** → `cmd/sysmon/main.go` (flags, DI, signal setup only)

**Unit tests (post-refactor):**
- `internal/activity/service/keeper_service_test.go` — KeeperService with mock Pointer
- Mock: `internal/activity/service/mock_pointer_test.go` or inline interface implementation

**Utilities (if needed):**
- `pkg/` directory (currently empty, reserved for reusable shared code)

## Special Directories

**docs/:**
- Purpose: Technical specifications and architecture documentation
- Generated: No
- Committed: Yes
- Key file: `TASK.md` (full requirements and target architecture in sections 6-10)

**Makefile targets:**
```makefile
build    # go build -o sysmon .
run      # build + execute in foreground (./sysmon)
start    # build + execute in background (nohup ... &), save PID to /tmp/sysmon.pid
stop     # kill background process by PID
status   # check if background process is running
clean    # remove sysmon binary and PID file
```

## File-by-File Reference

**`main.go` (67 lines):**
- Lines 3-20: cgo C preamble with nudge() implementation
- Lines 23-30: Go imports (flag, log, os, os/signal, syscall, time)
- Lines 32-65: func main() — flags, ticker, signal handling, loop

**`go.mod` (3 lines):**
- Module declaration: `module sysmon`
- Go version: `go 1.26.5`
- No dependencies

**`Makefile` (40 lines):**
- BINARY var: `sysmon`
- PIDFILE var: `/tmp/sysmon.pid`
- 6 targets: build, run, start, stop, status, clean
- All comments in Russian

**`README.md` (51 lines):**
- Russian language
- Sections: build instructions (cgo + Xcode), usage examples, flag reference, macOS Accessibility requirement

**`.gitignore` (10 lines):**
- Exclude: `/sysmon` (binary), `sysmon` (no-path variant), `.DS_Store`, `.idea/`

**`docs/TASK.md` (199 lines):**
- Section 1-5: Goals, context, functional/non-functional requirements, scope
- Section 6: Technical approach (cgo, API calls, time.Ticker pattern)
- Section 7-8: macOS TCC (Accessibility) permission, acceptance criteria (8 items, 2 pending)
- Section 10: **Target hexagonal architecture** — cmd/internal/pkg layout, dependency direction (domain ← service ← adapter), module detail (domain/port/service/adapter), concrete raison d'être (testability, portability)

---

*Structure analysis: 2026-07-11*
