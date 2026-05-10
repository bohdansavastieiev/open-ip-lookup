package dataset

import (
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
	"github.com/oschwald/geoip2-golang/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestCityReader(t *testing.T) *geoip2.Reader {
	t.Helper()
	writer, err := mmdbwriter.New(mmdbwriter.Options{DatabaseType: "GeoLite2-City"})
	require.NoError(t, err)

	pfx := netip.MustParsePrefix("8.0.0.0/8")
	record := mmdbtype.Map{
		"city": mmdbtype.Map{
			"names": mmdbtype.Map{
				"en": mmdbtype.String("test-city"),
			},
		},
		"country": mmdbtype.Map{
			"names": mmdbtype.Map{
				"en": mmdbtype.String("test-country"),
			},
			"iso_code": mmdbtype.String("TC"),
		},
		"subdivisions": mmdbtype.Slice{
			mmdbtype.Map{
				"names": mmdbtype.Map{
					"en": mmdbtype.String("test-region"),
				},
			},
		},
		"location": mmdbtype.Map{
			"latitude":  mmdbtype.Float64(10.5),
			"longitude": mmdbtype.Float64(20.5),
			"time_zone": mmdbtype.String("test/tz"),
		},
	}

	require.NoError(t, writer.Insert(netipRange(pfx), record))

	dir := t.TempDir()
	path := filepath.Join(dir, "test-city.mmdb")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	_, err = writer.WriteTo(f)
	require.NoError(t, err)

	reader, err := geoip2.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close() })
	return reader
}

func createTestASNReader(t *testing.T) *geoip2.Reader {
	t.Helper()
	writer, err := mmdbwriter.New(mmdbwriter.Options{DatabaseType: "GeoLite2-ASN"})
	require.NoError(t, err)

	pfx := netip.MustParsePrefix("8.0.0.0/8")
	record := mmdbtype.Map{
		"autonomous_system_number":       mmdbtype.Uint64(99999),
		"autonomous_system_organization": mmdbtype.String("test-asn-org"),
	}

	require.NoError(t, writer.Insert(netipRange(pfx), record))

	dir := t.TempDir()
	path := filepath.Join(dir, "test-asn.mmdb")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	_, err = writer.WriteTo(f)
	require.NoError(t, err)

	reader, err := geoip2.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close() })
	return reader
}

func createTestZeroASNReader(t *testing.T) *geoip2.Reader {
	t.Helper()
	writer, err := mmdbwriter.New(mmdbwriter.Options{DatabaseType: "GeoLite2-ASN"})
	require.NoError(t, err)

	pfx := netip.MustParsePrefix("8.0.0.0/8")
	require.NoError(t, writer.Insert(netipRange(pfx), mmdbtype.Map{}))

	dir := t.TempDir()
	path := filepath.Join(dir, "test-asn-zero.mmdb")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	_, err = writer.WriteTo(f)
	require.NoError(t, err)

	reader, err := geoip2.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close() })
	return reader
}

func netipRange(pfx netip.Prefix) *net.IPNet {
	ip := pfx.Addr().AsSlice()
	mask := net.CIDRMask(pfx.Bits(), pfx.Addr().BitLen())
	return &net.IPNet{
		IP:   ip,
		Mask: mask,
	}
}

func TestLoadMaxMindGeoReader_ValidType(t *testing.T) {
	path := createTestMMDBFile(t, "GeoLite2-City")

	reader, err := loadMaxMindGeoReader(path, maxmindDBTypeCity)
	require.NoError(t, err)
	require.NotNil(t, reader)
	_ = reader.Close()
}

func TestLoadMaxMindGeoReader_ValidTypeASN(t *testing.T) {
	path := createTestMMDBFile(t, "GeoLite2-ASN")

	reader, err := loadMaxMindGeoReader(path, maxmindDBTypeASN)
	require.NoError(t, err)
	require.NotNil(t, reader)
	_ = reader.Close()
}

