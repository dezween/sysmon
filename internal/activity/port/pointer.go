// Package port contains the driven and driving interfaces of the activity
// hexagon. Ports know nothing about concrete implementations: no cgo, no
// adapter import.
package port

//go:generate go run go.uber.org/mock/mockgen -destination=mock/mock_pointer.go -package=mock sysmon/internal/activity/port Pointer

// Pointer abstracts moving the system pointer by a relative offset and
// posting a synthetic mouse-moved event, which resets the OS idle timer.
type Pointer interface {
	Nudge(dx, dy int) error
}
