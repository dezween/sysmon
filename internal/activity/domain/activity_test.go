package domain_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sysmon/internal/activity/domain"
)

func TestNewInterval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		d       time.Duration
		wantErr bool
	}{
		{name: "positive duration is accepted", d: 10 * time.Second, wantErr: false},
		{name: "zero duration is rejected", d: 0, wantErr: true},
		{name: "negative duration is rejected", d: -time.Second, wantErr: true},
		{name: "one nanosecond is accepted (boundary)", d: 1 * time.Nanosecond, wantErr: false},
		{name: "very large duration is accepted (boundary)", d: math.MaxInt64, wantErr: false},
		{name: "math.MinInt64 duration is rejected (boundary)", d: math.MinInt64, wantErr: true},
		{name: "large negative duration is rejected", d: -24 * time.Hour, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.NewInterval(tt.d)

			if tt.wantErr {
				require.ErrorIs(t, err, domain.ErrNonPositiveInterval)
				assert.Equal(t, domain.Interval{}, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.d, got.Duration())
		})
	}
}

func TestNewOffset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		px      int
		wantErr bool
	}{
		{name: "positive offset is accepted", px: 1, wantErr: false},
		{name: "zero offset is rejected", px: 0, wantErr: true},
		{name: "negative offset is rejected", px: -1, wantErr: true},
		{name: "very large offset is accepted (boundary)", px: math.MaxInt, wantErr: false},
		{name: "math.MinInt offset is rejected (boundary)", px: math.MinInt, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.NewOffset(tt.px)

			if tt.wantErr {
				require.ErrorIs(t, err, domain.ErrNonPositiveOffset)
				assert.Equal(t, domain.Offset(0), got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.px, got.Int())
		})
	}
}

// TestInterval_Duration_RoundTrip asserts that Duration() returns exactly the
// value passed to NewInterval, with no rounding or truncation.
func TestInterval_Duration_RoundTrip(t *testing.T) {
	t.Parallel()

	values := []time.Duration{
		1 * time.Nanosecond,
		1,
		40 * time.Millisecond,
		10 * time.Second,
		365 * 24 * time.Hour,
		math.MaxInt64,
	}

	for _, d := range values {
		iv, err := domain.NewInterval(d)
		require.NoError(t, err)
		assert.Equal(t, d, iv.Duration())
	}
}

// TestOffset_Int_RoundTrip asserts that Int() returns exactly the pixel value
// passed to NewOffset.
func TestOffset_Int_RoundTrip(t *testing.T) {
	t.Parallel()

	values := []int{1, 2, 40, 1000, math.MaxInt}

	for _, px := range values {
		off, err := domain.NewOffset(px)
		require.NoError(t, err)
		assert.Equal(t, px, off.Int())
	}
}

// TestErrorsAreDistinct guards against a future refactor accidentally
// aliasing the two sentinel errors, which would make errors.Is checks
// ambiguous between Interval and Offset validation failures.
func TestErrorsAreDistinct(t *testing.T) {
	t.Parallel()

	require.NotErrorIs(t, domain.ErrNonPositiveInterval, domain.ErrNonPositiveOffset)
	require.NotErrorIs(t, domain.ErrNonPositiveOffset, domain.ErrNonPositiveInterval)
}
