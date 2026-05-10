package dataset

import (
	"net/netip"
	"testing"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
	"github.com/gaissmai/bart"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildASNMap_SetsIsBad(t *testing.T) {
	b := newTestBuilder()
	snap := snapshot{
		source.BountyyfiBadASNListAll: map[ASN][]string{13335: {"test-inc-1"}, 14618: {"test-inc-2"}},
	}
	b.snap = snap
	b.buildASNMap()

	assert.True(t, b.ds.asns[13335].isBad)
	assert.True(t, b.ds.asns[14618].isBad)
	assert.False(t, b.ds.asns[15169].isBad)
}

func TestBuildASNMap_SetsIsDC(t *testing.T) {
	b := newTestBuilder()
	snap := snapshot{
		source.UmkusIPIndexASNDCs: []ASN{13335, 14618},
	}
	b.snap = snap
	b.buildASNMap()

	assert.True(t, b.ds.asns[13335].isDC)
	assert.True(t, b.ds.asns[14618].isDC)
}

func TestBuildASNMap_MergesMultipleSources(t *testing.T) {
	b := newTestBuilder()
	snap := snapshot{
		source.BountyyfiBadASNListAll: map[ASN][]string{13335: {"test-inc-1"}},
		source.UmkusIPIndexASNDCs:     []ASN{13335, 14618},
	}
	b.snap = snap
	b.buildASNMap()

	assert.True(t, b.ds.asns[13335].isBad)
	assert.True(t, b.ds.asns[13335].isDC)
	assert.True(t, b.ds.asns[14618].isDC)
	assert.False(t, b.ds.asns[14618].isBad)
}

func TestBuildBogonTable_AddsBogon(t *testing.T) {
	b := newTestBuilder()
	pfx := netip.MustParsePrefix("10.0.0.0/8")
	snap := snapshot{
		source.CymruFullBogonsIPv4: []netip.Prefix{pfx},
	}
	b.snap = snap
	b.buildBogonTable()

	entryIndex, ok := b.ds.bogons.Get(pfx)
	require.True(t, ok)
	assert.Equal(t, IPKindUnallocated, b.ds.bogonEntries[entryIndex].kind)
}

func TestBuildPrefixTable_MergesOverlappingPrefixes(t *testing.T) {
	b := newTestBuilder()
	vpnPfx := netip.MustParsePrefix("192.0.2.0/24")
	dcPfx := netip.MustParsePrefix("192.0.2.0/24")
	snap := snapshot{
		source.X4bnetListsVPNVPNIPv4:        []netip.Prefix{vpnPfx},
		source.X4bnetListsVPNDatacenterIPv4: []netip.Prefix{dcPfx},
	}
	b.snap = snap
	b.buildPrefixTable()

	entryIndex, ok := b.ds.prefixes.Get(vpnPfx)
	require.True(t, ok)
	entry := b.ds.prefixEntries[entryIndex]
	assert.Equal(t, IPFlagVPN|IPFlagDatacenter, entry.flags)
}

func TestBuildPrefixTable_SetsASNFromIPverse(t *testing.T) {
	b := newTestBuilder()
	pfx := netip.MustParsePrefix("1.0.0.0/24")
	snap := snapshot{
		source.IPVerseASIPBlocksAll: map[ASN]ipverseASBlocksRecord{
			13335: {
				ASN: 13335,
				Prefixes: ipversePrefixes{
					IPv4: []netip.Prefix{pfx},
				},
			},
		},
	}
	b.snap = snap
	b.buildPrefixTable()

	entryIndex, ok := b.ds.prefixes.Get(pfx)
	require.True(t, ok)
	assert.Equal(t, ASN(13335), b.ds.prefixEntries[entryIndex].asn)
}

func TestBuildBogonTable_SetsIANAInfo(t *testing.T) {
	b := newTestBuilder()
	pfx := netip.MustParsePrefix("10.0.0.0/8")
	snap := snapshot{
		source.IANASpecialIPv4: map[netip.Prefix]ianaInfo{
			pfx: {name: "test-special", rfc: "test-rfc"},
		},
	}
	b.snap = snap
	b.buildBogonTable()

	entryIndex, ok := b.ds.bogons.Get(pfx)
	require.True(t, ok)
	entry := b.ds.bogonEntries[entryIndex]
	assert.Equal(t, IPKindSpecialUse, entry.kind)
	require.NotNil(t, entry.iana)
	assert.Equal(t, "test-special", entry.iana.name)
	assert.Equal(t, "test-rfc", entry.iana.rfc)
}

func TestBuildIPMap_SetsTorFlags(t *testing.T) {
	b := newTestBuilder()
	addr := netip.MustParseAddr("100.10.28.184")
	snap := snapshot{
		source.DanTorFull: []netip.Addr{addr},
	}
	b.snap = snap
	b.buildIPMap()

	entryIndex, ok := b.ds.ips[addr]
	require.True(t, ok)
	assert.Equal(t, IPFlagTorRelay, b.ds.ipEntries[entryIndex].flags)
}

func TestBuildIPMap_TorExitOverwritesRelay(t *testing.T) {
	b := newTestBuilder()
	addr := netip.MustParseAddr("100.10.28.184")
	snap := snapshot{
		source.DanTorFull: []netip.Addr{addr},
		source.DanTorExit: []netip.Addr{addr},
	}
	b.snap = snap
	b.buildIPMap()

	entryIndex, ok := b.ds.ips[addr]
	require.True(t, ok)
	assert.Equal(t, IPFlagTorExit, b.ds.ipEntries[entryIndex].flags)
}

func TestBuildIPMap_TorExitWithoutFull(t *testing.T) {
	b := newTestBuilder()
	addr := netip.MustParseAddr("100.10.28.184")
	snap := snapshot{
		source.DanTorExit: []netip.Addr{addr},
	}
	b.snap = snap
	b.buildIPMap()

	entryIndex, ok := b.ds.ips[addr]
	require.True(t, ok)
	assert.Equal(t, IPFlagTorExit, b.ds.ipEntries[entryIndex].flags)
}

func TestBuildIPMap_SetsVPNProvider(t *testing.T) {
	b := newTestBuilder()
	addr := netip.MustParseAddr("1.2.3.4")
	snap := snapshot{
		source.Az0VPNIP: map[netip.Addr]string{addr: "windscribe"},
	}
	b.snap = snap
	b.buildIPMap()

	entryIndex, ok := b.ds.ips[addr]
	require.True(t, ok)
	entry := b.ds.ipEntries[entryIndex]
	assert.Equal(t, IPFlagVPN, entry.flags)
	require.NotZero(t, entry.vpnProviderIndex)
	assert.Equal(t, "Windscribe", b.ds.vpnProviders[entry.vpnProviderIndex-1])
}

func TestBuildIPMap_SetsProxyLowConf(t *testing.T) {
	b := newTestBuilder()
	addr := netip.MustParseAddr("1.2.3.4")
	snap := snapshot{
		source.AvastelBotIPsLists1Day: map[netip.Addr]avastelAddrInfo{addr: {org: "test-org"}},
	}
	b.snap = snap
	b.buildIPMap()

	entryIndex, ok := b.ds.ips[addr]
	require.True(t, ok)
	assert.Equal(t, IPFlagProxyLowConf, b.ds.ipEntries[entryIndex].flags)
}

func TestBuildPrefixTable_FirstNonZeroASNWins(t *testing.T) {
	b := newTestBuilder()
	pfx := netip.MustParsePrefix("1.0.0.0/24")
	snap := snapshot{
		source.IPVerseASIPBlocksAll: map[ASN]ipverseASBlocksRecord{
			13335: {
				ASN: 13335,
				Prefixes: ipversePrefixes{
					IPv4: []netip.Prefix{pfx},
				},
			},
		},
		source.X4bnetListsVPNVPNIPv4: []netip.Prefix{pfx},
	}
	b.snap = snap
	b.buildPrefixTable()

	entryIndex, ok := b.ds.prefixes.Get(pfx)
	require.True(t, ok)
	entry := b.ds.prefixEntries[entryIndex]
	assert.Equal(t, ASN(13335), entry.asn)
	assert.Equal(t, IPFlagVPN, entry.flags)
}

func TestBuildPrefixTable_CloudProviderFromTobilg(t *testing.T) {
	b := newTestBuilder()
	pfx := netip.MustParsePrefix("1.178.1.0/24")
	snap := snapshot{
		source.TobilgCloudProviderRanges: map[netip.Prefix]cloudInfo{
			pfx: {provider: "AWS", region: "us-east-1"},
		},
	}
	b.snap = snap
	b.buildPrefixTable()

	entryIndex, ok := b.ds.prefixes.Get(pfx)
	require.True(t, ok)
	entry := b.ds.prefixEntries[entryIndex]
	assert.Equal(t, IPFlagDatacenter, entry.flags)
	require.NotZero(t, entry.cloudProviderIndex)
	cp := b.ds.cloudProviders[entry.cloudProviderIndex-1]
	assert.Equal(t, "AWS", cp.provider)
	assert.Equal(t, "us-east-1", cp.region)
}

func TestBuildPrefixTable_CloudProviderFromRezmoss(t *testing.T) {
	b := newTestBuilder()
	pfx := netip.MustParsePrefix("1.178.1.0/24")
	snap := snapshot{
		source.RezmossCloudProviders: map[netip.Prefix][]rezmossInfo{
			pfx: {{provider: "aws", service: "cloudfront", region: "GLOBAL"}},
		},
	}
	b.snap = snap
	b.buildPrefixTable()

	entryIndex, ok := b.ds.prefixes.Get(pfx)
	require.True(t, ok)
	cp := b.ds.cloudProviders[b.ds.prefixEntries[entryIndex].cloudProviderIndex-1]
	assert.Equal(t, "AWS", cp.provider)
	assert.Equal(t, "cloudfront", cp.service)
	assert.Equal(t, "GLOBAL", cp.region)
}

func TestBuildASNMap_SetsIPverseMetadata(t *testing.T) {
	b := newTestBuilder()
	country := "test-country"
	cc := "test-cc"
	cat := "test-category"
	nr := "test-role"
	snap := snapshot{
		source.IPVerseASMetadataAll: map[ASN]ipverseASMetadataRecord{
			99999: {
				ASN: 99999,
				Metadata: ipverseASMetadata{
					Handle:      "test-handle",
					Description: "test-desc",
					CountryCode: &cc,
					Country:     &country,
					Category:    &cat,
					NetworkRole: &nr,
				},
			},
		},
	}
	b.snap = snap
	b.buildASNMap()

	e := b.ds.asns[99999]
	assert.Equal(t, "test-handle", e.handle)
	assert.Equal(t, "test-desc", e.description)
	assert.Equal(t, "test-country", e.country)
	assert.Equal(t, "test-cc", e.countryCode)
	assert.Equal(t, "test-category", e.category)
	assert.Equal(t, "test-role", e.networkRole)
}

func TestBuildBogonTable_SpecialOverridesBogon(t *testing.T) {
	b := newTestBuilder()
	pfx := netip.MustParsePrefix("10.0.0.0/8")
	snap := snapshot{
		source.CymruFullBogonsIPv4: []netip.Prefix{pfx},
		source.IANASpecialIPv4: map[netip.Prefix]ianaInfo{
			pfx: {name: "test-special", rfc: "test-rfc"},
		},
	}
	b.snap = snap
	b.buildBogonTable()

	entryIndex, ok := b.ds.bogons.Get(pfx)
	require.True(t, ok)
	entry := b.ds.bogonEntries[entryIndex]
	assert.Equal(t, IPKindSpecialUse, entry.kind)
	require.NotNil(t, entry.iana)
	assert.Equal(t, "test-special", entry.iana.name)
	assert.Equal(t, "test-rfc", entry.iana.rfc)
}

func TestBuildBogonTable_CymruIPv6Bogon(t *testing.T) {
	b := newTestBuilder()
	pfx := netip.MustParsePrefix("::/0")
	snap := snapshot{
		source.CymruFullBogonsIPv6: []netip.Prefix{pfx},
	}
	b.snap = snap
	b.buildBogonTable()

	entryIndex, ok := b.ds.bogons.Get(pfx)
	require.True(t, ok)
	assert.Equal(t, IPKindUnallocated, b.ds.bogonEntries[entryIndex].kind)
}

func TestBuildBogonTable_IANAIPv6Special(t *testing.T) {
	b := newTestBuilder()
	pfx := netip.MustParsePrefix("::1/128")
	snap := snapshot{
		source.IANASpecialIPv6: map[netip.Prefix]ianaInfo{
			pfx: {name: "test-ipv6-special", rfc: "test-rfc-v6"},
		},
	}
	b.snap = snap
	b.buildBogonTable()

	entryIndex, ok := b.ds.bogons.Get(pfx)
	require.True(t, ok)
	entry := b.ds.bogonEntries[entryIndex]
	assert.Equal(t, IPKindSpecialUse, entry.kind)
	require.NotNil(t, entry.iana)
	info := entry.iana
	assert.Equal(t, "test-ipv6-special", info.name)
	assert.Equal(t, "test-rfc-v6", info.rfc)
}

func TestBuildBogonTable_IANACoversBogonsSubPrefix(t *testing.T) {
	b := newTestBuilder()
	ianaPfx := netip.MustParsePrefix("10.0.0.0/8")
	bogonPfx := netip.MustParsePrefix("10.0.0.0/24")
	snap := snapshot{
		source.IANASpecialIPv4: map[netip.Prefix]ianaInfo{
			ianaPfx: {name: "test-iana", rfc: "test-rfc"},
		},
		source.CymruFullBogonsIPv4: []netip.Prefix{bogonPfx},
	}
	b.snap = snap
	b.buildBogonTable()

	entryIndex, ok := b.ds.bogons.Get(ianaPfx)
	require.True(t, ok, "IANA /8 should be inserted")
	assert.Equal(t, IPKindSpecialUse, b.ds.bogonEntries[entryIndex].kind)

	_, ok = b.ds.bogons.Get(bogonPfx)
	assert.False(t, ok, "bogon /24 should not be inserted when IANA /8 covers it")
}

func TestBuildBogonTable_IANASubPrefixCoveredByBogonWider(t *testing.T) {
	b := newTestBuilder()
	ianaPfx := netip.MustParsePrefix("10.0.0.0/24")
	bogonPfx := netip.MustParsePrefix("10.0.0.0/8")
	snap := snapshot{
		source.IANASpecialIPv4: map[netip.Prefix]ianaInfo{
			ianaPfx: {name: "test-iana", rfc: "test-rfc"},
		},
		source.CymruFullBogonsIPv4: []netip.Prefix{bogonPfx},
	}
	b.snap = snap
	b.buildBogonTable()

	entryIndex, ok := b.ds.bogons.Get(ianaPfx)
	require.True(t, ok, "IANA /24 should be inserted")
	assert.Equal(t, IPKindSpecialUse, b.ds.bogonEntries[entryIndex].kind)

	entryIndex, ok = b.ds.bogons.Get(bogonPfx)
	require.True(t, ok, "bogon /8 should be inserted when IANA only has /24 (no supernet)")
	assert.Equal(t, IPKindUnallocated, b.ds.bogonEntries[entryIndex].kind)
}

func TestBuildPrefixTable_ProxyHighConfFrom8Day(t *testing.T) {
	b := newTestBuilder()
	pfx := netip.MustParsePrefix("192.0.2.0/24")
	snap := snapshot{
		source.AvastelBotIPsLists8Day: map[netip.Prefix]avastelPrefixInfo{pfx: {org: "test-org", confidence: 0.9}},
	}
	b.snap = snap
	b.buildPrefixTable()

	entryIndex, ok := b.ds.prefixes.Get(pfx)
	require.True(t, ok)
	assert.Equal(t, IPFlagProxyHighConf, b.ds.prefixEntries[entryIndex].flags)
}

func TestBuildPrefixTable_RezmossEmptyInfosSkipped(t *testing.T) {
	b := newTestBuilder()
	emptyPfx := netip.MustParsePrefix("192.0.2.0/24")
	snap := snapshot{
		source.RezmossCloudProviders: map[netip.Prefix][]rezmossInfo{
			emptyPfx: {},
		},
	}
	b.snap = snap
	b.buildPrefixTable()

	_, ok := b.ds.prefixes.Get(emptyPfx)
	assert.False(t, ok)
}

func TestBuild_AddCloudProviderDedup(t *testing.T) {
	b := newTestBuilder()
	snap := snapshot{
		source.TobilgCloudProviderRanges: map[netip.Prefix]cloudInfo{
			netip.MustParsePrefix("1.0.0.0/24"): {provider: "AWS", region: "us-east-1"},
			netip.MustParsePrefix("2.0.0.0/24"): {provider: "AWS", region: "us-east-1"},
		},
	}
	b.snap = snap
	b.buildPrefixTable()

	assert.Equal(t, 1, len(b.ds.cloudProviders))
}

func TestBuild_AddVPNProviderDedup(t *testing.T) {
	b := newTestBuilder()
	addr1 := netip.MustParseAddr("1.2.3.4")
	addr2 := netip.MustParseAddr("5.6.7.8")
	snap := snapshot{
		source.Az0VPNIP: map[netip.Addr]string{
			addr1: "nordvpn",
			addr2: "nordvpn",
		},
	}
	b.snap = snap
	b.buildIPMap()

	assert.Equal(t, 1, len(b.ds.vpnProviders))
	assert.Equal(t, "NordVPN", b.ds.vpnProviders[0])
}

func TestBuildPrefixTable_RezmossWinsOverTobilg(t *testing.T) {
	b := newTestBuilder()
	pfx := netip.MustParsePrefix("192.0.2.0/24")
	snap := snapshot{
		source.RezmossCloudProviders: map[netip.Prefix][]rezmossInfo{
			pfx: {{provider: "aws", service: "cloudfront", region: "GLOBAL"}},
		},
		source.TobilgCloudProviderRanges: map[netip.Prefix]cloudInfo{
			pfx: {provider: "AWS", region: "us-east-1"},
		},
	}
	b.snap = snap
	b.buildPrefixTable()

	entryIndex, ok := b.ds.prefixes.Get(pfx)
	require.True(t, ok)
	entry := b.ds.prefixEntries[entryIndex]
	assert.Equal(t, IPFlagDatacenter, entry.flags)
	require.NotZero(t, entry.cloudProviderIndex)
	cp := b.ds.cloudProviders[entry.cloudProviderIndex-1]
	assert.Equal(t, "AWS", cp.provider)
	assert.Equal(t, "cloudfront", cp.service)
	assert.Equal(t, "GLOBAL", cp.region)
}

func newTestBuilder() builder {
	return builder{
		ds: Dataset{
			bogons:        bart.Table[uint32]{},
			bogonEntries:  make([]bogonEntry, 0),
			prefixes:      bart.Table[uint32]{},
			prefixEntries: make([]prefixEntry, 0),
			ips:           make(map[netip.Addr]uint32),
			ipEntries:     make([]ipEntry, 0),
			asns:          make(map[ASN]asnEntry),
		},
		cloudSeen: make(map[cloudProviderInfo]uint32),
		vpnSeen:   make(map[string]uint32),
	}
}

func TestCapitalize(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"windscribe", "Windscribe"},
		{"AWS", "AWS"},
		{"azure", "Azure"},
		{"", ""},
		{"a", "A"},
		{"digitalocean", "Digitalocean"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expect, capitalize(tt.input))
		})
	}
}

func TestNormalizeCloudProvider(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"aws", "AWS"},
		{"AWS", "AWS"},
		{"digitalocean", "DigitalOcean"},
		{"googlecloud", "Google Cloud"},
		{"apple_private_relay", "Apple Private Relay"},
		{"unknown-provider", "Unknown-provider"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expect, normalizeCloudProvider(tt.input))
		})
	}
}

func TestNormalizeVPNProvider(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"windscribe", "Windscribe"},
		{"nordvpn", "NordVPN"},
		{"expressvpn", "ExpressVPN"},
		{"unknownvpn", "Unknownvpn"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expect, normalizeVPNProvider(tt.input))
		})
	}
}
