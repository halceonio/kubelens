package api

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"

	"github.com/halceonio/kubelens/backend/internal/config"
)

type KubeHandler struct {
	cfg        *config.Config
	client     *kubernetes.Clientset
	podInclude *regexp.Regexp
	appInclude *regexp.Regexp
	podExclude []labelFilter
	appExclude []labelFilter
	appStreams *appStreamPool
	cache      *resourceCache
	informers  *resourceInformers
	stats      *ResourceStats
	statsStop  chan struct{}
	metaClient metadata.Interface
	logHub     *logStreamHub
}

func NewKubeHandler(cfg *config.Config, client *kubernetes.Clientset, meta metadata.Interface) *KubeHandler {
	apiCache := cfg.Kubernetes.APICache
	podTTL := time.Duration(apiCache.PodListTTLSeconds) * time.Second
	appTTL := time.Duration(apiCache.AppListTTLSeconds) * time.Second
	crdTTL := time.Duration(apiCache.CRDListTTLSeconds) * time.Second
	retryBase := time.Duration(apiCache.RetryBaseDelayMillis) * time.Millisecond
	stats := newResourceStats()
	handler := &KubeHandler{
		cfg:        cfg,
		client:     client,
		podInclude: compileRegex(cfg.Kubernetes.PodFilters.IncludeRegex),
		appInclude: compileRegex(cfg.Kubernetes.AppFilters.IncludeRegex),
		podExclude: parseLabelFilters(cfg.Kubernetes.PodFilters.ExcludeLabels),
		appExclude: parseLabelFilters(cfg.Kubernetes.AppFilters.ExcludeLabels),
		cache:      newResourceCache(podTTL, appTTL, crdTTL, apiCache.RetryAttempts, retryBase, stats),
		stats:      stats,
		statsStop:  make(chan struct{}),
		metaClient: meta,
	}
	handler.logHub = newLogStreamHub(handler)
	handler.appStreams = newAppStreamPool(handler)
	if !apiCache.MetadataOnly && apiCache.EnableInformers != nil && *apiCache.EnableInformers && client != nil {
		resync := time.Duration(apiCache.InformerResyncSeconds) * time.Second
		handler.informers = newResourceInformers(client, cfg.Kubernetes.AllowedNamespaces, resync)
		handler.informers.Start()
	}
	handler.startStatsLogger()
	return handler
}

func (h *KubeHandler) Stop() {
	if h == nil {
		return
	}
	if h.statsStop != nil {
		close(h.statsStop)
	}
	if h.informers != nil {
		h.informers.Stop()
	}
	if h.logHub != nil {
		h.logHub.stop()
	}
}

func (h *KubeHandler) Stats() *ResourceStats {
	if h == nil {
		return nil
	}
	return h.stats
}

func (h *KubeHandler) startStatsLogger() {
	if h.stats == nil {
		return
	}
	ticker := time.NewTicker(60 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				h.stats.logSnapshot()
			case <-h.statsStop:
				ticker.Stop()
				return
			}
		}
	}()
}

func (h *KubeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/v1/namespaces" || r.URL.Path == "/api/v1/namespaces/" {
		h.handleNamespaces(w, r)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}

	ns := parts[0]
	if !h.isAllowedNamespace(ns) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}

	resourceType := parts[1]
	switch resourceType {
	case "pods":
		h.handlePods(w, r, ns, parts[2:])
	case "apps":
		h.handleApps(w, r, ns, parts[2:])
	default:
		http.NotFound(w, r)
	}
}

