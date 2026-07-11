# Testing Patterns

**Analysis Date:** 2026-07-11

## Current State: NO TESTS

**No test framework configured.** No `*_test.go` files present. No test runner setup.

## Test Framework (Target)

**When refactoring to hexagonal architecture (post-main), use:**

**Runner:**
- Go's built-in `testing` package (`go test ./...`)
- Config: `go.mod` declares `go 1.26.5`; no `go.work` or `testing.go` file

**Assertion Library:**
- stdlib `testing.T` with manual assertions (no external assertion library currently used)

**Run Commands:**
```bash
go test ./...              # Run all tests
go test -v ./...           # Verbose output
go test -cover ./...       # Coverage summary
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

## Gap: Untestable Current Structure

**Problem:**
- `main()` function mixes CLI, orchestration, and business logic (`nudge()` call)
- `nudge()` is a cgo C function directly calling CoreGraphics
- Cannot unit-test keeper logic without:
  - macOS system available
  - Accessibility (TCC) permission granted
  - Real mouse movement side effects

**Example of untestability:**
```go
// main.go:51-65 — This loop logic cannot be tested standalone:
for {
    select {
    case <-ticker.C:
        C.nudge(1, 0)      // ← Direct cgo call, requires macOS + permission
        time.Sleep(40 * time.Millisecond)
        C.nudge(-1, 0)
    case <-sig:
        return
    }
}
```

## Target Testing Structure (Post-Refactor)

### Proposed Test Pattern

**Test file:** `internal/activity/service/keeper_service_test.go`

```go
package service

import (
	"context"
	"testing"
	"time"

	"sysmon/internal/activity/port"
)

// MockPointer implements port.Pointer for testing
type MockPointer struct {
	NudgeCalls []struct {
		DX, DY int
	}
}

func (m *MockPointer) Nudge(dx, dy int) error {
	m.NudgeCalls = append(m.NudgeCalls, struct{ DX, DY int }{DX: dx, DY: dy})
	return nil
}

// TestKeeperServiceRuns verifies KeeperService ticker loop
func TestKeeperServiceRuns(t *testing.T) {
	mock := &MockPointer{}
	keeper := NewKeeperService(mock, 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run for ~250ms (should trigger ~2 ticks)
	go func() {
		time.Sleep(250 * time.Millisecond)
		cancel()
	}()

	err := keeper.Run(ctx)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Each tick calls Nudge(+1, 0) and Nudge(-1, 0)
	if len(mock.NudgeCalls) < 4 {
		t.Errorf("Expected at least 4 nudge calls, got %d", len(mock.NudgeCalls))
	}
}

// TestKeeperServiceContextCancellation verifies clean shutdown
func TestKeeperServiceContextCancellation(t *testing.T) {
	mock := &MockPointer{}
	keeper := NewKeeperService(mock, 1*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()  // Cancel immediately

	err := keeper.Run(ctx)
	if err != nil {
		t.Fatalf("Run() with cancelled context should return nil, got: %v", err)
	}
}
```

**Why this works:**
- `MockPointer` doesn't execute real cgo calls
- No macOS dependency
- No Accessibility permission required
- Testable on any platform (CI/CD)
- Validates timing and call sequence without side effects

### What to Test (Post-Refactor)

**Unit Tests — `KeeperService`:**
- Ticker fires at correct interval
- Nudge called correct number of times in time window
- Context cancellation stops keeper cleanly
- Error handling (if Pointer.Nudge fails)

**Unit Tests — `Interval` domain object (if added):**
- Validation (positive, > 0)
- Parsing from string

**Integration Tests (macOS only):**
- CGPointer successfully moves mouse (manual test)
- Accessibility permission check

**E2E Tests:**
- Binary runs, accepts flags, outputs expected logs (shell script test)

### Current Gap vs Acceptance Criteria

From `docs/TASK.md` section 8 (Acceptance Criteria):

- [x] `go build` works without external dependencies
- [x] Binary is Mach-O arm64
- [x] Runs with `-interval 300ms` and responds to SIGTERM
- [x] `make start/stop/status` work correctly
- [x] PID file prevents accidental kill of system sysmond
- [x] Repo is private with correct credential configuration
- [ ] **Accessibility permission verified on target machine (manual test)**
- [ ] **`KeeperService` unit-tested with fake `Pointer` — NOT YET IMPLEMENTED**

**Testing blockers:**
1. Target architecture not yet refactored (domain/port/service/adapter separation incomplete)
2. No KeeperService class to test
3. No Pointer interface to mock

---

*Testing analysis: 2026-07-11*
