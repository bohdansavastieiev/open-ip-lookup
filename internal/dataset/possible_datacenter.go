package dataset

import "strings"

var possibleDatacenterPositiveTokens = map[string]struct{}{
	"cloud":        {},
	"colo":         {},
	"colocation":   {},
	"colocrossing": {},
	"collocation":  {},
	"datacenter":   {},
	"datacentre":   {},
	"ddos":         {},
	"host":         {},
	"hoster":       {},
	"hosters":      {},
	"hosting":      {},
	"stormwall":    {},
	"vds":          {},
	"vps":          {},
	"webhost":      {},
	"webhosting":   {},
}

var possibleDatacenterPositivePhrases = [][]string{
	{"data", "center"},
	{"data", "centre"},
}

var possibleDatacenterNegativeTokens = map[string]struct{}{
	"academic":           {},
	"adsl":               {},
	"airline":            {},
	"airport":            {},
	"automotive":         {},
	"bank":               {},
	"broadband":          {},
	"cable":              {},
	"cernet":             {},
	"college":            {},
	"defence":            {},
	"defense":            {},
	"dod":                {},
	"dsl":                {},
	"education":          {},
	"electric":           {},
	"energy":             {},
	"fiber":              {},
	"fibre":              {},
	"ftth":               {},
	"government":         {},
	"hospital":           {},
	"insurance":          {},
	"isp":                {},
	"medical":            {},
	"military":           {},
	"ministry":           {},
	"mobile":             {},
	"motors":             {},
	"research":           {},
	"school":             {},
	"surf":               {},
	"tanet":              {},
	"telecom":            {},
	"telecommunications": {},
	"telco":              {},
	"university":         {},
	"wireless":           {},
}

func hasPossibleDatacenterKeywords(handle, description string) bool {
	tokens := metadataTokens(handle, description)
	if hasAnyToken(tokens, possibleDatacenterNegativeTokens) {
		return false
	}
	if hasAnyToken(tokens, possibleDatacenterPositiveTokens) {
		return true
	}

	return hasAnyPhrase(tokens, possibleDatacenterPositivePhrases)
}

func hasAnyToken(tokens []string, keywords map[string]struct{}) bool {
	for _, token := range tokens {
		if _, ok := keywords[token]; ok {
			return true
		}
	}
	return false
}

func hasAnyPhrase(tokens []string, phrases [][]string) bool {
	for _, phrase := range phrases {
		if hasPhrase(tokens, phrase) {
			return true
		}
	}
	return false
}

func hasPhrase(tokens, phrase []string) bool {
	for i := 0; i <= len(tokens)-len(phrase); i++ {
		if phraseMatches(tokens[i:], phrase) {
			return true
		}
	}
	return false
}

func phraseMatches(tokens, phrase []string) bool {
	for i, token := range phrase {
		if tokens[i] != token {
			return false
		}
	}
	return true
}

func metadataTokens(values ...string) []string {
	var tokens []string
	for _, value := range values {
		for _, token := range strings.FieldsFunc(strings.ToLower(value), isTokenSeparator) {
			if token != "" {
				tokens = append(tokens, token)
			}
		}
	}
	return tokens
}

func isTokenSeparator(r rune) bool {
	return (r < '0' || r > '9') && (r < 'a' || r > 'z')
}
