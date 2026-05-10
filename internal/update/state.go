package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
)

const stateLocalPath = "state.json"

type state struct {
	SyncSchedule syncSchedule              `json:"sync_schedule"`
	Sources      map[source.ID]sourceState `json:"sources,omitempty"`
}

func stateFilePath(dataDir string) string {
	return filepath.Join(updateDirPath(dataDir), stateLocalPath)
}

func availableSourceIDs(enabled []source.ID, sources map[source.ID]sourceState) []source.ID {
	var ids []source.ID
	for _, id := range enabled {
		ss, ok := sources[id]
		if ok && ss.HasLocalArtifact && ss.MarkedOutdatedAt.IsZero() {
			ids = append(ids, id)
		}
	}
	return ids
}

func filterOutdated(enabled []source.ID, sources map[source.ID]sourceState) []source.ID {
	var ids []source.ID
	for _, id := range enabled {
		ss, ok := sources[id]
		if !ok || ss.MarkedOutdatedAt.IsZero() {
			ids = append(ids, id)
		}
	}
	return ids
}

func loadLocalState(dataDir string) (state, bool, error) {
	path := stateFilePath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state{}, false, nil
		}
		return state{}, false, fmt.Errorf("read state file %q: %w", path, err)
	}

	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return state{}, false, fmt.Errorf("unmarshal state file %q: %w", path, err)
	}
	if s.Sources == nil {
		return state{}, false, fmt.Errorf("state file %q has nil sources map", path)
	}
	if err := s.validate(path); err != nil {
		return state{}, false, err
	}
	return s, true, nil
}

func (s state) saveStateJSON(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create parent dir %q: %w", dir, err)
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state file %q: %w", path, err)
	}

	f, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp state file for %q: %w", path, err)
	}
	tempPath := f.Name()

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("write temp state file %q: %w", tempPath, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("sync temp state file %q: %w", tempPath, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close temp state file %q: %w", tempPath, err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace state file %q: %w", path, err)
	}

	dirFile, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open parent dir %q for sync: %w", dir, err)
	}
	defer func() { _ = dirFile.Close() }()

	if err := dirFile.Sync(); err != nil {
		return fmt.Errorf("sync parent dir %q: %w", dir, err)
	}

	return nil
}
