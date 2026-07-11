# Technology Stack

**Analysis Date:** 2026-07-11

## Languages

**Primary:**
- Go 1.26.5 — Full application, enabled with cgo for system framework integration

## Runtime

**Environment:**
- Go 1.26.5 (specified in `go.mod`)

**Package Manager:**
- Go modules
- Lockfile: None (only `go.mod` with no external dependencies)

## Frameworks

**Core:**
- None (stdlib only)

**Build/Dev:**
- Makefile — Build automation (targets: `build`, `run`, `start`, `stop`, `status`, `clean`)

## Key Dependencies

**Critical:**
- None — Zero external Go dependencies. Uses only stdlib: `flag`, `log`, `os`, `os/signal`, `syscall`, `time`

**Infrastructure:**
- cgo with macOS CoreGraphics (`-framework ApplicationServices -framework CoreGraphics`)
  - Required for system mouse event generation
  - Statically linked at compile time

## Configuration

**Environment:**
- Command-line flags:
  - `-interval` (default: 10s) — Duration between mouse-move events (accepts Go duration syntax: `10s`, `30s`, `1m`)
  - `-quiet` (default: false) — Suppress stdout logs (redirects to stderr only)

**Build:**
- cgo enabled by default
- Xcode Command Line Tools required (`xcode-select --install`)

## Platform Requirements

**Development:**
- macOS (arm64)
- Go 1.26.5
- Xcode Command Line Tools (for cgo)

**Production:**
- macOS 26+
- arm64 (Apple Silicon)
- Terminal.app or iTerm2 with Accessibility permission granted (macOS TCC)

---

*Stack analysis: 2026-07-11*
