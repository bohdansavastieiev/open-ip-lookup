// Package source acts as a registry for the sources expected in the app.
package source

import (
	"maps"
	"time"
)

type ID string

type Definition struct {
	ID               ID
	LocalBaseName    string
	URL              string
	SHA256URL        string
	FreshnessKind    FreshnessKind
	ArtifactKind     ArtifactKind
	AuthKind         AuthKind
	outdatedInterval time.Duration
}

const (
	IANASpecialIPv4              ID = "iana_special_ipv4"
	IANASpecialIPv6              ID = "iana_special_ipv6"
	CymruFullBogonsIPv4          ID = "cymru_fullbogons_ipv4"
	CymruFullBogonsIPv6          ID = "cymru_fullbogons_ipv6"
	MaxMindGeoLite2City          ID = "maxmind_geolite2_city"
	MaxMindGeoLite2ASN           ID = "maxmind_geolite2_asn"
	DanTorExit                   ID = "dan_tor_exit"
	DanTorFull                   ID = "dan_tor_full"
	X4bnetListsVPNVPNIPv4        ID = "x4bnet_lists-vpn_vpn-ipv4"
	X4bnetListsVPNDatacenterIPv4 ID = "x4bnet_lists-vpn_datacenter-ipv4"
	TobilgCloudProviderRanges    ID = "tobilg_public-cloud-provider-ip-ranges_all"
	RezmossCloudProviders        ID = "rezmoss_cloud-provider-ip-addresses_all-providers"
	Az0VPNIP                     ID = "az0_vpn-ip_ip"
	Az0VPNHostname               ID = "az0_vpn-ip_hostname"
	AvastelBotIPsLists1Day       ID = "avastel_bot-ips-lists_1-day"
	AvastelBotIPsLists5Day       ID = "avastel_bot-ips-lists_5-day"
	AvastelBotIPsLists8Day       ID = "avastel_bot-ips-lists_8-day"
	BountyyfiBadASNListAll       ID = "bountyyfi_bad-asn-list_all"
	UmkusIPIndexASNDCs           ID = "umkus_ip-index_asn-dcs"
	IPVerseASIPBlocksAll         ID = "ipverse_as-ip-blocks_all"
	IPVerseASMetadataAll         ID = "ipverse_as-metadata_all"
)

const (
	oneDay    = 24 * time.Hour
	threeDays = 3 * oneDay
	sevenDays = 7 * oneDay
	none      = 0
)

