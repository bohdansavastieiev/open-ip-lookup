package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteTempTarGzDir_ExtractsFiles(t *testing.T) {
	archive := newTestTarGz(t, map[string]string{
		"1/aggregated.json": "one",
		"2/aggregated.json": "two",
	})

	path, err := writeTempTarGzDir(bytes.NewReader(archive), t.TempDir(), "ipverse")

	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(path, "1", "aggregated.json"))
	require.NoError(t, err)
	assert.Equal(t, "one", string(data))
}

func TestWriteTempTarGzDir_RejectsPathTraversal(t *testing.T) {
	archive := newTestTarGz(t, map[string]string{"../bad.txt": "bad"})
	dir := t.TempDir()

	path, err := writeTempTarGzDir(bytes.NewReader(archive), dir, "ipverse")

	require.ErrorIs(t, err, errInvalidArchivePath)
	assert.Empty(t, path)
	entries, readErr := os.ReadDir(dir)
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

func TestWriteTempTarGzDir_RejectsTooManyFiles(t *testing.T) {
	old := maxArchiveFiles
	maxArchiveFiles = 1
	t.Cleanup(func() { maxArchiveFiles = old })
	archive := newTestTarGz(t, map[string]string{
		"1/aggregated.json": "one",
		"2/aggregated.json": "two",
	})

	path, err := writeTempTarGzDir(bytes.NewReader(archive), t.TempDir(), "ipverse")

	require.ErrorIs(t, err, errArtifactTooLarge)
	assert.Empty(t, path)
}

func TestWriteTempTarGzDir_RejectsTotalExtractedBytesLimit(t *testing.T) {
	oldTotal := maxArchiveTotalBytes
	oldFile := maxArchiveFileBytes
	maxArchiveTotalBytes = 3
	maxArchiveFileBytes = 10
	t.Cleanup(func() {
		maxArchiveTotalBytes = oldTotal
		maxArchiveFileBytes = oldFile
	})
	archive := newTestTarGz(t, map[string]string{
		"1/aggregated.json": "12",
		"2/aggregated.json": "34",
	})

	path, err := writeTempTarGzDir(bytes.NewReader(archive), t.TempDir(), "ipverse")

	require.ErrorIs(t, err, errArtifactTooLarge)
	assert.Empty(t, path)
}

func TestWriteTempTarGzFile_RejectsFileSizeLimit(t *testing.T) {
	old := maxArchiveFileBytes
	maxArchiveFileBytes = 3
	t.Cleanup(func() { maxArchiveFileBytes = old })
	archive := newTestTarGz(t, map[string]string{"GeoLite2-City/test.mmdb": "1234"})

	path, err := writeTempTarGzFile(bytes.NewReader(archive), t.TempDir(), "city.mmdb", isMMDBPath)

	require.ErrorIs(t, err, errArtifactTooLarge)
	assert.Empty(t, path)
}

func newTestTarGz(t testing.TB, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o600,
			Size: int64(len(body)),
		}
		require.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}
