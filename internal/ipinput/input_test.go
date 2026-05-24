package ipinput

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse_SkipsInvalid(t *testing.T) {
	ips := Parse("hello 1.2.3.4 world 5.6.7.8 !")

	assert.Equal(t, []netip.Addr{
		netip.MustParseAddr("1.2.3.4"),
		netip.MustParseAddr("5.6.7.8"),
	}, ips)
}

func TestParse_ExtractsIPsFromPunctuation(t *testing.T) {
	ips := Parse("[1.2.3.4], ] [2001:db8::1] 999.999.999.999")

	assert.Equal(t, []netip.Addr{
		netip.MustParseAddr("1.2.3.4"),
		netip.MustParseAddr("2001:db8::1"),
	}, ips)
}

func TestParse_PreservesOccurrenceOrderAndDuplicates(t *testing.T) {
	ips := Parse("5.6.7.8 1.2.3.4 5.6.7.8")

	assert.Equal(t, []netip.Addr{
		netip.MustParseAddr("5.6.7.8"),
		netip.MustParseAddr("1.2.3.4"),
		netip.MustParseAddr("5.6.7.8"),
	}, ips)
}

func TestParse_ReturnsCanonicalAddresses(t *testing.T) {
	ips := Parse("2001:0db8:0:0:0:0:0:0001")

	assert.Len(t, ips, 1)
	assert.Equal(t, "2001:db8::1", ips[0].String())
}
