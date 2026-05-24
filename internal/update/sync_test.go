package update

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/config"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateSources_DirectFilePromotesAndSavesState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("If-None-Match"))
		w.Header().Set("ETag", "etag-1")
		_, _ = w.Write([]byte("1.1.1.1\n"))
	}))
	defer server.Close()

	dataDir := t.TempDir()
	u := newTestUpdater(dataDir, source.DanTorFull, server.URL)
	s := state{
		SyncSchedule: syncSchedule{NextFullSyncAt: time.Now().UTC().Add(-time.Hour)},
		Sources:      map[source.ID]sourceState{},
	}

	event, err := u.updateSources(
		context.Background(),
		SyncScopeStartup,
		[]source.ID{source.DanTorFull},
		&s,
		nil,
	)

	require.NoError(t, err)
	assert.Equal(t, SyncScopeStartup, event.Scope)
	assert.Equal(t, []source.ID{source.DanTorFull}, event.Available)
	assert.Equal(t, []source.ID{source.DanTorFull}, event.Refreshed)
	assert.Empty(t, event.Failed)
	assert.False(t, event.RetryPending)

	def := source.DefinitionFor(source.DanTorFull)
	data, err := os.ReadFile(filepath.Join(dataDir, def.LocalBaseName))
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1\n", string(data))

	saved, ok, err := loadLocalState(dataDir)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.True(t, saved.Sources[source.DanTorFull].HasLocalArtifact)
	assert.Equal(t, "etag-1", saved.Sources[source.DanTorFull].ETag)
	assert.False(t, saved.SyncSchedule.LastStartupSyncAt.IsZero())
}

func TestUpdateSources_TarGzDirPromotesAndSavesState(t *testing.T) {
	archive := newTestTarGz(t, map[string]string{"1/aggregated.json": "source-body"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", "etag-1")
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	dataDir := t.TempDir()
	u := newTestUpdater(dataDir, source.IPVerseASIPBlocksAll, server.URL)
	s := state{
		SyncSchedule: syncSchedule{NextFullSyncAt: time.Now().UTC().Add(-time.Hour)},
		Sources:      map[source.ID]sourceState{},
	}

	event, err := u.updateSources(
		context.Background(),
		SyncScopeStartup,
		[]source.ID{source.IPVerseASIPBlocksAll},
		&s,
		nil,
	)

	require.NoError(t, err)
	assert.Equal(t, []source.ID{source.IPVerseASIPBlocksAll}, event.Available)
	assert.Equal(t, []source.ID{source.IPVerseASIPBlocksAll}, event.Refreshed)
	assert.Empty(t, event.Failed)

	def := source.DefinitionFor(source.IPVerseASIPBlocksAll)
	data, err := os.ReadFile(filepath.Join(dataDir, def.LocalBaseName, "1", "aggregated.json"))
	require.NoError(t, err)
	assert.Equal(t, "source-body", string(data))
	assert.True(t, s.Sources[source.IPVerseASIPBlocksAll].HasLocalArtifact)
	assert.Equal(t, "etag-1", s.Sources[source.IPVerseASIPBlocksAll].ETag)
}

func TestUpdateSources_RefreshesDirectFilesConcurrently(t *testing.T) {
	bothStarted := make(chan struct{})
	var active atomic.Int32
	var closeOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if active.Add(1) == 2 {
			closeOnce.Do(func() { close(bothStarted) })
		}

		select {
		case <-bothStarted:
		case <-r.Context().Done():
			return
		}

		w.Header().Set("ETag", "etag"+r.URL.Path)
		_, _ = w.Write([]byte("1.1.1.1\n"))
	}))
	defer server.Close()

	dataDir := t.TempDir()
	sourceIDs := []source.ID{source.DanTorFull, source.DanTorExit}
	u := newTestUpdaterWithURLs(dataDir, sourceIDs, map[source.ID]string{
		source.DanTorFull: server.URL + "/full",
		source.DanTorExit: server.URL + "/exit",
	})
	s := state{Sources: map[source.ID]sourceState{}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	event, err := u.updateSources(ctx, SyncScopeStartup, sourceIDs, &s, nil)

	require.NoError(t, err)
	assert.Equal(t, sourceIDs, event.Available)
	assert.Equal(t, sourceIDs, event.Refreshed)
	assert.Empty(t, event.Failed)
}

