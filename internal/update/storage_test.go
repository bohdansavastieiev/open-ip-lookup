package update

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/config"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadStateForSync_ReturnsEmptyStateWhenStateAndSourcesMissing(t *testing.T) {
	dataDir := t.TempDir()
	cfg := validTestFullSyncConfig()
	require.NoError(t, os.Mkdir(filepath.Join(dataDir, updateDir), 0o750))

	s, err := loadStateForSync(dataDir, cfg)

	require.NoError(t, err)
	assert.Equal(t, state{
		SyncSchedule: syncSchedule{NextFullSyncAt: cfg.StartAtUTC},
		Sources:      map[source.ID]sourceState{},
	}, s)
}

func TestLoadStateForSync_PreservesDueSyncSchedule(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	want := state{
		SyncSchedule: syncSchedule{
			LastFullSyncAt:    now.Add(-73 * time.Hour),
			NextFullSyncAt:    now.Add(-49 * time.Hour),
			LastRetrySyncAt:   now.Add(-52 * time.Hour),
			NextRetrySyncAt:   now.Add(-50 * time.Hour),
			LastRetryInterval: retryInterval1h30m,
		},
		Sources: map[source.ID]sourceState{source.DanTorFull: {}},
	}
	require.NoError(t, want.saveStateJSON(stateFilePath(dataDir)))

	got, err := loadStateForSync(dataDir, validTestFullSyncConfig())

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestLoadStateForSync_ReturnsErrorWhenStateMissingButSourcePresent(t *testing.T) {
	dataDir := t.TempDir()
	def := source.DefinitionFor(source.DanTorFull)
	path := filepath.Join(dataDir, def.LocalBaseName)
	require.NoError(t, os.WriteFile(path, []byte("data"), 0o600))

	_, err := loadStateForSync(dataDir, validTestFullSyncConfig())

	require.ErrorIs(t, err, errNoStateButPresentData)
}

func TestLoadStateForSync_ReturnsErrorWhenStateMissingButUnknownFilePresent(t *testing.T) {
	dataDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "unknown.txt"), []byte("data"), 0o600))

	_, err := loadStateForSync(dataDir, validTestFullSyncConfig())

	require.Error(t, err)
	assert.False(t, errors.Is(err, errNoStateButPresentData))
}

func TestStorageTxnRollback_RestoresPromotedSource(t *testing.T) {
	dataDir := t.TempDir()
	def := source.DefinitionFor(source.DanTorFull)
	path := filepath.Join(dataDir, def.LocalBaseName)
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o600))
	tempPath, err := writeTempSource(strings.NewReader("new"), updateDirPath(dataDir), def.LocalBaseName)
	require.NoError(t, err)

	tx := newStorageTxn(dataDir)
	require.NoError(t, tx.promoteArtifact(def, tempPath))
	require.NoError(t, tx.rollback())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "old", string(data))
}

func TestStorageTxnRollback_RestoresRemovedSource(t *testing.T) {
	dataDir := t.TempDir()
	def := source.DefinitionFor(source.DanTorFull)
	path := filepath.Join(dataDir, def.LocalBaseName)
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o600))

	tx := newStorageTxn(dataDir)
	require.NoError(t, tx.removeArtifact(def))
	require.NoError(t, tx.rollback())

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "old", string(data))
}

func TestStorageTxnFinalize_RemovesRollbackFiles(t *testing.T) {
	dataDir := t.TempDir()
	def := source.DefinitionFor(source.DanTorFull)
	path := filepath.Join(dataDir, def.LocalBaseName)
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o600))
	tempPath, err := writeTempSource(strings.NewReader("new"), updateDirPath(dataDir), def.LocalBaseName)
	require.NoError(t, err)

	tx := newStorageTxn(dataDir)
	require.NoError(t, tx.promoteArtifact(def, tempPath))
	require.NoError(t, tx.finalize())

	entries, err := os.ReadDir(updateDirPath(dataDir))
	require.NoError(t, err)
	assert.Empty(t, entries)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))
}

