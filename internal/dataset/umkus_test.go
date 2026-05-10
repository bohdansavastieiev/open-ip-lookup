package dataset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadUmkusASNDCs_ParsesObservedFormat(t *testing.T) {
	path := testdataPath(t, "umkus/observed.txt")

	asns, err := loadUmkusASNDCs(path)
	require.NoError(t, err)
	assert.Equal(t, []ASN{112, 543}, asns)
}

func TestLoadUmkusASNDCs_ReturnsErrorForDuplicateASN(t *testing.T) {
	path := testdataPath(t, "umkus/duplicate.txt")

	asns, err := loadUmkusASNDCs(path)
	require.ErrorIs(t, err, ErrDuplicateData)
	assert.Nil(t, asns)
}
