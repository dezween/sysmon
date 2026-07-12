---
status: complete
---

# Quick task: realistic random cursor roaming

Replaced the invisible 1px nudge with realistic random roaming. Each `-interval`
tick, the cursor glides over ~40 small steps to a fresh uniform-random point
within the main display bounds, honoring ctx cancellation between steps. Old
`Nudge`/`Offset` code removed; `Interval` kept.

- **domain**: pure `Planner` with an injected `RandomSource` seam (satisfied by
  `*math/rand/v2.Rand`); clamped targets; deterministic tests (bounds,
  continuity, ends-at-target, corner / 1x1 / target==current).
- **port**: `Pointer` reshaped to `Position()` / `Bounds()` / `MoveTo(x,y)`;
  mock regenerated via mockgen.
- **service**: `KeeperService` roams once per tick, driving the path through
  `MoveTo`; synctest + mock tests.
- **adapter**: `CGPointer` implements Position/Bounds/MoveTo via CoreGraphics
  (`//go:build darwin`), releasing every CoreFoundation object.
- **cmd**: runtime-seeded `*rand.Rand` + planner injected; `-interval`/`-quiet`/
  signal-exit-0 and the accessibility preflight unchanged.

Runtime binary stays stdlib-only; test deps testify + go.uber.org/mock only.
Gates green: `go build ./...`, `golangci-lint run ./...` (0 issues),
`go test -race ./...`, and `GOOS=linux` build of domain/port/service.

Branch: `feat/random-roaming` (commits 043c51a..5bc019b).
