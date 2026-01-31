package api

import (
	"context"
	"math/rand"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"golang.org/x/sync/singleflight"
)

type cacheEntry[T any] struct {
	items   []T
	fetched time.Time
}

type resourceCache struct {
	mu              sync.RWMutex
	podTTL          time.Duration
	appTTL          time.Duration
	crdTTL          time.Duration
	retryCount      int
	retryBase       time.Duration
	stats           *ResourceStats
	pods            map[string]cacheEntry[corev1.Pod]
	deployments     map[string]cacheEntry[appsv1.Deployment]
	statefuls       map[string]cacheEntry[appsv1.StatefulSet]
	cnpg            map[string]cacheEntry[cnpgCluster]
	dragonfly       map[string]cacheEntry[dragonflyResource]
	metaPods        map[string]cacheEntry[metav1.PartialObjectMetadata]
	metaDeps        map[string]cacheEntry[metav1.PartialObjectMetadata]
	metaSts         map[string]cacheEntry[metav1.PartialObjectMetadata]
	metaCnpg        map[string]cacheEntry[metav1.PartialObjectMetadata]
	metaDragon      map[string]cacheEntry[metav1.PartialObjectMetadata]
	metaCustom      map[string]cacheEntry[metav1.PartialObjectMetadata]
	podMetrics      map[string]cacheEntry[podMetricItem]
	podGroup        singleflight.Group
	depGroup        singleflight.Group
	stsGroup        singleflight.Group
	cnpgGroup       singleflight.Group
	dfGroup         singleflight.Group
	metaPodGroup    singleflight.Group
	metaDepGroup    singleflight.Group
	metaStsGroup    singleflight.Group
	metaCnpgGroup   singleflight.Group
	metaDfGroup     singleflight.Group
	metaCustomGroup singleflight.Group
	podMetricsGroup singleflight.Group
}

func newResourceCache(podTTL, appTTL, crdTTL time.Duration, retryCount int, retryBase time.Duration, stats *ResourceStats) *resourceCache {
	return &resourceCache{
		podTTL:      podTTL,
		appTTL:      appTTL,
		crdTTL:      crdTTL,
		retryCount:  retryCount,
		retryBase:   retryBase,
		stats:       stats,
		pods:        map[string]cacheEntry[corev1.Pod]{},
		deployments: map[string]cacheEntry[appsv1.Deployment]{},
		statefuls:   map[string]cacheEntry[appsv1.StatefulSet]{},
		cnpg:        map[string]cacheEntry[cnpgCluster]{},
		dragonfly:   map[string]cacheEntry[dragonflyResource]{},
		metaPods:    map[string]cacheEntry[metav1.PartialObjectMetadata]{},
		metaDeps:    map[string]cacheEntry[metav1.PartialObjectMetadata]{},
		metaSts:     map[string]cacheEntry[metav1.PartialObjectMetadata]{},
		metaCnpg:    map[string]cacheEntry[metav1.PartialObjectMetadata]{},
		metaDragon:  map[string]cacheEntry[metav1.PartialObjectMetadata]{},
		metaCustom:  map[string]cacheEntry[metav1.PartialObjectMetadata]{},
		podMetrics:  map[string]cacheEntry[podMetricItem]{},
	}
}

func (c *resourceCache) getPods(namespace string) ([]corev1.Pod, bool) {
	return getCache(c, c.pods, namespace, c.podTTL)
}

func (c *resourceCache) setPods(namespace string, items []corev1.Pod) {
	setCache(c, c.pods, namespace, items)
}

func (c *resourceCache) getDeployments(namespace string) ([]appsv1.Deployment, bool) {
	return getCache(c, c.deployments, namespace, c.appTTL)
}

func (c *resourceCache) setDeployments(namespace string, items []appsv1.Deployment) {
	setCache(c, c.deployments, namespace, items)
}

func (c *resourceCache) getStatefulSets(namespace string) ([]appsv1.StatefulSet, bool) {
	return getCache(c, c.statefuls, namespace, c.appTTL)
}

func (c *resourceCache) setStatefulSets(namespace string, items []appsv1.StatefulSet) {
	setCache(c, c.statefuls, namespace, items)
}

func (c *resourceCache) getCnpg(namespace string) ([]cnpgCluster, bool) {
	return getCache(c, c.cnpg, namespace, c.crdTTL)
}

func (c *resourceCache) setCnpg(namespace string, items []cnpgCluster) {
	setCache(c, c.cnpg, namespace, items)
}

func (c *resourceCache) getDragonfly(namespace string) ([]dragonflyResource, bool) {
	return getCache(c, c.dragonfly, namespace, c.crdTTL)
}

func (c *resourceCache) setDragonfly(namespace string, items []dragonflyResource) {
	setCache(c, c.dragonfly, namespace, items)
}

func (c *resourceCache) getMetaPods(namespace string) ([]metav1.PartialObjectMetadata, bool) {
	return getCache(c, c.metaPods, namespace, c.podTTL)
}

func (c *resourceCache) setMetaPods(namespace string, items []metav1.PartialObjectMetadata) {
	setCache(c, c.metaPods, namespace, items)
}