func TestStorageTxnRollback_RestoresPromotedDirectory(t *testing.T) {
	dataDir := t.TempDir()
	def := source.DefinitionFor(source.IPVerseASIPBlocksAll)
	path := filepath.Join(dataDir, def.LocalBaseName)
	require.NoError(t, os.MkdirAll(filepath.Join(path, "1"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(path, "1", "aggregated.json"), []byte("old"), 0o600))

	tempPath := filepath.Join(updateDirPath(dataDir), "ipverse-new")
	require.NoError(t, os.MkdirAll(filepath.Join(tempPath, "2"), 0o750))
	newPath := filepath.Join(tempPath, "2", "aggregated.json")
	require.NoError(t, os.WriteFile(newPath, []byte("new"), 0o600))

	tx := newStorageTxn(dataDir)
	require.NoError(t, tx.promoteArtifact(def, tempPath))
	require.NoError(t, tx.rollback())

	data, err := os.ReadFile(filepath.Join(path, "1", "aggregated.json"))
	require.NoError(t, err)
	assert.Equal(t, "old", string(data))
}

func TestStorageTxnFinalize_RemovesDirectoryRollback(t *testing.T) {
	dataDir := t.TempDir()
	def := source.DefinitionFor(source.IPVerseASIPBlocksAll)
	path := filepath.Join(dataDir, def.LocalBaseName)
	require.NoError(t, os.MkdirAll(filepath.Join(path, "1"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(path, "1", "aggregated.json"), []byte("old"), 0o600))

	tempPath := filepath.Join(updateDirPath(dataDir), "ipverse-new")
	require.NoError(t, os.MkdirAll(filepath.Join(tempPath, "2"), 0o750))
	newPath := filepath.Join(tempPath, "2", "aggregated.json")
	require.NoError(t, os.WriteFile(newPath, []byte("new"), 0o600))

	tx := newStorageTxn(dataDir)
	require.NoError(t, tx.promoteArtifact(def, tempPath))
	require.NoError(t, tx.finalize())

	entries, err := os.ReadDir(updateDirPath(dataDir))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestLoadStateForSync_ReturnsErrorWhenLocalSourceMissingFromState(t *testing.T) {
	dataDir := t.TempDir()
	st := state{Sources: map[source.ID]sourceState{source.CymruFullBogonsIPv4: {}}}
	require.NoError(t, st.saveStateJSON(stateFilePath(dataDir)))
	def := source.DefinitionFor(source.DanTorFull)
	path := filepath.Join(dataDir, def.LocalBaseName)
	require.NoError(t, os.WriteFile(path, []byte("data"), 0o600))

	_, err := loadStateForSync(dataDir, validTestFullSyncConfig())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "present locally, but not in state")
}

func TestLoadStateForSync_ReturnsErrorWhenStateExpectsMissingLocalSource(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now().UTC()
	st := state{Sources: map[source.ID]sourceState{source.DanTorFull: {
		HasLocalArtifact: true,
		LastCheckedAt:    now,
		LastSuccessAt:    now,
		LastDownloadedAt: now,
	}}}
	require.NoError(t, st.saveStateJSON(stateFilePath(dataDir)))

	_, err := loadStateForSync(dataDir, validTestFullSyncConfig())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected to be present locally")
}

func TestLoadStateForSync_ReturnsErrorWhenLocalSourceShouldBeDeleted(t *testing.T) {
	dataDir := t.TempDir()
	st := state{Sources: map[source.ID]sourceState{source.DanTorFull: {}}}
	require.NoError(t, st.saveStateJSON(stateFilePath(dataDir)))
	def := source.DefinitionFor(source.DanTorFull)
	path := filepath.Join(dataDir, def.LocalBaseName)
	require.NoError(t, os.WriteFile(path, []byte("data"), 0o600))

	_, err := loadStateForSync(dataDir, validTestFullSyncConfig())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected to be deleted locally")
}

func validTestFullSyncConfig() config.FullSyncConfig {
	return config.FullSyncConfig{
		IntervalHours: 24,
		StartAtUTC:    time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
	}
}
