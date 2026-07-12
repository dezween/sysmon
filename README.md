# sysmon

[![CI](https://github.com/dezween/sysmon/actions/workflows/ci.yml/badge.svg)](https://github.com/dezween/sysmon/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/platform-macOS%20arm64-lightgrey?logo=apple)
![Release](https://img.shields.io/badge/release-v1.0.0-brightgreen)
![License](https://img.shields.io/badge/license-MIT-blue)

A tiny, zero-dependency macOS utility that keeps the machine "active": every
`-interval` it moves the cursor realistically at random across the screen -- a
smooth, human-like glide to a fresh random point -- which resets the system idle
timer so the screen does not sleep and presence status stays active.

- Realistic random roaming (not a teleport): a multi-step glide to a new random
  on-screen point, bounded to the display, no drift, interruptible mid-move.
- Configurable interval, quiet mode, clean SIGINT/SIGTERM shutdown, near-zero
  idle CPU.
- Standard-library-only runtime; hexagonal (ports & adapters) architecture.

## Requirements

- macOS on Apple Silicon (arm64); Xcode Command Line Tools
  (`xcode-select --install`) for cgo.
- **Accessibility permission** for the terminal (or app) you launch it from:
  System Settings -> Privacy & Security -> Accessibility. Without it, macOS
  silently drops the synthetic mouse events; sysmon warns about this at startup.

## Install

### Download the release

Grab `sysmon-darwin-arm64` from the
[latest release](https://github.com/dezween/sysmon/releases/latest):

```sh
chmod +x sysmon-darwin-arm64
xattr -d com.apple.quarantine sysmon-darwin-arm64   # clear Gatekeeper quarantine
./sysmon-darwin-arm64 -interval 10s
```

### Build from source

```sh
make build            # builds ./sysmon from ./cmd/sysmon
```

## Usage

Foreground (Ctrl+C to quit):

```sh
make run              # or: make run INTERVAL=3s
```

Background (no window):

```sh
make start            # default interval 10s
make start INTERVAL=3s
make status
make stop
```

Directly:

```sh
./sysmon -interval 30s         # every 30s
./sysmon -quiet -interval 10s  # suppress routine logs
```

Flags: `-interval` (Go duration, default `10s`, must be > 0) and `-quiet`
(raise the log level so only warnings and errors print).

## How it works

On each tick the cursor glides over a series of small steps to a new uniformly
random point within the main display, posting real `MouseMoved` events via
CoreGraphics. Between ticks it sleeps on a `time.Ticker`, so idle CPU is
effectively zero and the process is battery friendly.

## Architecture

Hexagonal (ports & adapters):

```
cmd/sysmon            entry point: flags, dependency injection, signal handling
internal/activity/
  domain              pure movement planner + Interval (no cgo/os; GOOS=linux-buildable)
  port                Pointer / Keeper interfaces (+ generated mock)
  service             KeeperService -- the roam loop
  adapter             CGPointer -- CoreGraphics, //go:build darwin
```

The runtime binary depends only on the standard library. Tests use testify +
go.uber.org/mock + `testing/synctest`.

## Development

```sh
make build
go test -race ./...
golangci-lint run ./...
```

CI runs build / lint / test on macOS for every push and pull request.

## License

MIT -- see [LICENSE](LICENSE).