func (c *resourceCache) getMetaDeployments(namespace string) ([]metav1.PartialObjectMetadata, bool) {
	return getCache(c, c.metaDeps, namespace, c.appTTL)
}

func (c *resourceCache) setMetaDeployments(namespace string, items []metav1.PartialObjectMetadata) {
	setCache(c, c.metaDeps, namespace, items)
}

func (c *resourceCache) getMetaStatefulSets(namespace string) ([]metav1.PartialObjectMetadata, bool) {
	return getCache(c, c.metaSts, namespace, c.appTTL)
}

func (c *resourceCache) setMetaStatefulSets(namespace string, items []metav1.PartialObjectMetadata) {
	setCache(c, c.metaSts, namespace, items)
}

func (c *resourceCache) getMetaCnpg(namespace string) ([]metav1.PartialObjectMetadata, bool) {
	return getCache(c, c.metaCnpg, namespace, c.crdTTL)
}

func (c *resourceCache) setMetaCnpg(namespace string, items []metav1.PartialObjectMetadata) {
	setCache(c, c.metaCnpg, namespace, items)
}

func (c *resourceCache) getMetaDragonfly(namespace string) ([]metav1.PartialObjectMetadata, bool) {
	return getCache(c, c.metaDragon, namespace, c.crdTTL)
}

func (c *resourceCache) setMetaDragonfly(namespace string, items []metav1.PartialObjectMetadata) {
	setCache(c, c.metaDragon, namespace, items)
}

func (c *resourceCache) getMetaCustom(key string) ([]metav1.PartialObjectMetadata, bool) {
	return getCache(c, c.metaCustom, key, c.crdTTL)
}

func (c *resourceCache) setMetaCustom(key string, items []metav1.PartialObjectMetadata) {
	setCache(c, c.metaCustom, key, items)
}

func (c *resourceCache) getPodMetrics(namespace string) ([]podMetricItem, bool) {
	return getCache(c, c.podMetrics, namespace, c.podTTL)
}

func (c *resourceCache) setPodMetrics(namespace string, items []podMetricItem) {
	setCache(c, c.podMetrics, namespace, items)
}

func getCache[T any](cache *resourceCache, store map[string]cacheEntry[T], namespace string, ttl time.Duration) ([]T, bool) {
	cache.mu.RLock()
	entry, ok := store[namespace]
	cache.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if ttl > 0 && time.Since(entry.fetched) > ttl {
		return nil, false
	}
	return entry.items, true
}

func setCache[T any](cache *resourceCache, store map[string]cacheEntry[T], namespace string, items []T) {
	cache.mu.Lock()
	store[namespace] = cacheEntry[T]{items: items, fetched: time.Now()}
	cache.mu.Unlock()
}

func (c *resourceCache) doPods(namespace string, fn func() ([]corev1.Pod, error)) ([]corev1.Pod, error) {
	v, err, _ := c.podGroup.Do(namespace, func() (any, error) {
		return fn()
	})
	if err != nil {
		return nil, err
	}
	items, _ := v.([]corev1.Pod)
	return items, nil
}

func (c *resourceCache) doDeployments(namespace string, fn func() ([]appsv1.Deployment, error)) ([]appsv1.Deployment, error) {
	v, err, _ := c.depGroup.Do(namespace, func() (any, error) {
		return fn()
	})
	if err != nil {
		return nil, err
	}
	items, _ := v.([]appsv1.Deployment)
	return items, nil
}

func (c *resourceCache) doStatefulSets(namespace string, fn func() ([]appsv1.StatefulSet, error)) ([]appsv1.StatefulSet, error) {
	v, err, _ := c.stsGroup.Do(namespace, func() (any, error) {
		return fn()
	})
	if err != nil {
		return nil, err
	}
	items, _ := v.([]appsv1.StatefulSet)
	return items, nil
}

func (c *resourceCache) doCnpg(namespace string, fn func() ([]cnpgCluster, error)) ([]cnpgCluster, error) {
	v, err, _ := c.cnpgGroup.Do(namespace, func() (any, error) {
		return fn()
	})
	if err != nil {
		return nil, err
	}
	items, _ := v.([]cnpgCluster)
	return items, nil
}

func (c *resourceCache) doDragonfly(namespace string, fn func() ([]dragonflyResource, error)) ([]dragonflyResource, error) {
	v, err, _ := c.dfGroup.Do(namespace, func() (any, error) {
		return fn()
	})
	if err != nil {
		return nil, err
	}
	items, _ := v.([]dragonflyResource)
	return items, nil
}

func (c *resourceCache) doMetaPods(namespace string, fn func() ([]metav1.PartialObjectMetadata, error)) ([]metav1.PartialObjectMetadata, error) {
	v, err, _ := c.metaPodGroup.Do(namespace, func() (any, error) {
		return fn()
	})
	if err != nil {
		return nil, err
	}
	items, _ := v.([]metav1.PartialObjectMetadata)
	return items, nil
}

