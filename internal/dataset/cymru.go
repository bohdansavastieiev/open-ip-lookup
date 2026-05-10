package dataset

import (
	"fmt"
	"net/netip"
)

func loadCymruFullbogonsIPv4(path string) ([]netip.Prefix, error) {
	return loadCymruSource(path, ipVersion4)
}

func loadCymruFullbogonsIPv6(path string) ([]netip.Prefix, error) {
	return loadCymruSource(path, ipVersion6)
}

func loadCymruSource(path string, version ipVersion) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0)
	skipLines := 2

	seenPrefixes := make(map[netip.Prefix]struct{})

	handleLine := func(line string) (bool, error) {
		prefix, err := netip.ParsePrefix(line)
		if err != nil {
			return false, fmt.Errorf("%w, prefix: %q", ErrInvalidPrefix, line)
		}
		prefix = prefix.Masked()
		if !version.matchesPrefix(prefix) {
			return false, fmt.Errorf("%w, prefix: %q, expected: %s", ErrUnexpectedIPVersion, line, version)
		}

		if _, ok := seenPrefixes[prefix]; ok {
			return false, fmt.Errorf("%w, prefix: %v", ErrDuplicateData, prefix)
		}

		seenPrefixes[prefix] = struct{}{}
		prefixes = append(prefixes, prefix)
		return true, nil
	}

	counter, err := scanTextSource(path, skipLines, handleLine)
	if err != nil {
		return nil, err
	}
	if counter == 0 {
		return nil, fmt.Errorf("%w", ErrNoValidElements)
	}

	return prefixes, nil
}
