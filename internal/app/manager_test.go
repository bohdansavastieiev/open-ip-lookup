package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/config"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/update"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldLoadDataset(t *testing.T) {
	tests := []struct {
		name          string
		event         update.SyncEvent
		serverStarted bool
		want          bool
	}{
		{
			name:          "loads on first event without changes",
			event:         update.SyncEvent{Scope: update.SyncScopeStartup},
			serverStarted: false,
			want:          true,
		},
		{
			name:          "skips unchanged event after start",
			event:         update.SyncEvent{Scope: update.SyncScopePartial},
			serverStarted: true,
			want:          false,
		},
		{
			name: "loads refreshed event after start",
			event: update.SyncEvent{
				Scope:     update.SyncScopePartial,
				Refreshed: []source.ID{source.IANASpecialIPv4},
			},
			serverStarted: true,
			want:          true,
		},
		{
			name: "loads outdated event after start",
			event: update.SyncEvent{
				Scope:    update.SyncScopePartial,
				Outdated: []source.ID{source.CymruFullBogonsIPv4},
			},
			serverStarted: true,
			want:          true,
		},
		{
			name: "skips failed only event after start",
			event: update.SyncEvent{
				Scope:        update.SyncScopePartial,
				Failed:       []source.ID{source.DanTorFull},
				RetryPending: true,
			},
			serverStarted: true,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldLoadDataset(tt.event, tt.serverStarted)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestShouldKeepServingAfterLoadError(t *testing.T) {
	assert.False(t, shouldKeepServingAfterLoadError(false))
	assert.True(t, shouldKeepServingAfterLoadError(true))
}

func TestOpenShares_CreatesDBUnderDataDir(t *testing.T) {
	dataDir := t.TempDir()
	m := New(config.Config{Sources: config.SourcesConfig{DataDir: dataDir}}, discardLogger())

	require.NoError(t, m.openShares())
	t.Cleanup(func() { assert.NoError(t, m.Close()) })

	_, err := os.Stat(filepath.Join(dataDir, "shares", "shares.sqlite"))
	require.NoError(t, err)
}

func TestManagerShareMethodsUseStore(t *testing.T) {
	dataDir := t.TempDir()
	m := New(config.Config{Sources: config.SourcesConfig{DataDir: dataDir}}, discardLogger())
	require.NoError(t, m.openShares())
	t.Cleanup(func() { assert.NoError(t, m.Close()) })

	created, err := m.CreateShare(context.Background(), "1.1.1.1 1.1.1.1")
	require.NoError(t, err)

	resolved, err := m.ResolveShare(context.Background(), created.Bearer)
	require.NoError(t, err)
	assert.Equal(t, created.ID, resolved.ID)
	assert.Equal(t, "1.1.1.1\n1.1.1.1", resolved.Input)
}

func TestHasMaxMindSources(t *testing.T) {
	tests := []struct {
		name      string
		available []source.ID
		want      bool
	}{
		{
			name:      "empty list returns false",
			available: nil,
			want:      false,
		},
		{
			name:      "no maxmind sources returns false",
			available: []source.ID{source.IANASpecialIPv4, source.CymruFullBogonsIPv4},
			want:      false,
		},
		{
			name:      "geolite2 city returns true",
			available: []source.ID{source.IANASpecialIPv4, source.MaxMindGeoLite2City},
			want:      true,
		},
		{
			name:      "geolite2 asn returns true",
			available: []source.ID{source.MaxMindGeoLite2ASN, source.CymruFullBogonsIPv4},
			want:      true,
		},
		{
			name:      "both maxmind sources return true",
			available: []source.ID{source.MaxMindGeoLite2City, source.MaxMindGeoLite2ASN},
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasMaxMindSources(tt.available)
			assert.Equal(t, tt.want, got)
		})
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
