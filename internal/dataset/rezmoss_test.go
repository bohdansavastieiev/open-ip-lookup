package dataset

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRezmossAllProviders_MasksPrefixes(t *testing.T) {
	path := testdataPath(t, "rezmoss/masked-prefix.json")

	infoByPrefix, err := loadRezmossAllProviders(path)
	require.NoError(t, err)
	assert.Equal(t, map[netip.Prefix][]rezmossInfo{
		netip.MustParsePrefix("173.245.48.0/20"): {
			{
				provider:    "cloudflare",
				service:     "",
				region:      "",
				lastUpdated: "2026-03-19 02:03:51",
			},
		},
	}, infoByPrefix)
}

func TestLoadRezmossAllProviders_ReturnsErrorForEmptyProvider(t *testing.T) {
	path := testdataPath(t, "rezmoss/empty-provider.json")

	infoByPrefix, err := loadRezmossAllProviders(path)
	require.ErrorIs(t, err, errRezmossProviderRequired)
	assert.Nil(t, infoByPrefix)
}

func TestLoadRezmossAllProviders_ReturnsErrorForMissingCIDR(t *testing.T) {
	path := writeDatasetTestFile(t, `[{"ip_version":"IPv4","provider":"cloudflare"}]`)

	infoByPrefix, err := loadRezmossAllProviders(path)

	require.ErrorIs(t, err, ErrInvalidPrefix)
	assert.Nil(t, infoByPrefix)
}

func TestLoadRezmossAllProviders_ReturnsErrorForMismatchedIPVersion(t *testing.T) {
	path := writeDatasetTestFile(
		t,
		`[{"cidr":"173.245.48.0/20","ip_version":"IPv6","provider":"cloudflare"}]`,
	)

	infoByPrefix, err := loadRezmossAllProviders(path)

	require.ErrorIs(t, err, ErrUnexpectedIPVersion)
	assert.Nil(t, infoByPrefix)
}
