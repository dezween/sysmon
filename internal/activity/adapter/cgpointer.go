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
// plain cursor reposition.
static void nudge(int dx, int dy) {
    CGEventRef cur = CGEventCreate(NULL);
    CGPoint p = CGEventGetLocation(cur);
    CFRelease(cur);

    CGPoint np = CGPointMake(p.x + dx, p.y + dy);
    CGEventRef move = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, np, kCGMouseButtonLeft);
    CGEventPost(kCGHIDEventTap, move);
    CFRelease(move);
}
*/
import "C"

// CGPointer implements port.Pointer using CoreGraphics on macOS.
type CGPointer struct{}

// NewCGPointer constructs a CGPointer.
func NewCGPointer() *CGPointer {
	return &CGPointer{}
}

// Nudge moves the system pointer by (dx, dy) and posts a real MouseMoved
// event via CoreGraphics.
func (c *CGPointer) Nudge(dx, dy int) error {
	C.nudge(C.int(dx), C.int(dy))
	return nil
}
