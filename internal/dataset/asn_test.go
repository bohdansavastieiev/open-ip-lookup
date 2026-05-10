package dataset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseASN_ReturnsErrorForZero(t *testing.T) {
	asn, err := parseASN("AS0")

	require.ErrorIs(t, err, ErrInvalidASN)
	assert.Zero(t, asn)
}

func TestParseASNNumeric_ReturnsErrorForZero(t *testing.T) {
	asn, err := parseASNNumeric("0")

	require.ErrorIs(t, err, ErrInvalidASN)
	assert.Zero(t, asn)
}