func (h *KubeHandler) streamPodLogs(w http.ResponseWriter, r *http.Request, namespace, name string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	req := parseLogRequest(r, h.cfg)
	sub, replay, unsubscribe, err := h.logHub.SubscribePod(r.Context(), namespace, name, req.container, req.tail, req.resume)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("log stream error: %v", err))
		return
	}
	defer unsubscribe()

	setSSEHeaders(w)
	flusher.Flush()

	for _, entry := range replay {
		if err := writeSSEEvent(w, newLogEvent(entry)); err != nil {
			return
		}
	}
	flusher.Flush()

	heartbeat := time.NewTicker(appStreamHeartbeatPeriod)
	defer heartbeat.Stop()
	statsTicker := time.NewTicker(appStreamStatsPeriod)
	defer statsTicker.Stop()
	statusPeriod := time.Duration(h.cfg.Logs.AppStreamResync) * time.Second
	if statusPeriod <= 0 {
		statusPeriod = 10 * time.Second
	}
	statusTicker := time.NewTicker(statusPeriod)
	defer statusTicker.Stop()

	var prevRestarts int32
	var prevReady bool
	if pod, err := h.client.CoreV1().Pods(namespace).Get(r.Context(), name, metav1.GetOptions{}); err == nil {
		prevRestarts, prevReady = summarizePodStatus(*pod)
	}

	sendMarker := func(kind, message string) error {
		event := newJSONEvent("marker", streamMarker{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			PodName:   name,
			Kind:      kind,
			Message:   message,
		})
		return writeSSEEvent(w, event)
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			event := newJSONEvent("heartbeat", streamHeartbeat{Timestamp: time.Now().UTC().Format(time.RFC3339Nano)})
			if err := writeSSEEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		case <-statsTicker.C:
			stats := streamStats{
				Dropped:  sub.dropped.Load(),
				Buffered: len(sub.ch),
			}
			event := newJSONEvent("stats", stats)
			if err := writeSSEEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		case <-statusTicker.C:
			pod, err := h.client.CoreV1().Pods(namespace).Get(r.Context(), name, metav1.GetOptions{})
			if err != nil {
				continue
			}
			restarts, ready := summarizePodStatus(*pod)
			if restarts > prevRestarts {
				msg := fmt.Sprintf("pod restart count increased: %d → %d", prevRestarts, restarts)
				if err := sendMarker("pod-restart", msg); err != nil {
					return
				}
				prevRestarts = restarts
			}
			if ready != prevReady {
				if ready {
					if err := sendMarker("pod-ready", "pod became ready"); err != nil {
						return
					}
				} else {
					if err := sendMarker("pod-not-ready", "pod became not ready"); err != nil {
						return
					}
				}
				prevReady = ready
			}
			flusher.Flush()
		case entry, ok := <-sub.ch:
			if !ok {
				return
			}
			if err := writeSSEEvent(w, newLogEvent(entry)); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (h *KubeHandler) streamAppLogs(w http.ResponseWriter, r *http.Request, namespace, name string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	opts := h.buildLogOptions(r)
	sub, unsubscribe, err := h.appStreams.subscribe(r.Context(), namespace, name, opts)
	if err != nil {
		status := http.StatusNotFound
		if !errors.Is(err, errAppNotFound) {
			status = http.StatusBadGateway
		}
		writeError(w, status, err.Error())
		return
	}
	defer unsubscribe()

	setSSEHeaders(w)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-sub.ch:
			if !ok {
				return
			}
			if err := writeSSEEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (h *KubeHandler) consumeLogStreamToChannel(ctx context.Context, stream ioReadCloser, podName, containerName string, ch chan<- logEntry) {
	reader := bufio.NewReader(stream)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		entry := h.parseLogLine(strings.TrimRight(line, "\n"), podName, containerName)
		select {
		case ch <- entry:
		case <-ctx.Done():
			return
		}
	}
}

type logEntry struct {
	ID            string `json:"id,omitempty"`
	Seq           uint64 `json:"seq,omitempty"`
	Timestamp     string `json:"timestamp"`
	Message       string `json:"message"`
	PodName       string `json:"podName"`
	ContainerName string `json:"containerName"`
}

type logResume struct {
	sinceID   string
	sinceTime *time.Time
}

type logRequest struct {
	container string
	tail      int64
	resume    logResume
}

func (h *KubeHandler) parseLogLine(line, podName, containerName string) logEntry {
	maxLen := h.cfg.Logs.MaxLineLength
	if maxLen <= 0 {
		maxLen = 10000
	}

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	message := line

	if idx := strings.IndexByte(line, ' '); idx > 0 {
		ts := line[:idx]
		if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			timestamp = parsed.UTC().Format(time.RFC3339Nano)
			message = strings.TrimSpace(line[idx+1:])
		} else if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
			timestamp = parsed.UTC().Format(time.RFC3339Nano)
			message = strings.TrimSpace(line[idx+1:])
		}
	}

	if len(message) > maxLen {
		message = message[:maxLen] + "...[truncated]"
	}

	return logEntry{
		Timestamp:     timestamp,
		Message:       message,
		PodName:       podName,
		ContainerName: containerName,
	}
}

