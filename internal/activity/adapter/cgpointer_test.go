//go:build darwin

package adapter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sysmon/internal/activity/adapter"
	"sysmon/internal/activity/port"
)

// TestNewCGPointer asserts construction never returns nil and that CGPointer
// satisfies port.Pointer.
func TestNewCGPointer(t *testing.T) {
	t.Parallel()

	p := adapter.NewCGPointer()

	require.NotNil(t, p)

	var _ port.Pointer = p
}

// TestCGPointer_Position_Succeeds is a non-disruptive smoke test: reading
// the current position never moves the cursor, so it is safe to run in CI
// regardless of Accessibility permission (CoreGraphics can read the cursor
// location without that permission).
func TestCGPointer_Position_Succeeds(t *testing.T) {
	t.Parallel()

	p := adapter.NewCGPointer()

	x, y, err := p.Position()

	require.NoError(t, err)
	assert.GreaterOrEqual(t, x, 0)
	assert.GreaterOrEqual(t, y, 0)
}

// TestCGPointer_Bounds_ReturnsPositiveDimensions asserts the main display's
// bounds are read as positive width/height.
func TestCGPointer_Bounds_ReturnsPositiveDimensions(t *testing.T) {
	t.Parallel()

	p := adapter.NewCGPointer()

	w, h, err := p.Bounds()

	require.NoError(t, err)
	assert.Positive(t, w)
	assert.Positive(t, h)
}

// TestCGPointer_MoveTo_ToCurrentPosition is a non-disruptive smoke test:
// moving to the cursor's own current position posts a real MouseMoved event
// without visibly moving the cursor, so it is expected to succeed in CI
// regardless of whether the test process has macOS Accessibility permission
// (CoreGraphics can still create and post the event; the system only drops
// it silently on delivery without that permission).
func TestCGPointer_MoveTo_ToCurrentPosition(t *testing.T) {
	t.Parallel()

	p := adapter.NewCGPointer()

	x, y, err := p.Position()
	require.NoError(t, err)

	err = p.MoveTo(x, y)

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
