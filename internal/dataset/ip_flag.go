package dataset

type IPFlag int

const (
	IPFlagVPN IPFlag = 1 << iota
	IPFlagDatacenter
	IPFlagPossibleDatacenter
	IPFlagTorExit
	IPFlagTorRelay
	IPFlagProxyHighConf
	IPFlagProxyLowConf
	IPFlagHighRiskASN
)

var ipFlagDefs = []struct {
	flag  IPFlag
	label string
}{
	{IPFlagVPN, "VPN"},
	{IPFlagDatacenter, "Datacenter"},
	{IPFlagPossibleDatacenter, "Possible Datacenter"},
	{IPFlagTorExit, "Tor Exit"},
	{IPFlagTorRelay, "Tor Relay"},
	{IPFlagProxyHighConf, "Proxy"},
	{IPFlagProxyLowConf, "Possible Proxy"},
	{IPFlagHighRiskASN, "High-Risk ASN"},
}

func (f IPFlag) Label() string {
	for _, d := range ipFlagDefs {
		if f == d.flag {
			return d.label
		}
	}
	return ""
}

func (f IPFlag) ActiveFlags() []IPFlag {
	var result []IPFlag
	for _, d := range ipFlagDefs {
		if f&d.flag != 0 {
			result = append(result, d.flag)
		}
	}
	return result
}
