package update

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshHTTPSource_DownloadsBodyAndUpdatesStateOn200(t *testing.T) {
	start := time.Now().UTC()
	lastModified := start.Add(-time.Hour).Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("If-None-Match"))
		assert.Empty(t, r.Header.Get("If-Modified-Since"))
		w.Header().Set("ETag", "etag-1")
		w.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
		_, _ = w.Write([]byte("1.1.1.1\n"))
	}))
	defer server.Close()

	def := testHTTPSourceDefinition(source.DanTorFull, server.URL)
	tempDir := t.TempDir()
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		sourceState{},
		tempDir,
	)
	end := time.Now().UTC()

	require.NoError(t, err)
	assert.True(t, got.success)
	assert.True(t, got.changed)
	assert.Equal(t, tempDir, filepath.Dir(got.tempPath))
	assert.Contains(t, filepath.Base(got.tempPath), "dan_tor_full-")
	assert.Equal(t, "etag-1", got.state.ETag)
	assert.Equal(t, lastModified, got.state.LastModified)
	assert.True(t, got.state.HasLocalArtifact)
	assertTimeBetween(t, got.state.LastCheckedAt, start, end)
	assertTimeBetween(t, got.state.LastSuccessAt, start, end)
	assertTimeBetween(t, got.state.LastDownloadedAt, start, end)
	assert.Nil(t, got.state.ConsecutiveErrors)

	data, err := os.ReadFile(got.tempPath)
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1\n", string(data))
}

func TestRefreshHTTPSource_UsesIfNoneMatchAndHandles304(t *testing.T) {
	start := time.Now().UTC()
	lastModified := start.Add(-time.Hour).Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "etag-2", r.Header.Get("If-None-Match"))
		assert.Empty(t, r.Header.Get("If-Modified-Since"))
		w.Header().Set("ETag", "etag-2")
		w.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	def := testHTTPSourceDefinition(source.CymruFullBogonsIPv4, server.URL)
	lastSuccessAt := time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC)
	previous := sourceState{
		HasLocalArtifact: true,
		ETag:             "etag-2",
		LastModified:     time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC),
		LastCheckedAt:    lastSuccessAt,
		LastSuccessAt:    lastSuccessAt,
		LastDownloadedAt: time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC),
	}
	tempDir := t.TempDir()
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		previous,
		tempDir,
	)
	end := time.Now().UTC()

	require.NoError(t, err)
	assert.True(t, got.success)
	assert.False(t, got.changed)
	assert.Empty(t, got.tempPath)
	assert.Equal(t, "etag-2", got.state.ETag)
	assert.Equal(t, previous.LastModified, got.state.LastModified)
	assert.True(t, got.state.HasLocalArtifact)
	assertTimeBetween(t, got.state.LastCheckedAt, start, end)
	assertTimeBetween(t, got.state.LastSuccessAt, start, end)
	assert.Equal(t, previous.LastDownloadedAt, got.state.LastDownloadedAt)
	assert.Nil(t, got.state.ConsecutiveErrors)
}

func TestRefreshHTTPSource_UsesIfModifiedSinceWhenNoETag(t *testing.T) {
	previousLastModified := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("If-None-Match"))
		assert.Equal(
			t,
			previousLastModified.Format(http.TimeFormat),
			r.Header.Get("If-Modified-Since"),
		)
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	def := testHTTPSourceDefinition(source.DanTorExit, server.URL)
	lastSuccessAt := time.Date(2026, 4, 18, 11, 0, 0, 0, time.UTC)
	previous := sourceState{
		HasLocalArtifact: true,
		LastModified:     previousLastModified,
		LastCheckedAt:    lastSuccessAt,
		LastSuccessAt:    lastSuccessAt,
		LastDownloadedAt: lastSuccessAt,
	}
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		previous,
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.True(t, got.success)
	assert.False(t, got.changed)
	assert.Empty(t, got.tempPath)
}

