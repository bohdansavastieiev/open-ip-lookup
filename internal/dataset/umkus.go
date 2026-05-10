package dataset

import (
	"fmt"
)

func loadUmkusASNDCs(path string) ([]ASN, error) {
	asns := make([]ASN, 0)
	seenASNs := make(map[ASN]struct{})

	handleLine := func(line string) (bool, error) {
		asn, ok, err := parseOptionalASNNumeric(line)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}

		if _, ok := seenASNs[asn]; ok {
			return false, fmt.Errorf("%w, ASN: %v", ErrDuplicateData, asn)
		}

		seenASNs[asn] = struct{}{}
		asns = append(asns, asn)
		return true, nil
	}

	counter, err := scanTextSource(path, 0, handleLine)
	if err != nil {
		return nil, err
	}
	if counter == 0 {
		return nil, fmt.Errorf("%w", ErrNoValidElements)
	}

	return asns, nil
}
