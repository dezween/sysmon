//go:build darwin

// Package adapter contains the concrete, platform-specific implementations
// of the activity hexagon's ports. This is the only package permitted to
// import "C" / cgo.
package adapter

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreGraphics
#include <ApplicationServices/ApplicationServices.h>

// getPosition reads the current cursor location. It returns 0 on success (x
// and y populated) or -1 if CoreGraphics could not create the query event,
// leaving x and y untouched.
static int getPosition(double *x, double *y) {
    CGEventRef cur = CGEventCreate(NULL);
    if (cur == NULL) {
        return -1;
    }
    CGPoint p = CGEventGetLocation(cur);
    CFRelease(cur);
    *x = p.x;
    *y = p.y;
    return 0;
}

// getBounds reads the main display's width and height in pixels. It always
// succeeds: CGDisplayBounds/CGMainDisplayID create no CoreFoundation object
// that needs releasing.
static void getBounds(double *w, double *h) {
    CGRect bounds = CGDisplayBounds(CGMainDisplayID());
    *w = bounds.size.width;
    *h = bounds.size.height;
}

// moveTo posts a real MouseMoved event at the absolute point (x, y) -- that
// event is what resets the system idle timer, unlike a plain cursor
// reposition. It returns 0 on success, or -1 if CoreGraphics could not
// create the event (leaving the cursor untouched), so it never releases a
// NULL reference.
static int moveTo(double x, double y) {
    CGPoint np = CGPointMake(x, y);
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

import (
	"errors"

	"sysmon/internal/activity/port"
)

// ErrPositionFailed is returned when CoreGraphics could not create the
// event needed to read the pointer's current position.
var ErrPositionFailed = errors.New("coregraphics failed to read pointer position")

// ErrMoveFailed is returned when CoreGraphics could not create the event
// needed to move the pointer. The cursor is left untouched in that case.
var ErrMoveFailed = errors.New("coregraphics failed to create mouse event")

// CGPointer implements port.Pointer using CoreGraphics on macOS.
type CGPointer struct{}

// Compile-time assertion that *CGPointer still satisfies port.Pointer.
var _ port.Pointer = (*CGPointer)(nil)

// NewCGPointer constructs a CGPointer.
func NewCGPointer() *CGPointer {
	return &CGPointer{}
}

// Position returns the pointer's current (x, y) location in screen pixels.
// It returns ErrPositionFailed if CoreGraphics could not create the query
// event.
func (c *CGPointer) Position() (int, int, error) {
	var x, y C.double
	if C.getPosition(&x, &y) != 0 {
		return 0, 0, ErrPositionFailed
	}
	return int(x), int(y), nil
}

// Bounds returns the main display's width and height in pixels.
func (c *CGPointer) Bounds() (int, int, error) {
	var w, h C.double
	C.getBounds(&w, &h)
	return int(w), int(h), nil
}

// MoveTo posts a real MouseMoved event placing the pointer at the absolute
// (x, y) location via CoreGraphics. It returns ErrMoveFailed if the event
// could not be created (the cursor is left untouched in that case).
func (c *CGPointer) MoveTo(x, y int) error {
	if C.moveTo(C.double(x), C.double(y)) != 0 {
		return ErrMoveFailed
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
