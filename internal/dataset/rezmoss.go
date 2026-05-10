package dataset

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
)

var errRezmossProviderRequired = errors.New("provider must be set")

type rezmossInfo struct {
	provider    string
	service     string
	region      string
	lastUpdated string
}

type rezmossRecord struct {
	Prefix      netip.Prefix `json:"cidr"`
	IPVersion   string       `json:"ip_version"`
	Provider    string       `json:"provider"`
	Service     string       `json:"service"`
	Region      string       `json:"region"`
	LastUpdated string       `json:"last_updated"`
}

func loadRezmossAllProviders(path string) (map[netip.Prefix][]rezmossInfo, error) {
	infoByPrefix := make(map[netip.Prefix][]rezmossInfo)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open JSON source %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	counter, err := decodeJSONArray(f, func(record rezmossRecord) error {
		provider := strings.TrimSpace(record.Provider)
		if provider == "" {
			return errRezmossProviderRequired
		}
		if !record.Prefix.IsValid() {
			return fmt.Errorf("%w, prefix: %q", ErrInvalidPrefix, record.Prefix)
		}
		if err := validateRezmossIPVersion(record); err != nil {
			return err
		}

		prefix := record.Prefix.Masked()
		infoByPrefix[prefix] = append(infoByPrefix[prefix], rezmossInfo{
			provider:    provider,
			service:     record.Service,
			region:      record.Region,
			lastUpdated: record.LastUpdated,
		})

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

func validateRezmossIPVersion(record rezmossRecord) error {
	var want string
	if record.Prefix.Addr().Is4() {
		want = ipVersion4.String()
	} else {
		want = ipVersion6.String()
	}
	if record.IPVersion == want {
		return nil
	}

	return fmt.Errorf(
		"%w, prefix: %q, expected: %s, got: %q",
		ErrUnexpectedIPVersion,
		record.Prefix,
		want,
		record.IPVersion,
	)
}
