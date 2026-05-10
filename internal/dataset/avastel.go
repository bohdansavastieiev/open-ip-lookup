package dataset

import (
	"encoding/csv"
	"fmt"
	"net/netip"
)

type avastelAddrInfo struct {
	org string
}

type avastelPrefixInfo struct {
	org        string
	confidence float32
}

type avastel1DayRecord struct {
	Addr netip.Addr `csv:"ip_address"`
	Org  string     `csv:"autonomous_system"`
}

type avastelMultidayRecord struct {
	Prefix     netip.Prefix `csv:"ip_address"`
	Org        string       `csv:"autonomous_system"`
	Confidence float32      `csv:"confidence"`
}

func loadAvastelInfoByAddr(path string) (map[netip.Addr]avastelAddrInfo, error) {
	infoByAddr := make(map[netip.Addr]avastelAddrInfo)
	skipLines := 4

	handleRecord := func(rec avastel1DayRecord) (bool, error) {
		if !rec.Addr.IsValid() {
			return false, fmt.Errorf("%w, addr: %q", ErrInvalidAddr, rec.Addr)
		}

		info, ok := infoByAddr[rec.Addr]
		if ok {
			return false, fmt.Errorf("%w, addr: %v", ErrDuplicateData, rec.Addr)
		}

		info.org = rec.Org
		infoByAddr[rec.Addr] = info

		return true, nil
	}

	counter, err := scanCSVSource(path, skipLines, configureAvastelCSV, handleRecord)
	if err != nil {
		return nil, err
	}
	if counter == 0 {
		return nil, fmt.Errorf("%w", ErrNoValidElements)
	}

	return infoByAddr, nil
}

func loadAvastelInfoByPrefix(path string) (map[netip.Prefix]avastelPrefixInfo, error) {
	infoByPrefix := make(map[netip.Prefix]avastelPrefixInfo)
	skipLines := 4

	handleRecord := func(rec avastelMultidayRecord) (bool, error) {
		if !rec.Prefix.IsValid() {
			return false, fmt.Errorf("%w, prefix: %q", ErrInvalidPrefix, rec.Prefix)
		}

		prefix := rec.Prefix.Masked()
		info, ok := infoByPrefix[prefix]
		if ok {
			return false, fmt.Errorf("%w, prefix: %v", ErrDuplicateData, prefix)
		}

		info.org = rec.Org
		info.confidence = rec.Confidence
		infoByPrefix[prefix] = info

		return true, nil
	}

	counter, err := scanCSVSource(path, skipLines, configureAvastelCSV, handleRecord)
	if err != nil {
		return nil, err
	}
	if counter == 0 {
		return nil, fmt.Errorf("%w", ErrNoValidElements)
	}

	return infoByPrefix, nil
}

func configureAvastelCSV(r *csv.Reader) {
	r.Comma = ';'
}
