package api

import (
	"fmt"
	"net/http"
	"strings"
)

func MetricsHandler(statsProvider func() *ResourceStats) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		stats := (*ResourceStats)(nil)
		if statsProvider != nil {
			stats = statsProvider()
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		if stats == nil {
			_, _ = w.Write([]byte("# kubelens metrics unavailable\n"))
			return
		}

		snap := stats.snapshot()
		lines := []string{
			"# HELP kubelens_k8s_cache_hits_total Cache hits by resource.",
			"# TYPE kubelens_k8s_cache_hits_total counter",
			fmt.Sprintf("kubelens_k8s_cache_hits_total{resource=\"pods\"} %d", snap.podsCacheHit),
			fmt.Sprintf("kubelens_k8s_cache_hits_total{resource=\"deployments\"} %d", snap.depCacheHit),
			fmt.Sprintf("kubelens_k8s_cache_hits_total{resource=\"statefulsets\"} %d", snap.stsCacheHit),
			fmt.Sprintf("kubelens_k8s_cache_hits_total{resource=\"cnpg\"} %d", snap.cnpgCacheHit),
			fmt.Sprintf("kubelens_k8s_cache_hits_total{resource=\"dragonfly\"} %d", snap.dragonCacheHit),
			"# HELP kubelens_k8s_cache_misses_total Cache misses by resource.",
			"# TYPE kubelens_k8s_cache_misses_total counter",
			fmt.Sprintf("kubelens_k8s_cache_misses_total{resource=\"pods\"} %d", snap.podsCacheMiss),
			fmt.Sprintf("kubelens_k8s_cache_misses_total{resource=\"deployments\"} %d", snap.depCacheMiss),
			fmt.Sprintf("kubelens_k8s_cache_misses_total{resource=\"statefulsets\"} %d", snap.stsCacheMiss),
			fmt.Sprintf("kubelens_k8s_cache_misses_total{resource=\"cnpg\"} %d", snap.cnpgCacheMiss),
			fmt.Sprintf("kubelens_k8s_cache_misses_total{resource=\"dragonfly\"} %d", snap.dragonCacheMiss),
			"# HELP kubelens_k8s_informer_hits_total Informer hits by resource.",
			"# TYPE kubelens_k8s_informer_hits_total counter",
			fmt.Sprintf("kubelens_k8s_informer_hits_total{resource=\"pods\"} %d", snap.podsInformerHit),
			fmt.Sprintf("kubelens_k8s_informer_hits_total{resource=\"deployments\"} %d", snap.depInformerHit),
			fmt.Sprintf("kubelens_k8s_informer_hits_total{resource=\"statefulsets\"} %d", snap.stsInformerHit),
			"# HELP kubelens_k8s_api_calls_total API calls by resource.",
			"# TYPE kubelens_k8s_api_calls_total counter",
			fmt.Sprintf("kubelens_k8s_api_calls_total{resource=\"pods\"} %d", snap.podsAPICall),
			fmt.Sprintf("kubelens_k8s_api_calls_total{resource=\"deployments\"} %d", snap.depAPICall),
			fmt.Sprintf("kubelens_k8s_api_calls_total{resource=\"statefulsets\"} %d", snap.stsAPICall),
			fmt.Sprintf("kubelens_k8s_api_calls_total{resource=\"cnpg\"} %d", snap.cnpgAPICall),
			fmt.Sprintf("kubelens_k8s_api_calls_total{resource=\"dragonfly\"} %d", snap.dragonAPICall),
			"# HELP kubelens_k8s_api_errors_total API errors by resource.",
			"# TYPE kubelens_k8s_api_errors_total counter",
			fmt.Sprintf("kubelens_k8s_api_errors_total{resource=\"pods\"} %d", snap.podsAPIErr),
			fmt.Sprintf("kubelens_k8s_api_errors_total{resource=\"deployments\"} %d", snap.depAPIErr),
			fmt.Sprintf("kubelens_k8s_api_errors_total{resource=\"statefulsets\"} %d", snap.stsAPIErr),
			fmt.Sprintf("kubelens_k8s_api_errors_total{resource=\"cnpg\"} %d", snap.cnpgAPIErr),
			fmt.Sprintf("kubelens_k8s_api_errors_total{resource=\"dragonfly\"} %d", snap.dragonAPIErr),
			"# HELP kubelens_k8s_throttle_retries_total Retry attempts due to apiserver throttling.",
			"# TYPE kubelens_k8s_throttle_retries_total counter",
			fmt.Sprintf("kubelens_k8s_throttle_retries_total %d", snap.throttleRetries),
		}
		_, _ = w.Write([]byte(strings.Join(lines, "\n") + "\n"))
	}
}
