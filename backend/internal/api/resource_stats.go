package api

import (
	"sync"
	"sync/atomic"

	"github.com/charmbracelet/log"
)

type ResourceStats struct {
	podsCacheHit    int64
	podsCacheMiss   int64
	podsInformerHit int64
	podsAPICall     int64
	podsAPIErr      int64
	depCacheHit     int64
	depCacheMiss    int64
	depInformerHit  int64
	depAPICall      int64
	depAPIErr       int64
	stsCacheHit     int64
	stsCacheMiss    int64
	stsInformerHit  int64
	stsAPICall      int64
	stsAPIErr       int64
	cnpgCacheHit    int64
	cnpgCacheMiss   int64
	cnpgAPICall     int64
	cnpgAPIErr      int64
	dragonCacheHit  int64
	dragonCacheMiss int64
	dragonAPICall   int64
	dragonAPIErr    int64
	throttleRetries int64
	mu              sync.Mutex
	last            ResourceStatsSnapshot
}

type ResourceStatsSnapshot struct {
	podsCacheHit    int64
	podsCacheMiss   int64
	podsInformerHit int64
	podsAPICall     int64
	podsAPIErr      int64
	depCacheHit     int64
	depCacheMiss    int64
	depInformerHit  int64
	depAPICall      int64
	depAPIErr       int64
	stsCacheHit     int64
	stsCacheMiss    int64
	stsInformerHit  int64
	stsAPICall      int64
	stsAPIErr       int64
	cnpgCacheHit    int64
	cnpgCacheMiss   int64
	cnpgAPICall     int64
	cnpgAPIErr      int64
	dragonCacheHit  int64
	dragonCacheMiss int64
	dragonAPICall   int64
	dragonAPIErr    int64
	throttleRetries int64
}

func newResourceStats() *ResourceStats {
	return &ResourceStats{}
}

func (s *ResourceStats) incPodsCacheHit()  { atomic.AddInt64(&s.podsCacheHit, 1) }
func (s *ResourceStats) incPodsCacheMiss() { atomic.AddInt64(&s.podsCacheMiss, 1) }
func (s *ResourceStats) incPodsInformerHit() {
	atomic.AddInt64(&s.podsInformerHit, 1)
}
func (s *ResourceStats) incPodsAPICall() { atomic.AddInt64(&s.podsAPICall, 1) }
func (s *ResourceStats) incPodsAPIErr()  { atomic.AddInt64(&s.podsAPIErr, 1) }

func (s *ResourceStats) incDepCacheHit()  { atomic.AddInt64(&s.depCacheHit, 1) }
func (s *ResourceStats) incDepCacheMiss() { atomic.AddInt64(&s.depCacheMiss, 1) }
func (s *ResourceStats) incDepInformerHit() {
	atomic.AddInt64(&s.depInformerHit, 1)
}
func (s *ResourceStats) incDepAPICall() { atomic.AddInt64(&s.depAPICall, 1) }
func (s *ResourceStats) incDepAPIErr()  { atomic.AddInt64(&s.depAPIErr, 1) }

func (s *ResourceStats) incStsCacheHit()  { atomic.AddInt64(&s.stsCacheHit, 1) }
func (s *ResourceStats) incStsCacheMiss() { atomic.AddInt64(&s.stsCacheMiss, 1) }
func (s *ResourceStats) incStsInformerHit() {
	atomic.AddInt64(&s.stsInformerHit, 1)
}
func (s *ResourceStats) incStsAPICall() { atomic.AddInt64(&s.stsAPICall, 1) }
func (s *ResourceStats) incStsAPIErr()  { atomic.AddInt64(&s.stsAPIErr, 1) }

func (s *ResourceStats) incCnpgCacheHit()  { atomic.AddInt64(&s.cnpgCacheHit, 1) }
func (s *ResourceStats) incCnpgCacheMiss() { atomic.AddInt64(&s.cnpgCacheMiss, 1) }
func (s *ResourceStats) incCnpgAPICall()   { atomic.AddInt64(&s.cnpgAPICall, 1) }
func (s *ResourceStats) incCnpgAPIErr()    { atomic.AddInt64(&s.cnpgAPIErr, 1) }

func (s *ResourceStats) incDragonCacheHit()  { atomic.AddInt64(&s.dragonCacheHit, 1) }
func (s *ResourceStats) incDragonCacheMiss() { atomic.AddInt64(&s.dragonCacheMiss, 1) }
func (s *ResourceStats) incDragonAPICall()   { atomic.AddInt64(&s.dragonAPICall, 1) }
func (s *ResourceStats) incDragonAPIErr()    { atomic.AddInt64(&s.dragonAPIErr, 1) }

func (s *ResourceStats) incThrottleRetry() { atomic.AddInt64(&s.throttleRetries, 1) }

