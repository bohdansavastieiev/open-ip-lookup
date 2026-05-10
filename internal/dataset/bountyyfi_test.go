package dataset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBountyyfiBadASN_ParsesObservedFormat(t *testing.T) {
	path := testdataPath(t, "bountyyfi/observed.txt")

	highRiskOrgByASN, err := loadBountyyfiBadASN(path)
	require.NoError(t, err)
	assert.Equal(t, map[ASN][]string{
		51167:  {"Contabo GmbH", "Contabo LLC"},
		14061:  {"DigitalOcean, LLC"},
		199739: nil,
	}, highRiskOrgByASN)
}

func TestLoadBountyyfiBadASN_ReturnsErrorForInvalidASNFormat(t *testing.T) {
	path := testdataPath(t, "bountyyfi/invalid-asn.txt")

	highRiskOrgByASN, err := loadBountyyfiBadASN(path)
	require.ErrorIs(t, err, ErrInvalidASN)
	assert.Nil(t, highRiskOrgByASN)
}