func parseLogRequest(r *http.Request, cfg *config.Config) logRequest {
	tail := parseTailLines(r.URL.Query().Get("tail"), cfg.Logs.DefaultTailLines, cfg.Logs.MaxTailLines)
	container := r.URL.Query().Get("container")

	var sinceTime *time.Time
	if since := r.URL.Query().Get("since"); since != "" {
		if t, ok := parseLogTime(since); ok {
			sinceTime = &t
		}
	}

	lastID := r.URL.Query().Get("since_id")
	if lastID == "" {
		lastID = r.Header.Get("Last-Event-ID")
	}
	if sinceTime == nil && lastID != "" {
		if t, ok := parseLogTime(lastID); ok {
			sinceTime = &t
		}
	}

	return logRequest{
		container: container,
		tail:      tail,
		resume: logResume{
			sinceID:   lastID,
			sinceTime: sinceTime,
		},
	}
}

func parseLogTime(raw string) (time.Time, bool) {
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func (h *KubeHandler) buildLogOptions(r *http.Request) *corev1.PodLogOptions {
	tail := parseTailLines(r.URL.Query().Get("tail"), h.cfg.Logs.DefaultTailLines, h.cfg.Logs.MaxTailLines)

	opts := &corev1.PodLogOptions{
		Follow:     true,
		Timestamps: true,
		TailLines:  &tail,
	}

	if container := r.URL.Query().Get("container"); container != "" {
		opts.Container = container
	}

	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339Nano, since); err == nil {
			opts.SinceTime = &metav1.Time{Time: t}
		}
	} else if last := r.Header.Get("Last-Event-ID"); last != "" {
		if t, err := time.Parse(time.RFC3339Nano, last); err == nil {
			opts.SinceTime = &metav1.Time{Time: t}
		}
	}

	return opts
}

func parseTailLines(raw string, def int, max int) int64 {
	tail := def
	if raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			tail = parsed
		}
	}
	if max > 0 && tail > max {
		tail = max
	}
	if tail <= 0 {
		tail = def
	}
	return int64(tail)
}

func hashStrings(items []string) string {
	if len(items) == 0 {
		return ""
	}
	sorted := make([]string, len(items))
	copy(sorted, items)
	sort.Strings(sorted)
	sum := sha256.New()
	for _, item := range sorted {
		sum.Write([]byte(item))
		sum.Write([]byte{0})
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func (h *KubeHandler) isAllowedNamespace(ns string) bool {
	for _, allowed := range h.cfg.Kubernetes.AllowedNamespaces {
		if allowed == ns {
			return true
		}
	}
	return false
}

var errAppNotFound = errors.New("app not found")

func setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
}

type sseEvent struct {
	Event string
	ID    string
	Data  []byte
}

func writeSSE(w http.ResponseWriter, entry logEntry) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	id := entry.ID
	if id == "" {
		id = entry.Timestamp
	}
	return writeSSEEvent(w, sseEvent{Event: "log", ID: id, Data: payload})
}

func writeSSEEvent(w http.ResponseWriter, event sseEvent) error {
	if event.Event == "" {
		event.Event = "message"
	}
	if event.ID != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", event.ID); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event.Event); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", event.Data); err != nil {
		return err
	}
	return nil
}

type ioReadCloser interface {
	Read(p []byte) (n int, err error)
	Close() error
}
