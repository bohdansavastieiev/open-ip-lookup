package update

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func updateTestdataPath(localPath string) string {
	return filepath.Join("testdata", filepath.FromSlash(localPath))
}

func TestParseCymruTimestamp(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{
			name:  "valid unix timestamp",
			input: "1772427301 (Mon Mar  2 04:55:01 2026 GMT)",
			want:  time.Date(2026, time.March, 2, 4, 55, 1, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCymruTimestamp(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseAz0Timestamp(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{
			name:  "rfc3339",
			input: "2026-03-02T06:11:04Z",
			want:  time.Date(2026, time.March, 2, 6, 11, 4, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAz0Timestamp(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseAvastelTimestampRFC3339(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{
			name:  "rfc3339nano",
			input: "2026-03-04T01:03:24.727718Z",
			want:  time.Date(2026, time.March, 4, 1, 3, 24, 727718000, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAvastelTimestampRFC3339(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseAvastelTimestampCustom(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  time.Time
	}{
		{
			name:  "plain datetime",
			input: "2026-03-04 01:00:55.339444",
			want:  time.Date(2026, time.March, 4, 1, 0, 55, 339444000, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAvastelTimestampCustom(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractUpdatedAt_CymruIPv4(t *testing.T) {
	path := updateTestdataPath("cymru.txt")
	got, err := extractUpdatedAt(source.CymruFullBogonsIPv4, path)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, time.March, 2, 4, 55, 1, 0, time.UTC), got)
}

func TestExtractUpdatedAt_CymruIPv6(t *testing.T) {
	path := updateTestdataPath("cymru.txt")
	got, err := extractUpdatedAt(source.CymruFullBogonsIPv6, path)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, time.March, 2, 4, 55, 1, 0, time.UTC), got)
}

func TestExtractUpdatedAt_Az0IP(t *testing.T) {
	path := updateTestdataPath("az0-ip.txt")
	got, err := extractUpdatedAt(source.Az0VPNIP, path)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, time.March, 2, 6, 11, 4, 0, time.UTC), got)
}

func TestExtractUpdatedAt_Az0Hostname(t *testing.T) {
	path := updateTestdataPath("az0-hostname.txt")
	got, err := extractUpdatedAt(source.Az0VPNHostname, path)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, time.March, 2, 6, 11, 6, 0, time.UTC), got)
}

func TestExtractUpdatedAt_Avastel1Day(t *testing.T) {
	path := updateTestdataPath("avastel-rfc3339.txt")
	got, err := extractUpdatedAt(source.AvastelBotIPsLists1Day, path)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, time.March, 4, 1, 3, 24, 727718000, time.UTC), got)
}

func TestExtractUpdatedAt_Avastel5Day(t *testing.T) {
	path := updateTestdataPath("avastel-custom.txt")
	got, err := extractUpdatedAt(source.AvastelBotIPsLists5Day, path)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, time.March, 4, 1, 0, 55, 339444000, time.UTC), got)
}

func TestExtractUpdatedAt_Avastel8Day(t *testing.T) {
	path := updateTestdataPath("avastel-custom.txt")
	got, err := extractUpdatedAt(source.AvastelBotIPsLists8Day, path)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, time.March, 4, 1, 0, 55, 339444000, time.UTC), got)
}

func TestExtractUpdatedAt_UnknownSource(t *testing.T) {
	_, err := extractUpdatedAt(source.ID("unknown_source"), "some/path.txt")
	require.ErrorIs(t, err, errUnknownTimestampSource)
}
