package dataset

import (
	"log/slog"
	"net/netip"
	"strings"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
	"github.com/gaissmai/bart"
)

type builder struct {
	ds        Dataset
	snap      snapshot
	cloudSeen map[cloudProviderInfo]uint32
	vpnSeen   map[string]uint32
}

func build(snap snapshot, logger *slog.Logger) (*Dataset, error) {
	b := builder{
		ds: Dataset{
			logger:        logger,
			bogons:        bart.Table[uint32]{},
			bogonEntries:  make([]bogonEntry, 0, 128),
			prefixes:      bart.Table[uint32]{},
			prefixEntries: make([]prefixEntry, 0, 256),
			ips:           make(map[netip.Addr]uint32),
			ipEntries:     make([]ipEntry, 0, 256),
			asns:          make(map[ASN]asnEntry),
		},
		snap:      snap,
		cloudSeen: make(map[cloudProviderInfo]uint32),
		vpnSeen:   make(map[string]uint32),
	}

	b.assignMaxmindReaders()
	b.buildASNMap()
	b.buildBogonTable()
	b.buildPrefixTable()
	b.buildIPMap()

	logger.Info("built dataset",
		slog.Int("asns", len(b.ds.asns)),
		slog.Int("bogons", len(b.ds.bogonEntries)),
		slog.Int("prefixes", len(b.ds.prefixEntries)),
		slog.Int("ips", len(b.ds.ipEntries)),
	)

	return &b.ds, nil
}

func (b *builder) assignMaxmindReaders() {
	b.ds.geoReader, _ = extractReader(b.snap, source.MaxMindGeoLite2City)
	b.ds.asnReader, _ = extractReader(b.snap, source.MaxMindGeoLite2ASN)
}

func (b *builder) buildASNMap() {
	if entry, ok := b.snap[source.IPVerseASMetadataAll]; ok {
		for asn, record := range entry.(map[ASN]ipverseASMetadataRecord) {
			e := b.ds.asns[asn]
			e.handle = record.Metadata.Handle
			e.description = record.Metadata.Description
			e.country = derefStr(record.Metadata.Country)
			e.countryCode = derefStr(record.Metadata.CountryCode)
			e.category = derefStr(record.Metadata.Category)
			e.networkRole = derefStr(record.Metadata.NetworkRole)
			b.ds.asns[asn] = e
		}
	}

	if entry, ok := b.snap[source.UmkusIPIndexASNDCs]; ok {
		for _, asn := range entry.([]ASN) {
			e := b.ds.asns[asn]
			e.isDC = true
			b.ds.asns[asn] = e
		}
	}

	if entry, ok := b.snap[source.BountyyfiBadASNListAll]; ok {
		for asn := range entry.(map[ASN][]string) {
			e := b.ds.asns[asn]
			e.isBad = true
			b.ds.asns[asn] = e
		}
	}
}

func (b *builder) buildBogonTable() {
	b.insertBogonSource(source.IANASpecialIPv4)
	b.insertBogonSource(source.IANASpecialIPv6)
	b.insertBogonSource(source.CymruFullBogonsIPv4)
	b.insertBogonSource(source.CymruFullBogonsIPv6)
}

func (b *builder) insertBogonSource(sourceID source.ID) {
	entry, ok := b.snap[sourceID]
	if !ok {
		return
	}

	switch sourceID {
	case source.IANASpecialIPv4, source.IANASpecialIPv6:
		for prefix, info := range entry.(map[netip.Prefix]ianaInfo) {
			b.ds.bogonEntries = append(b.ds.bogonEntries, bogonEntry{kind: IPKindSpecialUse, iana: &info})
			b.ds.bogons.Insert(prefix, safeUint32(len(b.ds.bogonEntries)-1))
		}
	case source.CymruFullBogonsIPv4, source.CymruFullBogonsIPv6:
		for _, prefix := range entry.([]netip.Prefix) {
			if hasSupernet(&b.ds.bogons, prefix) {
				continue
			}
			b.ds.bogonEntries = append(b.ds.bogonEntries, bogonEntry{kind: IPKindUnallocated})
			b.ds.bogons.Insert(prefix, safeUint32(len(b.ds.bogonEntries)-1))
		}
	}
}

func hasSupernet(table *bart.Table[uint32], prefix netip.Prefix) bool {
	for range table.Supernets(prefix) {
		return true
	}
	return false
}

func (b *builder) buildPrefixTable() {
	b.buildVPN()
	b.buildDatacenterFlags()
	b.buildCloudProviders()
	b.buildBotPrefixes()
	b.buildASNPrefixes()
}

