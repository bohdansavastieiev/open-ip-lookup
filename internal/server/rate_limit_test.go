package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_AllowsUpToLimitPerWindow(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	limiter := newRateLimiter(2, time.Minute)
	limiter.now = func() time.Time { return now }

	assert.True(t, limiter.allow("client"))
	assert.True(t, limiter.allow("client"))
	assert.False(t, limiter.allow("client"))

	now = now.Add(time.Minute)
	assert.True(t, limiter.allow("client"))
}

func TestRateLimiter_IsolatesClients(t *testing.T) {
	limiter := newRateLimiter(1, time.Minute)

	assert.True(t, limiter.allow("client-1"))
	assert.False(t, limiter.allow("client-1"))
	assert.True(t, limiter.allow("client-2"))
}

func TestRateLimiter_RejectsNewClientsAtCapacity(t *testing.T) {
	limiter := newRateLimiter(1, time.Minute)
	limiter.maxKeys = 2

	assert.True(t, limiter.allow("client-1"))
	assert.True(t, limiter.allow("client-2"))
	assert.False(t, limiter.allow("client-3"))
	assert.False(t, limiter.allow("client-1"))
}
