package config

import (
	"testing"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeValidSourcesConfig() SourcesConfig {
	return SourcesConfig{
		DataDir: "./data",
		FullSync: FullSyncConfig{
			IntervalHours: 24,
			StartAtUTC:    time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		Enabled: []source.ID{
			source.IANASpecialIPv4,
			source.DanTorFull,
		},
	}
}

func TestFullSyncConfigInterval(t *testing.T) {
	cfg := FullSyncConfig{IntervalHours: 12}

	assert.Equal(t, 12*time.Hour, cfg.Interval())
}

func TestSourcesConfigValidate_ReturnsNilForValidConfiguration(t *testing.T) {
	cfg := makeValidSourcesConfig()
	expected := cfg

	require.NoError(t, cfg.validate())
	assert.Equal(t, expected, cfg, "validate() mutated config")
}

func TestSourcesConfigValidate_ReturnsFieldErrorForInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*SourcesConfig)
		wantField string
		wantCode  Code
	}{
		{
			name: "data_dir has edge whitespace",
			mutate: func(c *SourcesConfig) {
				c.DataDir = " ./data"
			},
			wantField: "sources.data_dir",
			wantCode:  CodeWhitespace,
		},
		{
			name: "data_dir required",
			mutate: func(c *SourcesConfig) {
				c.DataDir = ""
			},
			wantField: "sources.data_dir",
			wantCode:  CodeRequired,
		},
		{
			name: "full sync interval below range",
			mutate: func(c *SourcesConfig) {
				c.FullSync.IntervalHours = minFullSyncIntervalHours - 1
			},
			wantField: "sources.full_sync.interval_hours",
			wantCode:  CodeRange,
		},
		{
			name: "full sync interval above range",
			mutate: func(c *SourcesConfig) {
				c.FullSync.IntervalHours = maxFullSyncIntervalHours + 1
			},
			wantField: "sources.full_sync.interval_hours",
			wantCode:  CodeRange,
		},
		{
			name: "full sync start time required",
			mutate: func(c *SourcesConfig) {
				c.FullSync.StartAtUTC = time.Time{}
			},
			wantField: "sources.full_sync.start_at_utc",
			wantCode:  CodeRequired,
		},
		{
			name: "enabled source id is unsupported",
			mutate: func(c *SourcesConfig) {
				c.Enabled[0] = "unsupported_id"
			},
			wantField: "sources.enabled[0]",
			wantCode:  CodeUnsupported,
		},
		{
			name: "enabled source ids must be unique",
			mutate: func(c *SourcesConfig) {
				c.Enabled[1] = c.Enabled[0]
			},
			wantField: "sources.enabled[1]",
			wantCode:  CodeDuplicate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := makeValidSourcesConfig()
			tt.mutate(&cfg)
			expected := cfg

			err := cfg.validate()
			require.Error(t, err)

			var fieldErr *FieldError
			require.ErrorAs(t, err, &fieldErr)

			assert.Equal(t, tt.wantField, fieldErr.Field)
			assert.Equal(t, tt.wantCode, fieldErr.Code)
			assert.NotEmpty(t, fieldErr.Detail)
			assert.Equal(t, expected, cfg, "Validate() mutated config")
		})
	}
}
