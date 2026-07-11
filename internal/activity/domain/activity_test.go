package domain_test

import (
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.NewInterval(tt.d)

			if tt.wantErr {
				require.Error(t, err)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.NewOffset(tt.px)

			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, domain.Offset(0), got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.px, got.Int())
		})
	}
}
