package dataset

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseHostnameInfo_NormalizesIDNAAndExtractsParts(t *testing.T) {
	info, err := parseHostnameInfo("WWW.BÜCHER.DE.")
	require.NoError(t, err)

	assert.Equal(t, hostnameInfo{
		hostname:  "www.xn--bcher-kva.de",
		subdomain: "www",
		domain:    "xn--bcher-kva",
		suffix:    "de",
	}, info)
}

func TestParseHostnameInfo_ReturnsErrorForInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "url", input: "https://example.com"},
		{name: "host with port", input: "example.com:443"},
		{name: "single label", input: "localhost"},
		{name: "invalid label char", input: "bad_name.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := parseHostnameInfo(tt.input)
			require.Error(t, err)
			assert.Equal(t, hostnameInfo{}, info)
		})
	}
}

func TestLoadAz0VPNIP_ParsesObservedFormat(t *testing.T) {
	path := testdataPath(t, "az0/vpn-ip-observed.txt")

	providerByAddr, err := loadAz0VPNIP(path)
	require.NoError(t, err)
	assert.Equal(t, map[netip.Addr]string{
		netip.MustParseAddr("1.2.3.4"): "windscribe",
		netip.MustParseAddr("2.3.4.5"): "nordvpn",
	}, providerByAddr)
}

func TestLoadAz0VPNHostname_ReturnsErrorForDuplicateNormalizedHostname(t *testing.T) {
	path := testdataPath(t, "az0/vpn-hostname-duplicate-normalized.txt")

	hostnameInfos, err := loadAz0VPNHostname(path)
	require.ErrorIs(t, err, ErrDuplicateData)
	assert.Nil(t, hostnameInfos)
}
