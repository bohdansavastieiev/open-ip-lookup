package dataset

import (
	"fmt"
	"strings"
)

func loadBountyyfiBadASN(path string) (map[ASN][]string, error) {
	highRiskOrgByASN := make(map[ASN][]string)

	handleLine := func(line string) (bool, error) {
		parts := strings.Split(line, " ")
		asn, err := parseASN(parts[0])
		if err != nil {
			return false, fmt.Errorf("%w, entry: %q", ErrInvalidASN, line)
		}

		if len(parts) == 1 {
			highRiskOrgByASN[asn] = nil
			return true, nil
		}

		highRiskOrgByASN[asn] = append(highRiskOrgByASN[asn], strings.Join(parts[1:], " "))
		return true, nil
	}

	counter, err := scanTextSource(path, 0, handleLine)
	if err != nil {
		return nil, err
	}
	if counter == 0 {
		return nil, fmt.Errorf("%w", ErrNoValidElements)
	}

	return highRiskOrgByASN, nil
}
