package dataset

import "github.com/oschwald/geoip2-golang/v2"

// Sanctions flags, from strongest to weakest:
//   - OFAC: territories under comprehensive OFAC sanctions (Cuba, Iran, North Korea, Crimea,
//     so-called DNR and LNR).
//   - Possible OFAC: IPs that may be in such a territory, including other occupied parts of Ukraine.
//   - Sanctioned Country: countries under broad targeted US sanctions.
//
// An IP keeps only the strongest one. The lists are intentionally broad: over-flagging is
// preferred to missing a sanctioned IP.

var ofacCountryCodes = map[string]struct{}{
	"CU": {},
	"IR": {},
	"KP": {},
}

var sanctionedCountryCodes = map[string]struct{}{
	"BY": {},
	"MM": {},
	"RU": {},
	"VE": {},
}

type ofacRegion struct {
	flag      IPFlag
	cityFlags map[string]IPFlag
}

// ofacUkraineRegions is keyed by GeoLite2 subdivision ISO code. GeoLite2 has no boundary for
// the DNR and LNR, so only cities inside the pre-2022 line are OFAC; the rest of those oblasts
// is Possible OFAC. In Zaporizhzhia and Kherson oblasts the Ukrainian-held capitals are excluded.
var ofacUkraineRegions = map[string]ofacRegion{
	"43": {flag: IPFlagOFAC}, // Crimea
	"40": {flag: IPFlagOFAC}, // Sevastopol
	"14": {
		flag: IPFlagPossibleOFAC, // Donetsk oblast
		cityFlags: map[string]IPFlag{
			"Amvrosiyivka": IPFlagOFAC,
			"Chystyakove":  IPFlagOFAC,
			"Donetsk":      IPFlagOFAC,
			"Horlivka":     IPFlagOFAC,
			"Makiyivka":    IPFlagOFAC,
			"Shakhtarsk":   IPFlagOFAC,
			"Yasynuvata":   IPFlagOFAC,
			"Zhdanovka":    IPFlagOFAC,
		},
	},
	"09": {
		flag: IPFlagPossibleOFAC, // Luhansk oblast
		cityFlags: map[string]IPFlag{
			"Antratsit": IPFlagOFAC,
			"Luhansk":   IPFlagOFAC,
			"Sorokyne":  IPFlagOFAC,
			"Zorynsk":   IPFlagOFAC,
		},
	},
	"23": {
		flag:      IPFlagPossibleOFAC, // Zaporizhzhia oblast
		cityFlags: map[string]IPFlag{"Zaporizhzhya": 0},
	},
	"65": {
		flag:      IPFlagPossibleOFAC, // Kherson oblast
		cityFlags: map[string]IPFlag{"Kherson": 0},
	},
}

// ofacASNFlags lists operators in occupied Ukraine. Their networks are usually registered and
// geolocated as Russia, so the ASN is the only signal that places them.
var ofacASNFlags = map[ASN]IPFlag{
	6789:   IPFlagOFAC,         // Crelcom, Crimea
	8654:   IPFlagOFAC,         // Crimeainfocom
	21445:  IPFlagOFAC,         // Miranda-Media, Crimea
	28761:  IPFlagOFAC,         // CrimeaCom South
	39047:  IPFlagOFAC,         // KerchNet, Crimea
	41269:  IPFlagOFAC,         // CrimeaTechnology
	43222:  IPFlagOFAC,         // Crimea-IX
	44139:  IPFlagOFAC,         // Simferopol International Airport
	47203:  IPFlagOFAC,         // Miranda-Media, Crimea
	47939:  IPFlagOFAC,         // YaltaNet, Crimea
	50140:  IPFlagOFAC,         // Yalta-AS, Crimea
	57093:  IPFlagOFAC,         // Yalta-TV KOM, Crimea
	59744:  IPFlagOFAC,         // CrimeaNet
	59833:  IPFlagOFAC,         // Sevastopol Telekom
	196665: IPFlagOFAC,         // CrimeaCom Free
	198899: IPFlagOFAC,         // Soda Crimea Plant
	200007: IPFlagOFAC,         // Sevastopol State University
	200441: IPFlagOFAC,         // CrimeaDC
	201776: IPFlagOFAC,         // Miranda-Media, Crimea
	203451: IPFlagOFAC,         // K-Telecom, Crimea
	204791: IPFlagOFAC,         // Starlink Crimea
	208090: IPFlagOFAC,         // ACOSC Crimea
	211245: IPFlagOFAC,         // Crimea Systemenergy
	215167: IPFlagOFAC,         // Krymtelecom
	39089:  IPFlagOFAC,         // Ugletelecom, DNR
	199827: IPFlagOFAC,         // Komtel, DNR
	202279: IPFlagOFAC,         // Republican Telecom Operator, DNR
	204108: IPFlagOFAC,         // Republic Operator of Networks, DNR
	206810: IPFlagOFAC,         // Ugletelecom, DNR
	211692: IPFlagOFAC,         // Level-Donetsk
	214721: IPFlagOFAC,         // Phoenix, DNR
	29031:  IPFlagOFAC,         // Lugansk Telephone Company
	39529:  IPFlagOFAC,         // Matrix Alchevsk, LNR
	39728:  IPFlagOFAC,         // Luganet
	43201:  IPFlagOFAC,         // Telematika, formerly Lugacom, LNR
	59823:  IPFlagOFAC,         // Matrix Alchevsk, LNR
	208890: IPFlagOFAC,         // Lugansk Communications, LNR
	213203: IPFlagPossibleOFAC, // Mir Telecom, occupied south
	215503: IPFlagPossibleOFAC, // Sea Telecom, Melitopol
	215654: IPFlagPossibleOFAC, // Genichesk Online, Kherson oblast
}

func geoSanctionsFlags(record *geoip2.City) IPFlag {
	var flags IPFlag
	if _, ok := ofacCountryCodes[record.Country.ISOCode]; ok {
		flags |= IPFlagOFAC
	}
	if _, ok := ofacCountryCodes[record.RegisteredCountry.ISOCode]; ok {
		flags |= IPFlagPossibleOFAC
	}
	if _, ok := sanctionedCountryCodes[record.Country.ISOCode]; ok {
		flags |= IPFlagSanctionedCountry
	}
	if record.Country.ISOCode == "UA" && len(record.Subdivisions) > 0 {
		flags |= ukraineRegionFlag(record.Subdivisions[0].ISOCode, record.City.Names.English)
	}
	return flags
}

func ukraineRegionFlag(regionCode, city string) IPFlag {
	region, ok := ofacUkraineRegions[regionCode]
	if !ok {
		return 0
	}
	if flag, ok := region.cityFlags[city]; ok {
		return flag
	}
	return region.flag
}

func strongestSanctionsFlag(flags IPFlag) IPFlag {
	switch {
	case flags&IPFlagOFAC != 0:
		return flags &^ (IPFlagPossibleOFAC | IPFlagSanctionedCountry)
	case flags&IPFlagPossibleOFAC != 0:
		return flags &^ IPFlagSanctionedCountry
	default:
		return flags
	}
}
