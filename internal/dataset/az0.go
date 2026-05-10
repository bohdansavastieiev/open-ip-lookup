package dataset

import (
	"fmt"
	"net/netip"
	"strings"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

type hostnameInfo struct {
	hostname  string
	subdomain string
	domain    string
	suffix    string
}

func loadAz0VPNIP(path string) (map[netip.Addr]string, error) {
	providerByAddr := make(map[netip.Addr]string)
	skipLines := 1

	handleLine := func(line string) (bool, error) {
		ipPart, provider := splitAddrAndComment(line)
		if ipPart == "" {
			return false, fmt.Errorf("%w, addr: %q", ErrInvalidAddr, line)
		}

		addr, err := netip.ParseAddr(ipPart)
		if err != nil {
			return false, fmt.Errorf("%w, addr: %q", ErrInvalidAddr, line)
		}

		if _, ok := providerByAddr[addr]; ok {
			return false, fmt.Errorf("%w, addr: %v", ErrDuplicateData, addr)
		}

		providerByAddr[addr] = provider
		return true, nil
	}

	counter, err := scanTextSource(path, skipLines, handleLine)
	if err != nil {
		return nil, err
	}
	if counter == 0 {
		return nil, fmt.Errorf("%w", ErrNoValidElements)
	}

	return providerByAddr, nil
}

func loadAz0VPNHostname(path string) ([]hostnameInfo, error) {
	hostnameInfos := make([]hostnameInfo, 0)
	seenHostnames := make(map[string]struct{})
	skipLines := 2

	handleLine := func(line string) (bool, error) {
		hostnameInfo, err := parseHostnameInfo(line)
		if err != nil {
			return false, err
		}

		if _, ok := seenHostnames[hostnameInfo.hostname]; ok {
			return false, fmt.Errorf("%w, hostname: %v", ErrDuplicateData, hostnameInfo.hostname)
		}

		seenHostnames[hostnameInfo.hostname] = struct{}{}
		hostnameInfos = append(hostnameInfos, hostnameInfo)
		return true, nil
	}

	counter, err := scanTextSource(path, skipLines, handleLine)
	if err != nil {
		return nil, err
	}
	if counter == 0 {
		return nil, fmt.Errorf("%w", ErrNoValidElements)
	}

	return hostnameInfos, nil
}

func parseHostnameInfo(s string) (hostnameInfo, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return hostnameInfo{}, fmt.Errorf("empty hostname")
	}

	if strings.ContainsAny(s, "/\\?#@") || strings.Contains(s, "://") || strings.Contains(s, ":") {
		return hostnameInfo{}, fmt.Errorf("hostname %q contains invalid characters", s)
	}

	s = strings.TrimSuffix(s, ".")
	ascii, err := idna.Lookup.ToASCII(s)
	if err != nil {
		return hostnameInfo{}, fmt.Errorf("hostname %q IDNA conversion failed: %w", s, err)
	}
	ascii = strings.ToLower(ascii)

	if !isDNSHostname(ascii) {
		return hostnameInfo{}, fmt.Errorf("hostname %q is not a valid DNS hostname", s)
	}

	info := hostnameInfo{hostname: ascii}
	info.suffix, _ = publicsuffix.PublicSuffix(ascii)

	etld1, err := publicsuffix.EffectiveTLDPlusOne(ascii)
	if err != nil {
		return hostnameInfo{}, fmt.Errorf("hostname %q has no valid domain: %w", s, err)
	}
	info.domain = strings.TrimSuffix(etld1, "."+info.suffix)
	if ascii != etld1 {
		info.subdomain = strings.TrimSuffix(ascii, "."+etld1)
	}

	return info, nil
}

func isDNSHostname(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}

	labels := strings.Split(s, ".")
	if len(labels) < 2 {
		return false
	}

	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return false
		}

		for i := range len(label) {
			c := label[i]
			isAlphaNum := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
			if !isAlphaNum && c != '-' {
				return false
			}

			if (i == 0 || i == len(label)-1) && c == '-' {
				return false
			}
		}
	}

	return true
}

func splitAddrAndComment(line string) (addr string, comment string) {
	addr = line
	if before, after, ok := strings.Cut(line, "#"); ok {
		addr = strings.TrimSpace(before)
		comment = strings.TrimSpace(after)
	}

	return strings.TrimSpace(addr), comment
}
