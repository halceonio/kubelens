package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
)

const (
	appStreamSubscriberBuffer = 256
	appStreamLogBuffer        = 512
	appStreamHeartbeatPeriod  = 15 * time.Second
	appStreamStatsPeriod      = 5 * time.Second
)

type appStreamPool struct {
	mu      sync.Mutex
	streams map[string]*appStream
	handler *KubeHandler
}

type appStream struct {
	key          string
	namespace    string
	name         string
	container    string
	tail         int64
	handler      *KubeHandler
	ctx          context.Context
	cancel       context.CancelFunc
	logCh        chan logEntry
	activePods   map[string]context.CancelFunc
	podStates    map[string]podState
	lastPodHash  string
	mu           sync.Mutex
	subscribers  map[string]*appSubscriber
	startOnce    sync.Once
	stopOnce     sync.Once
	resyncPeriod time.Duration
}

type appSubscriber struct {
	id      string
	ch      chan sseEvent
	dropped atomic.Int64
}

type podState struct {
	restarts int32
	ready    bool
}

type streamStats struct {
	Dropped  int64 `json:"dropped"`
	Buffered int   `json:"buffered"`
	Sources  int   `json:"sources"`
}

type streamMarker struct {
	Timestamp string `json:"timestamp"`
	PodName   string `json:"podName"`
	Kind      string `json:"kind"`
	Message   string `json:"message"`
}

type streamHeartbeat struct {
	Timestamp string `json:"timestamp"`
}

func newAppStreamPool(handler *KubeHandler) *appStreamPool {
	return &appStreamPool{
		streams: make(map[string]*appStream),
		handler: handler,
	}
}

func (p *appStreamPool) subscribe(ctx context.Context, namespace, name string, opts *corev1.PodLogOptions) (*appSubscriber, func(), error) {
	if opts == nil {
		return nil, nil, errors.New("log options missing")
	}
	key := fmt.Sprintf("%s/%s?container=%s&tail=%d", namespace, name, opts.Container, valueOrDefault(opts.TailLines, 0))

	p.mu.Lock()
	stream, ok := p.streams[key]
	if !ok {
		stream = newAppStream(p.handler, key, namespace, name, opts)
		p.streams[key] = stream
	}
	p.mu.Unlock()

	sub, unsubscribe := stream.subscribe(ctx)
	return sub, func() {
		unsubscribe()
		if stream.isIdle() {
			p.mu.Lock()
			if stream.isIdle() {
				delete(p.streams, key)
				stream.stop()
			}
			p.mu.Unlock()
		}
	}, nil
}

func newAppStream(handler *KubeHandler, key, namespace, name string, opts *corev1.PodLogOptions) *appStream {
	ctx, cancel := context.WithCancel(context.Background())
	resync := time.Duration(handler.cfg.Logs.AppStreamResync) * time.Second
	if resync <= 0 {
		resync = 10 * time.Second
	}
	stream := &appStream{
		key:          key,
		namespace:    namespace,
		name:         name,
		container:    opts.Container,
		tail:         valueOrDefault(opts.TailLines, 0),
		handler:      handler,
		ctx:          ctx,
		cancel:       cancel,
		logCh:        make(chan logEntry, appStreamLogBuffer),
		activePods:   make(map[string]context.CancelFunc),
		podStates:    make(map[string]podState),
		subscribers:  make(map[string]*appSubscriber),
		resyncPeriod: resync,
	}
	return stream
}

func (s *appStream) subscribe(ctx context.Context) (*appSubscriber, func()) {
	sub := &appSubscriber{
		id: fmt.Sprintf("%d", time.Now().UnixNano()),
		ch: make(chan sseEvent, appStreamSubscriberBuffer),
	}

	s.mu.Lock()
	s.subscribers[sub.id] = sub
	s.mu.Unlock()

	s.startOnce.Do(func() {
		go s.run()
	})

	unsubscribe := func() {
		s.mu.Lock()
		if existing, ok := s.subscribers[sub.id]; ok {
			delete(s.subscribers, sub.id)
			close(existing.ch)
		}
		s.mu.Unlock()
	}

	go func() {
		<-ctx.Done()
		unsubscribe()
	}()

	return sub, unsubscribe
}

func (s *appStream) isIdle() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.subscribers) == 0
}

func (s *appStream) stop() {
	s.stopOnce.Do(func() {
		s.cancel()
	})
}

func (s *appStream) run() {
	resyncTicker := time.NewTicker(s.resyncPeriod)
	heartbeatTicker := time.NewTicker(appStreamHeartbeatPeriod)
	statsTicker := time.NewTicker(appStreamStatsPeriod)
	defer resyncTicker.Stop()
	defer heartbeatTicker.Stop()
	defer statsTicker.Stop()

	if err := s.reconcilePods(true); err != nil {
		s.broadcastMarker("error", "", fmt.Sprintf("failed to resolve pods: %v", err))
	}

	for {
		select {
		case <-s.ctx.Done():
			s.shutdown()
			return
		case entry := <-s.logCh:
			s.broadcastEvent(newLogEvent(entry))
		case <-resyncTicker.C:
			if err := s.reconcilePods(false); err != nil {
				s.broadcastMarker("error", "", fmt.Sprintf("pod resync failed: %v", err))
			}
		case <-heartbeatTicker.C:
			s.broadcastHeartbeat()
		case <-statsTicker.C:
			s.broadcastStats()
		}
	}
}

func (s *appStream) shutdown() {
	s.mu.Lock()
	for _, cancel := range s.activePods {
		cancel()
	}
	for _, sub := range s.subscribers {
		close(sub.ch)
	}
	s.activePods = map[string]context.CancelFunc{}
	s.subscribers = map[string]*appSubscriber{}
	s.mu.Unlock()
}