func (s *ResourceStats) snapshot() ResourceStatsSnapshot {
	return ResourceStatsSnapshot{
		podsCacheHit:    atomic.LoadInt64(&s.podsCacheHit),
		podsCacheMiss:   atomic.LoadInt64(&s.podsCacheMiss),
		podsInformerHit: atomic.LoadInt64(&s.podsInformerHit),
		podsAPICall:     atomic.LoadInt64(&s.podsAPICall),
		podsAPIErr:      atomic.LoadInt64(&s.podsAPIErr),
		depCacheHit:     atomic.LoadInt64(&s.depCacheHit),
		depCacheMiss:    atomic.LoadInt64(&s.depCacheMiss),
		depInformerHit:  atomic.LoadInt64(&s.depInformerHit),
		depAPICall:      atomic.LoadInt64(&s.depAPICall),
		depAPIErr:       atomic.LoadInt64(&s.depAPIErr),
		stsCacheHit:     atomic.LoadInt64(&s.stsCacheHit),
		stsCacheMiss:    atomic.LoadInt64(&s.stsCacheMiss),
		stsInformerHit:  atomic.LoadInt64(&s.stsInformerHit),
		stsAPICall:      atomic.LoadInt64(&s.stsAPICall),
		stsAPIErr:       atomic.LoadInt64(&s.stsAPIErr),
		cnpgCacheHit:    atomic.LoadInt64(&s.cnpgCacheHit),
		cnpgCacheMiss:   atomic.LoadInt64(&s.cnpgCacheMiss),
		cnpgAPICall:     atomic.LoadInt64(&s.cnpgAPICall),
		cnpgAPIErr:      atomic.LoadInt64(&s.cnpgAPIErr),
		dragonCacheHit:  atomic.LoadInt64(&s.dragonCacheHit),
		dragonCacheMiss: atomic.LoadInt64(&s.dragonCacheMiss),
		dragonAPICall:   atomic.LoadInt64(&s.dragonAPICall),
		dragonAPIErr:    atomic.LoadInt64(&s.dragonAPIErr),
		throttleRetries: atomic.LoadInt64(&s.throttleRetries),
	}
}

func (s ResourceStatsSnapshot) total() int64 {
	return s.podsCacheHit + s.podsCacheMiss + s.podsInformerHit + s.podsAPICall + s.podsAPIErr +
		s.depCacheHit + s.depCacheMiss + s.depInformerHit + s.depAPICall + s.depAPIErr +
		s.stsCacheHit + s.stsCacheMiss + s.stsInformerHit + s.stsAPICall + s.stsAPIErr +
		s.cnpgCacheHit + s.cnpgCacheMiss + s.cnpgAPICall + s.cnpgAPIErr +
		s.dragonCacheHit + s.dragonCacheMiss + s.dragonAPICall + s.dragonAPIErr +
		s.throttleRetries
}

func (s *ResourceStats) logSnapshot() {
	snap := s.snapshot()
	s.mu.Lock()
	delta := snap.diff(s.last)
	s.last = snap
	s.mu.Unlock()
	if delta.total() == 0 {
		return
	}
	log.Info("k8s cache stats (interval)",
		"pods_hit", delta.podsCacheHit,
		"pods_miss", delta.podsCacheMiss,
		"pods_informer", delta.podsInformerHit,
		"pods_api", delta.podsAPICall,
		"pods_err", delta.podsAPIErr,
		"deploy_hit", delta.depCacheHit,
		"deploy_miss", delta.depCacheMiss,
		"deploy_informer", delta.depInformerHit,
		"deploy_api", delta.depAPICall,
		"deploy_err", delta.depAPIErr,
		"sts_hit", delta.stsCacheHit,
		"sts_miss", delta.stsCacheMiss,
		"sts_informer", delta.stsInformerHit,
		"sts_api", delta.stsAPICall,
		"sts_err", delta.stsAPIErr,
		"cnpg_hit", delta.cnpgCacheHit,
		"cnpg_miss", delta.cnpgCacheMiss,
		"cnpg_api", delta.cnpgAPICall,
		"cnpg_err", delta.cnpgAPIErr,
		"dragon_hit", delta.dragonCacheHit,
		"dragon_miss", delta.dragonCacheMiss,
		"dragon_api", delta.dragonAPICall,
		"dragon_err", delta.dragonAPIErr,
		"throttle_retries", delta.throttleRetries,
	)
}

func (s ResourceStatsSnapshot) diff(prev ResourceStatsSnapshot) ResourceStatsSnapshot {
	return ResourceStatsSnapshot{
		podsCacheHit:    s.podsCacheHit - prev.podsCacheHit,
		podsCacheMiss:   s.podsCacheMiss - prev.podsCacheMiss,
		podsInformerHit: s.podsInformerHit - prev.podsInformerHit,
		podsAPICall:     s.podsAPICall - prev.podsAPICall,
		podsAPIErr:      s.podsAPIErr - prev.podsAPIErr,
		depCacheHit:     s.depCacheHit - prev.depCacheHit,
		depCacheMiss:    s.depCacheMiss - prev.depCacheMiss,
		depInformerHit:  s.depInformerHit - prev.depInformerHit,
		depAPICall:      s.depAPICall - prev.depAPICall,
		depAPIErr:       s.depAPIErr - prev.depAPIErr,
		stsCacheHit:     s.stsCacheHit - prev.stsCacheHit,
		stsCacheMiss:    s.stsCacheMiss - prev.stsCacheMiss,
		stsInformerHit:  s.stsInformerHit - prev.stsInformerHit,
		stsAPICall:      s.stsAPICall - prev.stsAPICall,
		stsAPIErr:       s.stsAPIErr - prev.stsAPIErr,
		cnpgCacheHit:    s.cnpgCacheHit - prev.cnpgCacheHit,
		cnpgCacheMiss:   s.cnpgCacheMiss - prev.cnpgCacheMiss,
		cnpgAPICall:     s.cnpgAPICall - prev.cnpgAPICall,
		cnpgAPIErr:      s.cnpgAPIErr - prev.cnpgAPIErr,
		dragonCacheHit:  s.dragonCacheHit - prev.dragonCacheHit,
		dragonCacheMiss: s.dragonCacheMiss - prev.dragonCacheMiss,
		dragonAPICall:   s.dragonAPICall - prev.dragonAPICall,
		dragonAPIErr:    s.dragonAPIErr - prev.dragonAPIErr,
		throttleRetries: s.throttleRetries - prev.throttleRetries,
	}
}
