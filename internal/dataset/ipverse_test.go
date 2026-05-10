package dataset

import (
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeIPVersePrefixes(t *testing.T) {
	tests := []struct {
		name     string
		prefixes ipversePrefixes
		want     ipversePrefixes
		wantErr  bool
	}{
		{
			name: "masks ipv4 and ipv6 prefixes",
			prefixes: ipversePrefixes{
				IPv4: []netip.Prefix{netip.MustParsePrefix("192.0.2.1/24")},
				IPv6: []netip.Prefix{netip.MustParsePrefix("2001:db8::1/32")},
			},
			want: ipversePrefixes{
				IPv4: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
				IPv6: []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")},
			},
		},
		{
			name: "rejects unexpected family in ipv4 list",
			prefixes: ipversePrefixes{
				IPv4: []netip.Prefix{netip.MustParsePrefix("2001:db8::/32")},
			},
			wantErr: true,
		},
		{
			name: "rejects unexpected family in ipv6 list",
			prefixes: ipversePrefixes{
				IPv6: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
			},
			wantErr: true,
		},
		{
			name: "rejects duplicate prefixes after masking",
			prefixes: ipversePrefixes{
				IPv4: []netip.Prefix{
					netip.MustParsePrefix("192.0.2.0/24"),
					netip.MustParsePrefix("192.0.2.1/24"),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefixes := tt.prefixes
			err := normalizeIPVersePrefixes(&prefixes)

			if tt.wantErr {
				require.Error(t, err)
				require.True(t, errors.Is(err, ErrUnexpectedIPVersion) || errors.Is(err, ErrDuplicateData))
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, prefixes)
		})
	}
}

func TestLoadIPVerseASBlocks_ReturnsErrorForMismatchedASNDirectory(t *testing.T) {
	path := testdataPath(t, "ipverse/as-blocks-mismatched-asn")

	asBlocksByASN, err := loadIPVerseASBlocks(path)
	require.ErrorIs(t, err, errIPVerseASNMismatch)
	assert.Nil(t, asBlocksByASN)
}

func TestLoadIPVerseASMetadata_ReturnsErrorForDuplicateASN(t *testing.T) {
	path := testdataPath(t, "ipverse/as-metadata-duplicate.json")

	asMetadataByASN, err := loadIPVerseASMetadata(path)
	require.ErrorIs(t, err, ErrDuplicateData)
	assert.Nil(t, asMetadataByASN)
}

func TestLoadIPVerseASMetadata_SkipsASNZero(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "as.json")
	data := `[
		{"asn":0,"metadata":{"handle":"AS0","description":"reserved"}},
		{"asn":1,"metadata":{"handle":"LVLT-1","description":"Level 3 Parent LLC"}}
	]`
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))

	asMetadataByASN, err := loadIPVerseASMetadata(path)
	require.NoError(t, err)
	assert.NotContains(t, asMetadataByASN, ASN(0))
	assert.Contains(t, asMetadataByASN, ASN(1))
}