func (b *builder) buildVPN() {
	if entry, ok := b.snap[source.X4bnetListsVPNVPNIPv4]; ok {
		for _, prefix := range entry.([]netip.Prefix) {
			b.insertPrefix(prefix, prefixEntry{flags: IPFlagVPN})
		}
	}
}

func (b *builder) buildDatacenterFlags() {
	if entry, ok := b.snap[source.X4bnetListsVPNDatacenterIPv4]; ok {
		for _, prefix := range entry.([]netip.Prefix) {
			b.insertPrefix(prefix, prefixEntry{flags: IPFlagDatacenter})
		}
	}
}

func (b *builder) buildCloudProviders() {
	if entry, ok := b.snap[source.RezmossCloudProviders]; ok {
		for prefix, infos := range entry.(map[netip.Prefix][]rezmossInfo) {
			if len(infos) == 0 {
				continue
			}
			first := infos[0]
			cloudProviderIndex := b.addCloudProvider(cloudProviderInfo{
				provider: first.provider,
				service:  first.service,
				region:   first.region,
			})
			b.insertPrefix(prefix, prefixEntry{cloudProviderIndex: cloudProviderIndex})
		}
	}

	if entry, ok := b.snap[source.TobilgCloudProviderRanges]; ok {
		for prefix, info := range entry.(map[netip.Prefix]cloudInfo) {
			cloudProviderIndex := b.addCloudProvider(cloudProviderInfo{
				provider: info.provider,
				region:   info.region,
			})
			b.insertPrefix(prefix, prefixEntry{
				flags:              IPFlagDatacenter,
				cloudProviderIndex: cloudProviderIndex,
			})
		}
	}
}

func (b *builder) buildBotPrefixes() {
	if entry, ok := b.snap[source.AvastelBotIPsLists5Day]; ok {
		for prefix := range entry.(map[netip.Prefix]avastelPrefixInfo) {
			b.insertPrefix(prefix, prefixEntry{flags: IPFlagProxyHighConf})
		}
	}

	if entry, ok := b.snap[source.AvastelBotIPsLists8Day]; ok {
		for prefix := range entry.(map[netip.Prefix]avastelPrefixInfo) {
			b.insertPrefix(prefix, prefixEntry{flags: IPFlagProxyHighConf})
		}
	}
}

func (b *builder) buildASNPrefixes() {
	if entry, ok := b.snap[source.IPVerseASIPBlocksAll]; ok {
		for asn, record := range entry.(map[ASN]ipverseASBlocksRecord) {
			prefixes := make([]netip.Prefix, 0, len(record.Prefixes.IPv4)+len(record.Prefixes.IPv6))
			prefixes = append(prefixes, record.Prefixes.IPv4...)
			prefixes = append(prefixes, record.Prefixes.IPv6...)
			for _, prefix := range prefixes {
				b.insertPrefix(prefix, prefixEntry{asn: asn})
			}
		}
	}
}

func (b *builder) buildIPMap() {
	b.buildTorRelays()
	b.buildTorExits()
	b.buildVPNIPs()
	b.buildBotIPs()
}

func (b *builder) buildTorRelays() {
	if entry, ok := b.snap[source.DanTorFull]; ok {
		for _, addr := range entry.([]netip.Addr) {
			b.insertIP(addr, ipEntry{flags: IPFlagTorRelay})
		}
	}
}

func (b *builder) buildTorExits() {
	if entry, ok := b.snap[source.DanTorExit]; ok {
		for _, addr := range entry.([]netip.Addr) {
			if entryIndex, ok := b.ds.ips[addr]; ok {
				flags := b.ds.ipEntries[entryIndex].flags
				b.ds.ipEntries[entryIndex].flags = (flags & ^IPFlagTorRelay) | IPFlagTorExit
			} else {
				b.insertIP(addr, ipEntry{flags: IPFlagTorExit})
			}
		}
	}
}

func (b *builder) buildVPNIPs() {
	if entry, ok := b.snap[source.Az0VPNIP]; ok {
		for addr, provider := range entry.(map[netip.Addr]string) {
			vpnProviderIndex := b.addVPNProvider(provider)
			b.insertIP(addr, ipEntry{flags: IPFlagVPN, vpnProviderIndex: vpnProviderIndex})
		}
	}
}