func (s *appStream) reconcilePods(initial bool) error {
	selector, err := s.handler.appSelector(s.ctx, s.namespace, s.name)
	if err != nil {
		if errors.Is(err, errAppNotFound) {
			return err
		}
		return err
	}
	if selector == "" {
		return errAppNotFound
	}

	pods, err := s.handler.listPodsBySelectorCached(s.ctx, s.namespace, selector)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(pods))
	for _, pod := range pods {
		names = append(names, pod.Name)
	}
	newHash := hashStrings(names)
	changed := newHash != s.lastPodHash
	s.lastPodHash = newHash

	desired := make(map[string]corev1.Pod, len(pods))
	for _, pod := range pods {
		desired[pod.Name] = pod
	}

	if initial || changed || s.activePodCount() != len(desired) {
		s.syncPodStreams(desired)
	}
	s.emitPodMarkers(desired, initial)

	return nil
}

func (s *appStream) syncPodStreams(desired map[string]corev1.Pod) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for podName, cancel := range s.activePods {
		if _, ok := desired[podName]; !ok {
			cancel()
			delete(s.activePods, podName)
		}
	}

	for podName := range desired {
		if _, ok := s.activePods[podName]; ok {
			continue
		}
		streamCtx, streamCancel := context.WithCancel(s.ctx)
		s.activePods[podName] = streamCancel
		go s.consumePodStream(streamCtx, podName)
	}
}

func (s *appStream) consumePodStream(ctx context.Context, podName string) {
	defer s.markPodInactive(podName)
	sub, replay, unsubscribe, err := s.handler.logHub.SubscribePod(ctx, s.namespace, podName, s.container, s.tail, logResume{})
	if err != nil {
		return
	}
	defer unsubscribe()

	emit := func(entry logEntry) {
		select {
		case s.logCh <- entry:
		default:
		}
	}

	for _, entry := range replay {
		emit(entry)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-sub.ch:
			if !ok {
				return
			}
			emit(entry)
		}
	}
}

func (s *appStream) activePodCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.activePods)
}

func (s *appStream) markPodInactive(podName string) {
	s.mu.Lock()
	delete(s.activePods, podName)
	s.mu.Unlock()
}

func (s *appStream) emitPodMarkers(desired map[string]corev1.Pod, initial bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for podName := range s.podStates {
		if _, ok := desired[podName]; !ok {
			s.broadcastMarkerLocked("pod-removed", podName, "pod removed from app")
			delete(s.podStates, podName)
		}
	}

	for podName, pod := range desired {
		restarts, ready := summarizePodStatus(pod)
		prev, ok := s.podStates[podName]
		if !ok {
			s.podStates[podName] = podState{restarts: restarts, ready: ready}
			if !initial {
				s.broadcastMarkerLocked("pod-added", podName, "pod added to app")
			}
			continue
		}

		if restarts > prev.restarts {
			msg := fmt.Sprintf("pod restart count increased: %d → %d", prev.restarts, restarts)
			s.broadcastMarkerLocked("pod-restart", podName, msg)
			prev.restarts = restarts
		}

		if ready != prev.ready {
			if ready {
				s.broadcastMarkerLocked("pod-ready", podName, "pod became ready")
			} else {
				s.broadcastMarkerLocked("pod-not-ready", podName, "pod became not ready")
			}
			prev.ready = ready
		}

		s.podStates[podName] = prev
	}
}

func summarizePodStatus(pod corev1.Pod) (int32, bool) {
	var restarts int32
	for _, status := range pod.Status.ContainerStatuses {
		restarts += status.RestartCount
	}
	return restarts, isPodReady(&pod)
}

func isPodReady(pod *corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func (s *appStream) broadcastEvent(event sseEvent) {
	s.mu.Lock()
	for _, sub := range s.subscribers {
		select {
		case sub.ch <- event:
		default:
			sub.dropped.Add(1)
		}
	}
	s.mu.Unlock()
}

func (s *appStream) broadcastStats() {
	s.mu.Lock()
	sources := len(s.activePods)
	for _, sub := range s.subscribers {
		stats := streamStats{
			Dropped:  sub.dropped.Load(),
			Buffered: len(sub.ch),
			Sources:  sources,
		}
		event := newJSONEvent("stats", stats)
		select {
		case sub.ch <- event:
		default:
			sub.dropped.Add(1)
		}
	}
	s.mu.Unlock()
}

func (s *appStream) broadcastHeartbeat() {
	event := newJSONEvent("heartbeat", streamHeartbeat{Timestamp: time.Now().UTC().Format(time.RFC3339Nano)})
	s.broadcastEvent(event)
}

func (s *appStream) broadcastMarker(kind, podName, message string) {
	s.mu.Lock()
	s.broadcastMarkerLocked(kind, podName, message)
	s.mu.Unlock()
}

func (s *appStream) broadcastMarkerLocked(kind, podName, message string) {
	event := newJSONEvent("marker", streamMarker{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		PodName:   podName,
		Kind:      kind,
		Message:   message,
	})
	for _, sub := range s.subscribers {
		select {
		case sub.ch <- event:
		default:
			sub.dropped.Add(1)
		}
	}
}

func newJSONEvent(event string, payload any) sseEvent {
	data, _ := json.Marshal(payload)
	return sseEvent{
		Event: event,
		ID:    "",
		Data:  data,
	}
}

func newLogEvent(entry logEntry) sseEvent {
	data, _ := json.Marshal(entry)
	id := entry.ID
	if id == "" {
		id = entry.Timestamp
	}
	return sseEvent{
		Event: "log",
		ID:    id,
		Data:  data,
	}
}

func valueOrDefault(val *int64, def int64) int64 {
	if val == nil {
		return def
	}
	return *val
}
