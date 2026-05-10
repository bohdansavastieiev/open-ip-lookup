package dataset

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadX4bnetPrefixes_MasksPrefixes(t *testing.T) {
	path := testdataPath(t, "x4bnet/masked-prefix.txt")

	prefixes, err := loadX4bnetPrefixes(path)
	require.NoError(t, err)
	assert.Equal(t, []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}, prefixes)
}

func TestLoadX4bnetPrefixes_ReturnsErrorForUnexpectedAddressFamily(t *testing.T) {
	path := testdataPath(t, "x4bnet/wrong-family.txt")

	prefixes, err := loadX4bnetPrefixes(path)
	require.ErrorIs(t, err, ErrUnexpectedIPVersion)
	assert.Nil(t, prefixes)
}

func TestLoadX4bnetPrefixes_ReturnsErrorForDuplicateCanonicalPrefix(t *testing.T) {
	path := testdataPath(t, "x4bnet/duplicate-canonical-prefix.txt")

	prefixes, err := loadX4bnetPrefixes(path)
	require.ErrorIs(t, err, ErrDuplicateData)
	assert.Nil(t, prefixes)
}
