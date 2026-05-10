package dataset

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIANAAddressBlock_ParsesObservedFormats(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []netip.Prefix
		wantErr bool
	}{
		{
			name:  "annotated prefix is masked",
			input: "192.0.0.123/24 [2]",
			want: []netip.Prefix{
				netip.MustParsePrefix("192.0.0.0/24"),
			},
		},
		{
			name:  "multiple prefixes",
			input: "192.0.0.170/32, 192.0.0.171/32",
			want: []netip.Prefix{
				netip.MustParsePrefix("192.0.0.170/32"),
				netip.MustParsePrefix("192.0.0.171/32"),
			},
		},
		{
			name:    "invalid annotation",
			input:   "192.0.0.0/24 note",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIANAAddressBlock(tt.input)

			if tt.wantErr {
				require.ErrorIs(t, err, ErrInvalidPrefix)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLoadIANASource_ReturnsErrorForDuplicatePrefix(t *testing.T) {
	path := testdataPath(t, "iana/duplicate-prefix.csv")

	infoByPrefix, err := loadIANASource(path, ipVersion4)
	require.ErrorIs(t, err, ErrDuplicateData)
	assert.Nil(t, infoByPrefix)
}

func TestLoadIANASource_ReturnsErrorForUnexpectedAddressFamily(t *testing.T) {
	tests := []struct {
		name          string
		localBaseName string
		version       ipVersion
	}{
		{
			name:          "ipv4 source with ipv6 prefix",
			localBaseName: "iana/wrong-family-ipv4.csv",
			version:       ipVersion4,
		},
		{
			name:          "ipv6 source with ipv4 prefix",
			localBaseName: "iana/wrong-family-ipv6.csv",
			version:       ipVersion6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := testdataPath(t, tt.localBaseName)

			infoByPrefix, err := loadIANASource(path, tt.version)
			require.ErrorIs(t, err, ErrUnexpectedIPVersion)
			assert.Nil(t, infoByPrefix)
		})
	}
}
