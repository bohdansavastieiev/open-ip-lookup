package dataset

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
)

var errIPVerseASNMismatch = errors.New("ASN mismatch")

type ipverseASBlocksRecord struct {
	ASN      ASN                     `json:"asn"`
	Metadata ipverseASBlocksMetadata `json:"metadata"`
	Prefixes ipversePrefixes         `json:"prefixes"`
}

type ipverseASBlocksMetadata struct {
	Handle      *string `json:"handle"`
	Description *string `json:"description"`
	CountryCode *string `json:"countryCode"`
	Country     *string `json:"country"`
	Origin      string  `json:"origin"`
	Category    *string `json:"category"`
	NetworkRole *string `json:"networkRole"`
}

type ipverseASMetadataRecord struct {
	ASN           ASN               `json:"asn"`
	Metadata      ipverseASMetadata `json:"metadata"`
	Stats         *ipverseStats     `json:"stats"`
	LastAnnounced *string           `json:"lastAnnounced"`
}

type ipverseASMetadata struct {
	Handle       string  `json:"handle"`
	Description  string  `json:"description"`
	CountryCode  *string `json:"countryCode"`
	Country      *string `json:"country"`
	Origin       string  `json:"origin"`
	Category     *string `json:"category"`
	NetworkRole  *string `json:"networkRole"`
	Registered   *string `json:"registered"`
	LastModified *string `json:"lastModified"`
}

type ipversePrefixes struct {
	IPv4 []netip.Prefix `json:"ipv4"`
	IPv6 []netip.Prefix `json:"ipv6"`
}

type ipverseStats struct {
	IPv4                 *ipverseFamilyStats `json:"ipv4"`
	IPv6                 *ipverseFamilyStats `json:"ipv6"`
	Connectivity         ipverseConnectivity `json:"connectivity"`
	PrefixesLastModified string              `json:"prefixesLastModified"`
}

type ipverseFamilyStats struct {
	Prefixes           int    `json:"prefixes"`
	PrefixesAggregated int    `json:"prefixesAggregated"`
	LargestPrefix      int    `json:"largestPrefix"`
	TotalAddresses     uint64 `json:"totalAddresses"`
}

type ipverseConnectivity struct {
	Providers    int   `json:"providers"`
	ProviderASNs []ASN `json:"providerAsns"`
	Customers    int   `json:"customers"`
	Peers        int   `json:"peers"`
	Unclassified int   `json:"unclassified"`
	Degree       int   `json:"degree"`
	Reach        int   `json:"reach"`
}

func loadIPVerseASBlocks(path string) (map[ASN]ipverseASBlocksRecord, error) {
	rootDir := filepath.Join(path, "as")
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, fmt.Errorf("read extracted directory: %w", err)
	}

	asBlocksByASN := make(map[ASN]ipverseASBlocksRecord, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			return nil, fmt.Errorf("%w, entry: %q", ErrInvalidASN, entry.Name())
		}
		asn, ok, err := parseOptionalASNNumeric(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("%w, directory: %q", ErrInvalidASN, entry.Name())
		}
		if !ok {
			continue
		}

		aggPath := filepath.Join(rootDir, entry.Name(), "aggregated.json")
		f, err := os.Open(aggPath)
		if err != nil {
			return nil, fmt.Errorf("open JSON source %q: %w", aggPath, err)
		}

		var record ipverseASBlocksRecord
		decodeErr := decodeJSON(f, &record)
		closeErr := f.Close()
		if decodeErr != nil {
			return nil, fmt.Errorf("parse JSON source %q: %w", aggPath, decodeErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close JSON source %q: %w", aggPath, closeErr)
		}

		if record.ASN != asn {
			return nil, fmt.Errorf(
				"%w, directory: %q, expected: %v",
				errIPVerseASNMismatch,
				entry.Name(),
				record.ASN,
			)
		}
		if _, ok := asBlocksByASN[asn]; ok {
			return nil, fmt.Errorf("%w, ASN: %v", ErrDuplicateData, asn)
		}
		if err := normalizeIPVersePrefixes(&record.Prefixes); err != nil {
			return nil, fmt.Errorf("parse JSON source %q: %w", aggPath, err)
		}

		asBlocksByASN[asn] = record
	}
	if len(asBlocksByASN) == 0 {
		return nil, fmt.Errorf("%w", ErrNoValidElements)
	}

	return asBlocksByASN, nil
}

func loadIPVerseASMetadata(path string) (map[ASN]ipverseASMetadataRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open JSON source %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	asMetadataByASN := make(map[ASN]ipverseASMetadataRecord)
	counter, err := decodeJSONArray(f, func(record ipverseASMetadataRecord) error {
		if !record.ASN.valid() {
			return nil
		}
		if _, ok := asMetadataByASN[record.ASN]; ok {
			return fmt.Errorf("%w, ASN: %v", ErrDuplicateData, record.ASN)
		}

		asMetadataByASN[record.ASN] = record
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse JSON source %q: %w", path, err)
	}
	if counter == 0 || len(asMetadataByASN) == 0 {
		return nil, fmt.Errorf("%w", ErrNoValidElements)
	}

	return asMetadataByASN, nil
}

func normalizeIPVersePrefixes(prefixes *ipversePrefixes) error {
	seen := make(map[netip.Prefix]struct{}, len(prefixes.IPv4)+len(prefixes.IPv6))

	for i, prefix := range prefixes.IPv4 {
		prefix = prefix.Masked()
		if !ipVersion4.matchesPrefix(prefix) {
			return fmt.Errorf(
				"%w, prefix: %q, expected: %s",
				ErrUnexpectedIPVersion,
				prefix,
				ipVersion4,
			)
		}
		if _, ok := seen[prefix]; ok {
			return fmt.Errorf("%w, prefix: %v", ErrDuplicateData, prefix)
		}

		prefixes.IPv4[i] = prefix
		seen[prefix] = struct{}{}
	}

	for i, prefix := range prefixes.IPv6 {
		prefix = prefix.Masked()
		if !ipVersion6.matchesPrefix(prefix) {
			return fmt.Errorf(
				"%w, prefix: %q, expected: %s",
				ErrUnexpectedIPVersion,
				prefix,
				ipVersion6,
			)
		}
		if _, ok := seen[prefix]; ok {
			return fmt.Errorf("%w, prefix: %v", ErrDuplicateData, prefix)
		}

		prefixes.IPv6[i] = prefix
		seen[prefix] = struct{}{}
	}

	return nil
}
