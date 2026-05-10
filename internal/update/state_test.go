package update

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadState_ReturnsEmptyStateForMissingFile(t *testing.T) {
	dataDir := t.TempDir()

	s, ok, err := loadLocalState(dataDir)
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Equal(t, state{}, s)
}

func TestSaveStateJSON_RoundTrip(t *testing.T) {
	dataDir := t.TempDir()
	path := stateFilePath(dataDir)
	want := state{
		SyncSchedule: syncSchedule{
			LastFullSyncAt:    time.Date(2026, 4, 18, 9, 0, 0, 0, time.UTC),
			NextFullSyncAt:    time.Date(2026, 4, 19, 9, 0, 0, 0, time.UTC),
			LastRetrySyncAt:   time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC),
			NextRetrySyncAt:   time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC),
			LastRetryInterval: retryInterval1h30m,
		},
		Sources: map[source.ID]sourceState{
			source.MaxMindGeoLite2City: {
				HasLocalArtifact: true,
				ETag:             "etag-1",
				LastModified:     time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC),
				LastCheckedAt:    time.Date(2026, 4, 18, 11, 10, 0, 0, time.UTC),
				LastSuccessAt:    time.Date(2026, 4, 18, 11, 10, 0, 0, time.UTC),
				LastDownloadedAt: time.Date(2026, 4, 18, 11, 10, 0, 0, time.UTC),
				ConsecutiveErrors: []consecutiveError{{
					Kind:            errorKindHTTPStatus,
					Message:         "request source unexpected status: status 500",
					Count:           2,
					FirstHappenedAt: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC),
					LastHappenedAt:  time.Date(2026, 4, 18, 10, 5, 0, 0, time.UTC),
				}},
			},
		},
	}

	require.NoError(t, want.saveStateJSON(path))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"last_retry_interval": "1h30m0s"`)

	got, ok, err := loadLocalState(dataDir)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, want, got)
}

func TestStateAvailableSourceIDs(t *testing.T) {
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	enabled := []source.ID{
		source.DanTorFull,
		source.DanTorExit,
		source.IANASpecialIPv4,
	}
	st := state{Sources: map[source.ID]sourceState{
		source.DanTorFull: {
			HasLocalArtifact: true,
			LastCheckedAt:    now,
			LastSuccessAt:    now,
			LastDownloadedAt: now,
		},
		source.DanTorExit: {
			LastCheckedAt:    now,
			MarkedOutdatedAt: now,
		},
		source.IANASpecialIPv4: {},
		source.CymruFullBogonsIPv4: {
			HasLocalArtifact: true,
			LastCheckedAt:    now,
			LastSuccessAt:    now,
			LastDownloadedAt: now,
		},
	}}

	got := availableSourceIDs(enabled, st.Sources)

	assert.Equal(t, []source.ID{source.DanTorFull}, got)
}

func TestLoadState_ReturnsErrorForInvalidJSON(t *testing.T) {
	dataDir := t.TempDir()
	path := stateFilePath(dataDir)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte("{"), 0o600))

	got, ok, err := loadLocalState(dataDir)
	require.Error(t, err)
	assert.False(t, ok)
	assert.Equal(t, state{}, got)
}

func TestLoadState_ReturnsErrorForNilSourcesMap(t *testing.T) {
	dataDir := t.TempDir()
	path := stateFilePath(dataDir)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o600))

	got, ok, err := loadLocalState(dataDir)
	require.Error(t, err)
	assert.False(t, ok)
	assert.Equal(t, state{}, got)
}