func TestRunStartup_RetriesInitialRefreshThreeTimes(t *testing.T) {
	withShortRetryDelays(t, []time.Duration{time.Millisecond, time.Millisecond})
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("ETag", "etag-1")
		_, _ = w.Write([]byte("1.1.1.1\n"))
	}))
	defer server.Close()

	dataDir := t.TempDir()
	u := newTestUpdater(dataDir, source.DanTorFull, server.URL)
	s := state{
		SyncSchedule: syncSchedule{NextFullSyncAt: time.Now().UTC().Add(-time.Hour)},
		Sources:      map[source.ID]sourceState{},
	}

	event, err := u.runStartup(context.Background(), time.Now().UTC(), &s)

	require.NoError(t, err)
	assert.Equal(t, int32(3), attempts.Load())
	assert.Equal(t, []source.ID{source.DanTorFull}, event.Refreshed)
	assert.Empty(t, event.Failed)
}

func TestUpdateSources_StartupFailureSchedulesRetryWithoutExistingRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	dataDir := t.TempDir()
	u := newTestUpdater(dataDir, source.DanTorFull, server.URL)
	s := state{
		SyncSchedule: futureSyncSchedule(),
		Sources:      map[source.ID]sourceState{},
	}

	event, err := u.updateSources(
		context.Background(),
		SyncScopeStartup,
		[]source.ID{source.DanTorFull},
		&s,
		nil,
	)

	require.NoError(t, err)
	assert.Equal(t, []source.ID{source.DanTorFull}, event.Failed)
	assert.True(t, event.RetryPending)
	assert.False(t, s.SyncSchedule.NextRetrySyncAt.IsZero())
	assert.Equal(t, retryInterval45m, s.SyncSchedule.LastRetryInterval)
}

func TestUpdateSources_RetriesRetryableFailureUntilSuccess(t *testing.T) {
	withShortRetryDelays(t, []time.Duration{time.Millisecond, time.Millisecond})
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("ETag", "etag-1")
		_, _ = w.Write([]byte("1.1.1.1\n"))
	}))
	defer server.Close()

	dataDir := t.TempDir()
	u := newTestUpdater(dataDir, source.DanTorFull, server.URL)
	s := state{
		SyncSchedule: dueRetrySchedule(),
		Sources: map[source.ID]sourceState{source.DanTorFull: {
			RetryableFailure: true,
			ConsecutiveErrors: []consecutiveError{{
				Kind:            errorKindNetwork,
				Message:         "timeout",
				Count:           1,
				FirstHappenedAt: time.Now().UTC().Add(-time.Minute),
				LastHappenedAt:  time.Now().UTC().Add(-time.Minute),
			}},
		}},
	}

	event, err := u.updateSources(
		context.Background(),
		SyncScopePartial,
		[]source.ID{source.DanTorFull},
		&s,
		shortRetryDelays,
	)

	require.NoError(t, err)
	assert.Equal(t, int32(3), attempts.Load())
	assert.Equal(t, []source.ID{source.DanTorFull}, event.Refreshed)
	assert.Empty(t, event.Failed)
	assert.False(t, event.RetryPending)
}

func TestUpdateSources_DoesNotRetryNonRetryableFailure(t *testing.T) {
	withShortRetryDelays(t, []time.Duration{time.Millisecond, time.Millisecond})
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	dataDir := t.TempDir()
	u := newTestUpdater(dataDir, source.DanTorFull, server.URL)
	s := state{
		SyncSchedule: futureSyncSchedule(),
		Sources:      map[source.ID]sourceState{},
	}

	event, err := u.updateSources(
		context.Background(),
		SyncScopeStartup,
		[]source.ID{source.DanTorFull},
		&s,
		shortRetryDelays,
	)

	require.NoError(t, err)
	assert.Equal(t, int32(1), attempts.Load())
	assert.Equal(t, []source.ID{source.DanTorFull}, event.Failed)
	assert.False(t, event.RetryPending)
}

