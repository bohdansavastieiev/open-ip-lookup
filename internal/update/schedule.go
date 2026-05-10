package update

import (
	"encoding/json"
	"fmt"
	"time"
)

// full and retry - time when it is scheduled to initiate sync
// startup - when the sync event completed
type syncSchedule struct {
	LastFullSyncAt time.Time `json:"last_full_sync_at"`
	NextFullSyncAt time.Time `json:"next_full_sync_at"`

	LastRetrySyncAt   time.Time     `json:"last_retry_sync_at"`
	NextRetrySyncAt   time.Time     `json:"next_retry_sync_at"`
	LastRetryInterval retryInterval `json:"last_retry_interval"`

	LastStartupSyncAt time.Time `json:"last_startup_sync_at"`
}

type retryInterval time.Duration

const (
	retryIntervalNone  retryInterval = retryInterval(0)
	retryInterval45m   retryInterval = retryInterval(45 * time.Minute)
	retryInterval1h30m retryInterval = retryInterval(90 * time.Minute)
	retryInterval3h    retryInterval = retryInterval(3 * time.Hour)
	retryInterval6h    retryInterval = retryInterval(6 * time.Hour)
	retryInterval12h   retryInterval = retryInterval(12 * time.Hour)
)

func (i retryInterval) Duration() time.Duration {
	return time.Duration(i)
}

func (i retryInterval) next() retryInterval {
	switch i {
	case retryIntervalNone:
		return retryInterval45m
	case retryInterval45m:
		return retryInterval1h30m
	case retryInterval1h30m:
		return retryInterval3h
	case retryInterval3h:
		return retryInterval6h
	case retryInterval6h:
		return retryInterval12h
	case retryInterval12h:
		return retryInterval12h
	default:
		return retryIntervalNone
	}
}

func (s syncSchedule) newSchedule(
	now time.Time,
	fullSyncInterval time.Duration,
	retryNeeded bool,
) syncSchedule {
	new := s
	if !now.Before(s.NextFullSyncAt) {
		new.LastFullSyncAt = s.NextFullSyncAt
		for !now.Before(new.LastFullSyncAt.Add(fullSyncInterval)) {
			new.LastFullSyncAt = new.LastFullSyncAt.Add(fullSyncInterval)
		}
		new.NextFullSyncAt = new.LastFullSyncAt.Add(fullSyncInterval)
		new.LastRetrySyncAt = time.Time{}
		new.scheduleNextRetryAfter(now, retryIntervalNone, retryNeeded)
		return new
	}

	if !s.NextRetrySyncAt.IsZero() && !now.Before(s.NextRetrySyncAt) && retryNeeded {
		new.LastRetrySyncAt = s.NextRetrySyncAt
		new.scheduleNextRetryAfter(now, s.LastRetryInterval, true)
	}

	if !retryNeeded {
		new.NextRetrySyncAt = time.Time{}
		new.LastRetryInterval = retryIntervalNone
		new.LastRetrySyncAt = time.Time{}
	}

	return new
}

func (s *syncSchedule) scheduleNextRetryAfter(now time.Time, current retryInterval, retryNeeded bool) {
	if !retryNeeded {
		s.NextRetrySyncAt = time.Time{}
		s.LastRetryInterval = retryIntervalNone
		return
	}

	nextRetryAt, nextRetryInterval := s.nextRetryAfter(now, current)
	if nextRetryInterval == retryIntervalNone {
		s.NextRetrySyncAt = time.Time{}
		s.LastRetryInterval = retryIntervalNone
		return
	}

	s.NextRetrySyncAt = nextRetryAt
	s.LastRetryInterval = nextRetryInterval
}

func (s *syncSchedule) scheduleFirstRetryAfter(now time.Time) {
	nextRetryAt := now.Add(retryInterval45m.Duration())
	if !nextRetryAt.Before(s.NextFullSyncAt) {
		s.NextRetrySyncAt = time.Time{}
		s.LastRetryInterval = retryIntervalNone
		return
	}

	s.NextRetrySyncAt = nextRetryAt
	s.LastRetryInterval = retryInterval45m
}

func (s syncSchedule) nextRetryAfter(
	now time.Time,
	current retryInterval,
) (time.Time, retryInterval) {
	base := now
	if current != retryIntervalNone && !s.LastRetrySyncAt.IsZero() {
		base = s.LastRetrySyncAt
	}
	for {
		next := current.next()
		if next == retryIntervalNone {
			return time.Time{}, retryIntervalNone
		}

		nextRetryAt := base.Add(next.Duration())
		if !nextRetryAt.Before(s.NextFullSyncAt) {
			return time.Time{}, retryIntervalNone
		}
		if nextRetryAt.After(now) {
			return nextRetryAt, next
		}

		current = next
		base = nextRetryAt
	}
}

func (s syncSchedule) newStartupSchedule(
	now time.Time,
	completed time.Time,
	fullSyncInterval time.Duration,
	retryNeeded bool,
) syncSchedule {
	new := s.newSchedule(now, fullSyncInterval, retryNeeded)
	new.LastStartupSyncAt = completed
	return new
}

func (s syncSchedule) needsFullSync(now time.Time) bool {
	return !now.Before(s.NextFullSyncAt)
}

func (s syncSchedule) needsRetrySync(now time.Time) bool {
	return !s.NextRetrySyncAt.IsZero() &&
		now.Before(s.NextFullSyncAt) && !now.Before(s.NextRetrySyncAt)
}

func (s syncSchedule) nextSyncAt() time.Time {
	if !s.NextRetrySyncAt.IsZero() && s.NextRetrySyncAt.Before(s.NextFullSyncAt) {
		return s.NextRetrySyncAt
	}
	return s.NextFullSyncAt
}

func (i retryInterval) MarshalJSON() ([]byte, error) {
	return json.Marshal(i.Duration().String())
}

func (i *retryInterval) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("parse retry interval %q: %w", value, err)
	}

	*i = retryInterval(duration)
	return nil
}

func (i retryInterval) isValid() bool {
	switch i {
	case retryInterval45m,
		retryInterval1h30m,
		retryInterval3h,
		retryInterval6h,
		retryInterval12h:
		return true
	default:
		return false
	}
}
