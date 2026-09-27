package dataset

import (
	"net/netip"
	"testing"

	"github.com/oschwald/geoip2-golang/v2"
	"github.com/stretchr/testify/assert"
)

func TestGeoSanctionsFlags(t *testing.T) {
	tests := []struct {
		name       string
		country    string
		registered string
		region     string
		city       string
		want       IPFlag
	}{
		{name: "ofac country", country: "IR", want: IPFlagOFAC},
		{name: "sanctioned country", country: "RU", want: IPFlagSanctionedCountry},
		{name: "registered in sanctioned country only", country: "NL", registered: "RU"},
		{name: "registered in sanctioned country", country: "DE", registered: "IR", want: IPFlagPossibleOFAC},
		{name: "crimea", country: "UA", region: "43", city: "Simferopol", want: IPFlagOFAC},
		{name: "dnr city", country: "UA", region: "14", city: "Donetsk", want: IPFlagOFAC},
		{name: "rest of donetsk oblast", country: "UA", region: "14", city: "Kramatorsk", want: IPFlagPossibleOFAC},
		{name: "donetsk oblast without city", country: "UA", region: "14", want: IPFlagPossibleOFAC},
		{name: "occupied zaporizhzhia", country: "UA", region: "23", city: "Melitopol", want: IPFlagPossibleOFAC},
		{name: "held regional capital", country: "UA", region: "23", city: "Zaporizhzhya"},
		{name: "other ukrainian region", country: "UA", region: "63", city: "Kharkiv"},
		{
			name:    "same city name in russia",
			country: "RU",
			region:  "ROS",
			city:    "Donetsk",
			want:    IPFlagSanctionedCountry,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := &geoip2.City{}
			record.Country.ISOCode = tt.country
			record.RegisteredCountry.ISOCode = tt.registered
			record.City.Names.English = tt.city
			if tt.region != "" {
				record.Subdivisions = []geoip2.CitySubdivision{{ISOCode: tt.region}}
			}

			assert.Equal(t, tt.want, geoSanctionsFlags(record))
		})
	}
}

func TestStrongestSanctionsFlag(t *testing.T) {
	all := IPFlagOFAC | IPFlagPossibleOFAC | IPFlagSanctionedCountry

	assert.Equal(t, IPFlagVPN|IPFlagOFAC, strongestSanctionsFlag(IPFlagVPN|all))
	assert.Equal(t, IPFlagPossibleOFAC, strongestSanctionsFlag(IPFlagPossibleOFAC|IPFlagSanctionedCountry))
	assert.Equal(t, IPFlagSanctionedCountry, strongestSanctionsFlag(IPFlagSanctionedCountry))
}

func TestLookup_OFACFromASN(t *testing.T) {
	ds := newTestDataset()
	pfx := netip.MustParsePrefix("1.0.0.0/24")
	ds.prefixes.Insert(pfx, 0)
	ds.prefixEntries = append(ds.prefixEntries, prefixEntry{asn: 214721})

	result := ds.Lookup(netip.MustParseAddr("1.0.0.1"))

	assert.Contains(t, result.Flags, IPFlagOFAC)
	assert.NotContains(t, result.Flags, IPFlagPossibleOFAC)
}
