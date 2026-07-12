# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-07-12

### Added

- Realistic random cursor roaming: on each interval the cursor glides over a
  series of small steps to a fresh uniform-random point within the main
  display, bounded to the screen (never off-screen, no drift) and interruptible
  mid-glide.
- `-interval` flag (Go duration, default `10s`) and `-quiet` flag (raise the log
  level so only warnings and errors print).
- Structured logging via `log/slog`; a startup Accessibility-permission
  preflight warning.
- Makefile targets `build` / `run` / `start` / `stop` / `status` / `clean`, with
  an `INTERVAL` override (e.g. `make start INTERVAL=3s`).
- Hexagonal (ports & adapters) architecture: a pure domain movement planner,
  `Pointer` / `Keeper` ports, `KeeperService`, and a CoreGraphics `CGPointer`
  adapter behind `//go:build darwin`.
- Continuous integration (build / lint / test) on macOS; strict golangci-lint;
  domain and service unit-test coverage at 100%.
- Prebuilt `sysmon-darwin-arm64` binary attached to the release.

### Notes

- The runtime binary has zero third-party dependencies (standard library only).
- macOS on Apple Silicon; requires Accessibility permission to deliver events.

[1.0.0]: https://github.com/dezween/sysmon/releases/tag/v1.0.0
