// Package report shapes dataset results for display.
package report

import (
	"net/netip"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/dataset"
)

const MaxIPs = 500

type Report struct {
	Stats   Stats   `json:"stats"`
	Entries []Entry `json:"entries"`
}

type Stats struct {
	Total    int `json:"total"`
	Unique   int `json:"unique"`
	Reported int `json:"reported"`
}

type Entry struct {
	IP          string      `json:"ip"`
	Occurrences int         `json:"occurrences"`
	IsIPv6      bool        `json:"isIpv6"`
	Kind        string      `json:"kind"`
	Flags       []string    `json:"flags"`
	HasDetails  bool        `json:"hasDetails"`
	SpecialUse  *SpecialUse `json:"specialUse,omitempty"`
	Geo         *Geo        `json:"geo,omitempty"`
	ASN         *ASN        `json:"asn,omitempty"`
	Cloud       *Cloud      `json:"cloud,omitempty"`
	VPNProvider string      `json:"vpnProvider,omitempty"`
}

type SpecialUse struct {
	Name string `json:"name"`
	RFC  string `json:"rfc"`
}

type Geo struct {
	City         string  `json:"city,omitempty"`
	Country      string  `json:"country,omitempty"`
	CountryISO   string  `json:"countryIso,omitempty"`
	CountryEmoji string  `json:"countryEmoji,omitempty"`
	Region       string  `json:"region,omitempty"`
	HasLocation  bool    `json:"hasLocation"`
	Latitude     float64 `json:"latitude,omitempty"`
	Longitude    float64 `json:"longitude,omitempty"`
	Timezone     string  `json:"timezone,omitempty"`
}

type ASN struct {
	Number         uint32   `json:"number"`
	Organization   string   `json:"organization,omitempty"`
	RegistryHandle string   `json:"registryHandle,omitempty"`
	Country        string   `json:"country,omitempty"`
	CountryISO     string   `json:"countryIso,omitempty"`
	Category       string   `json:"category,omitempty"`
	NetworkRole    string   `json:"networkRole,omitempty"`
	Network        *Network `json:"network,omitempty"`
}

type Network struct {
	Prefix  string `json:"prefix"`
	FirstIP string `json:"firstIp"`
	LastIP  string `json:"lastIp"`
}

type Cloud struct {
	Provider string `json:"provider"`
	Service  string `json:"service,omitempty"`
	Region   string `json:"region,omitempty"`
}

func Get(raw string, ds *dataset.Dataset) *Report {
	ips := parse(raw)
	unique := dedup(ips)
	counts := countOccurrences(ips)

	reported := min(len(unique), MaxIPs)

	entries := make([]Entry, 0, reported)
	for _, ip := range unique[:reported] {
		entries = append(entries, buildEntry(ip, counts[ip], ds.Lookup(ip)))
	}

	return &Report{
		Stats: Stats{
			Total:    len(ips),
			Unique:   len(unique),
			Reported: reported,
		},
		Entries: entries,
	}
}

func buildEntry(ip netip.Addr, occurrences int, r dataset.IPResult) Entry {
	e := Entry{
		IP:          ip.String(),
		Occurrences: occurrences,
		IsIPv6:      ip.Is6(),
		Kind:        r.Kind.Label(),
	}
	if r.Kind == dataset.IPKindRoutable {
		e.Flags = flagLabels(r.Flags)
	}

	if r.SpecialUse != nil {
		e.SpecialUse = &SpecialUse{
			Name: r.SpecialUse.Name,
			RFC:  r.SpecialUse.RFC,
		}
	}

	if hasGeo(r.Geo) {
		e.Geo = &Geo{
			City:         r.Geo.City,
			Country:      r.Geo.Country,
			CountryISO:   r.Geo.CountryISO,
			CountryEmoji: flagEmoji(r.Geo.CountryISO),
			Region:       r.Geo.Region,
			HasLocation:  r.Geo.Latitude != 0 || r.Geo.Longitude != 0,
			Latitude:     r.Geo.Latitude,
			Longitude:    r.Geo.Longitude,
			Timezone:     r.Geo.Timezone,
		}
	}

	if r.ASN != nil && r.ASN.ASN != 0 {
		e.ASN = &ASN{
			Number:         uint32(r.ASN.ASN),
			Organization:   asnOrganization(r.ASN),
			RegistryHandle: asnRegistryHandle(r.ASN),
			Country:        r.ASN.Country,
			CountryISO:     r.ASN.CountryISO,
			Category:       humanize(r.ASN.Category),
			NetworkRole:    humanize(r.ASN.NetworkRole),
			Network:        networkInfo(r.ASN.Network),
		}
	}

	if r.Cloud != nil {
		e.Cloud = &Cloud{
			Provider: r.Cloud.Provider,
			Service:  r.Cloud.Service,
			Region:   r.Cloud.Region,
		}
	}

	e.VPNProvider = r.VPNProvider
	e.HasDetails = e.Geo != nil || e.ASN != nil || e.Cloud != nil ||
		e.VPNProvider != "" || len(e.Flags) > 0

	return e
}

