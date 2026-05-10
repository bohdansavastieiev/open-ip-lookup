package dataset

import (
	"fmt"
	"testing"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func allEnabledSources() []source.ID {
	var ids []source.ID
	ids = append(ids, allRequiredSourceIDs()...)
	ids = append(ids, source.MaxMindGeoLite2ASN)
	ids = append(ids, source.IPVerseASIPBlocksAll)
	ids = append(ids, source.Az0VPNIP)
	return ids
}

func allRequiredSourceIDs() []source.ID {
	var ids []source.ID
	for _, group := range requiredSourceGroups {
		if group.mode == requireAll {
			ids = append(ids, group.sources...)
		}
	}
	return ids
}

func TestValidateRequiredSourceIDs_AllRequiredPresent(t *testing.T) {
	require.NoError(t, validateRequiredSourceIDs(allEnabledSources()))
}

func TestValidateRequiredSourceIDs_MissingRequiredSource(t *testing.T) {
	enabled := allEnabledSources()
	missingID := enabled[0]
	enabled = enabled[1:]

	err := validateRequiredSourceIDs(enabled)
	require.Error(t, err)

	var reqErr sourceRequiredError
	require.ErrorAs(t, err, &reqErr)
	assert.Equal(t, missingID, reqErr.sourceID)
}

func TestValidateRequiredSourceIDs_MissingAllASN(t *testing.T) {
	enabled := allEnabledSources()
	filtered := make([]source.ID, 0, len(enabled))
	for _, id := range enabled {
		if id != source.MaxMindGeoLite2ASN && id != source.IPVerseASIPBlocksAll {
			filtered = append(filtered, id)
		}
	}

	err := validateRequiredSourceIDs(filtered)
	require.Error(t, err)

	var groupErr sourceGroupRequiredError
	require.ErrorAs(t, err, &groupErr)
	assert.Equal(t,
		[]source.ID{source.MaxMindGeoLite2ASN, source.IPVerseASIPBlocksAll},
		groupErr.sources)
}

func TestValidateRequiredSourceIDs_ASNGroupSatisfiedByMaxMind(t *testing.T) {
	enabled := allEnabledSources()
	filtered := make([]source.ID, 0, len(enabled))
	for _, id := range enabled {
		if id != source.IPVerseASIPBlocksAll {
			filtered = append(filtered, id)
		}
	}

	require.NoError(t, validateRequiredSourceIDs(filtered))
}

func TestValidateRequiredSourceIDs_ASNGroupSatisfiedByIPverse(t *testing.T) {
	enabled := allEnabledSources()
	filtered := make([]source.ID, 0, len(enabled))
	for _, id := range enabled {
		if id != source.MaxMindGeoLite2ASN {
			filtered = append(filtered, id)
		}
	}

	require.NoError(t, validateRequiredSourceIDs(filtered))
}

func TestValidateRequiredSourceIDs_MultipleErrorsJoined(t *testing.T) {
	err := validateRequiredSourceIDs(nil)
	require.Error(t, err)

	for _, id := range allRequiredSourceIDs() {
		var reqErr sourceRequiredError
		assert.ErrorAs(t, err, &reqErr, "expected error for source %q", id)
	}

	var groupErr sourceGroupRequiredError
	assert.ErrorAs(t, err, &groupErr)
	assert.Equal(t,
		[]source.ID{source.MaxMindGeoLite2ASN, source.IPVerseASIPBlocksAll},
		groupErr.sources)
}

func TestValidateRequiredSourcesSnapshot_AllPresent(t *testing.T) {
	snap := make(snapshot)
	for _, id := range allRequiredSourceIDs() {
		snap[id] = struct{}{}
	}
	snap[source.MaxMindGeoLite2ASN] = struct{}{}

	require.NoError(t, validateRequiredSourcesSnapshot(snap))
}

func TestValidateRequiredSourcesSnapshot_MissingRequiredSource(t *testing.T) {
	snap := make(snapshot)
	for _, id := range allRequiredSourceIDs() {
		if id == source.CymruFullBogonsIPv4 {
			continue
		}
		snap[id] = struct{}{}
	}
	snap[source.MaxMindGeoLite2ASN] = struct{}{}

	err := validateRequiredSourcesSnapshot(snap)
	require.Error(t, err)

	var reqErr sourceRequiredError
	require.ErrorAs(t, err, &reqErr)
	assert.Equal(t, source.CymruFullBogonsIPv4, reqErr.sourceID)
}

func TestValidateRequiredSourcesSnapshot_MissingAllASN(t *testing.T) {
	snap := make(snapshot)
	for _, id := range allRequiredSourceIDs() {
		snap[id] = struct{}{}
	}

	err := validateRequiredSourcesSnapshot(snap)
	require.Error(t, err)

	var groupErr sourceGroupRequiredError
	require.ErrorAs(t, err, &groupErr)
	assert.Equal(t,
		[]source.ID{source.MaxMindGeoLite2ASN, source.IPVerseASIPBlocksAll},
		groupErr.sources)
}

func TestValidateRequiredSourcesSnapshot_ASNGroupSatisfiedByIPverse(t *testing.T) {
	snap := make(snapshot)
	for _, id := range allRequiredSourceIDs() {
		snap[id] = struct{}{}
	}
	snap[source.IPVerseASIPBlocksAll] = struct{}{}

	require.NoError(t, validateRequiredSourcesSnapshot(snap))
}

func TestFormatSourceList(t *testing.T) {
	assert.Equal(t,
		fmt.Sprintf(`"%v" or "%v"`, source.AvastelBotIPsLists1Day, source.AvastelBotIPsLists5Day),
		formatSourceList([]source.ID{source.AvastelBotIPsLists1Day, source.AvastelBotIPsLists5Day}))
}