func TestLoadMaxMindGeoReader_WrongType(t *testing.T) {
	path := createTestMMDBFile(t, "GeoLite2-City")

	reader, err := loadMaxMindGeoReader(path, maxmindDBTypeASN)
	require.ErrorIs(t, err, errMaxMindDBTypeMismatch)
	assert.Nil(t, reader)
}

func TestLoadMaxMindGeoReader_EmptyType(t *testing.T) {
	path := createTestMMDBFile(t, "GeoLite2-City")

	reader, err := loadMaxMindGeoReader(path, maxmindDBType(""))
	require.ErrorIs(t, err, errMaxMindDBTypeMismatch)
	assert.Nil(t, reader)
}

func TestExtractReader_ReturnsErrorForUnexpectedType(t *testing.T) {
	snap := snapshot{
		source.MaxMindGeoLite2City: "not a reader",
	}

	reader, err := extractReader(snap, source.MaxMindGeoLite2City)
	require.ErrorIs(t, err, errMaxMindReaderType)
	assert.Nil(t, reader)
}

func createTestMMDBFile(t *testing.T, dbType string) string {
	t.Helper()
	writer, err := mmdbwriter.New(mmdbwriter.Options{DatabaseType: dbType})
	require.NoError(t, err)

	pfx := netip.MustParsePrefix("8.0.0.0/8")
	record := mmdbtype.Map{}
	require.NoError(t, writer.Insert(netipRange(pfx), record))

	dir := t.TempDir()
	path := filepath.Join(dir, "test.mmdb")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	_, err = writer.WriteTo(f)
	require.NoError(t, err)

	return path
}

func TestLookup_GeoFromMaxMind(t *testing.T) {
	ds := newTestDataset()
	ds.geoReader = createTestCityReader(t)

	ip := netip.MustParseAddr("8.8.8.8")
	result := ds.Lookup(ip)

	require.NotNil(t, result.Geo)
	assert.Equal(t, "test-city", result.Geo.City)
	assert.Equal(t, "test-country", result.Geo.Country)
	assert.Equal(t, "TC", result.Geo.CountryISO)
	assert.Equal(t, "test-region", result.Geo.Region)
	assert.Equal(t, 10.5, result.Geo.Latitude)
	assert.Equal(t, 20.5, result.Geo.Longitude)
	assert.Equal(t, "test/tz", result.Geo.Timezone)
}

func TestLookup_ASNFromMaxMind(t *testing.T) {
	ds := newTestDataset()
	ds.asnReader = createTestASNReader(t)

	ip := netip.MustParseAddr("8.8.8.8")
	result := ds.Lookup(ip)

	require.NotNil(t, result.ASN)
	assert.Equal(t, ASN(99999), result.ASN.ASN)
	assert.Equal(t, "test-asn-org", result.ASN.Handle)
}

func TestLookup_MaxMindASNHandleFallback(t *testing.T) {
	ds := newTestDataset()
	ds.asnReader = createTestASNReader(t)

	pfx := netip.MustParsePrefix("8.0.0.0/8")
	ds.prefixes.Insert(pfx, 0)
	ds.prefixEntries = append(ds.prefixEntries, prefixEntry{asn: 99999})
	ds.asns[99999] = asnEntry{handle: "test-handle"}

	ip := netip.MustParseAddr("8.8.8.8")
	result := ds.Lookup(ip)

	require.NotNil(t, result.ASN)
	assert.Equal(t, "test-handle", result.ASN.Handle)
}

func TestLookup_MaxMindASNHandleUsedWhenNoPrefixASN(t *testing.T) {
	ds := newTestDataset()
	ds.asnReader = createTestASNReader(t)

	ip := netip.MustParseAddr("8.8.8.8")
	result := ds.Lookup(ip)

	require.NotNil(t, result.ASN)
	assert.Equal(t, "test-asn-org", result.ASN.Handle)
}

func TestLookup_MaxMindASNZeroReturnsNoASN(t *testing.T) {
	ds := newTestDataset()
	ds.asnReader = createTestZeroASNReader(t)

	ip := netip.MustParseAddr("8.8.8.8")
	result := ds.Lookup(ip)

	assert.Nil(t, result.ASN)
}
