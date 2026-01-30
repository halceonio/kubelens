package api

import (
	"log"
	"sync/atomic"
)

type resourceStats struct {
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

type resourceStatsSnapshot struct {
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

func newResourceStats() *resourceStats {
	return &resourceStats{}
}

func (s *resourceStats) incPodsCacheHit()  { atomic.AddInt64(&s.podsCacheHit, 1) }
func (s *resourceStats) incPodsCacheMiss() { atomic.AddInt64(&s.podsCacheMiss, 1) }
func (s *resourceStats) incPodsInformerHit() {
	atomic.AddInt64(&s.podsInformerHit, 1)
}
func (s *resourceStats) incPodsAPICall() { atomic.AddInt64(&s.podsAPICall, 1) }
func (s *resourceStats) incPodsAPIErr()  { atomic.AddInt64(&s.podsAPIErr, 1) }

func (s *resourceStats) incDepCacheHit()  { atomic.AddInt64(&s.depCacheHit, 1) }
func (s *resourceStats) incDepCacheMiss() { atomic.AddInt64(&s.depCacheMiss, 1) }
func (s *resourceStats) incDepInformerHit() {
	atomic.AddInt64(&s.depInformerHit, 1)
}
func (s *resourceStats) incDepAPICall() { atomic.AddInt64(&s.depAPICall, 1) }
func (s *resourceStats) incDepAPIErr()  { atomic.AddInt64(&s.depAPIErr, 1) }

func (s *resourceStats) incStsCacheHit()  { atomic.AddInt64(&s.stsCacheHit, 1) }
func (s *resourceStats) incStsCacheMiss() { atomic.AddInt64(&s.stsCacheMiss, 1) }
func (s *resourceStats) incStsInformerHit() {
	atomic.AddInt64(&s.stsInformerHit, 1)
}
func (s *resourceStats) incStsAPICall() { atomic.AddInt64(&s.stsAPICall, 1) }
func (s *resourceStats) incStsAPIErr()  { atomic.AddInt64(&s.stsAPIErr, 1) }

func (s *resourceStats) incCnpgCacheHit()  { atomic.AddInt64(&s.cnpgCacheHit, 1) }
func (s *resourceStats) incCnpgCacheMiss() { atomic.AddInt64(&s.cnpgCacheMiss, 1) }
func (s *resourceStats) incCnpgAPICall()   { atomic.AddInt64(&s.cnpgAPICall, 1) }
func (s *resourceStats) incCnpgAPIErr()    { atomic.AddInt64(&s.cnpgAPIErr, 1) }

func (s *resourceStats) incDragonCacheHit()  { atomic.AddInt64(&s.dragonCacheHit, 1) }
func (s *resourceStats) incDragonCacheMiss() { atomic.AddInt64(&s.dragonCacheMiss, 1) }
func (s *resourceStats) incDragonAPICall()   { atomic.AddInt64(&s.dragonAPICall, 1) }
func (s *resourceStats) incDragonAPIErr()    { atomic.AddInt64(&s.dragonAPIErr, 1) }

func (s *resourceStats) incThrottleRetry() { atomic.AddInt64(&s.throttleRetries, 1) }

func (s *resourceStats) snapshotAndReset() resourceStatsSnapshot {
	return resourceStatsSnapshot{
		podsCacheHit:    atomic.SwapInt64(&s.podsCacheHit, 0),
		podsCacheMiss:   atomic.SwapInt64(&s.podsCacheMiss, 0),
		podsInformerHit: atomic.SwapInt64(&s.podsInformerHit, 0),
		podsAPICall:     atomic.SwapInt64(&s.podsAPICall, 0),
		podsAPIErr:      atomic.SwapInt64(&s.podsAPIErr, 0),
		depCacheHit:     atomic.SwapInt64(&s.depCacheHit, 0),
		depCacheMiss:    atomic.SwapInt64(&s.depCacheMiss, 0),
		depInformerHit:  atomic.SwapInt64(&s.depInformerHit, 0),
		depAPICall:      atomic.SwapInt64(&s.depAPICall, 0),
		depAPIErr:       atomic.SwapInt64(&s.depAPIErr, 0),
		stsCacheHit:     atomic.SwapInt64(&s.stsCacheHit, 0),
		stsCacheMiss:    atomic.SwapInt64(&s.stsCacheMiss, 0),
		stsInformerHit:  atomic.SwapInt64(&s.stsInformerHit, 0),
		stsAPICall:      atomic.SwapInt64(&s.stsAPICall, 0),
		stsAPIErr:       atomic.SwapInt64(&s.stsAPIErr, 0),
		cnpgCacheHit:    atomic.SwapInt64(&s.cnpgCacheHit, 0),
		cnpgCacheMiss:   atomic.SwapInt64(&s.cnpgCacheMiss, 0),
		cnpgAPICall:     atomic.SwapInt64(&s.cnpgAPICall, 0),
		cnpgAPIErr:      atomic.SwapInt64(&s.cnpgAPIErr, 0),
		dragonCacheHit:  atomic.SwapInt64(&s.dragonCacheHit, 0),
		dragonCacheMiss: atomic.SwapInt64(&s.dragonCacheMiss, 0),
		dragonAPICall:   atomic.SwapInt64(&s.dragonAPICall, 0),
		dragonAPIErr:    atomic.SwapInt64(&s.dragonAPIErr, 0),
		throttleRetries: atomic.SwapInt64(&s.throttleRetries, 0),
	}
}

func (s resourceStatsSnapshot) total() int64 {
	return s.podsCacheHit + s.podsCacheMiss + s.podsInformerHit + s.podsAPICall + s.podsAPIErr +
		s.depCacheHit + s.depCacheMiss + s.depInformerHit + s.depAPICall + s.depAPIErr +
		s.stsCacheHit + s.stsCacheMiss + s.stsInformerHit + s.stsAPICall + s.stsAPIErr +
		s.cnpgCacheHit + s.cnpgCacheMiss + s.cnpgAPICall + s.cnpgAPIErr +
		s.dragonCacheHit + s.dragonCacheMiss + s.dragonAPICall + s.dragonAPIErr +
		s.throttleRetries
}

func (s *resourceStats) logSnapshot() {
	snap := s.snapshotAndReset()
	if snap.total() == 0 {
		return
	}
	log.Printf("k8s-cache stats (interval): pods hit/miss/informer %d/%d/%d api %d err %d | deps hit/miss/informer %d/%d/%d api %d err %d | sts hit/miss/informer %d/%d/%d api %d err %d | cnpg hit/miss %d/%d api %d err %d | dragonfly hit/miss %d/%d api %d err %d | throttled retries %d",
		snap.podsCacheHit, snap.podsCacheMiss, snap.podsInformerHit, snap.podsAPICall, snap.podsAPIErr,
		snap.depCacheHit, snap.depCacheMiss, snap.depInformerHit, snap.depAPICall, snap.depAPIErr,
		snap.stsCacheHit, snap.stsCacheMiss, snap.stsInformerHit, snap.stsAPICall, snap.stsAPIErr,
		snap.cnpgCacheHit, snap.cnpgCacheMiss, snap.cnpgAPICall, snap.cnpgAPIErr,
		snap.dragonCacheHit, snap.dragonCacheMiss, snap.dragonAPICall, snap.dragonAPIErr,
		snap.throttleRetries,
	)
}