func TestRefreshHTTPSource_DoesNotUseConditionalHeadersWithoutLocalArtifact(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("If-None-Match"))
		assert.Empty(t, r.Header.Get("If-Modified-Since"))
		w.Header().Set("ETag", "etag-2")
		_, _ = w.Write([]byte("1.1.1.1\n"))
	}))
	defer server.Close()

	def := testHTTPSourceDefinition(source.DanTorFull, server.URL)
	previous := sourceState{ETag: "etag-1", LastModified: time.Now().UTC()}
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		previous,
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.True(t, got.success)
	assert.True(t, got.changed)
}

func TestRefreshHTTPSource_ReturnsFailureResultOnRequestError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close()

	def := testHTTPSourceDefinition(source.DanTorFull, url)
	got, err := refreshHTTPSource(
		context.Background(),
		http.DefaultClient,
		def,
		sourceState{},
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.False(t, got.success)
	assert.False(t, got.changed)
	assert.Empty(t, got.tempPath)
	require.Len(t, got.state.ConsecutiveErrors, 1)
	assert.Equal(t, errorKindNetwork, got.state.ConsecutiveErrors[0].Kind)
	assert.Equal(t, 1, got.state.ConsecutiveErrors[0].Count)
	assert.False(t, got.state.LastCheckedAt.IsZero())
	assert.True(t, got.state.LastSuccessAt.IsZero())
}

func TestRefreshHTTPSource_ReturnsFailureResultOnBodyReadError(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"ETag": []string{"etag-1"}},
			Body:       errorReadCloser{err: context.DeadlineExceeded},
			Request:    req,
		}, nil
	})}

	def := testHTTPSourceDefinition(source.DanTorFull, "https://example.test/source")
	got, err := refreshHTTPSource(
		context.Background(),
		client,
		def,
		sourceState{},
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.False(t, got.success)
	assert.True(t, got.retryable)
	assert.True(t, got.shortRetryable)
	require.Len(t, got.state.ConsecutiveErrors, 1)
	assert.Equal(t, errorKindNetwork, got.state.ConsecutiveErrors[0].Kind)
}

func TestRefreshHTTPSource_ReturnsFailureResultOnUnexpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	def := testHTTPSourceDefinition(source.DanTorFull, server.URL)
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		sourceState{},
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.False(t, got.success)
	assert.False(t, got.changed)
	assert.Empty(t, got.tempPath)
	require.Len(t, got.state.ConsecutiveErrors, 1)
	assert.Equal(t, errorKindHTTPStatus, got.state.ConsecutiveErrors[0].Kind)
	assert.NotEmpty(t, got.state.ConsecutiveErrors[0].Message)
	assert.False(t, got.state.LastCheckedAt.IsZero())
}

func TestRefreshHTTPSource_ReturnsErrorForInvalidLastModifiedHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", "bad-time")
		_, _ = w.Write([]byte("1.1.1.1\n"))
	}))
	defer server.Close()

	def := testHTTPSourceDefinition(source.DanTorFull, server.URL)
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		sourceState{},
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.False(t, got.success)
	assert.False(t, got.changed)
	assert.Empty(t, got.tempPath)
	assert.False(t, got.retryable)
	require.Len(t, got.state.ConsecutiveErrors, 1)
	assert.Equal(t, errorKindFreshness, got.state.ConsecutiveErrors[0].Kind)
}

func TestRefreshHTTPSource_ReturnsFailureWhenFreshnessMetadataMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("1.1.1.1\n"))
	}))
	defer server.Close()

	def := testHTTPSourceDefinition(source.DanTorFull, server.URL)
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		sourceState{},
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.False(t, got.success)
	assert.False(t, got.changed)
	assert.Empty(t, got.tempPath)
	assert.False(t, got.retryable)
	require.Len(t, got.state.ConsecutiveErrors, 1)
	assert.Equal(t, errorKindFreshness, got.state.ConsecutiveErrors[0].Kind)
}

