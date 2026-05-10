package dataset

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
)

type sourceRequiredError struct {
	sourceID source.ID
}

func (e sourceRequiredError) Error() string {
	return fmt.Sprintf("required source %q is not available", e.sourceID)
}

type sourceGroupRequiredError struct {
	sources []source.ID
}

func (e sourceGroupRequiredError) Error() string {
	return fmt.Sprintf("at least one of %s is required", formatSourceList(e.sources))
}

type requireMode int

const (
	requireAll requireMode = iota
	requireAny
)

type sourceGroup struct {
	name    string
	mode    requireMode
	sources []source.ID
}

var requiredSourceGroups = []sourceGroup{
	{name: "geo", mode: requireAll, sources: []source.ID{source.MaxMindGeoLite2City}},
	{
		name:    "asn",
		mode:    requireAny,
		sources: []source.ID{source.MaxMindGeoLite2ASN, source.IPVerseASIPBlocksAll},
	},
	{
		name:    "bogon",
		mode:    requireAll,
		sources: []source.ID{source.CymruFullBogonsIPv4, source.CymruFullBogonsIPv6},
	},
	{
		name:    "iana",
		mode:    requireAll,
		sources: []source.ID{source.IANASpecialIPv4, source.IANASpecialIPv6},
	},
}

func validateRequiredSourceIDs(sourceIDs []source.ID) error {
	selected := make(map[source.ID]struct{}, len(sourceIDs))
	for _, id := range sourceIDs {
		selected[id] = struct{}{}
	}
	return validateSourceGroups(selected)
}

func validateRequiredSourcesSnapshot(snap snapshot) error {
	present := make(map[source.ID]struct{}, len(snap))
	for id := range snap {
		present[id] = struct{}{}
	}
	return validateSourceGroups(present)
}

func validateSourceGroups(available map[source.ID]struct{}) error {
	var errs []error
	for _, group := range requiredSourceGroups {
		switch group.mode {
		case requireAll:
			for _, id := range group.sources {
				if _, ok := available[id]; !ok {
					errs = append(errs, sourceRequiredError{sourceID: id})
				}
			}
		case requireAny:
			found := false
			for _, id := range group.sources {
				if _, ok := available[id]; ok {
					found = true
					break
				}
			}
			if !found {
				errs = append(errs, sourceGroupRequiredError{sources: group.sources})
			}
		}
	}
	return errors.Join(errs...)
}

func formatSourceList(sources []source.ID) string {
	quoted := make([]string, len(sources))
	for i, s := range sources {
		quoted[i] = fmt.Sprintf("%q", string(s))
	}
	return strings.Join(quoted, " or ")
}
