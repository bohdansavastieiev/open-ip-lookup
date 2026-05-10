package update

import (
	"errors"
	"fmt"

	"github.com/bohdansavastieiev/open-ip-lookup/internal/source"
)

type stateValidationCode string

const (
	stateErrUnknownSourceID              stateValidationCode = "unknown_source_id"
	stateErrArtifactWithoutDownloadedAt  stateValidationCode = "artifact_without_downloaded_at"
	stateErrDownloadedAtWithoutSuccessAt stateValidationCode = "downloaded_at_without_success_at"
	stateErrDownloadedAtAfterSuccessAt   stateValidationCode = "downloaded_at_after_success_at"
	stateErrSuccessAtWithoutCheckedAt    stateValidationCode = "success_at_without_checked_at"
	stateErrSuccessAtAfterCheckedAt      stateValidationCode = "success_at_after_checked_at"
	stateErrErrorsWithoutCheckedAt       stateValidationCode = "errors_without_checked_at"
	stateErrRetryableWithoutErrors       stateValidationCode = "retryable_without_errors"
	stateErrOutdatedWithArtifact         stateValidationCode = "outdated_with_artifact"
	stateErrOutdatedWithoutCheckedAt     stateValidationCode = "outdated_without_checked_at"
	stateErrSuccessAtNotBeforeOutdated   stateValidationCode = "success_at_not_before_outdated"

	stateErrConsecutiveUnknownKind     stateValidationCode = "error_unknown_kind"
	stateErrConsecutiveWithoutMessage  stateValidationCode = "error_without_message"
	stateErrConsecutiveInvalidCount    stateValidationCode = "error_invalid_count"
	stateErrConsecutiveWithoutFirstAt  stateValidationCode = "error_without_first_at"
	stateErrConsecutiveWithoutLastAt   stateValidationCode = "error_without_last_at"
	stateErrConsecutiveLastBeforeFirst stateValidationCode = "error_last_at_before_first_at"

	stateErrScheduleWithoutFullSync    stateValidationCode = "schedule_without_full_sync"
	stateErrRetrySyncAtWithoutInterval stateValidationCode = "retry_sync_at_without_interval"
	stateErrRetryIntervalWithoutSyncAt stateValidationCode = "retry_interval_without_sync_at"
	stateErrRetryIntervalUnsupported   stateValidationCode = "retry_interval_unsupported"
	stateErrRetrySyncAtAfterFullSync   stateValidationCode = "retry_sync_at_after_full_sync"
)

type stateValidationError struct {
	path      string
	sourceID  source.ID
	statePath string
	code      stateValidationCode
}

func (e stateValidationError) Error() string {
	if e.statePath != "" {
		return fmt.Sprintf("invalid state %s in %q: %s", e.statePath, e.path, e.code)
	}

	return fmt.Sprintf("invalid state source %q in %q: %s", e.sourceID, e.path, e.code)
}

func newStateError(path string, id source.ID, code stateValidationCode) stateValidationError {
	return stateValidationError{path: path, sourceID: id, code: code}
}

func newStateScheduleError(path string, code stateValidationCode) stateValidationError {
	return stateValidationError{path: path, statePath: "sync_schedule", code: code}
}

func (s state) validate(path string) error {
	var errs error
	errs = errors.Join(errs, validateSyncSchedule(path, s.SyncSchedule))

	for id, sourceState := range s.Sources {
		if !id.IsValid() {
			errs = errors.Join(errs, newStateError(path, id, stateErrUnknownSourceID))
			continue
		}
		errs = errors.Join(errs, validateSourceState(path, id, sourceState))
	}
	return errs
}

func validateSyncSchedule(path string, schedule syncSchedule) error {
	var errs error
	if schedule.LastFullSyncAt.IsZero() && hasFullSyncDependentData(schedule) {
		err := newStateScheduleError(path, stateErrScheduleWithoutFullSync)
		errs = errors.Join(errs, err)
	}
	if !schedule.NextRetrySyncAt.IsZero() && schedule.LastRetryInterval == 0 {
		err := newStateScheduleError(path, stateErrRetrySyncAtWithoutInterval)
		errs = errors.Join(errs, err)
	}
	if schedule.NextRetrySyncAt.IsZero() && schedule.LastRetryInterval != 0 {
		err := newStateScheduleError(path, stateErrRetryIntervalWithoutSyncAt)
		errs = errors.Join(errs, err)
	}
	if schedule.LastRetryInterval != 0 && !schedule.LastRetryInterval.isValid() {
		err := newStateScheduleError(path, stateErrRetryIntervalUnsupported)
		errs = errors.Join(errs, err)
	}
	if !schedule.NextRetrySyncAt.IsZero() &&
		!schedule.NextFullSyncAt.IsZero() &&
		!schedule.NextRetrySyncAt.Before(schedule.NextFullSyncAt) {
		err := newStateScheduleError(path, stateErrRetrySyncAtAfterFullSync)
		errs = errors.Join(errs, err)
	}
	return errs
}

