package dataset

import (
	"fmt"
	"net/netip"
)

type ipVersion uint8

const (
	ipVersion4 ipVersion = iota + 1
	ipVersion6
)

func (v ipVersion) String() string {
	switch v {
	case ipVersion4:
		return "IPv4"
	case ipVersion6:
		return "IPv6"
	default:
		return fmt.Sprintf("ipVersion(%d)", v)
	}
}

func (v ipVersion) matchesPrefix(prefix netip.Prefix) bool {
	return v.matchesAddr(prefix.Addr())
}

func (v ipVersion) matchesAddr(addr netip.Addr) bool {
	if !addr.IsValid() {
		return false
	}

	switch v {
	case ipVersion4:
		return addr.Is4()
	case ipVersion6:
		return addr.Is6()
	default:
		return false
	}
}
