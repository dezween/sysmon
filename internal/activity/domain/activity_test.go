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
