// Package ipinput extracts IP addresses from submitted text.
package ipinput

import (
	"net/netip"
	"strings"
)

func Parse(raw string) []netip.Addr {
	entries := strings.FieldsFunc(raw, func(c rune) bool {
		return !isCandidateRune(c)
	})

	ips := make([]netip.Addr, 0, len(entries))
	for _, e := range entries {
		addr, err := netip.ParseAddr(e)
		if err != nil {
			continue
		}
		ips = append(ips, addr)
	}

	return ips
}

func isCandidateRune(c rune) bool {
	return c == '.' || c == ':' || c >= '0' && c <= '9' ||
		c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}
