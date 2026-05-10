package update

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var errArtifactTooLarge = errors.New("source artifact exceeds size limit")

const bytesPerMiB = 1024 * 1024

var (
	maxSourceDownloadBytes = int64(256 * bytesPerMiB)
	maxChecksumBytes       = int64(1024)
	maxArchiveFileBytes    = int64(256 * bytesPerMiB)
	maxArchiveTotalBytes   = int64(512 * bytesPerMiB)
	maxArchiveFiles        = 500_000
)

func tempFilePattern(localBaseName string) string {
	ext := filepath.Ext(localBaseName)
	if ext == "" {
		return localBaseName + "-*"
	}

	base := strings.TrimSuffix(localBaseName, ext)
	return base + "-*" + ext
}

func writeTempSource(body io.Reader, dir string, localBaseName string) (string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create dir %q: %w", dir, err)
	}

	tempPattern := tempFilePattern(localBaseName)
	file, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return "", fmt.Errorf("create temp file in %q with pattern %q: %w", dir, tempPattern, err)
	}
	tempPath := file.Name()

	if _, err := copyWithLimit(file, body, maxSourceDownloadBytes); err != nil {
		_ = file.Close()
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("write temp file %q: %w", tempPath, err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("sync temp file %q: %w", tempPath, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("close temp file %q: %w", tempPath, err)
	}
	return tempPath, nil
}

func copyWithLimit(dst io.Writer, src io.Reader, limit int64) (int64, error) {
	limited := &io.LimitedReader{R: src, N: limit + 1}
	n, err := io.Copy(dst, limited)
	if err != nil {
		return n, err
	}
	if n > limit {
		return n, errArtifactTooLarge
	}
	return n, nil
}

func readAllWithLimit(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(&io.LimitedReader{R: r, N: limit + 1})
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errArtifactTooLarge
	}
	return data, nil
}
