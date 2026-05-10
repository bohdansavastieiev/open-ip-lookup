package dataset

import (
	"fmt"
	"net/netip"
)

func loadX4bnetPrefixes(path string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0)
	seenPrefixes := make(map[netip.Prefix]struct{})

	handleLine := func(line string) (bool, error) {
		prefix, err := netip.ParsePrefix(line)
		if err != nil {
			return false, fmt.Errorf("%w, prefix: %q", ErrInvalidPrefix, line)
		}
		prefix = prefix.Masked()
		if !ipVersion4.matchesPrefix(prefix) {
			return false, fmt.Errorf(
				"%w, prefix: %q, expected: %s",
				ErrUnexpectedIPVersion,
				line,
				ipVersion4,
			)
		}

		if _, ok := seenPrefixes[prefix]; ok {
			return false, fmt.Errorf("%w, prefix: %v", ErrDuplicateData, prefix)
		}

		seenPrefixes[prefix] = struct{}{}
		prefixes = append(prefixes, prefix)

		return true, nil
	}

	counter, err := scanTextSource(path, 0, handleLine)
	if err != nil {
		return nil, err
	}
	if counter == 0 {
		return nil, fmt.Errorf("%w", ErrNoValidElements)
	}

	return prefixes, nil
}
