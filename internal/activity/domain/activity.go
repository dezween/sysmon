// Package domain contains the pure value objects and validation rules for the
// activity-keeping use case. It imports nothing from port, service, or
// adapter, and has no cgo or os dependency, so it builds and tests on any
// platform.
package domain

import (
	"errors"
	"time"
)

// ErrNonPositiveInterval is returned when a non-positive duration is supplied
// to NewInterval.
var ErrNonPositiveInterval = errors.New("interval must be greater than zero")

// ErrNonPositiveOffset is returned when a non-positive pixel value is
// supplied to NewOffset.
var ErrNonPositiveOffset = errors.New("offset must be greater than zero")

// Interval is a validated positive duration between nudges.
type Interval struct {
	d time.Duration
}

// NewInterval validates d and returns an Interval wrapping it. It rejects any
// duration that is zero or negative, preventing a busy-loop or a panic from a
// non-positive time.Ticker.
func NewInterval(d time.Duration) (Interval, error) {
	if d <= 0 {
		return Interval{}, ErrNonPositiveInterval
	}
	return Interval{d: d}, nil
}

// Duration returns the wrapped time.Duration.
func (i Interval) Duration() time.Duration {
	return i.d
}

// Offset is the validated pixel shift magnitude applied then reversed on
// each nudge.
type Offset int

// NewOffset validates px and returns an Offset wrapping it. It rejects any
// value that is zero or negative.
func NewOffset(px int) (Offset, error) {
	if px <= 0 {
		return 0, ErrNonPositiveOffset
	}
	return Offset(px), nil
}

// Int returns the wrapped pixel value.
func (o Offset) Int() int {
	return int(o)
}
