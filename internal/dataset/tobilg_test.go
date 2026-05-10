package dataset

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadTobilgCloud_MasksPrefixes(t *testing.T) {
	path := testdataPath(t, "tobilg/masked-prefix.json")

	infoByPrefix, err := loadTobilgCloud(path)
	require.NoError(t, err)
	assert.Equal(t, map[netip.Prefix]cloudInfo{
		netip.MustParsePrefix("1.178.1.0/24"): {
			provider: "AWS",
			region:   "us-west-2",
		},
	}, infoByPrefix)
}

func TestLoadTobilgCloud_ReturnsErrorForDuplicateCanonicalPrefix(t *testing.T) {
	path := testdataPath(t, "tobilg/duplicate-canonical-prefix.json")

	infoByPrefix, err := loadTobilgCloud(path)
	require.ErrorIs(t, err, ErrDuplicateData)
	assert.Nil(t, infoByPrefix)
}

func TestLoadTobilgCloud_ReturnsErrorForMissingCIDR(t *testing.T) {
	path := writeDatasetTestFile(t, `[{"cloud_provider":"AWS","region":"us-west-2"}]`)

	infoByPrefix, err := loadTobilgCloud(path)

	require.ErrorIs(t, err, ErrInvalidPrefix)
	assert.Nil(t, infoByPrefix)
}

func writeDatasetTestFile(t *testing.T, data string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "source.txt")
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
	return path
}
