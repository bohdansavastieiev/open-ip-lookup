package update

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAppendConsecutiveError_GroupsOnlyAdjacentEqualErrors(t *testing.T) {
	firstAt := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	secondAt := firstAt.Add(5 * time.Minute)
	thirdAt := secondAt.Add(5 * time.Minute)
	fourthAt := thirdAt.Add(5 * time.Minute)

	got := appendConsecutiveError(nil, errorKindNetwork, "timeout", firstAt)
	got = appendConsecutiveError(got, errorKindNetwork, "timeout", secondAt)
	got = appendConsecutiveError(got, errorKindHTTPStatus, "status 500", thirdAt)
	got = appendConsecutiveError(got, errorKindNetwork, "timeout", fourthAt)

	assert.Equal(t, []consecutiveError{
		{
			Kind:            errorKindNetwork,
			Message:         "timeout",
			Count:           2,
			FirstHappenedAt: firstAt,
			LastHappenedAt:  secondAt,
		},
		{
			Kind:            errorKindHTTPStatus,
			Message:         "status 500",
			Count:           1,
			FirstHappenedAt: thirdAt,
			LastHappenedAt:  thirdAt,
		},
		{
			Kind:            errorKindNetwork,
			Message:         "timeout",
			Count:           1,
			FirstHappenedAt: fourthAt,
			LastHappenedAt:  fourthAt,
		},
	}, got)
}