var allDefinitions = []Definition{
	{
		ID:               IANASpecialIPv4,
		LocalBaseName:    "iana_special_ipv4.csv",
		URL:              "https://www.iana.org/assignments/iana-ipv4-special-registry/iana-ipv4-special-registry-1.csv",
		FreshnessKind:    FreshnessKindLastModified,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: none,
	},
	{
		ID:               IANASpecialIPv6,
		LocalBaseName:    "iana_special_ipv6.csv",
		URL:              "https://www.iana.org/assignments/iana-ipv6-special-registry/iana-ipv6-special-registry-1.csv",
		FreshnessKind:    FreshnessKindLastModified,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: none,
	},
	{
		ID:               CymruFullBogonsIPv4,
		LocalBaseName:    "cymru_fullbogons_ipv4.txt",
		URL:              "https://www.team-cymru.org/Services/Bogons/fullbogons-ipv4.txt",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: threeDays,
	},
	{
		ID:               CymruFullBogonsIPv6,
		LocalBaseName:    "cymru_fullbogons_ipv6.txt",
		URL:              "https://www.team-cymru.org/Services/Bogons/fullbogons-ipv6.txt",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: threeDays,
	},
	{
		ID:               MaxMindGeoLite2City,
		LocalBaseName:    "maxmind_geolite2_city.mmdb",
		URL:              "https://download.maxmind.com/geoip/databases/GeoLite2-City/download?suffix=tar.gz",
		SHA256URL:        "https://download.maxmind.com/geoip/databases/GeoLite2-City/download?suffix=tar.gz.sha256",
		FreshnessKind:    FreshnessKindLastModified,
		ArtifactKind:     ArtifactKindTarGzFile,
		AuthKind:         AuthKindMaxMind,
		outdatedInterval: sevenDays,
	},
	{
		ID:               MaxMindGeoLite2ASN,
		LocalBaseName:    "maxmind_geolite2_asn.mmdb",
		URL:              "https://download.maxmind.com/geoip/databases/GeoLite2-ASN/download?suffix=tar.gz",
		SHA256URL:        "https://download.maxmind.com/geoip/databases/GeoLite2-ASN/download?suffix=tar.gz.sha256",
		FreshnessKind:    FreshnessKindLastModified,
		ArtifactKind:     ArtifactKindTarGzFile,
		AuthKind:         AuthKindMaxMind,
		outdatedInterval: sevenDays,
	},
	{
		ID:               DanTorExit,
		LocalBaseName:    "dan_tor_exit.txt",
		URL:              "https://www.dan.me.uk/torlist/?exit",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: threeDays,
	},
	{
		ID:               DanTorFull,
		LocalBaseName:    "dan_tor_full.txt",
		URL:              "https://www.dan.me.uk/torlist/?full",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: threeDays,
	},
	{
		ID:               X4bnetListsVPNVPNIPv4,
		LocalBaseName:    "x4bnet_lists-vpn_vpn-ipv4.txt",
		URL:              "https://raw.githubusercontent.com/X4BNet/lists_vpn/refs/heads/main/output/vpn/ipv4.txt",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: sevenDays,
	},
	{
		ID:               X4bnetListsVPNDatacenterIPv4,
		LocalBaseName:    "x4bnet_lists-vpn_datacenter-ipv4.txt",
		URL:              "https://raw.githubusercontent.com/X4BNet/lists_vpn/refs/heads/main/output/datacenter/ipv4.txt",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: sevenDays,
	},
	{
		ID:               TobilgCloudProviderRanges,
		LocalBaseName:    "tobilg_public-cloud-provider-ip-ranges_all.json",
		URL:              "https://raw.githubusercontent.com/tobilg/public-cloud-provider-ip-ranges/main/data/providers/all.json",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: sevenDays,
	},
	{
		ID:               RezmossCloudProviders,
		LocalBaseName:    "rezmoss_cloud-provider-ip-addresses_all-providers.json",
		URL:              "https://raw.githubusercontent.com/rezmoss/cloud-provider-ip-addresses/refs/heads/main/all_providers/all_providers.json",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: sevenDays,
	},
	{
		ID:               Az0VPNIP,
		LocalBaseName:    "az0_vpn-ip_ip.txt",
		URL:              "https://az0-vpnip-public.oooninja.com/ip.txt",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: sevenDays,
	},
	{
		ID:               Az0VPNHostname,
		LocalBaseName:    "az0_vpn-ip_hostname.txt",
		URL:              "https://az0-vpnip-public.oooninja.com/hostname.txt",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: sevenDays,
	},
	{
		ID:               AvastelBotIPsLists1Day,
		LocalBaseName:    "avastel_bot-ips-lists_1-day.txt",
		URL:              "https://raw.githubusercontent.com/antoinevastel/avastel-bot-ips-lists/refs/heads/master/avastel-proxy-bot-ips-1day.txt",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: sevenDays,
	},
	{
		ID:               AvastelBotIPsLists5Day,
		LocalBaseName:    "avastel_bot-ips-lists_5-day.txt",
		URL:              "https://raw.githubusercontent.com/antoinevastel/avastel-bot-ips-lists/refs/heads/master/avastel-proxy-bot-ips-blocklist-5days.txt",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: sevenDays,
	},
	{
		ID:               AvastelBotIPsLists8Day,
		LocalBaseName:    "avastel_bot-ips-lists_8-day.txt",
		URL:              "https://raw.githubusercontent.com/antoinevastel/avastel-bot-ips-lists/refs/heads/master/avastel-proxy-bot-ips-blocklist-8days.txt",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: sevenDays,
	},
	{
		ID:               BountyyfiBadASNListAll,
		LocalBaseName:    "bountyyfi_bad-asn-list_all.txt",
		URL:              "https://raw.githubusercontent.com/bountyyfi/bad-asn-list/refs/heads/main/all.txt",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: none,
	},
	{
		ID:               UmkusIPIndexASNDCs,
		LocalBaseName:    "umkus_ip-index_asn-dcs.txt",
		URL:              "https://raw.githubusercontent.com/Umkus/ip-index/refs/heads/master/data/asns_dcs.csv",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: sevenDays,
	},
	{
		ID:               IPVerseASIPBlocksAll,
		LocalBaseName:    "ipverse_as-ip-blocks_all",
		URL:              "https://github.com/ipverse/as-ip-blocks/releases/latest/download/as-ip-blocks.tar.gz",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindTarGzDir,
		AuthKind:         AuthKindNone,
		outdatedInterval: sevenDays,
	},
	{
		ID:               IPVerseASMetadataAll,
		LocalBaseName:    "ipverse_as-metadata_all.json",
		URL:              "https://raw.githubusercontent.com/ipverse/as-metadata/refs/heads/master/as.json",
		FreshnessKind:    FreshnessKindETag,
		ArtifactKind:     ArtifactKindDirectFile,
		AuthKind:         AuthKindNone,
		outdatedInterval: sevenDays,
	},
}

var definitionsByID = buildDefinitionsByID(allDefinitions)

func (id ID) IsValid() bool {
	_, ok := Lookup(id)
	return ok
}

func Definitions() []Definition {
	defs := make([]Definition, len(allDefinitions))
	copy(defs, allDefinitions)
	return defs
}

func DefinitionsByID() map[ID]Definition {
	defs := make(map[ID]Definition, len(definitionsByID))
	maps.Copy(defs, definitionsByID)
	return defs
}

func Lookup(id ID) (Definition, bool) {
	def, ok := definitionsByID[id]
	return def, ok
}

func LookupIDByLocalBaseName(baseName string) (ID, bool) {
	for _, def := range allDefinitions {
		if def.LocalBaseName == baseName {
			return def.ID, true
		}
	}
	return "", false
}

func DefinitionFor(id ID) Definition {
	def, ok := Lookup(id)
	if !ok {
		panic("unknown source definition: " + string(id))
	}
	return def
}

func (d Definition) OutdatedInterval() (time.Duration, bool) {
	if d.outdatedInterval <= 0 {
		return 0, false
	}

	return d.outdatedInterval, true
}

func buildDefinitionsByID(allDefinitions []Definition) map[ID]Definition {
	defs := make(map[ID]Definition, len(allDefinitions))
	for _, def := range allDefinitions {
		if !def.FreshnessKind.isValid() {
			panic("invalid source freshness kind: " + string(def.ID))
		}
		if !def.ArtifactKind.isValid() {
			panic("invalid source artifact kind: " + string(def.ID))
		}
		if !def.AuthKind.isValid() {
			panic("invalid source auth kind: " + string(def.ID))
		}
		if _, exists := defs[def.ID]; exists {
			panic("duplicate source definition: " + string(def.ID))
		}
		defs[def.ID] = def
	}
	return defs
}
