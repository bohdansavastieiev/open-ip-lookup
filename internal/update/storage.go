package update

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const updateDir = ".update"

func updateDirPath(dataDir string) string {
	return filepath.Join(dataDir, updateDir)
}

func sourceLocalPath(dataDir string, localBaseName string) string {
	return filepath.Join(dataDir, localBaseName)
}

func syncStorageDirs(dataDir string) error {
	return errors.Join(syncDir(dataDir), syncDir(updateDirPath(dataDir)))
}

func syncDir(path string) error {
	dirPath := path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		dirPath = filepath.Dir(path)
	}

	dir, err := os.Open(dirPath)
	if err != nil {
		return fmt.Errorf("open dir %q for sync: %w", dirPath, err)
	}
	defer func() { _ = dir.Close() }()

	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync dir %q: %w", dirPath, err)
	}
	return nil
}
