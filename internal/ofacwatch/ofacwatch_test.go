package ofacwatch

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func anchorSnapshot() snapshot {
	s := snapshot{Documents: []document{{Number: "2026-03501"}}}
	for _, a := range anchors {
		s.SDNPrograms = append(s.SDNPrograms, a.sdnProgram)
		if a.cfrPart != "" {
			s.CFRParts = append(s.CFRParts, cfrPart{ID: a.cfrPart})
		}
	}
	return s
}

func without[T comparable](items []T, drop ...T) []T {
	var result []T
	for _, item := range items {
		if !slices.Contains(drop, item) {
			result = append(result, item)
		}
	}
	return result
}

func TestChanges(t *testing.T) {
	withoutIranProgram := func() snapshot {
		s := anchorSnapshot()
		s.SDNPrograms = without(s.SDNPrograms, "IRAN")
		return s
	}

	tests := []struct {
		name string
		prev func() *snapshot
		cur  func() snapshot
		want []string
	}{
		{
			name: "no changes",
			prev: func() *snapshot { s := anchorSnapshot(); return &s },
			cur:  anchorSnapshot,
		},
		{
			name: "first check reports only missing anchors",
			prev: func() *snapshot { return nil },
			cur: func() snapshot {
				s := anchorSnapshot()
				s.SDNPrograms = append(without(s.SDNPrograms, "UKRAINE-EO13685", "RUSSIA-EO14065"), "NEW")
				return s
			},
			want: []string{
				"Crimea: SDN program UKRAINE-EO13685 is gone",
				"DNR and LNR: SDN program RUSSIA-EO14065 is gone",
			},
		},
		{
			name: "anchor removed",
			prev: func() *snapshot { s := anchorSnapshot(); return &s },
			cur: func() snapshot {
				s := withoutIranProgram()
				s.CFRParts = without(s.CFRParts, cfrPart{ID: "560"})
				return s
			},
			want: []string{"Iran: SDN program IRAN is gone", "Iran: 31 CFR part 560 is gone"},
		},
		{
			name: "missing anchor is reported once",
			prev: func() *snapshot { s := withoutIranProgram(); return &s },
			cur:  withoutIranProgram,
		},
		{
			name: "new program, part and document",
			prev: func() *snapshot { s := anchorSnapshot(); return &s },
			cur: func() snapshot {
				s := anchorSnapshot()
				s.SDNPrograms = append(s.SDNPrograms, "NEW-EO1")
				s.CFRParts = append(s.CFRParts, cfrPart{ID: "599", Title: "New Sanctions Regulations"})
				s.Documents = append(s.Documents, document{Number: "2027-1", Title: "Notice", URL: "u"})
				return s
			},
			want: []string{
				"New SDN program: NEW-EO1",
				"New 31 CFR part 599: New Sanctions Regulations",
				"Federal Register: Notice u",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, changes(tt.prev(), tt.cur()))
		})
	}
}

func TestParseSDNPrograms(t *testing.T) {
	input := `36,"AEROCARIBBEAN AIRLINES",-0- ,"CUBA",-0- ` + "\r\n" +
		`173,"EXAMPLE CO",-0- ,"SDGT] [IRAN",-0- ` + "\r\n" +
		"\x1a"

	programs, err := parseSDNPrograms(strings.NewReader(input))

	require.NoError(t, err)
	assert.Equal(t, []string{"CUBA", "IRAN", "SDGT"}, programs)
}

func TestParseSDNProgramsEmpty(t *testing.T) {
	_, err := parseSDNPrograms(strings.NewReader("\x1a"))

	assert.ErrorIs(t, err, errNoSDNPrograms)
}

func TestParseCFRParts(t *testing.T) {
	input := `{"type": "title", "children": [
		{"type": "chapter", "identifier": "IV", "children": [
			{"type": "part", "identifier": "403", "label_description": "Not OFAC"}]},
		{"type": "chapter", "identifier": "V", "children": [
			{"type": "part", "identifier": "515", "label_description": "Cuban Assets Control Regulations"},
			{"type": "part", "identifier": "542", "label_description": "[Reserved]", "reserved": true}]}]}`

	parts, err := parseCFRParts(strings.NewReader(input))

	require.NoError(t, err)
	assert.Equal(t, []cfrPart{{ID: "515", Title: "Cuban Assets Control Regulations"}}, parts)
}
