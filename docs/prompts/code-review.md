# Code Review Command (sysmon)

Perform a detailed code review for task: $ARGUMENTS

## System Context

This is **sysmon** -- a small local command-line utility written in Go for **macOS**.
It keeps the machine "active" by posting a real mouse-move event every N seconds
(resets the system idle timer so the screen does not sleep). Review all code
against the following real characteristics of THIS project -- do not import
distributed-systems assumptions that do not apply here.

- **Scale:** single user, single machine. No servers, no concurrency beyond one
  ticker goroutine. Distributed "high load" is irrelevant -- do NOT flag RPS,
  HPA, pooling, or messaging throughput.
- **Efficiency IS a goal (but not distributed scale):** the tool runs for hours
  in the background, so it must be lightweight -- near-zero idle CPU, tiny memory
  footprint, no busy-waiting, battery-friendly. Optimizing local resource usage
  is welcome; adding distributed-systems machinery is not.
- **Platform:** macOS only (arm64). Uses **cgo** + the CoreGraphics /
  ApplicationServices C API (`CGEventCreate`, `CGEventGetLocation`,
  `CGEventCreateMouseEvent`, `CGEventPost`). Requires Xcode Command Line Tools.
- **Runtime dependency:** needs macOS Accessibility (TCC) permission for the
  controlling terminal, or synthetic mouse events are silently dropped.
- **No network, no database, no external services, no messaging.** Any review
  item about Pub/Sub, Kafka, Redis, WebSocket, k8s, or graceful pod shutdown is
  out of scope -- mark N/A and move on.
- **Target architecture:** hexagonal (ports & adapters), per `docs/TASK.md`
  section 10 -- `cmd/sysmon/main.go` + `internal/activity/{domain,port,service,adapter}`.
  Correct layering (dependencies point inward to the domain) is the single most
  important architectural check for this project.
- **Dependencies:** standard library only; zero third-party Go modules. Flag any
  new external dependency as something requiring justification.
- **Best practices:** idiomatic Go, clean cgo memory management (`CFRelease` for
  every created CoreFoundation object), correct context / signal handling, no
  goroutine or ticker leaks, testable code (fake the `Pointer` port -- never move
  the real mouse in tests).

**When reviewing, evaluate against THIS context. Correctness, clean architecture,
resource-leak safety, lightweight footprint (low idle CPU / memory), and
testability matter here -- scale/distributed concerns do not.**

---

## Instructions

1. **Get task information:**
   - If a GitHub PR is provided or open for the branch, get details via
     `gh pr view` and `gh pr diff`.
   - Otherwise review the working diff via `git diff` (or `git diff main...HEAD`).

2. **Analyze changes:**
   - Review all changed files.
   - Identify architectural decisions and whether they match the hexagonal target.
   - Check Go best practices and idioms.
   - **Verify cgo memory safety** (no leaked CoreFoundation objects, correct
     `CFRelease`).
   - **Verify concurrency correctness** (ticker/goroutine cleanup, context
     cancellation, SIGINT/SIGTERM handling, no leaks on shutdown).
   - **Verify testability** (domain/service isolated from the macOS adapter; the
     `Pointer` port is fakeable).

3. **Create review document in this format:**

