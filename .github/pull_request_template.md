<!--
  sysmon pull request template.
  Fill in each section. Delete a section only if it is genuinely not applicable
  (and say why). Keep commit messages and this description free of AI mentions.
-->

## Summary

<!-- One or two sentences: what does this PR do and why? -->

## Type of change

<!-- Put an x in the boxes that apply. -->

- [ ] ✨ Feature — new user-facing capability
- [ ] 🐛 Bug fix — fixes incorrect behavior
- [ ] ♻️ Refactor — no behavior change (e.g. architecture / restructure)
- [ ] ⚡ Performance / resource efficiency
- [ ] 🧪 Tests
- [ ] 🔧 Tooling / CI / build
- [ ] 📝 Docs / planning

## What changed

<!-- Bullet the concrete changes. Reference files where useful, e.g. `internal/activity/service/keeper_service.go`. -->

-
-

## Why / context

<!-- The reasoning. Link the GSD phase and requirements this satisfies. -->

- **Phase:** <!-- e.g. Phase 1 — Hexagonal Refactor, or "n/a (tooling)" -->
- **Requirements:** <!-- e.g. ARCH-01, TEST-01, LOG-01 -->
- **Related:** <!-- issue / discussion links, if any -->

## How it was tested

<!-- Show the commands and their result. CI must also be green. -->

- [ ] `go build ./...`
- [ ] `go test -race ./...`
- [ ] `golangci-lint run ./...`
- [ ] Manual check on macOS (describe): <!-- e.g. ran `make start`, screen stayed awake, `make stop` killed only sysmon -->

## Checklist

<!-- These mirror docs/prompts/code-review.md — the reviewer will check them. -->

### Architecture & code
- [ ] Hexagonal layering respected — `domain` imports nothing from `port`/`service`/`adapter`; `service` depends only on `domain` + `port`; cgo/macOS code lives only in `adapter`
- [ ] Dependency direction points inward to the domain
- [ ] No new third-party dependency (or it is justified below)
- [ ] Exported symbols have godoc comments; naming is idiomatic

### macOS / cgo
- [ ] Every created CoreFoundation object is released (`CFRelease`) — no CF leak
- [ ] Platform-specific code is behind `//go:build darwin`
- [ ] Accessibility-permission behavior considered (events silently drop without it)

### Concurrency & resources
- [ ] No goroutine/ticker leak; `context` cancellation and SIGINT/SIGTERM handled; exits 0
- [ ] Lightweight: near-zero idle CPU, no busy-wait, no needless allocations on the hot path

### Safety & hygiene
- [ ] No process-management footguns (e.g. broad `pkill` matching unrelated processes)
- [ ] Flags/inputs validated
- [ ] No secrets, tokens, or corporate identity leaked; commits use the personal identity

## Risks & rollback

<!-- What could go wrong? How to revert? For a local utility this is usually low. -->

## Out of scope / follow-ups

<!-- What this PR deliberately does NOT do; link deferred items. -->

## Reviewer notes

<!-- Anything you want the reviewer to focus on. -->
