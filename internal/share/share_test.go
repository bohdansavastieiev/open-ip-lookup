package share

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultDBPath(t *testing.T) {
	assert.Equal(t, filepath.Join("data", "shares", "shares.sqlite"), DefaultDBPath("data"))
}

func TestOpen_RestrictsDBFilePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "shares.sqlite")
	store, err := Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })

	dirInfo, err := os.Stat(dir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestCreate_StoresCanonicalInputWithDuplicates(t *testing.T) {
	store, _ := openTestStore(t)

	created, err := store.Create(
		context.Background(),
		"src=1.1.1.1 dst=2001:0db8:0:0:0:0:0:0001 src=1.1.1.1",
	)

	require.NoError(t, err)
	assert.NotEmpty(t, created.Bearer)
	assert.Equal(t, "1.1.1.1\n2001:db8::1\n1.1.1.1", storedInput(t, store, created.ID))
}

func TestCreate_RejectsInputWithoutIPs(t *testing.T) {
	store, _ := openTestStore(t)

	_, err := store.Create(context.Background(), "no addresses here")

	require.ErrorIs(t, err, ErrNoIPs)
}

func TestCreate_DoesNotStoreRawBearer(t *testing.T) {
	store, _ := openTestStore(t)

	created, err := store.Create(context.Background(), "1.1.1.1")
	require.NoError(t, err)

	var hash []byte
	err = store.db.QueryRow("SELECT bearer_hash FROM shares WHERE id = ?", created.ID).Scan(&hash)
	require.NoError(t, err)
	assert.Len(t, hash, sha256HashBytes)
	assert.False(t, bytes.Equal([]byte(created.Bearer), hash))
}

func TestResolve_ReturnsInputAndExtendsExpiry(t *testing.T) {
	store, now := openTestStore(t)
	created, err := store.Create(context.Background(), "1.1.1.1 1.1.1.1")
	require.NoError(t, err)

	*now = (*now).Add(time.Hour)
	resolved, err := store.Resolve(context.Background(), created.Bearer)

	require.NoError(t, err)
	assert.Equal(t, created.ID, resolved.ID)
	assert.Equal(t, "1.1.1.1\n1.1.1.1", resolved.Input)
	assert.Equal(t, (*now).Add(DefaultTTL), resolved.ExpiresAt)
	assert.Equal(t, 1, resolved.VisitCount)
	assertStoredVisit(t, store, created.ID, (*now).Unix(), 1)
}

func TestResolve_ReturnsNotFoundForUnknownBearer(t *testing.T) {
	store, _ := openTestStore(t)

	_, err := store.Resolve(context.Background(), "missing")

	require.ErrorIs(t, err, ErrNotFound)
}

func TestResolve_DeletesExpiredShare(t *testing.T) {
	store, now := openTestStore(t)
	created, err := store.Create(context.Background(), "1.1.1.1")
	require.NoError(t, err)

	*now = (*now).Add(DefaultTTL)
	_, err = store.Resolve(context.Background(), created.Bearer)

	require.ErrorIs(t, err, ErrNotFound)
	assertShareCount(t, store, created.ID, 0)
}

func TestDeleteExpired_RemovesExpiredSharesOnly(t *testing.T) {
	store, now := openTestStore(t)
	expired, err := store.Create(context.Background(), "1.1.1.1")
	require.NoError(t, err)

	*now = (*now).Add(DefaultTTL + time.Second)
	active, err := store.Create(context.Background(), "2.2.2.2")
	require.NoError(t, err)

	deleted, err := store.DeleteExpired(context.Background())

	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)
	assertShareCount(t, store, expired.ID, 0)
	assertShareCount(t, store, active.ID, 1)
}

func openTestStore(t *testing.T) (*Store, *time.Time) {
	t.Helper()

	store, err := Open(filepath.Join(t.TempDir(), "shares.sqlite"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })

	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return now }
	return store, &now
}

func storedInput(t *testing.T, store *Store, id int64) string {
	t.Helper()

	var input string
	err := store.db.QueryRow("SELECT input FROM shares WHERE id = ?", id).Scan(&input)
	require.NoError(t, err)
	return input
}

func assertStoredVisit(t *testing.T, store *Store, id int64, lastVisitedAt int64, visitCount int) {
	t.Helper()

	var gotLastVisitedAt int64
	var gotVisitCount int
	err := store.db.QueryRow(`
		SELECT last_visited_at, visit_count
		FROM shares
		WHERE id = ?
	`, id).Scan(&gotLastVisitedAt, &gotVisitCount)
	require.NoError(t, err)
	assert.Equal(t, lastVisitedAt, gotLastVisitedAt)
	assert.Equal(t, visitCount, gotVisitCount)
}

func assertShareCount(t *testing.T, store *Store, id int64, want int) {
	t.Helper()

	var count int
	err := store.db.QueryRow("SELECT COUNT(*) FROM shares WHERE id = ?", id).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, want, count)
}

const sha256HashBytes = 32