func flagLabels(flags []dataset.IPFlag) []string {
	result := make([]string, 0, len(flags))
	for _, f := range flags {
		label := f.Label()
		if label == "" {
			continue
		}
		result = append(result, label)
	}
	return result
}

func hasGeo(g *dataset.GeoInfo) bool {
	return g != nil && (g.City != "" || g.Country != "" || g.CountryISO != "" ||
		g.Region != "" || g.Timezone != "" || g.Latitude != 0 || g.Longitude != 0)
}

func asnOrganization(a *dataset.ASNInfo) string {
	if a.Description != "" {
		return a.Description
	}
	return a.Handle
}

func asnRegistryHandle(a *dataset.ASNInfo) string {
	if a.Description == "" {
		return ""
	}
	return a.Handle
}

func networkInfo(prefix netip.Prefix) *Network {
	if !prefix.IsValid() {
		return nil
	}

	prefix = prefix.Masked()
	return &Network{
		Prefix:  prefix.String(),
		FirstIP: prefix.Addr().String(),
		LastIP:  lastIP(prefix).String(),
	}
}

func lastIP(prefix netip.Prefix) netip.Addr {
	addr := prefix.Addr()
	if addr.Is4() {
		return lastIPv4(addr, prefix.Bits())
	}
	return lastIPv6(addr, prefix.Bits())
}

func lastIPv4(addr netip.Addr, prefixBits int) netip.Addr {
	b := addr.As4()
	for bit := prefixBits; bit < 32; bit++ {
		b[bit/8] |= 1 << (7 - bit%8)
	}
	return netip.AddrFrom4(b)
}

func lastIPv6(addr netip.Addr, prefixBits int) netip.Addr {
	b := addr.As16()
	for bit := prefixBits; bit < 128; bit++ {
		b[bit/8] |= 1 << (7 - bit%8)
	}
	return netip.AddrFrom16(b)
}

func parse(raw string) []netip.Addr {
	entries := strings.FieldsFunc(raw, func(c rune) bool {
		return !isIPCandidateRune(c)
	})

	ips := make([]netip.Addr, 0, len(entries))
	for _, e := range entries {
		addr, err := netip.ParseAddr(e)
		if err != nil {
			continue
		}
		ips = append(ips, addr)
	}

	return ips
}

func isIPCandidateRune(c rune) bool {
	return c == '.' || c == ':' || c >= '0' && c <= '9' ||
		c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func dedup(ips []netip.Addr) []netip.Addr {
	seen := make(map[netip.Addr]struct{}, len(ips))
	result := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		if _, ok := seen[ip]; !ok {
			seen[ip] = struct{}{}
			result = append(result, ip)
		}
	}
	return result
}

func countOccurrences(ips []netip.Addr) map[netip.Addr]int {
	result := make(map[netip.Addr]int, len(ips))
	for _, ip := range ips {
		result[ip]++
	}
	return result
}

func flagEmoji(code string) string {
	if len(code) != 2 {
		return ""
	}
	c0, c1 := code[0], code[1]
	if c0 < 'A' || c0 > 'Z' || c1 < 'A' || c1 > 'Z' {
		return ""
	}
	return string([]rune{0x1F1E6 + rune(c0-'A'), 0x1F1E6 + rune(c1-'A')})
}

func humanize(s string) string {
	if s == "" {
		return s
	}

	s = strings.ReplaceAll(s, "_", " ")
	var buf strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsLetter(rune(s[i-1])) && unicode.IsDigit(r) {
			buf.WriteRune(' ')
		}
		buf.WriteRune(r)
	}

	s = buf.String()
	words := strings.Split(s, " ")
	for i, w := range words {
		if len(w) == 0 {
			continue
		}
		r, size := utf8.DecodeRuneInString(w)
		words[i] = string(unicode.ToUpper(r)) + w[size:]
	}
	s = strings.Join(words, " ")

	s = strings.ReplaceAll(s, "Isp", "ISP")

	return s
}
