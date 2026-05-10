package dataset

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/netip"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
	"github.com/gaissmai/bart"
	"github.com/oschwald/geoip2-golang/v2"
)

type Dataset struct {
	logger    *slog.Logger
	geoReader *geoip2.Reader
	asnReader *geoip2.Reader

	bogons        bart.Table[uint32]
	bogonEntries  []bogonEntry
	prefixes      bart.Table[uint32]
	prefixEntries []prefixEntry
	ips           map[netip.Addr]uint32
	ipEntries     []ipEntry
	asns          map[ASN]asnEntry

	cloudProviders []cloudProviderInfo
	vpnProviders   []string
}

type bogonEntry struct {
	kind IPKind
	iana *ianaInfo
}

type prefixEntry struct {
	flags              IPFlag
	asn                ASN
	cloudProviderIndex uint32
}

type ipEntry struct {
	flags            IPFlag
	vpnProviderIndex uint32
}

type asnEntry struct {
	isDC        bool
	isBad       bool
	handle      string
	description string
	country     string
	countryCode string
	category    string
	networkRole string
}

type cloudProviderInfo struct {
	provider string
	service  string
	region   string
}

func Load(dataDir string, sourceIDs []source.ID, logger *slog.Logger) (*Dataset, error) {
	if err := validateRequiredSourceIDs(sourceIDs); err != nil {
		return nil, err
	}

	snap, err := loadSnapshot(dataDir, sourceIDs, logger)
	if err != nil {
		return nil, err
	}

	if err := validateRequiredSourcesSnapshot(snap); err != nil {
		return nil, err
	}

	ds, err := build(snap, logger)
	if err != nil {
		if closeErr := snap.Close(); closeErr != nil {
			return nil, fmt.Errorf("build: %v; close snapshot: %w", err, closeErr)
		}
		return nil, err
	}

	return ds, nil
}

func (d *Dataset) Close() error {
	if d == nil {
		return nil
	}

	var errs []error
	if d.geoReader != nil {
		if err := d.geoReader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close geo reader: %w", err))
		}
	}
	if d.asnReader != nil {
		if err := d.asnReader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close asn reader: %w", err))
		}
	}

	return errors.Join(errs...)
}

func safeUint32[T ~int | ~uint](v T) uint32 {
	if uint64(v) > math.MaxUint32 {
		panic("integer overflow: dataset limit exceeded")
	}
	return uint32(v)
}
