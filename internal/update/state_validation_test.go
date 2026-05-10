package update

import (
	"testing"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateValidate_ReturnsNilForValidState(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	st := state{
		SyncSchedule: validSyncSchedule(now),
		Sources: map[source.ID]sourceState{
			source.DanTorFull: {
				HasLocalArtifact: true,
				LastCheckedAt:    now,
				LastSuccessAt:    now,
				LastDownloadedAt: now,
			},
			source.DanTorExit: {
				LastCheckedAt:    now,
				RetryableFailure: true,
				ConsecutiveErrors: []consecutiveError{{
					Kind:            errorKindNetwork,
					Message:         "timeout",
					Count:           1,
					FirstHappenedAt: now,
					LastHappenedAt:  now,
				}},
			},
		},
	}

	require.NoError(t, st.validate("state.json"))
}

func TestStateValidate_ReturnsErrorForUnknownSourceID(t *testing.T) {
	st := state{Sources: map[source.ID]sourceState{
		source.ID("unknown_source"): {},
	}}

	requireStateValidationCode(t, st.validate("state.json"), stateErrUnknownSourceID)
}

func TestStateValidate_ReturnsNilForValidOutdatedSourceStates(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		st   sourceState
	}{
		{
			name: "without prior success",
			st: sourceState{
				LastCheckedAt:    now,
				MarkedOutdatedAt: now,
			},
		},
		{
			name: "with prior success",
			st: sourceState{
				LastCheckedAt:    now,
				LastSuccessAt:    now.Add(-2 * time.Hour),
				MarkedOutdatedAt: now.Add(-time.Hour),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := state{Sources: map[source.ID]sourceState{source.DanTorFull: tt.st}}

			require.NoError(t, st.validate("state.json"))
		})
	}
}

func TestStateValidate_ReturnsErrorForSyncScheduleDataWithoutFullSync(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		schedule syncSchedule
	}{
		{
			name: "last retry sync at",
			schedule: syncSchedule{
				LastRetrySyncAt: now,
			},
		},
		{
			name: "last retry interval",
			schedule: syncSchedule{
				LastRetryInterval: retryInterval45m,
			},
		},
		{
			name: "next retry sync at",
			schedule: syncSchedule{
				NextRetrySyncAt: now,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := state{
				SyncSchedule: tt.schedule,
				Sources:      map[source.ID]sourceState{},
			}

			requireStateValidationCode(
				t,
				st.validate("state.json"),
				stateErrScheduleWithoutFullSync,
			)
		})
	}
}

func TestStateValidate_ReturnsErrorForInvalidSyncSchedule(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*syncSchedule)
		want   stateValidationCode
	}{
		{
			name: "retry sync at without interval",
			mutate: func(schedule *syncSchedule) {
				schedule.LastRetryInterval = 0
			},
			want: stateErrRetrySyncAtWithoutInterval,
		},
		{
			name: "retry interval without sync at",
			mutate: func(schedule *syncSchedule) {
				schedule.NextRetrySyncAt = time.Time{}
			},
			want: stateErrRetryIntervalWithoutSyncAt,
		},
		{
			name: "unsupported retry interval",
			mutate: func(schedule *syncSchedule) {
				schedule.LastRetryInterval = retryInterval(17 * time.Minute)
			},
			want: stateErrRetryIntervalUnsupported,
		},
		{
			name: "retry sync at after full sync",
			mutate: func(schedule *syncSchedule) {
				schedule.NextRetrySyncAt = schedule.NextFullSyncAt
			},
			want: stateErrRetrySyncAtAfterFullSync,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule := validSyncSchedule(now)
			tt.mutate(&schedule)
			st := state{
				SyncSchedule: schedule,
				Sources:      map[source.ID]sourceState{},
			}

			requireStateValidationCode(t, st.validate("state.json"), tt.want)
		})
	}
}

