package update

import (
	"context"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
)

func (u *Updater) runStartup(ctx context.Context, now time.Time, s *state) (SyncEvent, error) {
	if s.SyncSchedule.needsFullSync(now) {
		sourceIDs := filterOutdated(u.cfg.Enabled, s.Sources)
		return u.updateSources(ctx, SyncScopeStartup, sourceIDs, s, shortRetryDelays)
	}

	sourceIDs := newEnabledSourceIDs(u.cfg.Enabled, s.Sources)
	if s.SyncSchedule.needsRetrySync(now) {
		sourceIDs = append(sourceIDs, u.refreshableFailedSourceIDs(s.Sources)...)
	}
	return u.updateSources(ctx, SyncScopeStartup, sourceIDs, s, shortRetryDelays)
}

func newEnabledSourceIDs(enabled []source.ID, sources map[source.ID]sourceState) []source.ID {
	var newEnabled []source.ID
	for _, id := range enabled {
		if _, ok := sources[id]; !ok {
			newEnabled = append(newEnabled, id)
		}
	}
	return newEnabled
}
