package dataset

import (
	"fmt"
	"net/netip"
	"os"
)

type cloudInfo struct {
	provider string
	region   string
}

type tobilgRecord struct {
	Prefix   netip.Prefix `json:"cidr_block"`
	Provider string       `json:"cloud_provider"`
	Region   string       `json:"region"`
}

func loadTobilgCloud(path string) (map[netip.Prefix]cloudInfo, error) {
	infoByPrefix := make(map[netip.Prefix]cloudInfo)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open JSON source %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	counter, err := decodeJSONArray(f, func(record tobilgRecord) error {
		if !record.Prefix.IsValid() {
			return fmt.Errorf("%w, prefix: %q", ErrInvalidPrefix, record.Prefix)
		}

		prefix := record.Prefix.Masked()

		if _, ok := infoByPrefix[prefix]; ok {
			return fmt.Errorf("%w, prefix: %v", ErrDuplicateData, prefix)
		}

		infoByPrefix[prefix] = cloudInfo{
			provider: record.Provider,
			region:   record.Region,
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse JSON source %q: %w", path, err)
	}
	if counter == 0 {
		return nil, fmt.Errorf("%w", ErrNoValidElements)
	}

	return infoByPrefix, nil
}