func TestStateValidate_ReturnsErrorForInvalidSourceStates(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		st   sourceState
		want stateValidationCode
	}{
		{
			name: "artifact without download",
			st:   sourceState{HasLocalArtifact: true},
			want: stateErrArtifactWithoutDownloadedAt,
		},
		{
			name: "downloaded_at without success_at",
			st:   sourceState{LastDownloadedAt: now},
			want: stateErrDownloadedAtWithoutSuccessAt,
		},
		{
			name: "downloaded_at after success_at",
			st: sourceState{
				LastDownloadedAt: now,
				LastSuccessAt:    now.Add(-time.Minute),
			},
			want: stateErrDownloadedAtAfterSuccessAt,
		},
		{
			name: "success_at without checked_at",
			st:   sourceState{LastSuccessAt: now},
			want: stateErrSuccessAtWithoutCheckedAt,
		},
		{
			name: "success_at after checked_at",
			st: sourceState{
				LastSuccessAt: now,
				LastCheckedAt: now.Add(-time.Minute),
			},
			want: stateErrSuccessAtAfterCheckedAt,
		},
		{
			name: "errors without checked_at",
			st: sourceState{
				ConsecutiveErrors: []consecutiveError{validConsecutiveError(now)},
			},
			want: stateErrErrorsWithoutCheckedAt,
		},
		{
			name: "retryable without errors",
			st: sourceState{
				RetryableFailure: true,
			},
			want: stateErrRetryableWithoutErrors,
		},
		{
			name: "outdated with artifact",
			st: sourceState{
				HasLocalArtifact: true,
				LastCheckedAt:    now,
				LastSuccessAt:    now.Add(-2 * time.Hour),
				LastDownloadedAt: now.Add(-2 * time.Hour),
				MarkedOutdatedAt: now.Add(-time.Hour),
			},
			want: stateErrOutdatedWithArtifact,
		},
		{
			name: "outdated without checked_at",
			st: sourceState{
				MarkedOutdatedAt: now,
			},
			want: stateErrOutdatedWithoutCheckedAt,
		},
		{
			name: "success_at equal to outdated_at",
			st: sourceState{
				LastCheckedAt:    now.Add(time.Hour),
				LastSuccessAt:    now,
				MarkedOutdatedAt: now,
			},
			want: stateErrSuccessAtNotBeforeOutdated,
		},
		{
			name: "success_at after outdated_at",
			st: sourceState{
				LastCheckedAt:    now.Add(2 * time.Hour),
				LastSuccessAt:    now.Add(time.Hour),
				MarkedOutdatedAt: now,
			},
			want: stateErrSuccessAtNotBeforeOutdated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := state{Sources: map[source.ID]sourceState{source.DanTorFull: tt.st}}
			requireStateValidationCode(t, st.validate("state.json"), tt.want)
		})
	}
}

func TestStateValidate_ReturnsErrorForInvalidConsecutiveErrors(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	validErr := validConsecutiveError(now)

	unknownKind := validErr
	unknownKind.Kind = "unknown"

	withoutMessage := validErr
	withoutMessage.Message = ""

	invalidCount := validErr
	invalidCount.Count = 0

	withoutFirstAt := validErr
	withoutFirstAt.FirstHappenedAt = time.Time{}

	withoutLastAt := validErr
	withoutLastAt.LastHappenedAt = time.Time{}

	lastAtBeforeFirstAt := validErr
	lastAtBeforeFirstAt.LastHappenedAt = now.Add(-time.Minute)

	tests := []struct {
		name string
		err  consecutiveError
		want stateValidationCode
	}{
		{
			name: "unknown kind",
			err:  unknownKind,
			want: stateErrConsecutiveUnknownKind,
		},
		{
			name: "missing message",
			err:  withoutMessage,
			want: stateErrConsecutiveWithoutMessage,
		},
		{
			name: "invalid count",
			err:  invalidCount,
			want: stateErrConsecutiveInvalidCount,
		},
		{
			name: "missing first time",
			err:  withoutFirstAt,
			want: stateErrConsecutiveWithoutFirstAt,
		},
		{
			name: "missing last time",
			err:  withoutLastAt,
			want: stateErrConsecutiveWithoutLastAt,
		},
		{
			name: "last before first",
			err:  lastAtBeforeFirstAt,
			want: stateErrConsecutiveLastBeforeFirst,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := state{Sources: map[source.ID]sourceState{source.DanTorFull: {
				LastCheckedAt:     now,
				ConsecutiveErrors: []consecutiveError{tt.err},
			}}}
			requireStateValidationCode(t, st.validate("state.json"), tt.want)
		})
	}
}

func validConsecutiveError(now time.Time) consecutiveError {
	return consecutiveError{
		Kind:            errorKindNetwork,
		Message:         "timeout",
		Count:           1,
		FirstHappenedAt: now,
		LastHappenedAt:  now,
	}
}

func validSyncSchedule(now time.Time) syncSchedule {
	return syncSchedule{
		LastFullSyncAt:    now.Add(-24 * time.Hour),
		LastRetrySyncAt:   now.Add(-time.Hour),
		NextFullSyncAt:    now.Add(24 * time.Hour),
		NextRetrySyncAt:   now.Add(time.Hour),
		LastRetryInterval: retryInterval1h30m,
	}
}

func requireStateValidationCode(t testing.TB, err error, code stateValidationCode) {
	t.Helper()

	var stateErr stateValidationError
	require.ErrorAs(t, err, &stateErr)
	assert.Equal(t, code, stateErr.code)
}
