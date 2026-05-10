package dataset

import (
	"net/netip"

	"github.com/oschwald/geoip2-golang/v2"
)

type IPResult struct {
	IP    netip.Addr
	Kind  IPKind
	Flags []IPFlag

	SpecialUse  *SpecialUseInfo
	Cloud       *CloudInfo
	ASN         *ASNInfo
	Geo         *GeoInfo
	VPNProvider string
}

type SpecialUseInfo struct {
	Name string
	RFC  string
}

type CloudInfo struct {
	Provider string
	Service  string
	Region   string
}

type ASNInfo struct {
	ASN         ASN
	Handle      string
	Description string
	Country     string
	CountryISO  string
	Category    string
	NetworkRole string
	Network     netip.Prefix
}

type GeoInfo struct {
	City       string
	Country    string
	CountryISO string
	Region     string
	Latitude   float64
	Longitude  float64
	Timezone   string
}

func (d *Dataset) Lookup(ip netip.Addr) IPResult {
	result := IPResult{IP: ip}

	if entryIndex, ok := d.bogons.Lookup(ip); ok {
		entry := d.bogonEntries[entryIndex]
		result.Kind = entry.kind
		if entry.iana != nil {
			result.SpecialUse = &SpecialUseInfo{
				Name: entry.iana.name,
				RFC:  entry.iana.rfc,
			}
		}
		return result
	}
	result.Kind = IPKindRoutable

	var allFlags IPFlag

	pfx := netip.PrefixFrom(ip, ip.BitLen())
	for prefix, entryIndex := range d.prefixes.Supernets(pfx) {
		entry := d.prefixEntries[entryIndex]
		allFlags |= entry.flags

		if result.Cloud == nil && entry.cloudProviderIndex != 0 {
			cloud := d.cloudProviders[entry.cloudProviderIndex-1]
			result.Cloud = &CloudInfo{
				Provider: cloud.provider,
				Service:  cloud.service,
				Region:   cloud.region,
			}
		}

		if result.ASN == nil && entry.asn.valid() {
			result.ASN = &ASNInfo{
				ASN:     entry.asn,
				Network: prefix,
			}

			entry, ok := d.asns[result.ASN.ASN]
			if !ok {
				continue
			}

			result.ASN.Handle = entry.handle
			result.ASN.Description = entry.description
			result.ASN.Country = entry.country
			result.ASN.CountryISO = entry.countryCode
			result.ASN.Category = entry.category
			result.ASN.NetworkRole = entry.networkRole

			if entry.isDC {
				allFlags |= IPFlagDatacenter
			}
			if entry.isBad {
				allFlags |= IPFlagLowReputation
			}
		}
	}

	if entryIndex, ok := d.ips[ip]; ok {
		ipEntry := d.ipEntries[entryIndex]
		allFlags |= ipEntry.flags

		if result.VPNProvider == "" && ipEntry.vpnProviderIndex != 0 {
			result.VPNProvider = d.vpnProviders[ipEntry.vpnProviderIndex-1]
		}
	}

	if allFlags&IPFlagProxyHighConf != 0 {
		allFlags &^= IPFlagProxyLowConf
	}

	if d.geoReader != nil {
		geoRecord, err := d.geoReader.City(ip)
		if err == nil {
			result.Geo = &GeoInfo{
				City:       geoRecord.City.Names.English,
				Country:    geoRecord.Country.Names.English,
				CountryISO: geoRecord.Country.ISOCode,
				Region:     firstSubdivision(geoRecord),
				Latitude:   derefFloat64(geoRecord.Location.Latitude),
				Longitude:  derefFloat64(geoRecord.Location.Longitude),
				Timezone:   geoRecord.Location.TimeZone,
			}
		}
	}

	result.Flags = allFlags.ActiveFlags()

	if result.ASN == nil && d.asnReader != nil {
		asnRecord, err := d.asnReader.ASN(ip)
		if err == nil && asnRecord.AutonomousSystemNumber != 0 {
			result.ASN = &ASNInfo{
				ASN:     ASN(safeUint32(asnRecord.AutonomousSystemNumber)),
				Network: asnRecord.Network,
				Handle:  asnRecord.AutonomousSystemOrganization,
			}
		}
	}

	return result
}

func firstSubdivision(record *geoip2.City) string {
	if len(record.Subdivisions) > 0 {
		return record.Subdivisions[0].Names.English
	}
	return ""
}

func derefFloat64(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}
