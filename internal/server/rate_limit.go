package server

import (
	"sync"
	"time"
)

const defaultRateLimitMaxKeys = 4096

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	maxKeys int
	now     func() time.Time
	records map[string]rateLimitRecord
}

type rateLimitRecord struct {
	resetAt time.Time
	count   int
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:   limit,
		window:  window,
		maxKeys: defaultRateLimitMaxKeys,
		now:     time.Now,
		records: make(map[string]rateLimitRecord),
	}
}

func (l *rateLimiter) allow(key string) bool {
	now := l.now().UTC()
	l.mu.Lock()
	defer l.mu.Unlock()

	l.pruneExpired(now)
	record, ok := l.records[key]
	if !ok {
		if len(l.records) >= l.maxKeys {
			return false
		}
		l.records[key] = rateLimitRecord{resetAt: now.Add(l.window), count: 1}
		return true
	}

	if record.count >= l.limit {
		return false
	}
	record.count++
	l.records[key] = record
	return true
}

func (l *rateLimiter) pruneExpired(now time.Time) {
	for key, record := range l.records {
		if !now.Before(record.resetAt) {
			delete(l.records, key)
		}
	}
}