func (c *resourceCache) doMetaDeployments(namespace string, fn func() ([]metav1.PartialObjectMetadata, error)) ([]metav1.PartialObjectMetadata, error) {
	v, err, _ := c.metaDepGroup.Do(namespace, func() (any, error) {
		return fn()
	})
	if err != nil {
		return nil, err
	}
	items, _ := v.([]metav1.PartialObjectMetadata)
	return items, nil
}

func (c *resourceCache) doMetaStatefulSets(namespace string, fn func() ([]metav1.PartialObjectMetadata, error)) ([]metav1.PartialObjectMetadata, error) {
	v, err, _ := c.metaStsGroup.Do(namespace, func() (any, error) {
		return fn()
	})
	if err != nil {
		return nil, err
	}
	items, _ := v.([]metav1.PartialObjectMetadata)
	return items, nil
}

func (c *resourceCache) doMetaCnpg(namespace string, fn func() ([]metav1.PartialObjectMetadata, error)) ([]metav1.PartialObjectMetadata, error) {
	v, err, _ := c.metaCnpgGroup.Do(namespace, func() (any, error) {
		return fn()
	})
	if err != nil {
		return nil, err
	}
	items, _ := v.([]metav1.PartialObjectMetadata)
	return items, nil
}

func (c *resourceCache) doMetaDragonfly(namespace string, fn func() ([]metav1.PartialObjectMetadata, error)) ([]metav1.PartialObjectMetadata, error) {
	v, err, _ := c.metaDfGroup.Do(namespace, func() (any, error) {
		return fn()
	})
	if err != nil {
		return nil, err
	}
	items, _ := v.([]metav1.PartialObjectMetadata)
	return items, nil
}

func (c *resourceCache) doMetaCustom(key string, fn func() ([]metav1.PartialObjectMetadata, error)) ([]metav1.PartialObjectMetadata, error) {
	v, err, _ := c.metaCustomGroup.Do(key, func() (any, error) {
		return fn()
	})
	if err != nil {
		return nil, err
	}
	items, _ := v.([]metav1.PartialObjectMetadata)
	return items, nil
}

func (c *resourceCache) doPodMetrics(namespace string, fn func() ([]podMetricItem, error)) ([]podMetricItem, error) {
	v, err, _ := c.podMetricsGroup.Do(namespace, func() (any, error) {
		return fn()
	})
	if err != nil {
		return nil, err
	}
	items, _ := v.([]podMetricItem)
	return items, nil
}

func listPods(ctx context.Context, client *kubernetes.Clientset, namespace string, cache *resourceCache) ([]corev1.Pod, error) {
	var pods *corev1.PodList
	if cache != nil && cache.stats != nil {
		cache.stats.incPodsAPICall()
	}
	err := retryK8s(ctx, cache, func(ctx context.Context) error {
		var err error
		pods, err = client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		return err
	})
	if err != nil {
		if cache != nil && cache.stats != nil {
			cache.stats.incPodsAPIErr()
		}
		return nil, err
	}
	if pods == nil {
		return []corev1.Pod{}, nil
	}
	return pods.Items, nil
}

func listDeployments(ctx context.Context, client *kubernetes.Clientset, namespace string, cache *resourceCache) ([]appsv1.Deployment, error) {
	var deployments *appsv1.DeploymentList
	if cache != nil && cache.stats != nil {
		cache.stats.incDepAPICall()
	}
	err := retryK8s(ctx, cache, func(ctx context.Context) error {
		var err error
		deployments, err = client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		return err
	})
	if err != nil {
		if cache != nil && cache.stats != nil {
			cache.stats.incDepAPIErr()
		}
		return nil, err
	}
	if deployments == nil {
		return []appsv1.Deployment{}, nil
	}
	return deployments.Items, nil
}

func listStatefulSets(ctx context.Context, client *kubernetes.Clientset, namespace string, cache *resourceCache) ([]appsv1.StatefulSet, error) {
	var sets *appsv1.StatefulSetList
	if cache != nil && cache.stats != nil {
		cache.stats.incStsAPICall()
	}
	err := retryK8s(ctx, cache, func(ctx context.Context) error {
		var err error
		sets, err = client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
		return err
	})
	if err != nil {
		if cache != nil && cache.stats != nil {
			cache.stats.incStsAPIErr()
		}
		return nil, err
	}
	if sets == nil {
		return []appsv1.StatefulSet{}, nil
	}
	return sets.Items, nil
}

func retryK8s(ctx context.Context, cache *resourceCache, fn func(context.Context) error) error {
	if cache == nil || cache.retryCount <= 1 || cache.retryBase <= 0 {
		return fn(ctx)
	}

	var lastErr error
	for attempt := 0; attempt < cache.retryCount; attempt++ {
		if err := fn(ctx); err != nil {
			lastErr = err
			if !apierrors.IsTooManyRequests(err) {
				return err
			}
			if cache != nil && cache.stats != nil {
				cache.stats.incThrottleRetry()
			}
			delay := cache.retryBase * time.Duration(1<<attempt)
			jitter := time.Duration(rand.Int63n(int64(cache.retryBase / 2)))
			wait := delay + jitter
			select {
			case <-time.After(wait):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			return nil
		}
	}
	return lastErr
}
