package api

import (
	"context"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	podCacheTTL   = 2 * time.Second
	appCacheTTL   = 5 * time.Second
	crdCacheTTL   = 10 * time.Second
	cacheMinItems = 0
)

type cacheEntry[T any] struct {
	items   []T
	fetched time.Time
}

type resourceCache struct {
	mu          sync.RWMutex
	pods        map[string]cacheEntry[corev1.Pod]
	deployments map[string]cacheEntry[appsv1.Deployment]
	statefuls   map[string]cacheEntry[appsv1.StatefulSet]
	cnpg        map[string]cacheEntry[cnpgCluster]
	dragonfly   map[string]cacheEntry[dragonflyResource]
}

func newResourceCache() *resourceCache {
	return &resourceCache{
		pods:        map[string]cacheEntry[corev1.Pod]{},
		deployments: map[string]cacheEntry[appsv1.Deployment]{},
		statefuls:   map[string]cacheEntry[appsv1.StatefulSet]{},
		cnpg:        map[string]cacheEntry[cnpgCluster]{},
		dragonfly:   map[string]cacheEntry[dragonflyResource]{},
	}
}

func (c *resourceCache) getPods(namespace string) ([]corev1.Pod, bool) {
	c.mu.RLock()
	entry, ok := c.pods[namespace]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Since(entry.fetched) > podCacheTTL {
		return nil, false
	}
	return entry.items, true
}

func (c *resourceCache) setPods(namespace string, items []corev1.Pod) {
	c.mu.Lock()
	c.pods[namespace] = cacheEntry[corev1.Pod]{items: items, fetched: time.Now()}
	c.mu.Unlock()
}

func (c *resourceCache) getDeployments(namespace string) ([]appsv1.Deployment, bool) {
	c.mu.RLock()
	entry, ok := c.deployments[namespace]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Since(entry.fetched) > appCacheTTL {
		return nil, false
	}
	return entry.items, true
}

func (c *resourceCache) setDeployments(namespace string, items []appsv1.Deployment) {
	c.mu.Lock()
	c.deployments[namespace] = cacheEntry[appsv1.Deployment]{items: items, fetched: time.Now()}
	c.mu.Unlock()
}

func (c *resourceCache) getStatefulSets(namespace string) ([]appsv1.StatefulSet, bool) {
	c.mu.RLock()
	entry, ok := c.statefuls[namespace]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Since(entry.fetched) > appCacheTTL {
		return nil, false
	}
	return entry.items, true
}

func (c *resourceCache) setStatefulSets(namespace string, items []appsv1.StatefulSet) {
	c.mu.Lock()
	c.statefuls[namespace] = cacheEntry[appsv1.StatefulSet]{items: items, fetched: time.Now()}
	c.mu.Unlock()
}

func (c *resourceCache) getCnpg(namespace string) ([]cnpgCluster, bool) {
	c.mu.RLock()
	entry, ok := c.cnpg[namespace]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Since(entry.fetched) > crdCacheTTL {
		return nil, false
	}
	return entry.items, true
}

func (c *resourceCache) setCnpg(namespace string, items []cnpgCluster) {
	c.mu.Lock()
	c.cnpg[namespace] = cacheEntry[cnpgCluster]{items: items, fetched: time.Now()}
	c.mu.Unlock()
}

func (c *resourceCache) getDragonfly(namespace string) ([]dragonflyResource, bool) {
	c.mu.RLock()
	entry, ok := c.dragonfly[namespace]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if time.Since(entry.fetched) > crdCacheTTL {
		return nil, false
	}
	return entry.items, true
}

func (c *resourceCache) setDragonfly(namespace string, items []dragonflyResource) {
	c.mu.Lock()
	c.dragonfly[namespace] = cacheEntry[dragonflyResource]{items: items, fetched: time.Now()}
	c.mu.Unlock()
}

func listPods(ctx context.Context, client *kubernetes.Clientset, namespace string) ([]corev1.Pod, error) {
	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if pods == nil {
		return []corev1.Pod{}, nil
	}
	if len(pods.Items) == cacheMinItems {
		return []corev1.Pod{}, nil
	}
	return pods.Items, nil
}

func listDeployments(ctx context.Context, client *kubernetes.Clientset, namespace string) ([]appsv1.Deployment, error) {
	deployments, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if deployments == nil || len(deployments.Items) == cacheMinItems {
		return []appsv1.Deployment{}, nil
	}
	return deployments.Items, nil
}

func listStatefulSets(ctx context.Context, client *kubernetes.Clientset, namespace string) ([]appsv1.StatefulSet, error) {
	sets, err := client.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	if sets == nil || len(sets.Items) == cacheMinItems {
		return []appsv1.StatefulSet{}, nil
	}
	return sets.Items, nil
}
