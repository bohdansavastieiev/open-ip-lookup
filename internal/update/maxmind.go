package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
)

const (
	maxMindAccountIDEnv  = "MAXMIND_ACCOUNT_ID"
	maxMindLicenseKeyEnv = "MAXMIND_LICENSE_KEY"
)

var errMissingMaxMindCredentials = errors.New("missing MaxMind credentials")
var errMaxMindChecksumFetch = errors.New("fetch MaxMind checksum")
var errMaxMindChecksumMismatch = errors.New("MaxMind checksum mismatch")
var errInvalidMaxMindChecksum = errors.New("invalid MaxMind checksum")

func setHTTPRequestAuth(req *http.Request, definition source.Definition) error {
	if definition.AuthKind != source.AuthKindMaxMind {
		return nil
	}

	accountID := os.Getenv(maxMindAccountIDEnv)
	licenseKey := os.Getenv(maxMindLicenseKeyEnv)
	if accountID == "" || licenseKey == "" {
		return errMissingMaxMindCredentials
	}
	req.SetBasicAuth(accountID, licenseKey)
	return nil
}

func writeTempMaxMindArtifact(
	body io.Reader,
	tempDir string,
	definition source.Definition,
	wantHash []byte,
) (string, error) {
	archivePath, gotHash, err := writeTempHashedSource(body, tempDir, definition.LocalBaseName)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(archivePath) }()

	if !bytes.Equal(gotHash, wantHash) {
		return "", errMaxMindChecksumMismatch
	}

	f, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("open MaxMind archive %q: %w", archivePath, err)
	}
	defer func() { _ = f.Close() }()

	return writeTempTarGzFile(f, tempDir, definition.LocalBaseName, isMMDBPath)
}

func fetchExpectedChecksum(
	ctx context.Context,
	client *http.Client,
	definition source.Definition,
) ([]byte, error) {
	if definition.AuthKind != source.AuthKindMaxMind {
		return nil, nil
	}
	return fetchMaxMindSHA256(ctx, client, definition)
}

func writeTempHashedSource(body io.Reader, dir string, localBaseName string) (string, []byte, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", nil, fmt.Errorf("create dir %q: %w", dir, err)
	}

	file, err := os.CreateTemp(dir, tempFilePattern(localBaseName)+".download")
	if err != nil {
		return "", nil, fmt.Errorf("create temp download in %q: %w", dir, err)
	}
	tempPath := file.Name()
	hash := sha256.New()

	if _, err := copyWithLimit(io.MultiWriter(file, hash), body, maxSourceDownloadBytes); err != nil {
		_ = file.Close()
		_ = os.Remove(tempPath)
		return "", nil, fmt.Errorf("write temp download %q: %w", tempPath, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(tempPath)
		return "", nil, fmt.Errorf("sync temp download %q: %w", tempPath, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", nil, fmt.Errorf("close temp download %q: %w", tempPath, err)
	}
	return tempPath, hash.Sum(nil), nil
}

func fetchMaxMindSHA256(
	ctx context.Context,
	client *http.Client,
	definition source.Definition,
) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, definition.SHA256URL, nil)
	if err != nil {
		return nil, fmt.Errorf("build checksum request for %q: %w", definition.ID, err)
	}
	if err := setHTTPRequestAuth(req, definition); err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errMaxMindChecksumFetch, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", errMaxMindChecksumFetch, resp.StatusCode)
	}

	data, err := readAllWithLimit(resp.Body, maxChecksumBytes)
	if err != nil {
		if errors.Is(err, errArtifactTooLarge) {
			return nil, fmt.Errorf("%w: %w", errInvalidMaxMindChecksum, err)
		}
		return nil, fmt.Errorf("%w: read body: %w", errMaxMindChecksumFetch, err)
	}
	return parseMaxMindSHA256(string(data))
}

func parseMaxMindSHA256(value string) ([]byte, error) {
	fields := strings.Fields(value)
	if len(fields) == 0 || len(fields[0]) != sha256.Size*2 {
		return nil, errInvalidMaxMindChecksum
	}

	hash, err := hex.DecodeString(fields[0])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errInvalidMaxMindChecksum, err)
	}
	return hash, nil
}

func isMMDBPath(path string) bool {
	return strings.HasSuffix(path, ".mmdb")
}
