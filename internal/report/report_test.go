package report

import (
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/dataset"
	"github.com/stretchr/testify/assert"
)

func TestFlagEmoji(t *testing.T) {
	tests := []struct {
		code string
		want string
	}{
		{"US", "🇺🇸"},
		{"GB", "🇬🇧"},
		{"DE", "🇩🇪"},
		{"", ""},
		{"A", ""},
		{"ABC", ""},
		{"12", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, flagEmoji(tt.code))
	}
}

func TestHumanize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"isp", "ISP"},
		{"education_research", "Education Research"},
		{"tier1_transit", "Tier 1 Transit"},
		{"hosting", "Hosting"},
		{"access_provider", "Access Provider"},
		{"", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, humanize(tt.input))
	}
}

func TestGet_CapsAtMaxIPs(t *testing.T) {
	ips := parse(makeIPList(MaxIPs + 50))
	unique := dedup(ips)
	assert.Equal(t, MaxIPs+50, len(ips))
	assert.Equal(t, MaxIPs, min(len(unique), MaxIPs))
}

func TestGet_StatsRemainExactWhenReportedCapped(t *testing.T) {
	rpt := Get(makeIPList(MaxIPs+50), &dataset.Dataset{})

	assert.Equal(t, MaxIPs+50, rpt.Stats.Total)
	assert.Equal(t, MaxIPs+50, rpt.Stats.Unique)
	assert.Equal(t, MaxIPs, rpt.Stats.Reported)
	assert.Len(t, rpt.Entries, MaxIPs)
}

func TestGet_EntryOccurrencesCountInputDuplicates(t *testing.T) {
	rpt := Get("1.2.3.4 5.6.7.8 1.2.3.4 1.2.3.4", &dataset.Dataset{})

	assert.Len(t, rpt.Entries, 2)
	assert.Equal(t, 3, rpt.Entries[0].Occurrences)
	assert.Equal(t, 1, rpt.Entries[1].Occurrences)
}

func TestDedup_PreservesOrder(t *testing.T) {
	ips := []netip.Addr{
		netip.MustParseAddr("5.6.7.8"),
		netip.MustParseAddr("1.2.3.4"),
		netip.MustParseAddr("5.6.7.8"),
	}
	result := dedup(ips)
	assert.Len(t, result, 2)
	assert.Equal(t, netip.MustParseAddr("5.6.7.8"), result[0])
	assert.Equal(t, netip.MustParseAddr("1.2.3.4"), result[1])
}

func TestParse_SkipsInvalid(t *testing.T) {
	ips := parse("hello 1.2.3.4 world 5.6.7.8 !")
	assert.Len(t, ips, 2)
}

func TestParse_ExtractsIPsFromPunctuation(t *testing.T) {
	ips := parse("[1.2.3.4], ] [2001:db8::1] 999.999.999.999")

	assert.Equal(t, []netip.Addr{
		netip.MustParseAddr("1.2.3.4"),
		netip.MustParseAddr("2001:db8::1"),
	}, ips)
}

func TestGet_ReportsCanonicalIPs(t *testing.T) {
	rpt := Get("2001:0db8:0:0:0:0:0:0001", &dataset.Dataset{})

	assert.Len(t, rpt.Entries, 1)
	assert.Equal(t, "2001:db8::1", rpt.Entries[0].IP)
}

func TestBuildEntry_NonRoutableOmitsFlags(t *testing.T) {
	entry := buildEntry(netip.MustParseAddr("192.0.2.1"), 1, dataset.IPResult{
		Kind:  dataset.IPKindSpecialUse,
		Flags: []dataset.IPFlag{dataset.IPFlagVPN},
	})

	assert.Empty(t, entry.Flags)
}

func TestBuildEntry_ASNZeroReturnsNoASN(t *testing.T) {
	entry := buildEntry(netip.MustParseAddr("1.2.3.4"), 1, dataset.IPResult{
		ASN: &dataset.ASNInfo{ASN: 0},
	})

	assert.Nil(t, entry.ASN)
	assert.False(t, entry.HasDetails)
}

func TestBuildEntry_EmptyGeoReturnsNoGeo(t *testing.T) {
	entry := buildEntry(netip.MustParseAddr("1.2.3.4"), 1, dataset.IPResult{
		Geo: &dataset.GeoInfo{},
	})

	assert.Nil(t, entry.Geo)
	assert.False(t, entry.HasDetails)
}

func TestBuildEntry_SetsHasDetails(t *testing.T) {
	entry := buildEntry(netip.MustParseAddr("1.2.3.4"), 1, dataset.IPResult{
		Geo: &dataset.GeoInfo{CountryISO: "US"},
	})

	assert.NotNil(t, entry.Geo)
	assert.True(t, entry.HasDetails)
}

func TestBuildEntry_ASNOrganizationAndRegistryHandle(t *testing.T) {
	entry := buildEntry(netip.MustParseAddr("1.2.3.4"), 1, dataset.IPResult{
		ASN: &dataset.ASNInfo{
			ASN:         64500,
			Handle:      "EXAMPLE-NET",
			Description: "Example Network LLC",
			Network:     netip.MustParsePrefix("1.2.3.0/24"),
		},
	})

	assert.Equal(t, "Example Network LLC", entry.ASN.Organization)
	assert.Equal(t, "EXAMPLE-NET", entry.ASN.RegistryHandle)
}

func TestBuildEntry_ASNOrganizationFallsBackToHandle(t *testing.T) {
	entry := buildEntry(netip.MustParseAddr("1.2.3.4"), 1, dataset.IPResult{
		ASN: &dataset.ASNInfo{
			ASN:     64500,
			Handle:  "Example Network LLC",
			Network: netip.MustParsePrefix("1.2.3.0/24"),
		},
	})

	assert.Equal(t, "Example Network LLC", entry.ASN.Organization)
	assert.Empty(t, entry.ASN.RegistryHandle)
}

func TestBuildEntry_ASNNetworkIncludesRange(t *testing.T) {
	entry := buildEntry(netip.MustParseAddr("2001:db8::1"), 1, dataset.IPResult{
		ASN: &dataset.ASNInfo{
			ASN:     64500,
			Network: netip.MustParsePrefix("2001:db8::/126"),
		},
	})

	assert.Equal(t, "2001:db8::/126", entry.ASN.Network.Prefix)
	assert.Equal(t, "2001:db8::", entry.ASN.Network.FirstIP)
	assert.Equal(t, "2001:db8::3", entry.ASN.Network.LastIP)
}

func TestNetworkInfo_IPv4Range(t *testing.T) {
	network := networkInfo(netip.MustParsePrefix("1.2.3.0/30"))

	assert.Equal(t, "1.2.3.0/30", network.Prefix)
	assert.Equal(t, "1.2.3.0", network.FirstIP)
	assert.Equal(t, "1.2.3.3", network.LastIP)
}

func makeIPList(n int) string {
	var result strings.Builder
	for i := range n {
		if i > 0 {
			result.WriteString("\n")
		}
		o1 := (i >> 16) & 0xff
		o2 := (i >> 8) & 0xff
		o3 := i & 0xff
		fmt.Fprintf(&result, "%d.%d.%d.1", o1, o2, o3)
	}
	return result.String()
}
