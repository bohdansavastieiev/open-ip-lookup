package update

import (
	"errors"
	"fmt"
	"os"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
)

type storageTxn struct {
	dataDir    string
	rollbacks  []func() error
	finalizers []func() error
}

func newStorageTxn(dataDir string) *storageTxn {
	return &storageTxn{dataDir: dataDir}
}

func (tx *storageTxn) promoteArtifact(definition source.Definition, tempPath string) error {
	if err := os.MkdirAll(tx.dataDir, 0o750); err != nil {
		_ = os.RemoveAll(tempPath)
		return fmt.Errorf("create data dir %q: %w", tx.dataDir, err)
	}

	path := sourceLocalPath(tx.dataDir, definition.LocalBaseName)
	rollbackPath, hadExisting, err := moveSourceToRollback(tx.dataDir, definition.LocalBaseName)
	if err != nil {
		_ = os.RemoveAll(tempPath)
		return err
	}

	tx.rollbacks = append(tx.rollbacks, func() error {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove promoted source %q at %q: %w", definition.ID, path, err)
		}
		if hadExisting {
			if err := restoreSourceFromRollback(tx.dataDir, definition.LocalBaseName, rollbackPath); err != nil {
				return err
			}
		}
		return syncStorageDirs(tx.dataDir)
	})
	if hadExisting {
		tx.finalizers = append(tx.finalizers, func() error {
			if err := os.RemoveAll(rollbackPath); err != nil {
				return fmt.Errorf("remove rollback source %q at %q: %w", definition.ID, rollbackPath, err)
			}
			return syncStorageDirs(tx.dataDir)
		})
	}

	if err := os.Rename(tempPath, path); err != nil {
		_ = tx.rollback()
		_ = os.RemoveAll(tempPath)
		return fmt.Errorf("promote source %q to %q: %w", definition.ID, path, err)
	}

	if err := syncStorageDirs(tx.dataDir); err != nil {
		return err
	}
	return nil
}

func (tx *storageTxn) removeArtifact(definition source.Definition) error {
	rollbackPath, hadExisting, err := moveSourceToRollback(tx.dataDir, definition.LocalBaseName)
	if err != nil {
		return err
	}
	if !hadExisting {
		return nil
	}

	tx.rollbacks = append(tx.rollbacks, func() error {
		if err := restoreSourceFromRollback(tx.dataDir, definition.LocalBaseName, rollbackPath); err != nil {
			return err
		}
		return syncStorageDirs(tx.dataDir)
	})
	tx.finalizers = append(tx.finalizers, func() error {
		if err := os.RemoveAll(rollbackPath); err != nil {
			return fmt.Errorf("remove rollback source %q at %q: %w", definition.ID, rollbackPath, err)
		}
		return syncStorageDirs(tx.dataDir)
	})
	return nil
}

func (tx *storageTxn) rollback() error {
	var errs error
	for i := len(tx.rollbacks) - 1; i >= 0; i-- {
		errs = errors.Join(errs, tx.rollbacks[i]())
	}
	tx.rollbacks = nil
	tx.finalizers = nil
	return errs
}

func (tx *storageTxn) finalize() error {
	var errs error
	for _, finalize := range tx.finalizers {
		errs = errors.Join(errs, finalize())
	}
	tx.rollbacks = nil
	tx.finalizers = nil
	return errs
}

func moveSourceToRollback(dataDir string, localBaseName string) (string, bool, error) {
	path := sourceLocalPath(dataDir, localBaseName)
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("stat source path %q: %w", path, err)
	}

	rollbackPath, err := newRollbackPath(dataDir, localBaseName)
	if err != nil {
		return "", false, err
	}
	if err := os.Rename(path, rollbackPath); err != nil {
		return "", false, fmt.Errorf("move source %q to rollback path %q: %w", path, rollbackPath, err)
	}
	if err := syncStorageDirs(dataDir); err != nil {
		restoreErr := restoreSourceFromRollback(dataDir, localBaseName, rollbackPath)
		return "", false, errors.Join(err, restoreErr)
	}
	return rollbackPath, true, nil
}

func restoreSourceFromRollback(dataDir string, localBaseName string, rollbackPath string) error {
	path := sourceLocalPath(dataDir, localBaseName)
	if err := os.Rename(rollbackPath, path); err != nil {
		return fmt.Errorf("restore source rollback %q to %q: %w", rollbackPath, path, err)
	}
	return syncStorageDirs(dataDir)
}

func newRollbackPath(dataDir string, localBaseName string) (string, error) {
	dir := updateDirPath(dataDir)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create rollback dir %q: %w", dir, err)
	}
	tempPattern := tempFilePattern(localBaseName) + ".rollback"
	file, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return "", fmt.Errorf(
			"create rollback temp file in %q with pattern %q: %w",
			dir,
			tempPattern,
			err,
		)
	}
	rollbackPath := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(rollbackPath)
		return "", fmt.Errorf("close rollback temp file %q: %w", rollbackPath, err)
	}
	if err := os.Remove(rollbackPath); err != nil {
		return "", fmt.Errorf("remove rollback temp file %q: %w", rollbackPath, err)
	}
	return rollbackPath, nil
}