func TestRefreshHTTPSource_ReturnsFailureForInvalidDanBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", "etag-1")
		_, _ = w.Write([]byte("You can re-fetch the list in: 15 minutes"))
	}))
	defer server.Close()

	def := testHTTPSourceDefinition(source.DanTorFull, server.URL)
	tempDir := t.TempDir()
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		sourceState{},
		tempDir,
	)

	require.NoError(t, err)
	assert.False(t, got.success)
	assert.False(t, got.changed)
	assert.Empty(t, got.tempPath)
	assert.True(t, got.retryable)
	assert.False(t, got.shortRetryable)
	require.Len(t, got.state.ConsecutiveErrors, 1)
	assert.Equal(t, errorKindContent, got.state.ConsecutiveErrors[0].Kind)
	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestRefreshHTTPSource_ReturnsFailureForOversizedDirectDownload(t *testing.T) {
	old := maxSourceDownloadBytes
	maxSourceDownloadBytes = 4
	t.Cleanup(func() { maxSourceDownloadBytes = old })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", "etag-1")
		_, _ = w.Write([]byte("1.1.1.1\n"))
	}))
	defer server.Close()

	def := testHTTPSourceDefinition(source.DanTorFull, server.URL)
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		sourceState{},
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.False(t, got.success)
	assert.True(t, got.retryable)
	require.Len(t, got.state.ConsecutiveErrors, 1)
	assert.Equal(t, errorKindContent, got.state.ConsecutiveErrors[0].Kind)
}

func TestRefreshHTTPSource_ExtractsTarGzDir(t *testing.T) {
	archive := newTestTarGz(t, map[string]string{"1/aggregated.json": "body"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", "etag-1")
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	def := source.DefinitionFor(source.IPVerseASIPBlocksAll)
	def.URL = server.URL
	tempDir := t.TempDir()
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		sourceState{},
		tempDir,
	)

	require.NoError(t, err)
	assert.True(t, got.success)
	assert.True(t, got.changed)
	assert.Equal(t, "etag-1", got.state.ETag)
	data, err := os.ReadFile(filepath.Join(got.tempPath, "1", "aggregated.json"))
	require.NoError(t, err)
	assert.Equal(t, "body", string(data))
}

func TestRefreshHTTPSource_ReturnsFailureForInvalidTarGz(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", "etag-1")
		_, _ = w.Write([]byte("not-gzip"))
	}))
	defer server.Close()

	def := source.DefinitionFor(source.IPVerseASIPBlocksAll)
	def.URL = server.URL
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		sourceState{},
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.False(t, got.success)
	assert.False(t, got.changed)
	assert.True(t, got.retryable)
	require.Len(t, got.state.ConsecutiveErrors, 1)
	assert.Equal(t, errorKindContent, got.state.ConsecutiveErrors[0].Kind)
}

func TestRefreshHTTPSource_MaxMindUsesAuthChecksumAndExtractsMMDB(t *testing.T) {
	t.Setenv(maxMindAccountIDEnv, "account")
	t.Setenv(maxMindLicenseKeyEnv, "license")
	archive := newTestTarGz(t, map[string]string{"GeoLite2-City/test.mmdb": "mmdb"})
	checksum := fmt.Sprintf("%x  GeoLite2-City.tar.gz\n", sha256.Sum256(archive))
	var order atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "account", username)
		assert.Equal(t, "license", password)

		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		if r.URL.Path == "/checksum" {
			assert.Equal(t, int32(1), order.Add(1))
			_, _ = w.Write([]byte(checksum))
			return
		}
		assert.Equal(t, int32(2), order.Add(1))
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	def := source.DefinitionFor(source.MaxMindGeoLite2City)
	def.URL = server.URL + "/download"
	def.SHA256URL = server.URL + "/checksum"
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		sourceState{},
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.True(t, got.success)
	assert.True(t, got.changed)
	data, err := os.ReadFile(got.tempPath)
	require.NoError(t, err)
	assert.Equal(t, "mmdb", string(data))
}

func TestRefreshHTTPSource_MaxMindHandlesNotModified(t *testing.T) {
	t.Setenv(maxMindAccountIDEnv, "account")
	t.Setenv(maxMindLicenseKeyEnv, "license")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/checksum" {
			_, _ = w.Write([]byte("0000000000000000000000000000000000000000000000000000000000000000"))
			return
		}
		assert.Equal(t, "etag-1", r.Header.Get("If-None-Match"))
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	def := source.DefinitionFor(source.MaxMindGeoLite2City)
	def.URL = server.URL + "/download"
	def.SHA256URL = server.URL + "/checksum"
	previous := sourceState{
		HasLocalArtifact: true,
		ETag:             "etag-1",
		LastDownloadedAt: time.Now().UTC().Add(-time.Hour),
		LastSuccessAt:    time.Now().UTC().Add(-time.Hour),
		LastCheckedAt:    time.Now().UTC().Add(-time.Hour),
	}
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		previous,
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.True(t, got.success)
	assert.False(t, got.changed)
	assert.Empty(t, got.tempPath)
	assert.Equal(t, previous.LastDownloadedAt, got.state.LastDownloadedAt)
}

