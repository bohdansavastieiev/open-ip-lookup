package dataset

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAvastelInfoByAddr_ParsesObservedFormat(t *testing.T) {
	path := testdataPath(t, "avastel/1-day-observed.txt")

	infoByAddr, err := loadAvastelInfoByAddr(path, nil)
	require.NoError(t, err)
	assert.Equal(t, map[netip.Addr]avastelAddrInfo{
		netip.MustParseAddr("154.37.79.123"): {org: "QuickPacket, LLC"},
	}, infoByAddr)
}

func TestLoadAvastelInfoByAddr_SkipsWrongFieldCountRecord(t *testing.T) {
	path := writeDatasetTestFile(t, `# header
# update
# count

ip_address;autonomous_system
154.37.79.123;QuickPacket, LLC
190.12.131.238;Ainet internet eamp;informatica ltda
`)

	infoByAddr, err := loadAvastelInfoByAddr(path, nil)

	require.NoError(t, err)
	assert.Equal(t, map[netip.Addr]avastelAddrInfo{
		netip.MustParseAddr("154.37.79.123"): {org: "QuickPacket, LLC"},
	}, infoByAddr)
}

func TestLoadAvastelInfoByPrefix_ParsesObservedFormat(t *testing.T) {
	path := testdataPath(t, "avastel/5-day-observed.txt")

	infoByPrefix, err := loadAvastelInfoByPrefix(path, nil)
	require.NoError(t, err)
	assert.Equal(t, map[netip.Prefix]avastelPrefixInfo{
		netip.MustParsePrefix("154.37.79.0/24"): {
			org:        "QuickPacket, LLC",
			confidence: 1,
		},
	}, infoByPrefix)
}

func TestLoadAvastelInfoByAddr_ReturnsErrorForEmptyFile(t *testing.T) {
	path := testdataPath(t, "avastel/empty.txt")

	infoByAddr, err := loadAvastelInfoByAddr(path, nil)
	require.ErrorIs(t, err, ErrNoValidElements)
	assert.Nil(t, infoByAddr)
}

func TestLoadAvastelInfoByAddr_ReturnsErrorForMissingAddrColumn(t *testing.T) {
	path := writeDatasetTestFile(t, `# header
# update
# count

autonomous_system
QuickPacket, LLC
`)

	infoByAddr, err := loadAvastelInfoByAddr(path, nil)

	require.ErrorIs(t, err, ErrInvalidAddr)
	assert.Nil(t, infoByAddr)
}

func TestLoadAvastelInfoByPrefix_ReturnsErrorForMissingPrefixColumn(t *testing.T) {
	path := writeDatasetTestFile(t, `# header
# update
# count

autonomous_system;confidence
QuickPacket, LLC;1.0
`)

	infoByPrefix, err := loadAvastelInfoByPrefix(path, nil)

	require.ErrorIs(t, err, ErrInvalidPrefix)
	assert.Nil(t, infoByPrefix)
}
