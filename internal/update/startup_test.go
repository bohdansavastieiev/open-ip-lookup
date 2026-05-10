package update

import (
	"testing"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
	"github.com/stretchr/testify/assert"
)

func TestNeedsStartupFullSync(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		schedule syncSchedule
		want     bool
	}{
		{
			name: "empty schedule",
			want: true,
		},
		{
			name: "exactly due",
			schedule: syncSchedule{
				NextFullSyncAt: now,
			},
			want: true,
		},
		{
			name: "before due",
			schedule: syncSchedule{
				NextFullSyncAt: now.Add(time.Minute),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.schedule.needsFullSync(now)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNeedsStartupRetrySync(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		schedule syncSchedule
		want     bool
	}{
		{
			name: "retry exactly due",
			schedule: syncSchedule{
				NextFullSyncAt:  now.Add(time.Hour),
				NextRetrySyncAt: now,
			},
			want: true,
		},
		{
			name: "retry before due",
			schedule: syncSchedule{
				NextFullSyncAt:  now.Add(time.Hour),
				NextRetrySyncAt: now.Add(time.Minute),
			},
			want: false,
		},
		{
			name: "full sync due",
			schedule: syncSchedule{
				NextFullSyncAt:  now,
				NextRetrySyncAt: now,
			},
			want: false,
		},
		{
			name: "empty schedule",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.schedule.needsRetrySync(now)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewEnabledSourceIDs(t *testing.T) {
	enabled := []source.ID{
		source.DanTorFull,
		source.DanTorExit,
		source.IANASpecialIPv4,
	}
	st := state{Sources: map[source.ID]sourceState{
		source.DanTorFull:          {},
		source.CymruFullBogonsIPv4: {},
	}}

	got := newEnabledSourceIDs(enabled, st.Sources)

	assert.Equal(t, []source.ID{source.DanTorExit, source.IANASpecialIPv4}, got)
}
