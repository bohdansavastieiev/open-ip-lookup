package update

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewSchedule_AdvancesDueFullSyncAfterSync(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	schedule := syncSchedule{
		LastFullSyncAt:    now.Add(-73 * time.Hour),
		NextFullSyncAt:    now.Add(-49 * time.Hour),
		LastRetrySyncAt:   now.Add(-52 * time.Hour),
		NextRetrySyncAt:   now.Add(-50 * time.Hour),
		LastRetryInterval: retryInterval1h30m,
	}

	got := schedule.newSchedule(now, 24*time.Hour, false)

	assert.Equal(t, syncSchedule{
		LastFullSyncAt:  now.Add(-time.Hour),
		NextFullSyncAt:  now.Add(23 * time.Hour),
		NextRetrySyncAt: time.Time{},
	}, got)
}

func TestNewSchedule_SchedulesFirstRetryAfterDueFullSyncFailure(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	schedule := syncSchedule{
		LastFullSyncAt: now.Add(-24 * time.Hour),
		NextFullSyncAt: now.Add(-time.Hour),
	}

	got := schedule.newSchedule(now, 24*time.Hour, true)

	assert.Equal(t, now.Add(45*time.Minute), got.NextRetrySyncAt)
	assert.Equal(t, retryInterval45m, got.LastRetryInterval)
	assert.True(t, got.LastRetrySyncAt.IsZero())
}

func TestNewSchedule_ClearsRetryStateWhenRetryNoLongerNeeded(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	schedule := syncSchedule{
		LastFullSyncAt:    now.Add(-2 * time.Hour),
		LastRetrySyncAt:   now.Add(-time.Hour),
		NextFullSyncAt:    now.Add(22 * time.Hour),
		NextRetrySyncAt:   now.Add(time.Hour),
		LastRetryInterval: retryInterval45m,
	}

	got := schedule.newSchedule(now, 24*time.Hour, false)

	assert.True(t, got.NextRetrySyncAt.IsZero())
	assert.Equal(t, retryIntervalNone, got.LastRetryInterval)
	assert.True(t, got.LastRetrySyncAt.IsZero())
}

func TestNextSyncAt_PrefersEarlierRetry(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	schedule := syncSchedule{
		NextFullSyncAt:  now.Add(2 * time.Hour),
		NextRetrySyncAt: now.Add(time.Hour),
	}

	got := schedule.nextSyncAt()

	assert.Equal(t, now.Add(time.Hour), got)
}
