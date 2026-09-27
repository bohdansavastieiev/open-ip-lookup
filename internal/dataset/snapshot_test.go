package dataset

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSnapshotSkipsSourceThatFailsToLoad(t *testing.T) {
	dir := t.TempDir()
	broken := source.DefinitionFor(source.X4bnetListsVPNVPNIPv4)
	valid := source.DefinitionFor(source.X4bnetListsVPNDatacenterIPv4)
	require.NoError(t, os.WriteFile(filepath.Join(dir, broken.LocalBaseName), []byte("<html>\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, valid.LocalBaseName), []byte("1.2.3.0/24\n"), 0o600))

	snap, skipped, err := loadSnapshot(dir, []source.ID{broken.ID, valid.ID}, slog.New(slog.DiscardHandler))

	require.NoError(t, err)
	assert.Contains(t, snap, valid.ID)
	assert.NotContains(t, snap, broken.ID)
	assert.Error(t, skipped[broken.ID])
}
