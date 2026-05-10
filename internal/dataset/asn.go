package dataset

import (
	"fmt"
	"strconv"
	"strings"
)

type ASN uint32

func (a ASN) valid() bool {
	return a != 0
}

// parseASN parses ASN strings in the AS12345 representation.
func parseASN(s string) (ASN, error) {
	if !strings.HasPrefix(s, "AS") {
		return 0, fmt.Errorf("%w, ASN: %q", ErrInvalidASN, s)
	}

	return parseASNNumeric(s[2:])
}

// parseASNNumeric parses ASN strings in the 12345 representation.
func parseASNNumeric(s string) (ASN, error) {
	asn, ok, err := parseOptionalASNNumeric(s)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("%w, ASN: %q", ErrInvalidASN, s)
	}

	return asn, nil
}

func parseOptionalASNNumeric(s string) (ASN, bool, error) {
	raw, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, false, fmt.Errorf("%w, ASN: %q", ErrInvalidASN, s)
	}

	asn := ASN(raw)
	return asn, asn.valid(), nil
}
