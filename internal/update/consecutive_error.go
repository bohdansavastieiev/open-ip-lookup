package update

import "time"

type consecutiveErrorKind string

const (
	errorKindNetwork    consecutiveErrorKind = "network"
	errorKindHTTPStatus consecutiveErrorKind = "http_status"
	errorKindFreshness  consecutiveErrorKind = "freshness"
	errorKindContent    consecutiveErrorKind = "content"
	errorKindConfig     consecutiveErrorKind = "config"
)

type consecutiveError struct {
	Kind            consecutiveErrorKind `json:"kind"`
	Message         string               `json:"message"`
	Count           int                  `json:"count"`
	FirstHappenedAt time.Time            `json:"first_happened_at"`
	LastHappenedAt  time.Time            `json:"last_happened_at"`
}

func appendConsecutiveError(
	previous []consecutiveError,
	kind consecutiveErrorKind,
	message string,
	happenedAt time.Time,
) []consecutiveError {
	if len(previous) == 0 {
		return []consecutiveError{{
			Kind:            kind,
			Message:         message,
			Count:           1,
			FirstHappenedAt: happenedAt,
			LastHappenedAt:  happenedAt,
		}}
	}

	last := previous[len(previous)-1]
	if last.Kind == kind && last.Message == message {
		previous[len(previous)-1].Count++
		previous[len(previous)-1].LastHappenedAt = happenedAt
		return previous
	}

	return append(previous, consecutiveError{
		Kind:            kind,
		Message:         message,
		Count:           1,
		FirstHappenedAt: happenedAt,
		LastHappenedAt:  happenedAt,
	})
}
