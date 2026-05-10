package source

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookup_ReturnsFalseForUnknownSource(t *testing.T) {
	_, ok := Lookup(ID("unknown_source"))
	assert.False(t, ok)
}

func TestLookupAndDefinitionFor_ReturnRegisteredDefinitions(t *testing.T) {
	defs := Definitions()
	require.NotEmpty(t, defs)

	for _, want := range defs {
		got, ok := Lookup(want.ID)
		require.True(t, ok)
		assert.Equal(t, want, got)
		assert.Equal(t, want, DefinitionFor(want.ID))
	}
}

func TestDefinitionsByID_ReturnsCopy(t *testing.T) {
	defs := DefinitionsByID()
	require.NotEmpty(t, defs)

	defs[DanTorFull] = Definition{}
	got, ok := Lookup(DanTorFull)

	require.True(t, ok)
	assert.NotEmpty(t, got.ID)
	assert.NotEqual(t, Definition{}, got)
}

func TestDefinitionFor_PanicsForUnknownSource(t *testing.T) {
	assert.Panics(t, func() { _ = DefinitionFor(ID("unknown_source")) })
}

func TestBuildDefinitionsByID_ReturnsAllDefinitions(t *testing.T) {
	defs := Definitions()
	byID := buildDefinitionsByID(defs)

	require.Len(t, byID, len(defs))
	for _, def := range defs {
		assert.Equal(t, def, byID[def.ID])
	}
}

func TestBuildDefinitionsByID_PanicsOnDuplicateID(t *testing.T) {
	defs := []Definition{
		validTestDefinition(MaxMindGeoLite2City),
		validTestDefinition(MaxMindGeoLite2City),
	}
	assert.Panics(t, func() { _ = buildDefinitionsByID(defs) })
}

func TestBuildDefinitionsByID_PanicsOnInvalidKinds(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Definition)
	}{
		{
			name: "freshness kind",
			mutate: func(def *Definition) {
				def.FreshnessKind = 0
			},
		},
		{
			name: "artifact kind",
			mutate: func(def *Definition) {
				def.ArtifactKind = 0
			},
		},
		{
			name: "auth kind",
			mutate: func(def *Definition) {
				def.AuthKind = 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := validTestDefinition(DanTorFull)
			tt.mutate(&def)

			assert.Panics(t, func() { _ = buildDefinitionsByID([]Definition{def}) })
		})
	}
}

func TestAll_RegistryEntriesAreUniqueAndComplete(t *testing.T) {
	defs := Definitions()
	require.NotEmpty(t, defs)

	seenIDs := make(map[ID]struct{}, len(defs))
	seenPaths := make(map[string]struct{}, len(defs))
	seenURLs := make(map[string]struct{}, len(defs))

	for _, def := range defs {
		assert.NotEmpty(t, def.ID)
		assert.NotEmpty(t, def.LocalBaseName)
		assert.NotEmpty(t, def.URL)
		assert.True(t, def.FreshnessKind.isValid())
		assert.True(t, def.ArtifactKind.isValid())
		assert.True(t, def.AuthKind.isValid())

		interval, ok := def.OutdatedInterval()
		if ok {
			assert.Positive(t, interval)
		} else {
			assert.Zero(t, interval)
		}

		_, duplicateID := seenIDs[def.ID]
		assert.False(t, duplicateID)
		seenIDs[def.ID] = struct{}{}

		_, duplicatePath := seenPaths[def.LocalBaseName]
		assert.False(t, duplicatePath)
		seenPaths[def.LocalBaseName] = struct{}{}

		_, duplicateURL := seenURLs[def.URL]
		assert.False(t, duplicateURL)
		seenURLs[def.URL] = struct{}{}
	}
}

func validTestDefinition(id ID) Definition {
	return Definition{
		ID:            id,
		LocalBaseName: string(id) + ".txt",
		URL:           "https://example.com/" + string(id),
		FreshnessKind: FreshnessKindETag,
		ArtifactKind:  ArtifactKindDirectFile,
		AuthKind:      AuthKindNone,
	}
}