func TestUpdateSources_DirectFileNotModifiedKeepsArtifact(t *testing.T) {
	previousCheckedAt := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "etag-1", r.Header.Get("If-None-Match"))
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	dataDir := t.TempDir()
	u := newTestUpdater(dataDir, source.DanTorFull, server.URL)
	def := source.DefinitionFor(source.DanTorFull)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, def.LocalBaseName), []byte("old"), 0o600))
	s := state{
		SyncSchedule: futureSyncSchedule(),
		Sources: map[source.ID]sourceState{source.DanTorFull: {
			HasLocalArtifact: true,
			ETag:             "etag-1",
			LastCheckedAt:    previousCheckedAt,
			LastSuccessAt:    previousCheckedAt,
			LastDownloadedAt: previousCheckedAt,
		}},
	}

	event, err := u.updateSources(
		context.Background(),
		SyncScopePartial,
		[]source.ID{source.DanTorFull},
		&s,
		nil,
	)

	require.NoError(t, err)
	assert.Equal(t, []source.ID{source.DanTorFull}, event.Available)
	assert.Empty(t, event.Refreshed)
	assert.Empty(t, event.Failed)
	assert.True(t, s.Sources[source.DanTorFull].LastSuccessAt.After(previousCheckedAt))
	assert.Equal(t, previousCheckedAt, s.Sources[source.DanTorFull].LastDownloadedAt)

	data, err := os.ReadFile(filepath.Join(dataDir, def.LocalBaseName))
	require.NoError(t, err)
	assert.Equal(t, "old", string(data))
}

func TestUpdateSources_DirectFileFailureKeepsAvailableUntilOutdated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	dataDir := t.TempDir()
	u := newTestUpdater(dataDir, source.DanTorFull, server.URL)
	def := source.DefinitionFor(source.DanTorFull)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, def.LocalBaseName), []byte("old"), 0o600))
	recent := time.Now().UTC().Add(-time.Hour)
	s := state{
		SyncSchedule: dueRetrySchedule(),
		Sources: map[source.ID]sourceState{source.DanTorFull: {
			HasLocalArtifact: true,
			ETag:             "etag-1",
			LastCheckedAt:    recent,
			LastSuccessAt:    recent,
			LastDownloadedAt: recent,
		}},
	}

	event, err := u.updateSources(
		context.Background(),
		SyncScopePartial,
		[]source.ID{source.DanTorFull},
		&s,
		nil,
	)

	require.NoError(t, err)
	assert.Equal(t, []source.ID{source.DanTorFull}, event.Available)
	assert.Equal(t, []source.ID{source.DanTorFull}, event.Failed)
	assert.Empty(t, event.Outdated)
	assert.True(t, event.RetryPending)
	assert.True(t, s.Sources[source.DanTorFull].HasLocalArtifact)
	require.Len(t, s.Sources[source.DanTorFull].ConsecutiveErrors, 1)
}

