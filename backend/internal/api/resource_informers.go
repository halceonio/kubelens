package api

import (
	"sync"
	"sync/atomic"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	appinformers "k8s.io/client-go/informers/apps/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type namespaceInformers struct {
	namespace   string
	factory     informers.SharedInformerFactory
	podInformer coreinformers.PodInformer
	depInformer appinformers.DeploymentInformer
	stsInformer appinformers.StatefulSetInformer
	stop        chan struct{}
	cacheSynced atomic.Bool
}

type resourceInformers struct {
	mu         sync.RWMutex
	namespaces map[string]*namespaceInformers
}

func newResourceInformers(client *kubernetes.Clientset, namespaces []string, resync time.Duration) *resourceInformers {
	ri := &resourceInformers{
		namespaces: make(map[string]*namespaceInformers, len(namespaces)),
	}
	for _, ns := range namespaces {
		factory := informers.NewSharedInformerFactoryWithOptions(client, resync, informers.WithNamespace(ns))
		nsInf := &namespaceInformers{
			namespace:   ns,
			factory:     factory,
			podInformer: factory.Core().V1().Pods(),
			depInformer: factory.Apps().V1().Deployments(),
			stsInformer: factory.Apps().V1().StatefulSets(),
			stop:        make(chan struct{}),
		}
		ri.namespaces[ns] = nsInf
	}
	return ri
}

func (r *resourceInformers) Start() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, nsInf := range r.namespaces {
		nsInf.factory.Start(nsInf.stop)
		go func(inf *namespaceInformers) {
			synced := cache.WaitForCacheSync(
				inf.stop,
				inf.podInformer.Informer().HasSynced,
				inf.depInformer.Informer().HasSynced,
				inf.stsInformer.Informer().HasSynced,
			)
			if synced {
				inf.cacheSynced.Store(true)
			}
		}(nsInf)
	}
}

func (r *resourceInformers) Stop() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, nsInf := range r.namespaces {
		select {
		case <-nsInf.stop:
		default:
			close(nsInf.stop)
		}
	}
}

func (r *resourceInformers) listPods(namespace string) ([]corev1.Pod, bool) {
	nsInf := r.getNamespace(namespace)
	if nsInf == nil || !nsInf.cacheSynced.Load() {
		return nil, false
	}
	items, err := nsInf.podInformer.Lister().List(labels.Everything())
	if err != nil {
		return nil, false
	}
	return derefPods(items), true
}

func (r *resourceInformers) listPodsBySelector(namespace string, selector labels.Selector) ([]corev1.Pod, bool) {
	nsInf := r.getNamespace(namespace)
	if nsInf == nil || !nsInf.cacheSynced.Load() {
		return nil, false
	}
	items, err := nsInf.podInformer.Lister().List(selector)
	if err != nil {
		return nil, false
	}
	return derefPods(items), true
}

func (r *resourceInformers) listDeployments(namespace string) ([]appsv1.Deployment, bool) {
	nsInf := r.getNamespace(namespace)
	if nsInf == nil || !nsInf.cacheSynced.Load() {
		return nil, false
	}
	items, err := nsInf.depInformer.Lister().List(labels.Everything())
	if err != nil {
		return nil, false
	}
	return derefDeployments(items), true
}

func (r *resourceInformers) listStatefulSets(namespace string) ([]appsv1.StatefulSet, bool) {
	nsInf := r.getNamespace(namespace)
	if nsInf == nil || !nsInf.cacheSynced.Load() {
		return nil, false
	}
	items, err := nsInf.stsInformer.Lister().List(labels.Everything())
	if err != nil {
		return nil, false
	}
	return derefStatefulSets(items), true
}

func (r *resourceInformers) getNamespace(namespace string) *namespaceInformers {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.namespaces[namespace]
}

func derefPods(items []*corev1.Pod) []corev1.Pod {
	out := make([]corev1.Pod, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, *item)
	}
	return out
}

func derefDeployments(items []*appsv1.Deployment) []appsv1.Deployment {
	out := make([]appsv1.Deployment, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, *item)
	}
	return out
}

func derefStatefulSets(items []*appsv1.StatefulSet) []appsv1.StatefulSet {
	out := make([]appsv1.StatefulSet, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, *item)
	}
	return out
}
