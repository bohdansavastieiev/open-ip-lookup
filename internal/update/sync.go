package update

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
)

type SyncEvent struct {
	Scope        SyncScope
	Available    []source.ID
	Refreshed    []source.ID
	Failed       []source.ID
	Outdated     []source.ID
	RetryPending bool
}

type SyncScope string

const (
	SyncScopeStartup SyncScope = "startup"
	SyncScopeFull    SyncScope = "full"
	SyncScopePartial SyncScope = "partial"
)

func (u *Updater) updateSources(
	ctx context.Context,
	scope SyncScope,
	sourceIDs []source.ID,
	s *state,
	retryDelays []time.Duration,
) (SyncEvent, error) {
	startedAt := time.Now()
	if len(sourceIDs) > 0 {
		u.logger.Info(
			"source sync started",
			slog.String("scope", string(scope)),
			slog.Int("sources", len(sourceIDs)),
		)
	}
	var refreshed []source.ID
	var failed []source.ID
	var outdated []source.ID
	tx := newStorageTxn(u.cfg.DataDir)

	results, err := u.refreshSources(ctx, sourceIDs, s.Sources, retryDelays)
	if err != nil {
		return SyncEvent{}, err
	}
	defer cleanupRefreshTemps(results)

	for _, result := range results {
		id := result.id
		definition := result.definition
		if result.unsupported {
			st := s.Sources[id]
			if !st.HasLocalArtifact || !st.MarkedOutdatedAt.IsZero() {
				failed = append(failed, id)
				u.logger.Info(
					"source update unsupported",
					slog.String("source", string(id)),
					slog.Duration("duration", result.duration),
				)
			}
			continue
		}

		if result.update.changed {
			if err := tx.promoteArtifact(definition, result.update.tempPath); err != nil {
				return SyncEvent{}, errors.Join(err, tx.rollback())
			}
			refreshed = append(refreshed, id)
			u.logger.Info("source promoted", slog.String("source", string(id)))
		}

		s.Sources[id] = result.update.state
		if result.update.success {
			continue
		}

		failed = append(failed, id)
		outdatedNow, err := u.markOutdatedIfExpired(id, definition, s.Sources, tx)
		if err != nil {
			return SyncEvent{}, errors.Join(err, tx.rollback())
		}
		st := s.Sources[id]
		available := st.HasLocalArtifact && st.MarkedOutdatedAt.IsZero()
		u.logger.Info(
			"source failure recorded",
			slog.String("source", string(id)),
			slog.Bool("retryable", result.update.retryable),
			slog.Bool("available", available),
			slog.Bool("outdated", outdatedNow),
		)
		if outdatedNow {
			outdated = append(outdated, id)
			u.logger.Info("source marked outdated", slog.String("source", string(id)))
		}
	}

	retryNeededAfter := len(u.refreshableFailedSourceIDs(s.Sources)) > 0
	syncedAt := time.Now()
	u.updateSyncSchedule(scope, startedAt.UTC(), syncedAt.UTC(), retryNeededAfter, &s.SyncSchedule)
	retryPending := !s.SyncSchedule.NextRetrySyncAt.IsZero()

	if err := s.saveStateJSON(stateFilePath(u.cfg.DataDir)); err != nil {
		return SyncEvent{}, errors.Join(err, tx.rollback())
	}
	finalizeStartedAt := time.Now()
	hadFinalizers := len(tx.finalizers) > 0
	if err := tx.finalize(); err != nil {
		u.logger.Info("finalize storage transaction", slog.Any("err", err))
	}
	if hadFinalizers {
		u.logger.Info(
			"storage transaction finalized",
			slog.Duration("duration", time.Since(finalizeStartedAt)),
		)
	}
	completedAt := time.Now()
	status := "completed"
	if len(sourceIDs) == 0 {
		status = "skipped"
	}
	u.logger.Info(
		"source sync completed",
		slog.String("scope", string(scope)),
		slog.String("status", status),
		slog.Duration("duration", completedAt.Sub(startedAt)),
		slog.Int("available", len(availableSourceIDs(u.cfg.Enabled, s.Sources))),
		slog.Int("refreshed", len(refreshed)),
		slog.Int("failed", len(failed)),
		slog.Int("outdated", len(outdated)),
		slog.Bool("retry_pending", retryPending),
	)

	return SyncEvent{
		Scope:        scope,
		Available:    availableSourceIDs(u.cfg.Enabled, s.Sources),
		Refreshed:    refreshed,
		Failed:       failed,
		Outdated:     outdated,
		RetryPending: retryPending,
	}, nil
}

func isSourceUpdateSupported(definition source.Definition) bool {
	switch definition.AuthKind {
	case source.AuthKindNone:
		return definition.ArtifactKind == source.ArtifactKindDirectFile ||
			definition.ArtifactKind == source.ArtifactKindTarGzDir
	case source.AuthKindMaxMind:
		return definition.ArtifactKind == source.ArtifactKindTarGzFile
	default:
		return false
	}
}

func (u *Updater) refreshableFailedSourceIDs(sources map[source.ID]sourceState) []source.ID {
	var ids []source.ID
	for _, id := range u.cfg.Enabled {
		definition := source.DefinitionFor(id)
		st, ok := sources[id]
		if ok && isSourceUpdateSupported(definition) && hasRefreshableFailure(st) {
			ids = append(ids, id)
		}
	}
	return ids
}

func hasRefreshableFailure(state sourceState) bool {
	return state.RetryableFailure &&
		len(state.ConsecutiveErrors) > 0 &&
		state.MarkedOutdatedAt.IsZero()
}

func (u *Updater) markOutdatedIfExpired(
	id source.ID,
	definition source.Definition,
	sources map[source.ID]sourceState,
	tx *storageTxn,
) (bool, error) {
	st := sources[id]
	if !st.HasLocalArtifact || !st.MarkedOutdatedAt.IsZero() {
		return false, nil
	}
	interval, ok := definition.OutdatedInterval()
	if !ok || !sourceFailureExpired(st, interval) {
		return false, nil
	}

	if !isSourceUpdateSupported(definition) {
		return false, nil
	}
	if err := tx.removeArtifact(definition); err != nil {
		return false, err
	}

	st.HasLocalArtifact = false
	st.MarkedOutdatedAt = st.LastCheckedAt
	st.RetryableFailure = false
	sources[id] = st
	return true, nil
}

func sourceFailureExpired(st sourceState, interval time.Duration) bool {
	if len(st.ConsecutiveErrors) == 0 {
		return false
	}

	firstFailureAt := st.ConsecutiveErrors[0].FirstHappenedAt
	return !firstFailureAt.IsZero() && st.LastCheckedAt.After(firstFailureAt.Add(interval))
}

func (u *Updater) updateSyncSchedule(
	scope SyncScope,
	startedAt time.Time,
	completedAt time.Time,
	retryNeededAfter bool,
	schedule *syncSchedule,
) {
	interval := u.cfg.FullSync.Interval()
	if scope == SyncScopeStartup {
		*schedule = schedule.newStartupSchedule(
			startedAt,
			completedAt,
			interval,
			retryNeededAfter,
		)
		if retryNeededAfter && schedule.NextRetrySyncAt.IsZero() {
			schedule.scheduleFirstRetryAfter(completedAt)
		}
		return
	}
	*schedule = schedule.newSchedule(startedAt, interval, retryNeededAfter)
	if scope != SyncScopeFull && retryNeededAfter && schedule.NextRetrySyncAt.IsZero() {
		schedule.scheduleFirstRetryAfter(completedAt)
	}
}
