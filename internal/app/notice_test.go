package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanctionsNoticeUntilKeepsFirstStart(t *testing.T) {
	dataDir := t.TempDir()
	firstStart := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

	until, err := sanctionsNoticeUntil(dataDir, firstStart)
	require.NoError(t, err)
	assert.Equal(t, firstStart.Add(sanctionsNoticeDuration), until)

	until, err = sanctionsNoticeUntil(dataDir, firstStart.Add(3*24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, firstStart.Add(sanctionsNoticeDuration), until)
}
