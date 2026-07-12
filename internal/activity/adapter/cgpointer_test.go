//go:build darwin

package adapter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sysmon/internal/activity/adapter"
)

// TestNewCGPointer asserts construction never returns nil.
func TestNewCGPointer(t *testing.T) {
	t.Parallel()

	p := adapter.NewCGPointer()

	require.NotNil(t, p)
}

// TestCGPointer_Nudge_ZeroOffset is a non-disruptive smoke test: a (0, 0)
// nudge posts a real MouseMoved event at the cursor's current location, so
// the cursor does not visibly move regardless of whether the test process
// has macOS Accessibility permission. CoreGraphics can still create and post
// the event without that permission (the event is only silently dropped by
// the system on delivery), so this call is expected to succeed in CI.
func TestCGPointer_Nudge_ZeroOffset(t *testing.T) {
	t.Parallel()

	p := adapter.NewCGPointer()

	err := p.Nudge(0, 0)

	require.NoError(t, err)
}

// TestAccessibilityTrusted asserts the accessor returns a bool without
// panicking. The actual value depends on whether the test runner's terminal
// has been granted Accessibility permission, which CI cannot guarantee, so
// this test intentionally does not assert true or false.
func TestAccessibilityTrusted(t *testing.T) {
	t.Parallel()

	require.NotPanics(t, func() {
		got := adapter.AccessibilityTrusted()
		assert.IsType(t, false, got)
	})
}
