package dataset

import (
	"fmt"
	"net/netip"
)

func loadDanAddrs(path string) ([]netip.Addr, error) {
	addrs := make([]netip.Addr, 0)
	seenAddrs := make(map[netip.Addr]struct{})

	handleLine := func(line string) (bool, error) {
		addr, err := netip.ParseAddr(line)
		if err != nil {
			return false, fmt.Errorf("%w, addr: %q", ErrInvalidAddr, line)
		}

		if _, ok := seenAddrs[addr]; ok {
			return false, fmt.Errorf("%w, addr: %v", ErrDuplicateData, addr)
		}

		seenAddrs[addr] = struct{}{}
		addrs = append(addrs, addr)

		return true, nil
	}

	counter, err := scanTextSource(path, 0, handleLine)
	if err != nil {
		return nil, err
	}
	if counter == 0 {
		return nil, fmt.Errorf("%w", ErrNoValidElements)
	}

	return addrs, nil
}
