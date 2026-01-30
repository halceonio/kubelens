package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/halceon/kubelens/backend/internal/config"
)

type KubeHandler struct {
	cfg    *config.Config
	client *kubernetes.Clientset
}

func NewKubeHandler(cfg *config.Config, client *kubernetes.Clientset) *KubeHandler {
	return &KubeHandler{cfg: cfg, client: client}
}

func (h *KubeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/namespaces/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 {
		http.NotFound(w, r)
		return
	}

	ns := parts[0]
	resourceType := parts[1]
	name := parts[2]
	subresource := parts[3]

	if subresource != "logs" {
		http.NotFound(w, r)
		return
	}

	if !h.isAllowedNamespace(ns) {
		writeError(w, http.StatusForbidden, "namespace not allowed")
		return
	}

	switch resourceType {
	case "pods":
		h.streamPodLogs(w, r, ns, name)
	case "apps":
		h.streamAppLogs(w, r, ns, name)
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

	opts := h.buildLogOptions(r)
	stream, err := h.client.CoreV1().Pods(namespace).GetLogs(name, opts).Stream(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("log stream error: %v", err))
		return
	}
	defer stream.Close()

	setSSEHeaders(w)
	flusher.Flush()

	h.consumeLogStream(r.Context(), w, flusher, stream, name, opts.Container)
}

func (h *KubeHandler) streamAppLogs(w http.ResponseWriter, r *http.Request, namespace, name string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	pods, err := h.listPodsForApp(r.Context(), namespace, name)
	if err != nil {
		status := http.StatusNotFound
		if !errors.Is(err, errAppNotFound) {
			status = http.StatusBadGateway
		}
		writeError(w, status, err.Error())
		return
	}
	if len(pods) == 0 {
		writeError(w, http.StatusNotFound, "no pods found for app")
		return
	}

	setSSEHeaders(w)
	flusher.Flush()

	logCh := make(chan logEntry, 128)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	var wg sync.WaitGroup
	for _, pod := range pods {
		podName := pod
		wg.Add(1)
		go func() {
			defer wg.Done()
			opts := h.buildLogOptions(r)
			stream, err := h.client.CoreV1().Pods(namespace).GetLogs(podName, opts).Stream(ctx)
			if err != nil {
				return
			}
			defer stream.Close()
			h.consumeLogStreamToChannel(ctx, stream, podName, opts.Container, logCh)
		}()
	}

	go func() {
		wg.Wait()
		close(logCh)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case entry, ok := <-logCh:
			if !ok {
				return
			}
			if err := writeSSE(w, entry); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (h *KubeHandler) consumeLogStream(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, stream ioReadCloser, podName, containerName string) {
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
		if err := writeSSE(w, entry); err != nil {
			return
		}
		flusher.Flush()
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
	Timestamp     string `json:"timestamp"`
	Message       string `json:"message"`
	PodName       string `json:"podName"`
	ContainerName string `json:"containerName"`
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

func (h *KubeHandler) isAllowedNamespace(ns string) bool {
	for _, allowed := range h.cfg.Kubernetes.AllowedNamespaces {
		if allowed == ns {
			return true
		}
	}
	return false
}

var errAppNotFound = errors.New("app not found")

func (h *KubeHandler) listPodsForApp(ctx context.Context, namespace, name string) ([]string, error) {
	selector, err := h.appSelector(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	if selector == "" {
		return nil, errAppNotFound
	}
	pods, err := h.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}

	podNames := make([]string, 0, len(pods.Items))
	for _, pod := range pods.Items {
		podNames = append(podNames, pod.Name)
	}
	return podNames, nil
}

func (h *KubeHandler) appSelector(ctx context.Context, namespace, name string) (string, error) {
	deployment, err := h.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return selectorString(deployment.Spec.Selector), nil
	}

	statefulSet, err := h.client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		return selectorString(statefulSet.Spec.Selector), nil
	}

	return "", errAppNotFound
}

func selectorString(selector *metav1.LabelSelector) string {
	if selector == nil {
		return ""
	}
	if s, err := metav1.LabelSelectorAsSelector(selector); err == nil {
		return s.String()
	}
	return ""
}

func setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
}

func writeSSE(w http.ResponseWriter, entry logEntry) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %s\nevent: log\ndata: %s\n\n", entry.Timestamp, payload)
	return err
}

type ioReadCloser interface {
	Read(p []byte) (n int, err error)
	Close() error
}
