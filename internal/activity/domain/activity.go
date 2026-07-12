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

// Interval is a validated positive duration between roams.
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