func TestUpdateSources_DirectFileFailureRemovesOutdatedArtifact(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	dataDir := t.TempDir()
	u := newTestUpdater(dataDir, source.DanTorFull, server.URL)
	def := source.DefinitionFor(source.DanTorFull)
	path := filepath.Join(dataDir, def.LocalBaseName)
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o600))
	old := time.Now().UTC().Add(-4 * 24 * time.Hour)
	s := state{
		SyncSchedule: futureSyncSchedule(),
		Sources: map[source.ID]sourceState{source.DanTorFull: {
			HasLocalArtifact: true,
			ETag:             "etag-1",
			LastCheckedAt:    old,
			LastSuccessAt:    old,
			LastDownloadedAt: old,
			RetryableFailure: true,
			ConsecutiveErrors: []consecutiveError{{
				Kind:            errorKindNetwork,
				Message:         "previous failure",
				Count:           1,
				FirstHappenedAt: old,
				LastHappenedAt:  old,
			}},
		}},
	}

	event, err := u.updateSources(
		context.Background(),
		SyncScopePartial,
		[]source.ID{source.DanTorFull},
		&s,
		nil,
	)

	require.NoError(t, err)
	assert.Empty(t, event.Available)
	assert.Equal(t, []source.ID{source.DanTorFull}, event.Failed)
	assert.Equal(t, []source.ID{source.DanTorFull}, event.Outdated)
	assert.False(t, event.RetryPending)
	assert.False(t, s.Sources[source.DanTorFull].HasLocalArtifact)
	assert.False(t, s.Sources[source.DanTorFull].MarkedOutdatedAt.IsZero())
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestUpdateSources_TarGzDirFailureRemovesOutdatedArtifact(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	dataDir := t.TempDir()
	u := newTestUpdater(dataDir, source.IPVerseASIPBlocksAll, server.URL)
	def := source.DefinitionFor(source.IPVerseASIPBlocksAll)
	path := filepath.Join(dataDir, def.LocalBaseName)
	require.NoError(t, os.MkdirAll(filepath.Join(path, "1"), 0o750))
	oldPath := filepath.Join(path, "1", "aggregated.json")
	require.NoError(t, os.WriteFile(oldPath, []byte("old"), 0o600))
	old := time.Now().UTC().Add(-8 * 24 * time.Hour)
	s := state{
		SyncSchedule: futureSyncSchedule(),
		Sources: map[source.ID]sourceState{source.IPVerseASIPBlocksAll: {
			HasLocalArtifact: true,
			ETag:             "etag-1",
			LastCheckedAt:    old,
			LastSuccessAt:    old,
			LastDownloadedAt: old,
			RetryableFailure: true,
			ConsecutiveErrors: []consecutiveError{{
				Kind:            errorKindNetwork,
				Message:         "previous failure",
				Count:           1,
				FirstHappenedAt: old,
				LastHappenedAt:  old,
			}},
		}},
	}

	event, err := u.updateSources(
		context.Background(),
		SyncScopePartial,
		[]source.ID{source.IPVerseASIPBlocksAll},
		&s,
		nil,
	)

	require.NoError(t, err)
	assert.Empty(t, event.Available)
	assert.Equal(t, []source.ID{source.IPVerseASIPBlocksAll}, event.Failed)
	assert.Equal(t, []source.ID{source.IPVerseASIPBlocksAll}, event.Outdated)
	assert.False(t, s.Sources[source.IPVerseASIPBlocksAll].HasLocalArtifact)
	_, err = os.Stat(path)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestUpdateSources_DanInvalidBodyKeepsArtifactAndSchedulesRetry(t *testing.T) {
	withShortRetryDelays(t, []time.Duration{time.Millisecond, time.Millisecond})
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("ETag", "etag-2")
		_, _ = w.Write([]byte("You can re-fetch the list in: 15 minutes"))
	}))
	defer server.Close()

	dataDir := t.TempDir()
	u := newTestUpdater(dataDir, source.DanTorFull, server.URL)
	def := source.DefinitionFor(source.DanTorFull)
	path := filepath.Join(dataDir, def.LocalBaseName)
	require.NoError(t, os.WriteFile(path, []byte("1.1.1.1\n"), 0o600))
	recent := time.Now().UTC().Add(-time.Hour)
	s := state{
		SyncSchedule: futureSyncSchedule(),
		Sources: map[source.ID]sourceState{source.DanTorFull: {
			HasLocalArtifact: true,
			ETag:             "etag-1",
			LastCheckedAt:    recent,
			LastSuccessAt:    recent,
			LastDownloadedAt: recent,
		}},
	}

	event, err := u.updateSources(
		context.Background(),
		SyncScopePartial,
		[]source.ID{source.DanTorFull},
		&s,
		shortRetryDelays,
	)

	require.NoError(t, err)
	assert.Equal(t, int32(1), attempts.Load())
	assert.Equal(t, []source.ID{source.DanTorFull}, event.Available)
	assert.Equal(t, []source.ID{source.DanTorFull}, event.Failed)
	assert.True(t, event.RetryPending)
	assert.Empty(t, event.Refreshed)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1\n", string(data))
	require.Len(t, s.Sources[source.DanTorFull].ConsecutiveErrors, 1)
	assert.Equal(t, errorKindContent, s.Sources[source.DanTorFull].ConsecutiveErrors[0].Kind)
}

