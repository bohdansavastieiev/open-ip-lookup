package update

import (
	"errors"
	"fmt"
	"os"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/config"
	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
)

var errNoStateButPresentData = errors.New("data present while state is empty")

func loadStateForSync(
	dataDir string,
	fullSync config.FullSyncConfig,
) (state, error) {
	s, ok, err := loadLocalState(dataDir)
	if err != nil {
		return state{}, err
	}
	if !ok {
		if err := validateNoLocalSources(dataDir); err != nil {
			return state{}, err
		}
		s = state{Sources: make(map[source.ID]sourceState)}
	} else if err := s.validateSyncWithLocalSources(dataDir); err != nil {
		return state{}, err
	}

	if s.SyncSchedule.NextFullSyncAt.IsZero() {
		s.SyncSchedule.NextFullSyncAt = fullSync.StartAtUTC
	}
	return s, nil
}

func (s *state) validateSyncWithLocalSources(dataDir string) error {
	sourceIDs, err := validateLocalSourceEntries(dataDir)
	var errs error
	if err != nil {
		errs = errors.Join(errs, err)
	}

	localSources := make(map[source.ID]struct{}, len(sourceIDs))
	for _, id := range sourceIDs {
		localSources[id] = struct{}{}
		_, ok := s.Sources[id]
		if !ok {
			err := fmt.Errorf("source is present locally, but not in state: %v", id)
			errs = errors.Join(errs, err)
		}
	}

	for stateID, sourceState := range s.Sources {
		_, presentLocally := localSources[stateID]
		if !presentLocally && sourceState.HasLocalArtifact {
			err := fmt.Errorf("source expected to be present locally: %v", stateID)
			errs = errors.Join(errs, err)
		}
		if presentLocally && !sourceState.HasLocalArtifact {
			err := fmt.Errorf("source expected to be deleted locally: %v", stateID)
			errs = errors.Join(errs, err)
		}
	}

	return errs
}

func validateLocalSourceEntries(dataDir string) ([]source.ID, error) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	sources := make([]source.ID, 0)
	var errs error
	for _, entry := range entries {
		baseName := entry.Name()
		sourceID, ok := source.LookupIDByLocalBaseName(baseName)
		if ok {
			sources = append(sources, sourceID)
			continue
		}

		if entry.IsDir() && baseName == updateDir {
			continue
		}

		errs = errors.Join(errs, fmt.Errorf("unexpected file or dir found: %v", baseName))
	}

	return sources, errs
}

func validateNoLocalSources(dataDir string) error {
	sources, err := validateLocalSourceEntries(dataDir)
	if err != nil {
		return err
	}
	if len(sources) > 0 {
		return errNoStateButPresentData
	}
	return nil
}
