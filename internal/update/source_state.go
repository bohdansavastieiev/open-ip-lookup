package update

import (
	"fmt"
	"net/http"
	"time"
)

type sourceState struct {
	HasLocalArtifact bool `json:"has_local_artifact,omitempty"`

	ETag         string    `json:"etag,omitempty"`
	LastModified time.Time `json:"last_modified"`

	LastCheckedAt     time.Time          `json:"last_checked_at"`
	LastSuccessAt     time.Time          `json:"last_success_at"`
	LastDownloadedAt  time.Time          `json:"last_downloaded_at"`
	MarkedOutdatedAt  time.Time          `json:"marked_outdated_at"`
	RetryableFailure  bool               `json:"retryable_failure,omitempty"`
	ConsecutiveErrors []consecutiveError `json:"consecutive_errors,omitempty"`
}

func newNotModifiedSourceState(previous sourceState, completedAt time.Time) sourceState {
	state := previous
	state.LastCheckedAt = completedAt
	state.LastSuccessAt = completedAt
	state.RetryableFailure = false
	state.ConsecutiveErrors = nil
	return state
}

func newOKSourceState(
	previous sourceState,
	headers http.Header,
	completedAt time.Time,
) (sourceState, error) {
	state := previous

	state.ETag = headers.Get("ETag")
	lastModified, err := lastModifiedFromHeader(headers)
	if err != nil {
		return sourceState{}, err
	}
	state.LastModified = lastModified

	state.LastCheckedAt = completedAt
	state.LastSuccessAt = completedAt
	state.RetryableFailure = false
	state.ConsecutiveErrors = nil
	state.HasLocalArtifact = true
	state.LastDownloadedAt = completedAt
	state.MarkedOutdatedAt = time.Time{}

	return state, nil
}

func lastModifiedFromHeader(headers http.Header) (time.Time, error) {
	value := headers.Get("Last-Modified")
	if value == "" {
		return time.Time{}, nil
	}

	parsed, err := time.Parse(http.TimeFormat, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse Last-Modified header %q: %w", value, err)
	}
	return parsed.UTC(), nil
}

func newUnsuccessfulSourceState(
	previous sourceState,
	completedAt time.Time,
	kind consecutiveErrorKind,
	retryable bool,
	err error,
) sourceState {
	state := previous
	state.LastCheckedAt = completedAt
	state.RetryableFailure = retryable
	state.ConsecutiveErrors = appendConsecutiveError(
		state.ConsecutiveErrors,
		kind,
		err.Error(),
		completedAt,
	)
	return state
}