func TestUpdateSources_FirstFailureKeepsStaleArtifactAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	dataDir := t.TempDir()
	u := newTestUpdater(dataDir, source.DanTorFull, server.URL)
	def := source.DefinitionFor(source.DanTorFull)
	path := filepath.Join(dataDir, def.LocalBaseName)
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o600))
	oldDownload := time.Now().UTC().Add(-4 * 24 * time.Hour)
	recentSuccess := time.Now().UTC().Add(-time.Hour)
	s := state{
		SyncSchedule: futureSyncSchedule(),
		Sources: map[source.ID]sourceState{source.DanTorFull: {
			HasLocalArtifact: true,
			ETag:             "etag-1",
			LastCheckedAt:    recentSuccess,
			LastSuccessAt:    recentSuccess,
			LastDownloadedAt: oldDownload,
		}},
	}

	event, err := u.updateSources(
		context.Background(),
		SyncScopePartial,
		[]source.ID{source.DanTorFull},
		&s,
		nil,
	)

	require.NoError(t, err)
	assert.Equal(t, []source.ID{source.DanTorFull}, event.Available)
	assert.Equal(t, []source.ID{source.DanTorFull}, event.Failed)
	assert.Empty(t, event.Outdated)
	_, err = os.Stat(path)
	require.NoError(t, err)
}

func TestRunStartup_FullSyncSkipsMarkedOutdatedSources(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("ETag", "etag-1")
		_, _ = w.Write([]byte("body-data"))
	}))
	defer server.Close()

	dataDir := t.TempDir()
	u := newTestUpdater(dataDir, source.DanTorFull, server.URL)
	s := state{
		SyncSchedule: syncSchedule{NextFullSyncAt: time.Now().UTC().Add(-time.Hour)},
		Sources: map[source.ID]sourceState{source.DanTorFull: {
			MarkedOutdatedAt: time.Now().UTC().Add(-24 * time.Hour),
		}},
	}

	event, err := u.runStartup(context.Background(), time.Now().UTC(), &s)

	require.NoError(t, err)
	assert.Zero(t, attempts.Load())
	assert.Empty(t, event.Refreshed)
	assert.Empty(t, event.Available)
	assert.Empty(t, event.Failed)
	assert.False(t, event.RetryPending)
}

func TestRunScheduled_FullSyncSkipsMarkedOutdatedSources(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.Header().Set("ETag", "etag-1")
		_, _ = w.Write([]byte("body-data"))
	}))
	defer server.Close()

	dataDir := t.TempDir()
	u := newTestUpdater(dataDir, source.DanTorFull, server.URL)
	s := state{
		SyncSchedule: syncSchedule{NextFullSyncAt: time.Now().UTC().Add(-time.Hour)},
		Sources: map[source.ID]sourceState{source.DanTorFull: {
			MarkedOutdatedAt: time.Now().UTC().Add(-24 * time.Hour),
		}},
	}

	event, ok, err := u.runScheduled(context.Background(), &s)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Zero(t, attempts.Load())
	assert.Empty(t, event.Refreshed)
	assert.Empty(t, event.Available)
	assert.Empty(t, event.Failed)
	assert.False(t, event.RetryPending)
}