func hasFullSyncDependentData(schedule syncSchedule) bool {
	return !schedule.LastRetrySyncAt.IsZero() ||
		!schedule.NextRetrySyncAt.IsZero() ||
		schedule.LastRetryInterval != 0
}

func validateSourceState(path string, id source.ID, st sourceState) error {
	var errs error
	if st.HasLocalArtifact && st.LastDownloadedAt.IsZero() {
		errs = errors.Join(errs, newStateError(path, id, stateErrArtifactWithoutDownloadedAt))
	}
	if !st.LastDownloadedAt.IsZero() && st.LastSuccessAt.IsZero() {
		errs = errors.Join(errs, newStateError(path, id, stateErrDownloadedAtWithoutSuccessAt))
	}
	if st.LastDownloadedAt.After(st.LastSuccessAt) {
		errs = errors.Join(errs, newStateError(path, id, stateErrDownloadedAtAfterSuccessAt))
	}
	if !st.LastSuccessAt.IsZero() && st.LastCheckedAt.IsZero() {
		errs = errors.Join(errs, newStateError(path, id, stateErrSuccessAtWithoutCheckedAt))
	}
	if st.LastSuccessAt.After(st.LastCheckedAt) {
		errs = errors.Join(errs, newStateError(path, id, stateErrSuccessAtAfterCheckedAt))
	}
	if len(st.ConsecutiveErrors) > 0 && st.LastCheckedAt.IsZero() {
		errs = errors.Join(errs, newStateError(path, id, stateErrErrorsWithoutCheckedAt))
	}
	if st.RetryableFailure && len(st.ConsecutiveErrors) == 0 {
		errs = errors.Join(errs, newStateError(path, id, stateErrRetryableWithoutErrors))
	}
	if !st.MarkedOutdatedAt.IsZero() && st.HasLocalArtifact {
		errs = errors.Join(errs, newStateError(path, id, stateErrOutdatedWithArtifact))
	}
	if !st.MarkedOutdatedAt.IsZero() && st.LastCheckedAt.IsZero() {
		errs = errors.Join(errs, newStateError(path, id, stateErrOutdatedWithoutCheckedAt))
	}
	if !st.MarkedOutdatedAt.IsZero() && !st.LastSuccessAt.IsZero() &&
		!st.LastSuccessAt.Before(st.MarkedOutdatedAt) {
		errs = errors.Join(errs, newStateError(path, id, stateErrSuccessAtNotBeforeOutdated))
	}
	for _, consecutiveErr := range st.ConsecutiveErrors {
		errs = errors.Join(errs, validateConsecutiveError(path, id, consecutiveErr))
	}
	return errs
}

func validateConsecutiveError(path string, id source.ID, err consecutiveError) error {
	var errs error
	if err.Kind != errorKindNetwork &&
		err.Kind != errorKindHTTPStatus &&
		err.Kind != errorKindFreshness &&
		err.Kind != errorKindContent &&
		err.Kind != errorKindConfig {
		errs = errors.Join(errs, newStateError(path, id, stateErrConsecutiveUnknownKind))
	}
	if err.Message == "" {
		errs = errors.Join(errs, newStateError(path, id, stateErrConsecutiveWithoutMessage))
	}
	if err.Count <= 0 {
		errs = errors.Join(errs, newStateError(path, id, stateErrConsecutiveInvalidCount))
	}
	if err.FirstHappenedAt.IsZero() {
		errs = errors.Join(errs, newStateError(path, id, stateErrConsecutiveWithoutFirstAt))
	}
	if err.LastHappenedAt.IsZero() {
		errs = errors.Join(errs, newStateError(path, id, stateErrConsecutiveWithoutLastAt))
	}
	if err.LastHappenedAt.Before(err.FirstHappenedAt) {
		code := stateErrConsecutiveLastBeforeFirst
		errs = errors.Join(errs, newStateError(path, id, code))
	}
	return errs
}