func (b *builder) buildBotIPs() {
	if entry, ok := b.snap[source.AvastelBotIPsLists1Day]; ok {
		for addr := range entry.(map[netip.Addr]avastelAddrInfo) {
			b.insertIP(addr, ipEntry{flags: IPFlagProxyLowConf})
		}
	}
}

func (b *builder) addCloudProvider(info cloudProviderInfo) uint32 {
	info.provider = normalizeCloudProvider(info.provider)
	if cloudProviderIndex, ok := b.cloudSeen[info]; ok {
		return cloudProviderIndex
	}
	cloudProviderIndex := safeUint32(len(b.ds.cloudProviders)) + 1
	b.ds.cloudProviders = append(b.ds.cloudProviders, info)
	b.cloudSeen[info] = cloudProviderIndex
	return cloudProviderIndex
}

func (b *builder) addVPNProvider(provider string) uint32 {
	provider = normalizeVPNProvider(provider)
	if vpnProviderIndex, ok := b.vpnSeen[provider]; ok {
		return vpnProviderIndex
	}
	vpnProviderIndex := safeUint32(len(b.ds.vpnProviders)) + 1
	b.ds.vpnProviders = append(b.ds.vpnProviders, provider)
	b.vpnSeen[provider] = vpnProviderIndex
	return vpnProviderIndex
}

var cloudProviderNames = map[string]string{
	"aws":                 "AWS",
	"azure":               "Azure",
	"cloudflare":          "Cloudflare",
	"digitalocean":        "DigitalOcean",
	"fastly":              "Fastly",
	"googlecloud":         "Google Cloud",
	"linode":              "Linode",
	"oracle":              "Oracle",
	"vultr":               "Vultr",
	"zoom":                "Zoom",
	"amazonbot":           "Amazonbot",
	"applebot":            "Applebot",
	"apple_private_relay": "Apple Private Relay",
	"atlassian":           "Atlassian",
	"bingbot":             "Bingbot",
	"commoncrawl":         "Common Crawl",
	"duckduckbot":         "DuckDuckBot",
	"github":              "GitHub",
	"googlebot":           "Googlebot",
	"gptbot":              "GPTBot",
	"perplexitybot":       "Perplexitybot",
	"telegram":            "Telegram",
}

var vpnProviderNames = map[string]string{
	"windscribe":    "Windscribe",
	"nordvpn":       "NordVPN",
	"expressvpn":    "ExpressVPN",
	"cyberghost":    "CyberGhost",
	"pia":           "PIA",
	"hide.me":       "Hide.me",
	"hideme":        "Hide.me",
	"hola":          "Hola",
	"browsec":       "Browsec",
	"protonvpn.com": "ProtonVPN",
	"protonvpn.net": "ProtonVPN",
	"proton.me":     "Proton",
}

func normalizeCloudProvider(name string) string {
	if display, ok := cloudProviderNames[name]; ok {
		return display
	}
	return capitalize(name)
}

func normalizeVPNProvider(name string) string {
	if display, ok := vpnProviderNames[name]; ok {
		return display
	}
	return capitalize(name)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (b *builder) insertPrefix(prefix netip.Prefix, entry prefixEntry) {
	b.ds.prefixes.Modify(prefix, func(old uint32, exists bool) (uint32, bool) {
		if !exists {
			entryIndex := safeUint32(len(b.ds.prefixEntries))
			b.ds.prefixEntries = append(b.ds.prefixEntries, entry)
			return entryIndex, false
		}
		b.mergePrefixEntry(old, entry)
		return old, false
	})
}

func (b *builder) mergePrefixEntry(entryIndex uint32, newEntry prefixEntry) {
	entry := &b.ds.prefixEntries[entryIndex]
	entry.flags |= newEntry.flags
	if !entry.asn.valid() && newEntry.asn.valid() {
		entry.asn = newEntry.asn
	}
	if entry.cloudProviderIndex == 0 && newEntry.cloudProviderIndex != 0 {
		entry.cloudProviderIndex = newEntry.cloudProviderIndex
	}
}

func (b *builder) insertIP(addr netip.Addr, entry ipEntry) {
	if entryIndex, ok := b.ds.ips[addr]; ok {
		existing := &b.ds.ipEntries[entryIndex]
		existing.flags |= entry.flags
		if existing.vpnProviderIndex == 0 && entry.vpnProviderIndex != 0 {
			existing.vpnProviderIndex = entry.vpnProviderIndex
		}
		return
	}
	entryIndex := safeUint32(len(b.ds.ipEntries))
	b.ds.ipEntries = append(b.ds.ipEntries, entry)
	b.ds.ips[addr] = entryIndex
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