func TestUpdateSources_MaxMindMissingCredentialsFailsSource(t *testing.T) {
	t.Setenv(maxMindAccountIDEnv, "")
	t.Setenv(maxMindLicenseKeyEnv, "")
	dataDir := t.TempDir()
	u := newTestUpdater(dataDir, source.MaxMindGeoLite2City, "https://example.test")
	s := state{
		SyncSchedule: syncSchedule{NextFullSyncAt: time.Now().UTC().Add(-time.Hour)},
		Sources:      map[source.ID]sourceState{},
	}

	event, err := u.updateSources(
		context.Background(),
		SyncScopeStartup,
		[]source.ID{source.MaxMindGeoLite2City},
		&s,
		nil,
	)

	require.NoError(t, err)
	assert.Empty(t, event.Available)
	assert.Equal(t, []source.ID{source.MaxMindGeoLite2City}, event.Failed)
	assert.False(t, event.RetryPending)
	require.Len(t, s.Sources[source.MaxMindGeoLite2City].ConsecutiveErrors, 1)
	assert.Equal(t, errorKindConfig, s.Sources[source.MaxMindGeoLite2City].ConsecutiveErrors[0].Kind)
}

func TestValidateEnabledSourceUpdates_AllKnownSourcesSupported(t *testing.T) {
	definitions := source.Definitions()
	ids := make([]source.ID, 0, len(definitions))
	for _, definition := range definitions {
		ids = append(ids, definition.ID)
	}

	require.NoError(t, validateEnabledSourceUpdates(ids))
}

func TestValidateSourceUpdateSupported_RejectsUnsupportedCombination(t *testing.T) {
	err := validateSourceUpdateSupported(source.Definition{
		ID:           "test_source",
		ArtifactKind: source.ArtifactKindTarGzFile,
		AuthKind:     source.AuthKindNone,
	})

	require.ErrorIs(t, err, errUnsupportedSourceUpdate)
}

func newTestUpdater(dataDir string, id source.ID, url string) *Updater {
	return newTestUpdaterWithURLs(dataDir, []source.ID{id}, map[source.ID]string{id: url})
}

func withShortRetryDelays(t testing.TB, delays []time.Duration) {
	t.Helper()
	previous := shortRetryDelays
	shortRetryDelays = delays
	t.Cleanup(func() {
		shortRetryDelays = previous
	})
}

func newTestUpdaterWithURLs(
	dataDir string,
	ids []source.ID,
	urls map[source.ID]string,
) *Updater {
	u := New(config.SourcesConfig{
		DataDir: dataDir,
		FullSync: config.FullSyncConfig{
			IntervalHours: 24,
			StartAtUTC:    time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
		},
		Enabled: ids,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	u.sourceDataDir = dataDir

	rewrites := make(map[string]*neturl.URL, len(urls))
	for id, rawURL := range urls {
		def := source.DefinitionFor(id)
		parsed, err := neturl.Parse(rawURL)
		if err != nil {
			panic(err)
		}
		rewrites[def.URL] = parsed
	}
	u.httpClient.Transport = rewriteTransport{targets: rewrites}
	return u
}

type rewriteTransport struct {
	targets map[string]*neturl.URL
}

func (rt rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	target, ok := rt.targets[req.URL.String()]
	if !ok {
		return http.DefaultTransport.RoundTrip(req)
	}
	rewritten := req.Clone(req.Context())
	urlCopy := *target
	rewritten.URL = &urlCopy
	rewritten.Host = target.Host
	return http.DefaultTransport.RoundTrip(rewritten)
}

func futureSyncSchedule() syncSchedule {
	now := time.Now().UTC()
	return syncSchedule{
		LastFullSyncAt: now.Add(-time.Hour),
		NextFullSyncAt: now.Add(time.Hour),
	}
}

func dueRetrySchedule() syncSchedule {
	now := time.Now().UTC()
	return syncSchedule{
		LastFullSyncAt:    now.Add(-2 * time.Hour),
		LastRetrySyncAt:   now.Add(-90 * time.Minute),
		NextFullSyncAt:    now.Add(22 * time.Hour),
		NextRetrySyncAt:   now.Add(-time.Minute),
		LastRetryInterval: retryInterval45m,
	}
}