```markdown
# [Task]: [Title] Review

**Repo:** dezween/sysmon
**Branch:** `[branch-name]`
**Date:** [YYYY-MM-DD]

---

## Executive Summary

**Implementation Quality:** (X/5)
**Architecture (hexagonal layering):** OK / Needs work / Broken

| # | Issue | Severity | Status |
|---|-------|----------|--------|
| 1 | [Issue description] | P0 / P1 / P2 | OK / Issue |

---

## Detailed Analysis

### Issue #N: [Issue Title]

**File:** `path/to/file.go`
**Lines:** XX-YY

**Current Code:**
\`\`\`go
// code snippet
\`\`\`

#### Problem Explanation

[Detailed explanation of WHY this is a problem]

**Technical Details:**
- [What happens under the hood]
- [Why the current approach is problematic]

**Impact:**
- [Concrete failure scenario for this local utility]
- [e.g. resource leak, ticker not stopped, cgo object leaked, event silently
  dropped, mouse actually moving in a test, broken layering]

#### Solution Options

##### Option A: [Name] -- RECOMMENDED

\`\`\`go
// solution code
\`\`\`

**Pros:**
- [Advantage 1]
- [Advantage 2]

**Cons:**
- [Disadvantage 1]

**Risk:** Low / Medium / High

##### Option B: [Name]

\`\`\`go
// alternative solution
\`\`\`

**Pros:**
- [Advantage 1]

**Cons:**
- [Disadvantage 1]

**Risk:** Low / Medium / High

#### Recommendation

[Explain WHY Option X is recommended for this specific case]

**Verdict:** [Assessment]

---

## Best Practices Checklist

### Correctness & Behavior
| Check | Status | Notes |
|-------|--------|-------|
| Mouse event actually posted (not just cursor warp) | OK/Issue | kCGEventMouseMoved via CGEventPost |
| Interval configurable and validated | OK/Issue | -interval flag, > 0 |
| Clean shutdown on SIGINT/SIGTERM | OK/Issue | exit 0, ticker stopped |
| Idle timer actually reset | OK/Issue | HID event tap |

### Architecture (hexagonal)
| Check | Status | Notes |
|-------|--------|-------|
| domain has no external/system imports | OK/Issue | pure Go, no cgo/os |
| service depends only on domain + port | OK/Issue | no adapter import |
| adapter implements port, holds all cgo/macOS code | OK/Issue | |
| dependency direction points inward | OK/Issue | cmd -> service -> port <- adapter |
| cmd/ only wires dependencies (no logic) | OK/Issue | |

### Go & Concurrency
| Check | Status | Notes |
|-------|--------|-------|
| No goroutine leak | OK/Issue | |
| Ticker stopped (defer Stop) | OK/Issue | |
| context cancellation propagated | OK/Issue | |
| No data races | OK/Issue | go test -race |
| Errors handled, not ignored | OK/Issue | |

### cgo / macOS
| Check | Status | Notes |
|-------|--------|-------|
| Every created CF object released (CFRelease) | OK/Issue | no CF memory leak |
| Platform build tag present (//go:build darwin) | OK/Issue | on adapter |
| Accessibility-permission failure handled/documented | OK/Issue | silent-drop risk |

### Resource Efficiency & Footprint
| Check | Status | Notes |
|-------|--------|-------|
| Idle CPU near zero (sleeps between ticks, no busy-wait) | OK/Issue | time.Ticker/select, not a spin loop |
| No per-tick allocation growth / leak | OK/Issue | stable memory over time |
| Minimal allocations on the hot path | OK/Issue | reuse where cheap |
| Battery / power friendly | OK/Issue | no needless wakeups, no polling faster than needed |
| Small binary and memory footprint | OK/Issue | stdlib only, no heavy deps |
| No unnecessary background goroutines | OK/Issue | one worker is enough |

### Testing
| Check | Status | Notes |
|-------|--------|-------|
| KeeperService unit-tested with a fake Pointer | OK/Issue | no real mouse move |
| Domain rules tested | OK/Issue | interval validation etc. |
| Tests do not require macOS permission | OK/Issue | |

### Safety & Quality
| Check | Status | Notes |
|-------|--------|-------|
| No process-management footguns | OK/Issue | e.g. broad pkill matching unrelated procs |
| Flag input validated | OK/Issue | |
| No new third-party dependency without reason | OK/Issue | stdlib only |
| Godoc comments on exported symbols | OK/Issue | |
| Naming and style idiomatic | OK/Issue | |

---

## Final Verdict

**Status:** APPROVED / APPROVED WITH COMMENTS / CHANGES REQUESTED

### What's Excellent:
- [Positive point with explanation]

### Required Changes (P0 - must fix before merge):
- [Issue] - [Why critical]

### Recommended Improvements (P1 - fix soon):
- [Issue] - [Why]

### Nice to Have (P2 - future):
- [Suggestion]

---

## GitHub PR Comment

\`\`\`markdown
## [VERDICT]

### Implementation Quality: X/5
### Architecture: [OK / Needs work]

**Summary:**
[2-3 sentences about what was reviewed and overall quality]

### Issues Found:
| # | Severity | Issue | Recommended Fix |
|---|----------|-------|-----------------|
| 1 | P0 | [Issue] | [Solution] |
| 2 | P1 | [Issue] | [Solution] |

### Best Practices:
- [What's done well]
- [What needs attention]

[LGTM / Changes requested]
\`\`\`

---

**Review Date:** [DATE]
```

4. **Save review:**
   - Create file `docs/reviews/[task]-REVIEW.md` in this project.
   - Alert immediately if critical P0 issues are found.

5. **CRITICAL RULES:**
   - Everything in English
   - NO AI mentions anywhere
   - NO time estimates (hours, days, weeks)
   - Write as Team Lead for developers
   - Always provide multiple solution options with a recommendation
   - Explain problems in detail (WHY it's a problem)
   - Judge against THIS project's context (local macOS CLI) -- never invent
     scale/distributed/messaging concerns that do not apply
   - Architecture must be correct -- respect hexagonal layering from docs/TASK.md
   - Follow idiomatic Go best practices

6. **TEXT FORMATTING RULES:**
   - NEVER use Unicode arrows: use ASCII arrows instead: -> <-
   - NEVER use fancy quotes: use straight quotes: " '
   - NEVER use the ellipsis character: use three dots: ...
   - NEVER use em-dash: use double dash: --
   - NEVER use bullet-point glyphs: use dash or asterisk: - *
   - Keep all text ASCII-compatible where possible
