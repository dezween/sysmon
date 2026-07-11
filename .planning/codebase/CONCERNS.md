# Codebase Concerns

**Analysis Date:** 2026-07-11

## Tech Debt

### 1. Flat Architecture vs Documented Hexagonal Target

**Issue:** Codebase is a single flat `main.go` monolith. `docs/TASK.md` section 10 explicitly specifies target hexagonal architecture (ports & adapters) but it is **not yet implemented**.

**Files:** `main.go` (entire application)

**Impact:**
- Cannot unit-test keeper logic in isolation
- Cannot mock platform dependencies (CoreGraphics)
- Cannot port to other platforms (Linux/Windows) without code duplication
- Business logic tightly coupled to infrastructure (cgo calls in main loop)

**Fix approach:**
- Refactor to target structure: `cmd/sysmon/main.go` + `internal/activity/{domain,port,service,adapter}`
- Extract `nudge()` logic to `Pointer` interface in `port/pointer.go`
- Extract ticker loop to `KeeperService` in `service/keeper_service.go`
- Implement CoreGraphics in `adapter/cgpointer.go` with `//go:build darwin`
- Write unit tests for `KeeperService` with mock `Pointer`
- **Acceptance criteria** (section 8 of TASK.md): "Проект приведён к гексагональной структуре" — currently INCOMPLETE

---

## No Tests

**Issue:** Zero test coverage. No `_test.go` files. No test framework configured.

**Files:** (No test files exist)

**Impact:**
- No automated verification of keeper loop correctness
- No regression detection when refactoring
- Cannot validate interval accuracy or signal handling
- Blocks acceptance criterion: "KeeperService покрыт unit-тестом"

**Fix approach:**
- Wait for architecture refactor (hexagonal) — cannot test flat monolith effectively
- Once `KeeperService` exists, add `keeper_service_test.go` with mock `Pointer`
- Unit tests: interval accuracy, context cancellation, nudge call sequence
- **Priority:** MEDIUM — blocked by refactoring; unblocks once architecture complete

---

## Platform-Specific Code Not Isolated

**Issue:** cgo and CoreGraphics calls live in `main.go` alongside generic Go logic. No build tags separating macOS-only code.

**Files:** `main.go:3-20` (cgo block inline with main logic)

**Impact:**
- Compiler fails if attempted on non-macOS systems (cgo block is unconditional)
- Cannot build "framework" for cross-platform use without conditional compilation
- Difficult to maintain clear platform abstraction

**Fix approach:**
- Add `//go:build darwin` tag to files with cgo/CoreGraphics
- Move cgo to `internal/activity/adapter/cgpointer.go` with build tag
- Create platform-agnostic `Pointer` interface in `port/pointer.go`
- Future: add `//go:build linux` implementation for Linux support
- **Benefit:** Future portability; documentation of platform boundaries

---

## Silent Failure if Accessibility Permission Missing

**Issue:** If terminal lacks macOS Accessibility permission, `sysmon` runs without error, but `CGEventPost()` silently fails. User sees no indication that idle-timer is NOT being reset.

**Files:** `main.go:56-58` (nudge calls), `main.go:3-19` (cgo C function)

**Impact:**
- User may think sysmon is working; system still goes to sleep
- No error signal to debug or diagnose
- High cognitive load to discover permission requirement

**Current Mitigation:**
- `README.md` (lines 42-50) documents Accessibility requirement
- `docs/TASK.md` section 7 explains TCC and remediation

**Fix approach:**
- (No code change needed; mitigation is documentation + user education)
- **Alternative:** Add validation at startup: attempt trial nudge and log warning if it fails (complex, requires permission check API)
- **Recommendation:** Improve README visibility; add prominent message in startup log mentioning permission requirement
- **Priority:** LOW — OS design, not application bug; documented

---

## MacOS-Only Runtime Dependency Easy to Forget

**Issue:** Build succeeds on macOS; accepts Xcode Command Line Tools at compile time. But runtime ALSO requires Accessibility permission, which is a separate check at execution time. Easy for deployer to forget TCC step.

**Files:** `main.go` (implicit requirement), `README.md:42-50` (documented)

**Impact:**
- Silent failure in deployment scenarios (CI/CD, remote runs)
- User error (forgot to grant permission)
- Difficult to debug post-deployment

**Current Mitigation:**
- `docs/TASK.md` section 7 and section 8 acceptance criteria explicitly call out permission requirement
- Makefile has no validation step

**Fix approach:**
- Add pre-flight check in `cmd/sysmon/main.go` (post-refactor):
  - Test nudge on startup; log warning if CGEventPost appears to fail
  - Example: `CGEventPost(kCGHIDEventTap, testEvent)` with result validation
- Add `make check-permissions` target to Makefile
- **Priority:** MEDIUM — affects production reliability; low fix cost

---

## Hardcoded Values

**Issue:** Nudge offset (+1px, -1px) and delay between nudges (40ms) are literal constants in C code, not parameterized.

**Files:** `main.go:15` (offset), `main.go:56-57` (delay)

**Impact:**
- Cannot adjust nudge magnitude or timing without recompiling
- No flexibility for future optimization or platform-specific tuning

**Fix approach:**
- Extract to domain constants or parameters post-refactor
- `Offset` value object in `internal/activity/domain/offset.go` (with validation)
- `nudgeDelay` as configuration (hard-coded in domain or read from env)
- **Priority:** LOW — current values work; low necessity for user configuration

---

## Missing Acceptance Criteria (Task.md Section 8)

**Issue:** Two acceptance criteria marked `[ ]` (not completed):

1. **Line 108-109:** Manual verification that Accessibility permission works on target machine and screen doesn't sleep
   - Status: Requires physical macOS machine with TCC setup and manual testing
   - Blocker: None; documentation is present

2. **Lines 110-113:** Refactoring to hexagonal architecture + unit test of KeeperService
   - Status: **NOT IMPLEMENTED** — this is the primary tech debt item
   - Blocker: Blocks testing, portability, clean code

**Files:** `docs/TASK.md:108-113`

**Impact:**
- Project marked incomplete by original requirements
- Testing requirement unfulfilled
- Architecture remains debt

**Fix approach:**
- See "Flat Architecture vs Documented Hexagonal Target" (item 1) — this is the same issue
- Refactor to target structure (1-2 day task post-analysis)
- Write `keeper_service_test.go` with mock Pointer (quick once structure exists)
- Manual test on target machine (owner responsibility)

---

## Summary: Priority Matrix

| Issue | Priority | Impact | Effort |
|-------|----------|--------|--------|
| Flat architecture vs target | **HIGH** | Blocks testing, portability, clean code | Medium (refactor: ~4-8h) |
| No tests | **HIGH** | No verification; blocks acceptance | Medium (once architecture exists: ~2h) |
| Platform code not isolated | **MEDIUM** | Future cross-platform blocked | Low (move to adapter: ~1h) |
| Silent Accessibility failure | **LOW** | User confusion; documented workaround exists | Low (doc clarification only) |
| Missing permission check at runtime | **MEDIUM** | Helps deployment; optional | Low (startup validation: ~1h) |
| Hardcoded nudge values | **LOW** | Not needed unless tuning required | Low (domain constants: ~1h) |
| Acceptance criteria incomplete | **MEDIUM** | Project not "done" per spec | Medium (tied to architecture refactor) |

---

*Concerns audit: 2026-07-11*
