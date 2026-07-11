//go:build darwin

// Package adapter contains the concrete, platform-specific implementations
// of the activity hexagon's ports. This is the only package permitted to
// import "C" / cgo.
package adapter

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreGraphics
#include <ApplicationServices/ApplicationServices.h>

// nudge moves the cursor by (dx, dy) from its current position and posts a real
// MouseMoved event -- that event is what resets the system idle timer, unlike a
// plain cursor reposition. It returns 0 on success, or -1 if CoreGraphics could
// not create an event (leaving the cursor untouched), so it never releases a
// NULL reference.
static int nudge(int dx, int dy) {
    CGEventRef cur = CGEventCreate(NULL);
    if (cur == NULL) {
        return -1;
    }
    CGPoint p = CGEventGetLocation(cur);
    CFRelease(cur);

    CGPoint np = CGPointMake(p.x + dx, p.y + dy);
    CGEventRef move = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, np, kCGMouseButtonLeft);
    if (move == NULL) {
        return -1;
    }
    CGEventPost(kCGHIDEventTap, move);
    CFRelease(move);
    return 0;
}

// accessibilityTrusted reports whether this process is trusted for the macOS
// Accessibility API. Without trust, posted mouse events are silently dropped.
static int accessibilityTrusted(void) {
    return AXIsProcessTrusted() ? 1 : 0;
}
*/
import "C"

import "errors"

// ErrNudgeFailed is returned when CoreGraphics could not create the event
// needed to move the pointer. The cursor is left untouched in that case.
var ErrNudgeFailed = errors.New("coregraphics failed to create mouse event")

// CGPointer implements port.Pointer using CoreGraphics on macOS.
type CGPointer struct{}

// NewCGPointer constructs a CGPointer.
func NewCGPointer() *CGPointer {
	return &CGPointer{}
}

// Nudge moves the system pointer by (dx, dy) and posts a real MouseMoved event
// via CoreGraphics. It returns ErrNudgeFailed if the event could not be created
// (the cursor is left untouched in that case).
func (c *CGPointer) Nudge(dx, dy int) error {
	if C.nudge(C.int(dx), C.int(dy)) != 0 {
		return ErrNudgeFailed
	}
	return nil
}

// AccessibilityTrusted reports whether this process has macOS Accessibility
// permission. When false, synthetic mouse events are silently dropped and the
// idle timer is never reset, so callers should warn the user to grant access in
// System Settings -> Privacy & Security -> Accessibility.
func AccessibilityTrusted() bool {
	return C.accessibilityTrusted() != 0
}
