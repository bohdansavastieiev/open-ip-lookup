package dataset

import (
	"fmt"
	"net/netip"
	"strings"
)

type ianaInfo struct {
	name string
	rfc  string
}

type ianaRecord struct {
	AddressBlock string `csv:"Address Block"`
	Name         string `csv:"Name"`
	RFC          string `csv:"RFC"`
}

func loadIANASpecialIPv4(path string) (map[netip.Prefix]ianaInfo, error) {
	return loadIANASource(path, ipVersion4)
}

func loadIANASpecialIPv6(path string) (map[netip.Prefix]ianaInfo, error) {
	return loadIANASource(path, ipVersion6)
}

func loadIANASource(path string, version ipVersion) (map[netip.Prefix]ianaInfo, error) {
	infoByPrefix := make(map[netip.Prefix]ianaInfo)

	handleRecord := func(rec ianaRecord) (bool, error) {
		prefixes, err := parseIANAAddressBlock(rec.AddressBlock)
		if err != nil {
			return false, err
		}

		info := ianaInfo{
			name: strings.Join(strings.Fields(rec.Name), " "),
			rfc:  strings.Join(strings.Fields(rec.RFC), " "),
		}

		for _, prefix := range prefixes {
			if !version.matchesPrefix(prefix) {
				return false, fmt.Errorf(
					"%w, column: %q, expected: %s, got: %q",
					ErrUnexpectedIPVersion,
					"Address Block",
					version,
					rec.AddressBlock,
				)
			}

			if _, ok := infoByPrefix[prefix]; ok {
				return false, fmt.Errorf("%w, prefix: %v", ErrDuplicateData, prefix)
			}

			infoByPrefix[prefix] = info
		}

		return true, nil
	}

	counter, err := scanCSVSource(path, 0, nil, nil, handleRecord)
	if err != nil {
		return nil, err
	}
	if counter == 0 {
		return nil, fmt.Errorf("%w", ErrNoValidElements)
	}

	return infoByPrefix, nil
}

func parseIANAAddressBlock(s string) ([]netip.Prefix, error) {
	parts := strings.Split(s, ",")
	prefixes := make([]netip.Prefix, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("%w, column: %q, value: %q", ErrInvalidPrefix, "Address Block", s)
		}

		if before, after, ok := strings.Cut(part, " "); ok {
			after = strings.TrimSpace(after)
			if !strings.HasPrefix(after, "[") || !strings.HasSuffix(after, "]") {
				return nil, fmt.Errorf("%w, column: %q, value: %q", ErrInvalidPrefix, "Address Block", s)
			}

			part = strings.TrimSpace(before)
		}

		prefix, err := netip.ParsePrefix(part)
		if err != nil {
			return nil, fmt.Errorf("%w, column: %q, value: %q", ErrInvalidPrefix, "Address Block", s)
		}
		prefix = prefix.Masked()

		prefixes = append(prefixes, prefix)
	}

	if len(prefixes) == 0 {
		return nil, fmt.Errorf("%w, column: %q, value: %q", ErrInvalidPrefix, "Address Block", s)
	}

	return prefixes, nil
}
