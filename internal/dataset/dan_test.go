package dataset

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDanAddrs_ParsesObservedFormat(t *testing.T) {
	path := testdataPath(t, "dan/observed.txt")

	addrs, err := loadDanAddrs(path)
	require.NoError(t, err)
	assert.Equal(t, []netip.Addr{
		netip.MustParseAddr("100.10.28.184"),
		netip.MustParseAddr("102.130.113.9"),
	}, addrs)
}

func TestLoadDanAddrs_ReturnsErrorForDuplicateAddr(t *testing.T) {
	path := testdataPath(t, "dan/duplicate.txt")

	addrs, err := loadDanAddrs(path)
	require.ErrorIs(t, err, ErrDuplicateData)
	assert.Nil(t, addrs)
}