func TestRefreshHTTPSource_MaxMindRejectsChecksumMismatch(t *testing.T) {
	t.Setenv(maxMindAccountIDEnv, "account")
	t.Setenv(maxMindLicenseKeyEnv, "license")
	archive := newTestTarGz(t, map[string]string{"GeoLite2-City/test.mmdb": "mmdb"})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		if r.URL.Path == "/checksum" {
			_, _ = w.Write([]byte("0000000000000000000000000000000000000000000000000000000000000000"))
			return
		}
		_, _ = w.Write(archive)
	}))
	defer server.Close()

	def := source.DefinitionFor(source.MaxMindGeoLite2City)
	def.URL = server.URL + "/download"
	def.SHA256URL = server.URL + "/checksum"
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		sourceState{},
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.False(t, got.success)
	assert.True(t, got.retryable)
	require.Len(t, got.state.ConsecutiveErrors, 1)
	assert.Equal(t, errorKindContent, got.state.ConsecutiveErrors[0].Kind)
}

func TestRefreshHTTPSource_MaxMindRejectsInvalidChecksumBody(t *testing.T) {
	t.Setenv(maxMindAccountIDEnv, "account")
	t.Setenv(maxMindLicenseKeyEnv, "license")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-a-checksum"))
	}))
	defer server.Close()

	def := source.DefinitionFor(source.MaxMindGeoLite2City)
	def.URL = server.URL + "/download"
	def.SHA256URL = server.URL + "/checksum"
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		sourceState{},
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.False(t, got.success)
	require.Len(t, got.state.ConsecutiveErrors, 1)
	assert.Equal(t, errorKindContent, got.state.ConsecutiveErrors[0].Kind)
}

func TestRefreshHTTPSource_MaxMindRejectsOversizedChecksumBody(t *testing.T) {
	t.Setenv(maxMindAccountIDEnv, "account")
	t.Setenv(maxMindLicenseKeyEnv, "license")
	old := maxChecksumBytes
	maxChecksumBytes = 4
	t.Cleanup(func() { maxChecksumBytes = old })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("too-large"))
	}))
	defer server.Close()

	def := source.DefinitionFor(source.MaxMindGeoLite2City)
	def.URL = server.URL + "/download"
	def.SHA256URL = server.URL + "/checksum"
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		sourceState{},
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.False(t, got.success)
	require.Len(t, got.state.ConsecutiveErrors, 1)
	assert.Equal(t, errorKindContent, got.state.ConsecutiveErrors[0].Kind)
}

func TestRefreshHTTPSource_MaxMindRejectsOversizedArchiveDownload(t *testing.T) {
	t.Setenv(maxMindAccountIDEnv, "account")
	t.Setenv(maxMindLicenseKeyEnv, "license")
	old := maxSourceDownloadBytes
	maxSourceDownloadBytes = 4
	t.Cleanup(func() { maxSourceDownloadBytes = old })
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		if r.URL.Path == "/checksum" {
			_, _ = w.Write([]byte(strings.Repeat("0", sha256.Size*2)))
			return
		}
		_, _ = w.Write([]byte("too-large"))
	}))
	defer server.Close()

	def := source.DefinitionFor(source.MaxMindGeoLite2City)
	def.URL = server.URL + "/download"
	def.SHA256URL = server.URL + "/checksum"
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		sourceState{},
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.False(t, got.success)
	require.Len(t, got.state.ConsecutiveErrors, 1)
	assert.Equal(t, errorKindContent, got.state.ConsecutiveErrors[0].Kind)
}

