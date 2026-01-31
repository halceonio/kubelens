package api

import (
	"sync"
	"time"
)

type logLimiter struct {
	mu           sync.Mutex
	defaultRate  float64
	defaultBurst float64
	overrides    map[string]rateConfig
	buckets      map[string]*tokenBucket
}

type tokenBucket struct {
	tokens float64
	last   time.Time
}

type rateConfig struct {
	rate  float64
	burst float64
}

func newLogLimiter(ratePerMinute int, burst int, overrides []rateConfigEntry) *logLimiter {
	hasDefault := ratePerMinute > 0
	hasOverrides := len(overrides) > 0
	if !hasDefault && !hasOverrides {
		return nil
	}
	if burst <= 0 {
		burst = ratePerMinute
	}
	configOverrides := map[string]rateConfig{}
	for _, entry := range overrides {
		if entry.namespace == "" || entry.ratePerMinute <= 0 {
			continue
		}
		overrideBurst := entry.burst
		if overrideBurst <= 0 {
			overrideBurst = entry.ratePerMinute
		}
		configOverrides[entry.namespace] = rateConfig{
			rate:  implementRate(entry.ratePerMinute),
			burst: float64(overrideBurst),
		}
	}
	return &logLimiter{
		defaultRate:  implementRate(ratePerMinute),
		defaultBurst: float64(burst),
		overrides:    configOverrides,
		buckets:      map[string]*tokenBucket{},
	}
}

func implementRate(ratePerMinute int) float64 {
	return float64(ratePerMinute) / 60.0
}

func (l *logLimiter) Allow(namespace, key string) bool {
	if l == nil {
		return true
	}
	cfg := l.lookupConfig(namespace)
	if cfg.rate <= 0 {
		return true
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	bucketKey := namespace + "|" + key
	bucket, ok := l.buckets[bucketKey]
	if !ok {
		bucket = &tokenBucket{tokens: cfg.burst, last: time.Now()}
		l.buckets[bucketKey] = bucket
	}

	now := time.Now()
	elapsed := now.Sub(bucket.last).Seconds()
	bucket.last = now

	bucket.tokens += elapsed * cfg.rate
	if bucket.tokens > cfg.burst {
		bucket.tokens = cfg.burst
	}
	if bucket.tokens < 1 {
		return false
	}
	bucket.tokens -= 1
	return true
}

func (l *logLimiter) lookupConfig(namespace string) rateConfig {
	if l == nil {
		return rateConfig{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if cfg, ok := l.overrides[namespace]; ok {
		return cfg
	}
	return rateConfig{rate: l.defaultRate, burst: l.defaultBurst}
}

type rateConfigEntry struct {
	namespace     string
	ratePerMinute int
	burst         int
}
