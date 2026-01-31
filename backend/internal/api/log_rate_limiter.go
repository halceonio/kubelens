package api

import (
	"sync"
	"time"
)

type logLimiter struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	buckets map[string]*tokenBucket
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

func newLogLimiter(ratePerMinute int, burst int) *logLimiter {
	if ratePerMinute <= 0 {
		return nil
	}
	if burst <= 0 {
		burst = ratePerMinute
	}
	return &logLimiter{
		rate:    implementRate(ratePerMinute),
		burst:   float64(burst),
		buckets: map[string]*tokenBucket{},
	}
}

func implementRate(ratePerMinute int) float64 {
	return float64(ratePerMinute) / 60.0
}

func (l *logLimiter) Allow(key string) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, ok := l.buckets[key]
	if !ok {
		bucket = &tokenBucket{tokens: l.burst, last: time.Now()}
		l.buckets[key] = bucket
	}

	now := time.Now()
	elapsed := now.Sub(bucket.last).Seconds()
	bucket.last = now

	bucket.tokens += elapsed * l.rate
	if bucket.tokens > l.burst {
		bucket.tokens = l.burst
	}
	if bucket.tokens < 1 {
		return false
	}
	bucket.tokens -= 1
	return true
}
