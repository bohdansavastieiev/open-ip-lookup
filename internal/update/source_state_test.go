package update

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUnsuccessfulSourceState_AppendsErrorAndPreservesSuccessData(t *testing.T) {
	completedAt := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	previous := sourceState{
		HasLocalArtifact: true,
		LastCheckedAt:    time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC),
		LastSuccessAt:    time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC),
		LastDownloadedAt: time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC),
		ConsecutiveErrors: []consecutiveError{{
			Kind:            errorKindNetwork,
			Message:         "timeout",
			Count:           1,
			FirstHappenedAt: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC),
			LastHappenedAt:  time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC),
		}},
	}

	got := newUnsuccessfulSourceState(
		previous,
		completedAt,
		errorKindNetwork,
		true,
		errors.New("timeout"),
	)

	assert.Equal(t, completedAt, got.LastCheckedAt)
	assert.True(t, got.RetryableFailure)
	assert.True(t, got.HasLocalArtifact)
	assert.Equal(t, previous.LastSuccessAt, got.LastSuccessAt)
	assert.Equal(t, previous.LastDownloadedAt, got.LastDownloadedAt)
	require.Len(t, got.ConsecutiveErrors, 1)
	assert.Equal(t, 2, got.ConsecutiveErrors[0].Count)
	assert.Equal(t, completedAt, got.ConsecutiveErrors[0].LastHappenedAt)
}

func TestNewNotModifiedSourceState_UpdatesSuccessTimesAndClearsErrors(t *testing.T) {
	completedAt := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	previous := sourceState{
		HasLocalArtifact: true,
		LastCheckedAt:    time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC),
		LastSuccessAt:    time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC),
		LastDownloadedAt: time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC),
		ConsecutiveErrors: []consecutiveError{{
			Kind:            errorKindNetwork,
			Message:         "timeout",
			Count:           2,
			FirstHappenedAt: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC),
			LastHappenedAt:  time.Date(2026, 4, 18, 10, 5, 0, 0, time.UTC),
		}},
	}

	got := newNotModifiedSourceState(previous, completedAt)

	assert.Equal(t, completedAt, got.LastCheckedAt)
	assert.True(t, got.HasLocalArtifact)
	assert.Equal(t, completedAt, got.LastSuccessAt)
	assert.False(t, got.RetryableFailure)
	assert.Equal(t, previous.LastDownloadedAt, got.LastDownloadedAt)
	assert.Nil(t, got.ConsecutiveErrors)
}

func TestNewOKSourceState_UpdatesValidatorsAndClearsErrors(t *testing.T) {
	completedAt := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	lastModified := completedAt.Add(-time.Hour).Truncate(time.Second)
	headers := http.Header{}
	headers.Set("ETag", "etag-2")
	headers.Set("Last-Modified", lastModified.Format(http.TimeFormat))
	previous := sourceState{
		LastCheckedAt: completedAt.Add(-2 * time.Hour),
		ConsecutiveErrors: []consecutiveError{{
			Kind:            errorKindHTTPStatus,
			Message:         "status 500",
			Count:           1,
			FirstHappenedAt: completedAt.Add(-2 * time.Hour),
			LastHappenedAt:  completedAt.Add(-2 * time.Hour),
		}},
	}

	got, err := newOKSourceState(previous, headers, completedAt)
	require.NoError(t, err)

	assert.Equal(t, "etag-2", got.ETag)
	assert.Equal(t, lastModified, got.LastModified)
	assert.Equal(t, completedAt, got.LastCheckedAt)
	assert.Equal(t, completedAt, got.LastSuccessAt)
	assert.Equal(t, completedAt, got.LastDownloadedAt)
	assert.True(t, got.HasLocalArtifact)
	assert.False(t, got.RetryableFailure)
	assert.Nil(t, got.ConsecutiveErrors)
}

func TestLastModifiedFromHeader(t *testing.T) {
	validTime := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)

	t.Run("returns zero time when header missing", func(t *testing.T) {
		got, err := lastModifiedFromHeader(http.Header{})
		require.NoError(t, err)
		assert.True(t, got.IsZero())
	})

	t.Run("parses valid header", func(t *testing.T) {
		headers := http.Header{"Last-Modified": []string{validTime.Format(http.TimeFormat)}}
		got, err := lastModifiedFromHeader(headers)
		require.NoError(t, err)
		assert.Equal(t, validTime, got)
	})

	t.Run("returns error for invalid header", func(t *testing.T) {
		_, err := lastModifiedFromHeader(http.Header{"Last-Modified": []string{"bad-time"}})
		require.Error(t, err)
	})
}
