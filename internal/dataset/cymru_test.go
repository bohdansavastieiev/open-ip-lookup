package dataset

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCymruSource_ParsesObservedHeaderAndData(t *testing.T) {
	path := testdataPath(t, "cymru/ipv4-observed.txt")

	prefixes, err := loadCymruSource(path, ipVersion4)
	require.NoError(t, err)
	assert.Equal(t, []netip.Prefix{
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("192.0.2.0/24"),
	}, prefixes)
}

func TestLoadCymruSource_ReturnsErrorForUnexpectedAddressFamily(t *testing.T) {
	tests := []struct {
		name          string
		localBaseName string
		version       ipVersion
	}{
		{
			name:          "ipv4 source with ipv6 prefix",
			localBaseName: "cymru/wrong-family-ipv4.txt",
			version:       ipVersion4,
		},
		{
			name:          "ipv6 source with ipv4 prefix",
			localBaseName: "cymru/wrong-family-ipv6.txt",
			version:       ipVersion6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := testdataPath(t, tt.localBaseName)

			prefixes, err := loadCymruSource(path, tt.version)
			require.ErrorIs(t, err, ErrUnexpectedIPVersion)
			assert.Nil(t, prefixes)
		})
	}
}
