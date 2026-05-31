package dataset

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasPossibleDatacenterKeywords(t *testing.T) {
	tests := []struct {
		name        string
		handle      string
		description string
		want        bool
	}{
		{
			name:        "matches handle keyword",
			handle:      "EXAMPLE-CLOUD",
			description: "Example Network LLC",
			want:        true,
		},
		{
			name:        "matches description keyword",
			handle:      "EXAMPLE-NET",
			description: "Example Hosting LLC",
			want:        true,
		},
		{
			name:        "matches data center phrase",
			handle:      "EXAMPLE-NET",
			description: "Example Data Center LLC",
			want:        true,
		},
		{
			name:        "negative keyword suppresses positive",
			handle:      "EXAMPLE-CLOUD",
			description: "Example Telecom LLC",
			want:        false,
		},
		{
			name:        "does not match substring",
			handle:      "GHOST-NET",
			description: "Example Network LLC",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, hasPossibleDatacenterKeywords(tt.handle, tt.description))
		})
	}
}
