package update

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	errEmptyArchive          = errors.New("archive has no files")
	errInvalidArchive        = errors.New("invalid archive")
	errInvalidArchivePath    = errors.New("archive path escapes destination")
	errMultipleArchiveFiles  = errors.New("archive has multiple matching files")
	errNoMatchingArchiveFile = errors.New("archive has no matching file")
)

func writeTempTarGzDir(body io.Reader, dir string, localBaseName string) (string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create dir %q: %w", dir, err)
	}

	tempPath, err := os.MkdirTemp(dir, tempFilePattern(localBaseName))
	if err != nil {
		return "", fmt.Errorf("create temp dir in %q: %w", dir, err)
	}

	if err := extractTarGzDir(body, tempPath); err != nil {
		_ = os.RemoveAll(tempPath)
		return "", err
	}
	return tempPath, nil
}

func writeTempTarGzFile(
	body io.Reader,
	dir string,
	localBaseName string,
	match func(string) bool,
) (string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create dir %q: %w", dir, err)
	}

	tempPath, err := newEmptyTempPath(dir, localBaseName)
	if err != nil {
		return "", err
	}
	if err := extractTarGzFile(body, tempPath, match); err != nil {
		_ = os.Remove(tempPath)
		return "", err
	}
	return tempPath, nil
}

func extractTarGzDir(body io.Reader, dest string) error {
	var totalBytes int64
	return readTarGz(body, func(tr *tar.Reader, hdr *tar.Header) error {
		relPath, ok := cleanTarPath(hdr.Name)
		if !ok {
			return fmt.Errorf("%w: %w: %q", errInvalidArchive, errInvalidArchivePath, hdr.Name)
		}
		if relPath == "" {
			return nil
		}

		target := filepath.Join(dest, relPath)
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o750); err != nil {
				return fmt.Errorf("create extracted dir %q: %w", target, err)
			}
		case tar.TypeReg:
			remaining := maxArchiveTotalBytes - totalBytes
			if remaining <= 0 {
				return archiveLimitError("total extracted bytes")
			}

			limit := min(maxArchiveFileBytes, remaining)
			n, err := extractTarFile(tr, target, limit)
			if err != nil {
				return err
			}
			totalBytes += n
		case tar.TypeXHeader, tar.TypeXGlobalHeader:
			return nil
		default:
			return unsupportedTarEntryError(hdr)
		}
		return nil
	})
}

func extractTarGzFile(body io.Reader, dest string, match func(string) bool) error {
	matched := 0
	err := readTarGz(body, func(tr *tar.Reader, hdr *tar.Header) error {
		relPath, ok := cleanTarPath(hdr.Name)
		if !ok {
			return fmt.Errorf("%w: %w: %q", errInvalidArchive, errInvalidArchivePath, hdr.Name)
		}
		if relPath == "" || hdr.Typeflag == tar.TypeDir || !match(relPath) {
			return nil
		}
		if hdr.Typeflag != tar.TypeReg {
			return unsupportedTarEntryError(hdr)
		}
		matched++
		if matched > 1 {
			return fmt.Errorf("%w: %w", errInvalidArchive, errMultipleArchiveFiles)
		}
		_, err := extractTarFile(tr, dest, maxArchiveFileBytes)
		return err
	})
	if err != nil {
		return err
	}
	if matched == 0 {
		return fmt.Errorf("%w: %w", errInvalidArchive, errNoMatchingArchiveFile)
	}
	return nil
}

type tarEntryHandler func(*tar.Reader, *tar.Header) error

func readTarGz(body io.Reader, handle tarEntryHandler) error {
	gz, err := gzip.NewReader(body)
	if err != nil {
		return fmt.Errorf("%w: open gzip stream: %w", errInvalidArchive, err)
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	files := 0
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("%w: read tar entry: %w", errInvalidArchive, err)
		}

		switch hdr.Typeflag {
		case tar.TypeReg:
			files++
			if files > maxArchiveFiles {
				return archiveLimitError("file count")
			}
		}
		if err := handle(tr, hdr); err != nil {
			return err
		}
	}

	if files == 0 {
		return fmt.Errorf("%w: %w", errInvalidArchive, errEmptyArchive)
	}
	return nil
}

func unsupportedTarEntryError(hdr *tar.Header) error {
	return fmt.Errorf(
		"%w: unsupported tar entry type %d: %q",
		errInvalidArchive,
		hdr.Typeflag,
		hdr.Name,
	)
}

func archiveLimitError(kind string) error {
	return fmt.Errorf("%w: %w: %s", errInvalidArchive, errArtifactTooLarge, kind)
}

func newEmptyTempPath(dir string, localBaseName string) (string, error) {
	file, err := os.CreateTemp(dir, tempFilePattern(localBaseName))
	if err != nil {
		return "", fmt.Errorf("create temp file in %q: %w", dir, err)
	}
	tempPath := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return "", fmt.Errorf("close temp file %q: %w", tempPath, err)
	}
	if err := os.Remove(tempPath); err != nil {
		return "", fmt.Errorf("remove temp file %q: %w", tempPath, err)
	}
	return tempPath, nil
}

func extractTarFile(r io.Reader, target string, limit int64) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return 0, fmt.Errorf("create parent dir for %q: %w", target, err)
	}

	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, fmt.Errorf("create extracted file %q: %w", target, err)
	}
	n, err := copyWithLimit(f, r, limit)
	if err != nil {
		_ = f.Close()
		return n, fmt.Errorf("write extracted file %q: %w", target, err)
	}
	if err := f.Close(); err != nil {
		return n, fmt.Errorf("close extracted file %q: %w", target, err)
	}
	return n, nil
}

func cleanTarPath(name string) (string, bool) {
	if path.IsAbs(name) {
		return "", false
	}

	cleaned := path.Clean(strings.TrimPrefix(name, "./"))
	if cleaned == "." {
		return "", true
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false
	}
	return filepath.FromSlash(cleaned), true
}