func TestRefreshHTTPSource_MaxMindChecksumFetchFailure(t *testing.T) {
	t.Setenv(maxMindAccountIDEnv, "account")
	t.Setenv(maxMindLicenseKeyEnv, "license")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	def := source.DefinitionFor(source.MaxMindGeoLite2City)
	def.URL = server.URL + "/download"
	def.SHA256URL = server.URL + "/checksum"
	got, err := refreshHTTPSource(
		context.Background(),
		server.Client(),
		def,
		sourceState{},
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.False(t, got.success)
	assert.True(t, got.retryable)
	require.Len(t, got.state.ConsecutiveErrors, 1)
	assert.Equal(t, errorKindNetwork, got.state.ConsecutiveErrors[0].Kind)
}

func TestRefreshHTTPSource_MaxMindChecksumBodyReadError(t *testing.T) {
	t.Setenv(maxMindAccountIDEnv, "account")
	t.Setenv(maxMindLicenseKeyEnv, "license")
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       errorReadCloser{err: context.DeadlineExceeded},
			Request:    req,
		}, nil
	})}

	def := source.DefinitionFor(source.MaxMindGeoLite2City)
	def.URL = "https://example.test/download"
	def.SHA256URL = "https://example.test/checksum"
	got, err := refreshHTTPSource(
		context.Background(),
		client,
		def,
		sourceState{},
		t.TempDir(),
	)

	require.NoError(t, err)
	assert.False(t, got.success)
	assert.True(t, got.retryable)
	require.Len(t, got.state.ConsecutiveErrors, 1)
	assert.Equal(t, errorKindNetwork, got.state.ConsecutiveErrors[0].Kind)
}

func TestRefreshHTTPSource_MaxMindRejectsMissingOrMultipleMMDBs(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
	}{
		{
			name:  "missing mmdb",
			files: map[string]string{"GeoLite2-City/README.txt": "readme"},
		},
		{
			name: "multiple mmdbs",
			files: map[string]string{
				"GeoLite2-City/one.mmdb": "one",
				"GeoLite2-City/two.mmdb": "two",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(maxMindAccountIDEnv, "account")
			t.Setenv(maxMindLicenseKeyEnv, "license")
			archive := newTestTarGz(t, tt.files)
			checksum := fmt.Sprintf("%x\n", sha256.Sum256(archive))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
				if r.URL.Path == "/checksum" {
					_, _ = w.Write([]byte(checksum))
					return
				}
				_, _ = w.Write(archive)
			}))
			defer server.Close()

			def := source.DefinitionFor(source.MaxMindGeoLite2City)
			def.URL = server.URL + "/download"
			def.SHA256URL = server.URL + "/checksum"
			got, err := refreshHTTPSource(
				context.Background(),
				server.Client(),
				def,
				sourceState{},
				t.TempDir(),
			)

			require.NoError(t, err)
			assert.False(t, got.success)
			require.Len(t, got.state.ConsecutiveErrors, 1)
			assert.Equal(t, errorKindContent, got.state.ConsecutiveErrors[0].Kind)
		})
	}
}

func TestSetNotModifiedPreconditionHeader_PrefersETag(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	setNotModifiedPreconditionHeader(
		req.Header,
		"etag-1",
		time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC),
	)

	assert.Equal(t, "etag-1", req.Header.Get("If-None-Match"))
	assert.Empty(t, req.Header.Get("If-Modified-Since"))
}

func TestSetNotModifiedPreconditionHeader_UsesLastModifiedWithoutETag(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	lastModified := time.Date(2026, 4, 17, 10, 0, 0, 0, time.UTC)
	setNotModifiedPreconditionHeader(req.Header, "", lastModified)

	assert.Equal(t, lastModified.Format(http.TimeFormat), req.Header.Get("If-Modified-Since"))
	assert.Empty(t, req.Header.Get("If-None-Match"))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errorReadCloser struct {
	err error
}

func (r errorReadCloser) Read([]byte) (int, error) {
	return 0, r.err
}

func (r errorReadCloser) Close() error { return nil }

func testHTTPSourceDefinition(id source.ID, url string) source.Definition {
	def := source.DefinitionFor(id)
	def.URL = url
	return def
}

func assertTimeBetween(t *testing.T, got time.Time, start time.Time, end time.Time) {
	t.Helper()
	assert.False(t, got.IsZero())
	assert.False(t, got.Before(start))
	assert.False(t, got.After(end))
}
