// Package port contains the driven and driving interfaces of the activity
// hexagon. Ports know nothing about concrete implementations: no cgo, no
// adapter import.
package port

//go:generate mockgen -destination=mock/mock_pointer.go -package=mock sysmon/internal/activity/port Pointer

// Pointer abstracts reading and moving the system pointer: reading its
// current position, reading the main-display bounds, and posting a
// synthetic absolute mouse-moved event, which resets the OS idle timer.
type Pointer interface {
	// Position returns the pointer's current (x, y) location in screen
	// pixels.
	Position() (x, y int, err error)
	// Bounds returns the main display's width and height in pixels.
	Bounds() (w, h int, err error)
	// MoveTo posts a synthetic mouse-moved event placing the pointer at the
	// absolute (x, y) location.
	MoveTo(x, y int) error
}
