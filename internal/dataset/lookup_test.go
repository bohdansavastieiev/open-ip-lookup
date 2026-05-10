package dataset

import (
	"net/netip"
	"testing"

	"github.com/gaissmai/bart"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookup_SupernetsWalk(t *testing.T) {
	ds := newTestDataset()
	pfx1 := netip.MustParsePrefix("10.0.0.0/8")
	pfx2 := netip.MustParsePrefix("10.0.0.0/24")

	ds.prefixes.Insert(pfx1, 0)
	ds.prefixEntries = append(ds.prefixEntries, prefixEntry{flags: IPFlagVPN})
	ds.prefixes.Insert(pfx2, 1)
	ds.prefixEntries = append(ds.prefixEntries, prefixEntry{flags: IPFlagVPN | IPFlagDatacenter})

	ip := netip.MustParseAddr("10.0.0.1")
	result := ds.Lookup(ip)

	assert.Contains(t, result.Flags, IPFlagVPN)
	assert.Contains(t, result.Flags, IPFlagDatacenter)
}

func TestLookup_MostSpecificWins(t *testing.T) {
	ds := newTestDataset()
	broadPfx := netip.MustParsePrefix("1.0.0.0/8")
	specificPfx := netip.MustParsePrefix("1.0.0.0/24")

	ds.prefixes.Insert(broadPfx, 0)
	ds.prefixEntries = append(ds.prefixEntries, prefixEntry{
		flags:              IPFlagDatacenter,
		cloudProviderIndex: 1,
	})
	ds.cloudProviders = append(
		ds.cloudProviders,
		cloudProviderInfo{provider: "test-provider-wide"},
		cloudProviderInfo{
			provider: "test-provider-specific",
			region:   "test-region",
		})

	ds.prefixes.Insert(specificPfx, 1)
	ds.prefixEntries = append(ds.prefixEntries, prefixEntry{
		flags:              IPFlagDatacenter,
		cloudProviderIndex: 2,
	})

	ip := netip.MustParseAddr("1.0.0.1")
	result := ds.Lookup(ip)

	require.NotNil(t, result.Cloud)
	assert.Equal(t, "test-provider-specific", result.Cloud.Provider)
	assert.Equal(t, "test-region", result.Cloud.Region)
}

func TestLookup_IPMapFlags(t *testing.T) {
	ds := newTestDataset()
	pfx := netip.MustParsePrefix("192.0.2.0/24")
	ds.prefixes.Insert(pfx, 0)
	ds.prefixEntries = append(ds.prefixEntries, prefixEntry{flags: IPFlagVPN})

	addr := netip.MustParseAddr("192.0.2.1")
	ds.ips[addr] = 0
	ds.ipEntries = append(ds.ipEntries, ipEntry{flags: IPFlagTorExit})

	result := ds.Lookup(addr)

	assert.Contains(t, result.Flags, IPFlagVPN)
	assert.Contains(t, result.Flags, IPFlagTorExit)
}

func TestLookup_VPNProviderFromIPMap(t *testing.T) {
	ds := newTestDataset()
	addr := netip.MustParseAddr("1.2.3.4")

	ds.ips[addr] = 0
	ds.ipEntries = append(ds.ipEntries, ipEntry{flags: IPFlagVPN, vpnProviderIndex: 1})
	ds.vpnProviders = append(ds.vpnProviders, "Windscribe")

	result := ds.Lookup(addr)

	assert.Equal(t, "Windscribe", result.VPNProvider)
	assert.Contains(t, result.Flags, IPFlagVPN)
}

func TestLookup_ProxyHighConfOverridesLowConf(t *testing.T) {
	ds := newTestDataset()
	pfx := netip.MustParsePrefix("192.168.1.0/24")
	ds.prefixes.Insert(pfx, 0)
	ds.prefixEntries = append(ds.prefixEntries, prefixEntry{flags: IPFlagProxyHighConf})

	addr := netip.MustParseAddr("192.168.1.1")
	ds.ips[addr] = 0
	ds.ipEntries = append(ds.ipEntries, ipEntry{flags: IPFlagProxyLowConf})

	result := ds.Lookup(addr)

	assert.Contains(t, result.Flags, IPFlagProxyHighConf)
	assert.NotContains(t, result.Flags, IPFlagProxyLowConf)
}

func TestLookup_SpecialUseInfo(t *testing.T) {
	ds := newTestDataset()
	pfx := netip.MustParsePrefix("10.0.0.0/8")
	ds.bogons.Insert(pfx, 0)
	ds.bogonEntries = append(ds.bogonEntries,
		bogonEntry{
			kind: IPKindSpecialUse,
			iana: &ianaInfo{name: "test-special", rfc: "test-rfc"},
		})

	ip := netip.MustParseAddr("10.0.0.1")
	result := ds.Lookup(ip)

	require.NotNil(t, result.SpecialUse)
	assert.Equal(t, "test-special", result.SpecialUse.Name)
	assert.Equal(t, "test-rfc", result.SpecialUse.RFC)
	assert.Equal(t, IPKindSpecialUse, result.Kind)
}

func TestLookup_ASNFromPrefix(t *testing.T) {
	ds := newTestDataset()
	pfx := netip.MustParsePrefix("1.0.0.0/24")
	ds.prefixes.Insert(pfx, 0)
	ds.prefixEntries = append(ds.prefixEntries, prefixEntry{asn: 13335})

	ds.asns[13335] = asnEntry{
		handle:      "test-handle",
		description: "test-desc",
		country:     "test-cc",
		isDC:        true,
	}

	ip := netip.MustParseAddr("1.0.0.1")
	result := ds.Lookup(ip)

	require.NotNil(t, result.ASN)
	assert.Equal(t, ASN(13335), result.ASN.ASN)
	assert.Equal(t, "test-handle", result.ASN.Handle)
	assert.Contains(t, result.Flags, IPFlagDatacenter)
}

func TestLookup_LowReputationFromASN(t *testing.T) {
	ds := newTestDataset()
	pfx := netip.MustParsePrefix("1.0.0.0/24")
	ds.prefixes.Insert(pfx, 0)
	ds.prefixEntries = append(ds.prefixEntries, prefixEntry{asn: 14618})

	ds.asns[14618] = asnEntry{
		isBad: true,
	}

	ip := netip.MustParseAddr("1.0.0.1")
	result := ds.Lookup(ip)

	assert.Contains(t, result.Flags, IPFlagLowReputation)
}

func TestLookup_TorExitNotRelay(t *testing.T) {
	ds := newTestDataset()
	addr := netip.MustParseAddr("100.10.28.184")

	ds.ips[addr] = 0
	ds.ipEntries = append(ds.ipEntries, ipEntry{flags: IPFlagTorExit})

	result := ds.Lookup(addr)

	assert.Contains(t, result.Flags, IPFlagTorExit)
	assert.NotContains(t, result.Flags, IPFlagTorRelay)
}

func TestLookup_TorRelayOnly(t *testing.T) {
	ds := newTestDataset()
	addr := netip.MustParseAddr("100.10.28.184")

	ds.ips[addr] = 0
	ds.ipEntries = append(ds.ipEntries, ipEntry{flags: IPFlagTorRelay})

	result := ds.Lookup(addr)

	assert.Contains(t, result.Flags, IPFlagTorRelay)
	assert.NotContains(t, result.Flags, IPFlagTorExit)
}

func TestLookup_UnallocatedEarlyExit(t *testing.T) {
	ds := newTestDataset()
	pfx := netip.MustParsePrefix("10.0.0.0/8")
	ds.bogons.Insert(pfx, 0)
	ds.bogonEntries = append(ds.bogonEntries, bogonEntry{kind: IPKindUnallocated})

	ip := netip.MustParseAddr("10.0.0.1")
	result := ds.Lookup(ip)

	assert.Equal(t, IPKindUnallocated, result.Kind)
	assert.Nil(t, result.Geo)
	assert.Nil(t, result.ASN)
	assert.Nil(t, result.Cloud)
}

func TestLookup_SpecialUseEarlyExitWithIANA(t *testing.T) {
	ds := newTestDataset()
	pfx := netip.MustParsePrefix("10.0.0.0/8")
	ds.bogons.Insert(pfx, 0)
	ds.bogonEntries = append(
		ds.bogonEntries,
		bogonEntry{
			kind: IPKindSpecialUse,
			iana: &ianaInfo{name: "test-special", rfc: "test-rfc"},
		})

	ip := netip.MustParseAddr("10.0.0.1")
	result := ds.Lookup(ip)

	assert.Equal(t, IPKindSpecialUse, result.Kind)
	require.NotNil(t, result.SpecialUse)
	assert.Equal(t, "test-special", result.SpecialUse.Name)
	assert.Nil(t, result.Geo)
	assert.Nil(t, result.ASN)
}

func TestLookup_DatacenterFromPrefixAndASN(t *testing.T) {
	ds := newTestDataset()
	pfx := netip.MustParsePrefix("192.0.2.0/24")
	ds.prefixes.Insert(pfx, 0)
	ds.prefixEntries = append(ds.prefixEntries, prefixEntry{flags: IPFlagDatacenter, asn: 99999})

	ds.asns[99999] = asnEntry{isDC: true}

	ip := netip.MustParseAddr("192.0.2.1")
	result := ds.Lookup(ip)

	assert.Contains(t, result.Flags, IPFlagDatacenter)
	require.NotNil(t, result.ASN)
}

func TestLookup_ASNInfoWithNetworkFromPrefix(t *testing.T) {
	ds := newTestDataset()
	pfx := netip.MustParsePrefix("1.0.0.0/24")
	ds.prefixes.Insert(pfx, 0)
	ds.prefixEntries = append(ds.prefixEntries, prefixEntry{asn: 99999})

	ds.asns[99999] = asnEntry{
		handle:      "test-handle",
		description: "test-desc",
	}

	ip := netip.MustParseAddr("1.0.0.1")
	result := ds.Lookup(ip)

	require.NotNil(t, result.ASN)
	assert.Equal(t, ASN(99999), result.ASN.ASN)
	assert.Equal(t, pfx, result.ASN.Network)
	assert.Equal(t, "test-handle", result.ASN.Handle)
}

func TestLookup_IPverseMetadataInASNInfo(t *testing.T) {
	ds := newTestDataset()
	pfx := netip.MustParsePrefix("1.0.0.0/24")
	ds.prefixes.Insert(pfx, 0)
	ds.prefixEntries = append(ds.prefixEntries, prefixEntry{asn: 99999})

	ds.asns[99999] = asnEntry{
		handle:      "test-handle",
		description: "test-desc",
		country:     "test-country",
		countryCode: "test-cc",
		category:    "test-category",
		networkRole: "test-role",
	}

	ip := netip.MustParseAddr("1.0.0.1")
	result := ds.Lookup(ip)

	require.NotNil(t, result.ASN)
	assert.Equal(t, "test-handle", result.ASN.Handle)
	assert.Equal(t, "test-desc", result.ASN.Description)
	assert.Equal(t, "test-country", result.ASN.Country)
	assert.Equal(t, "test-cc", result.ASN.CountryISO)
	assert.Equal(t, "test-category", result.ASN.Category)
	assert.Equal(t, "test-role", result.ASN.NetworkRole)
}

func TestLookup_UnallocatedEarlyExitSkipsCloudAndASN(t *testing.T) {
	ds := newTestDataset()
	pfx := netip.MustParsePrefix("10.0.0.0/8")
	ds.bogons.Insert(pfx, 0)
	ds.bogonEntries = append(ds.bogonEntries, bogonEntry{kind: IPKindUnallocated})

	ds.prefixes.Insert(pfx, 0)
	ds.prefixEntries = append(ds.prefixEntries,
		prefixEntry{
			flags:              IPFlagDatacenter,
			cloudProviderIndex: 1, asn: 99999,
		})
	ds.cloudProviders = append(ds.cloudProviders, cloudProviderInfo{provider: "test-cloud"})
	ds.asns[99999] = asnEntry{isDC: true}

	ip := netip.MustParseAddr("10.0.0.1")
	result := ds.Lookup(ip)

	assert.Equal(t, IPKindUnallocated, result.Kind)
	assert.Nil(t, result.Cloud)
	assert.Nil(t, result.ASN)
	assert.Nil(t, result.Geo)
}

func TestLookup_NilReadersNoError(t *testing.T) {
	ds := newTestDataset()

	pfx := netip.MustParsePrefix("1.0.0.0/24")
	ds.prefixes.Insert(pfx, 0)
	ds.prefixEntries = append(ds.prefixEntries, prefixEntry{flags: IPFlagVPN})

	ip := netip.MustParseAddr("1.0.0.1")
	result := ds.Lookup(ip)

	assert.Contains(t, result.Flags, IPFlagVPN)
	assert.Nil(t, result.Geo)
	assert.Nil(t, result.ASN)
}

func TestLookup_SpecialUseOverridesUnallocatedDifferentPrefixes(t *testing.T) {
	ds := newTestDataset()
	broadPfx := netip.MustParsePrefix("10.0.0.0/8")
	specificPfx := netip.MustParsePrefix("10.0.0.0/24")

	ds.bogons.Insert(broadPfx, 0)
	ds.bogonEntries = append(ds.bogonEntries, bogonEntry{kind: IPKindUnallocated})

	ds.bogons.Insert(specificPfx, 1)
	ds.bogonEntries = append(ds.bogonEntries,
		bogonEntry{
			kind: IPKindSpecialUse,
			iana: &ianaInfo{name: "test-special", rfc: "test-rfc"},
		})

	ip := netip.MustParseAddr("10.0.0.1")
	result := ds.Lookup(ip)

	assert.Equal(t, IPKindSpecialUse, result.Kind)
	require.NotNil(t, result.SpecialUse)
	assert.Equal(t, "test-special", result.SpecialUse.Name)
}

func TestLookup_FullIntegration(t *testing.T) {
	ds := newTestDataset()
	ds.geoReader = createTestCityReader(t)
	ds.asnReader = createTestASNReader(t)

	pfx := netip.MustParsePrefix("8.0.0.0/8")
	ds.prefixes.Insert(pfx, 0)
	ds.prefixEntries = append(
		ds.prefixEntries,
		prefixEntry{flags: IPFlagDatacenter, cloudProviderIndex: 1, asn: 99999})
	ds.cloudProviders = append(
		ds.cloudProviders,
		cloudProviderInfo{provider: "AWS", region: "us-east-1"})
	ds.asns[99999] = asnEntry{handle: "test-handle", isDC: true}

	ip := netip.MustParseAddr("8.8.8.8")
	result := ds.Lookup(ip)

	assert.Contains(t, result.Flags, IPFlagDatacenter)
	require.NotNil(t, result.Cloud)
	assert.Equal(t, "AWS", result.Cloud.Provider)
	require.NotNil(t, result.ASN)
	assert.Equal(t, ASN(99999), result.ASN.ASN)
	assert.Equal(t, "test-handle", result.ASN.Handle)
	require.NotNil(t, result.Geo)
	assert.Equal(t, "test-city", result.Geo.City)
}

func newTestDataset() Dataset {
	return Dataset{
		bogons:        bart.Table[uint32]{},
		bogonEntries:  make([]bogonEntry, 0),
		prefixes:      bart.Table[uint32]{},
		prefixEntries: make([]prefixEntry, 0),
		ips:           make(map[netip.Addr]uint32),
		ipEntries:     make([]ipEntry, 0),
		asns:          make(map[ASN]asnEntry),
	}
}
