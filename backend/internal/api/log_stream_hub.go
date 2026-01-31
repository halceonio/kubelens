package api

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/charmbracelet/log"
	"github.com/redis/go-redis/v9"

	"github.com/halceonio/kubelens/backend/internal/storage"
)

const (
	defaultWorkerIdleTTL      = 60 * time.Second
	defaultWorkerBufferLines  = 10000
	defaultWorkerBufferBytes  = 50 * 1024 * 1024
	defaultSubscriberBuffer   = 2000
	defaultRedisStreamPrefix  = "kubelens:logs"
	defaultRedisStreamMaxLen  = 10000
	defaultRedisStreamBlock   = 2 * time.Second
	defaultRedisLockTTL       = 15 * time.Second
	defaultRedisLockKeySuffix = ":lock"
)

type logStreamHub struct {
	handler          *KubeHandler
	redis            *redis.Client
	redisEnabled     bool
	redisPrefix      string
	redisMaxLen      int64
	redisBlock       time.Duration
	redisLockTTL     time.Duration
	bufferLines      int
	bufferBytes      int
	subscriberBuffer int
	idleTTL          time.Duration
	instanceID       string
	clusterName      string
	mu               sync.Mutex
	streams          map[string]*logStream
}

type logStream struct {
	key         string
	redisKey    string
	namespace   string
	pod         string
	container   string
	handler     *KubeHandler
	hub         *logStreamHub
	ctx         context.Context
	cancel      context.CancelFunc
	subscribers map[string]*logSubscriber
	buffer      *logBuffer
	startOnce   sync.Once
	stopOnce    sync.Once
	mu          sync.Mutex
	idleTimer   *time.Timer

	leader      bool
	lockKey     string
	lockValue   string
	k8sCancel   context.CancelFunc
	redisCancel context.CancelFunc
	lastRedisID string
	seq         atomic.Uint64
	lastEventAt atomic.Int64
	reconnects  atomic.Int64
	startSince  *time.Time
}

type logSubscriber struct {
	id      string
	ch      chan logEntry
	dropped atomic.Int64
}

type logBuffer struct {
	mu         sync.RWMutex
	maxEntries int
	maxBytes   int
	entries    []logEntry
	bytes      int
}

type logStreamStatus struct {
	Role          string `json:"role"`
	RedisEnabled  bool   `json:"redis_enabled"`
	Leader        bool   `json:"leader"`
	Reconnects    int64  `json:"reconnects"`
	LastEventAt   string `json:"last_event_at"`
	LagMillis     int64  `json:"lag_ms"`
	Subscribers   int    `json:"subscribers"`
	BufferedLines int    `json:"buffered_lines"`
	BufferBytes   int    `json:"buffer_bytes"`
}

type LogStreamStats struct {
	ActiveStreams      int
	ActiveSubscribers  int
	DroppedTotal       int64
	BufferedLinesTotal int
	BufferBytesTotal   int
	Leaders            int
	ReconnectsTotal    int64
	LagMsMax           int64
	LagMsAvg           int64
}

func newLogStreamHub(handler *KubeHandler) *logStreamHub {
	cfg := handler.cfg.Logs
	bufferLines := cfg.WorkerBufferLines
	if bufferLines <= 0 {
		bufferLines = defaultWorkerBufferLines
	}
	bufferBytes := cfg.WorkerBufferMaxBytes
	if bufferBytes <= 0 {
		bufferBytes = defaultWorkerBufferBytes
	}
	subscriberBuffer := cfg.SubscriberBufferLines
	if subscriberBuffer <= 0 {
		subscriberBuffer = defaultSubscriberBuffer
	}
	idleTTL := time.Duration(cfg.WorkerIdleTTLSeconds) * time.Second
	if idleTTL <= 0 {
		idleTTL = defaultWorkerIdleTTL
	}

	clusterName := handler.cfg.Kubernetes.ClusterName
	if clusterName == "" {
		clusterName = "cluster"
	}

	hub := &logStreamHub{
		handler:          handler,
		redisPrefix:      cfg.RedisStreamPrefix,
		redisMaxLen:      int64(cfg.RedisStreamMaxLen),
		redisBlock:       time.Duration(cfg.RedisStreamBlockMillis) * time.Millisecond,
		redisLockTTL:     time.Duration(cfg.RedisLockTTLSeconds) * time.Second,
		bufferLines:      bufferLines,
		bufferBytes:      bufferBytes,
		subscriberBuffer: subscriberBuffer,
		idleTTL:          idleTTL,
		instanceID:       randomID(),
		clusterName:      clusterName,
		streams:          map[string]*logStream{},
	}

	if hub.redisPrefix == "" {
		hub.redisPrefix = defaultRedisStreamPrefix
	}
	if hub.redisMaxLen <= 0 {
		hub.redisMaxLen = defaultRedisStreamMaxLen
	}
	if hub.redisBlock <= 0 {
		hub.redisBlock = defaultRedisStreamBlock
	}
	if hub.redisLockTTL <= 0 {
		hub.redisLockTTL = defaultRedisLockTTL
	}

	if cfg.UseRedisStreams {
		redisURL := cfg.RedisURLOverride
		if redisURL == "" {
			redisURL = handler.cfg.Cache.RedisURL
		}
		if redisURL != "" {
			client, err := storage.NewRedisClientFromURL(context.Background(), redisURL)
			if err != nil {
				log.Warn("log streams: redis disabled", "err", err)
			} else {
				hub.redis = client
				hub.redisEnabled = true
			}
		} else {
			log.Warn("log streams: redis disabled (missing redis_url)")
		}
	}

	return hub
}

func (h *logStreamHub) stop() {
	h.mu.Lock()
	for _, stream := range h.streams {
		stream.stop()
	}
	h.streams = map[string]*logStream{}
	h.mu.Unlock()
	if h.redis != nil {
		_ = h.redis.Close()
	}
}

func (h *logStreamHub) Stats() LogStreamStats {
	if h == nil {
		return LogStreamStats{}
	}
	h.mu.Lock()
	streams := make([]*logStream, 0, len(h.streams))
	for _, stream := range h.streams {
		streams = append(streams, stream)
	}
	h.mu.Unlock()

	stats := LogStreamStats{}
	lagTotal := int64(0)
	lagCount := int64(0)
	for _, stream := range streams {
		snap := stream.snapshotStats()
		stats.ActiveStreams++
		stats.ActiveSubscribers += snap.subscribers
		stats.DroppedTotal += snap.dropped
		stats.BufferedLinesTotal += snap.bufferedLines
		stats.BufferBytesTotal += snap.bufferBytes
		if snap.leader {
			stats.Leaders++
		}
		stats.ReconnectsTotal += snap.reconnects
		if snap.lagMs > stats.LagMsMax {
			stats.LagMsMax = snap.lagMs
		}
		lagTotal += snap.lagMs
		lagCount++
	}
	if lagCount > 0 {
		stats.LagMsAvg = lagTotal / lagCount
	}
	return stats
}

func (h *logStreamHub) Status(namespace, pod, container string) (logStreamStatus, bool) {
	if h == nil {
		return logStreamStatus{}, false
	}
	key := h.streamKey(namespace, pod, container)
	h.mu.Lock()
	stream, ok := h.streams[key]
	h.mu.Unlock()
	if !ok {
		return logStreamStatus{}, false
	}
	return stream.statusSnapshot(), true
}

func (h *logStreamHub) SubscribePod(ctx context.Context, namespace, pod, container string, tail int64, resume logResume) (*logSubscriber, []logEntry, func(), error) {
	key := h.streamKey(namespace, pod, container)

	h.mu.Lock()
	stream, ok := h.streams[key]
	if !ok {
		stream = newLogStream(h, namespace, pod, container, resume.sinceTime)
		h.streams[key] = stream
	}
	h.mu.Unlock()

	sub, replay := stream.subscribe(ctx, resume, tail)
	unsubscribe := func() {
		stream.unsubscribe(sub.id)
		if stream.isIdle() {
			stream.scheduleIdleStop()
		}
	}

	return sub, replay, unsubscribe, nil
}

func (h *logStreamHub) streamKey(namespace, pod, container string) string {
	if container == "" {
		container = "default"
	}
	return fmt.Sprintf("%s/%s/%s", namespace, pod, container)
}

func (h *logStreamHub) redisStreamKey(key string) string {
	return fmt.Sprintf("%s:%s:%s", h.redisPrefix, h.clusterName, key)
}

func newLogStream(hub *logStreamHub, namespace, pod, container string, startSince *time.Time) *logStream {
	ctx, cancel := context.WithCancel(context.Background())
	key := hub.streamKey(namespace, pod, container)
	stream := &logStream{
		key:         key,
		redisKey:    hub.redisStreamKey(key),
		namespace:   namespace,
		pod:         pod,
		container:   container,
		handler:     hub.handler,
		hub:         hub,
		ctx:         ctx,
		cancel:      cancel,
		subscribers: map[string]*logSubscriber{},
		buffer: &logBuffer{
			maxEntries: hub.bufferLines,
			maxBytes:   hub.bufferBytes,
		},
		lockKey:    hub.redisStreamKey(key) + defaultRedisLockKeySuffix,
		lockValue:  hub.instanceID,
		startSince: startSince,
	}
	return stream
}

func (s *logStream) subscribe(ctx context.Context, resume logResume, tail int64) (*logSubscriber, []logEntry) {
	sub := &logSubscriber{
		id: fmt.Sprintf("%d", time.Now().UnixNano()),
		ch: make(chan logEntry, s.hub.subscriberBuffer),
	}

	s.mu.Lock()
	s.subscribers[sub.id] = sub
	if s.idleTimer != nil {
		s.idleTimer.Stop()
		s.idleTimer = nil
	}
	s.mu.Unlock()

	s.startOnce.Do(func() {
		go s.run()
	})

	replay := s.replay(ctx, resume, tail)

	go func() {
		<-ctx.Done()
		s.unsubscribe(sub.id)
		if s.isIdle() {
			s.scheduleIdleStop()
		}
	}()

	return sub, replay
}

func (s *logStream) unsubscribe(id string) {
	s.mu.Lock()
	if sub, ok := s.subscribers[id]; ok {
		delete(s.subscribers, id)
		close(sub.ch)
	}
	s.mu.Unlock()
}

func (s *logStream) isIdle() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.subscribers) == 0
}

func (s *logStream) scheduleIdleStop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.idleTimer != nil {
		s.idleTimer.Stop()
	}
	s.idleTimer = time.AfterFunc(s.hub.idleTTL, func() {
		if !s.isIdle() {
			return
		}
		s.hub.mu.Lock()
		delete(s.hub.streams, s.key)
		s.hub.mu.Unlock()
		s.stop()
	})
}

func (s *logStream) stop() {
	s.stopOnce.Do(func() {
		s.cancel()
	})
}

func (s *logStream) run() {
	if !s.hub.redisEnabled {
		s.startK8s()
		<-s.ctx.Done()
		s.stopK8s()
		return
	}

	if s.tryAcquireLock() {
		s.leader = true
		s.startK8s()
	} else {
		s.startRedisConsumer()
	}
	lockTicker := time.NewTicker(s.lockRefreshInterval())
	defer lockTicker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.stopRedisConsumer()
			s.stopK8s()
			s.releaseLock()
			return
		case <-lockTicker.C:
			if s.leader {
				if !s.refreshLock() {
					s.leader = false
					s.stopK8s()
					s.startRedisConsumer()
				}
			} else {
				if s.tryAcquireLock() {
					s.leader = true
					s.stopRedisConsumer()
					s.startK8s()
				}
			}
		}
	}
}

func (s *logStream) lockRefreshInterval() time.Duration {
	if s.hub.redisLockTTL <= 0 {
		return 5 * time.Second
	}
	interval := s.hub.redisLockTTL / 2
	if interval < 2*time.Second {
		return 2 * time.Second
	}
	return interval
}

func (s *logStream) startK8s() {
	if s.k8sCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.k8sCancel = cancel
	go s.consumeK8s(ctx)
}

func (s *logStream) stopK8s() {
	if s.k8sCancel != nil {
		s.k8sCancel()
		s.k8sCancel = nil
	}
}

func (s *logStream) startRedisConsumer() {
	if !s.hub.redisEnabled || s.redisCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.redisCancel = cancel
	go s.consumeRedis(ctx)
}

func (s *logStream) stopRedisConsumer() {
	if s.redisCancel != nil {
		s.redisCancel()
		s.redisCancel = nil
	}
}

func (s *logStream) consumeK8s(ctx context.Context) {
	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		opts := &corev1.PodLogOptions{
			Follow:     true,
			Timestamps: true,
			TailLines:  s.k8sTailLines(),
			Container:  s.container,
		}
		if s.startSince != nil {
			opts.SinceTime = &metav1.Time{Time: s.startSince.UTC()}
		}

		stream, err := s.handler.client.CoreV1().Pods(s.namespace).GetLogs(s.pod, opts).Stream(ctx)
		if err != nil {
			time.Sleep(backoff)
			if backoff < 10*time.Second {
				backoff *= 2
			}
			continue
		}
		backoff = time.Second

		reader := bufio.NewReader(stream)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				s.reconnects.Add(1)
				_ = stream.Close()
				break
			}
			entry := s.handler.parseLogLine(strings.TrimRight(line, "\n"), s.pod, s.container)
			s.ingestK8sEntry(ctx, entry)
		}
	}
}

func (s *logStream) k8sTailLines() *int64 {
	tail := int64(s.hub.bufferLines)
	if s.handler.cfg.Logs.MaxTailLines > 0 && tail > int64(s.handler.cfg.Logs.MaxTailLines) {
		tail = int64(s.handler.cfg.Logs.MaxTailLines)
	}
	if tail <= 0 {
		return nil
	}
	return &tail
}

func (s *logStream) ingestK8sEntry(ctx context.Context, entry logEntry) {
	seq := s.seq.Add(1)
	entry.Seq = seq
	entry.ID = strconv.FormatUint(seq, 10)
	s.lastEventAt.Store(time.Now().UTC().UnixNano())
	if s.hub.redisEnabled {
		id, err := s.addRedisEntry(ctx, entry)
		if err != nil {
			log.Warn("log stream redis add failed", "err", err)
		} else {
			entry.ID = id
		}
	}
	s.buffer.append(entry)
	s.broadcast(entry)
}

func (s *logStream) consumeRedis(ctx context.Context) {
	prefill, lastID, err := s.fetchRedisTail(ctx, s.hub.bufferLines)
	if err == nil {
		for _, entry := range prefill {
			s.buffer.append(entry)
		}
	}
	if lastID != "" {
		s.lastRedisID = lastID
	} else {
		s.lastRedisID = "0-0"
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		args := &redis.XReadArgs{
			Streams: []string{s.redisKey, s.lastRedisID},
			Block:   s.hub.redisBlock,
		}
		res, err := s.hub.redis.XRead(ctx, args).Result()
		if err != nil && !errors.Is(err, context.Canceled) && err != redis.Nil {
			s.reconnects.Add(1)
			time.Sleep(time.Second)
			continue
		}
		if len(res) == 0 {
			continue
		}
		for _, stream := range res {
			for _, msg := range stream.Messages {
				entry, ok := parseRedisMessage(msg, s.pod, s.container)
				if !ok {
					continue
				}
				s.lastRedisID = msg.ID
				s.lastEventAt.Store(time.Now().UTC().UnixNano())
				s.buffer.append(entry)
				s.broadcast(entry)
			}
		}
	}
}

func (s *logStream) addRedisEntry(ctx context.Context, entry logEntry) (string, error) {
	args := &redis.XAddArgs{
		Stream: s.redisKey,
		Values: map[string]any{
			"ts":        entry.Timestamp,
			"msg":       entry.Message,
			"pod":       entry.PodName,
			"container": entry.ContainerName,
			"seq":       entry.Seq,
		},
	}
	if s.hub.redisMaxLen > 0 {
		args.MaxLen = s.hub.redisMaxLen
		args.Approx = true
	}
	return s.hub.redis.XAdd(ctx, args).Result()
}

func (s *logStream) fetchRedisTail(ctx context.Context, count int) ([]logEntry, string, error) {
	if !s.hub.redisEnabled || count <= 0 {
		return nil, "", nil
	}
	msgs, err := s.hub.redis.XRevRangeN(ctx, s.redisKey, "+", "-", int64(count)).Result()
	if err != nil && err != redis.Nil {
		return nil, "", err
	}
	if len(msgs) == 0 {
		return nil, "", nil
	}
	entries := make([]logEntry, 0, len(msgs))
	for i := len(msgs) - 1; i >= 0; i-- {
		msg := msgs[i]
		entry, ok := parseRedisMessage(msg, s.pod, s.container)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, msgs[0].ID, nil
}

func (s *logStream) fetchRedisSince(ctx context.Context, sinceID string, count int) ([]logEntry, string, error) {
	if !s.hub.redisEnabled {
		return nil, "", nil
	}
	if sinceID == "" {
		sinceID = "0-0"
	}
	if count <= 0 {
		count = s.hub.bufferLines
	}
	msgs, err := s.hub.redis.XRangeN(ctx, s.redisKey, sinceID, "+", int64(count)).Result()
	if err != nil && err != redis.Nil {
		return nil, "", err
	}
	if len(msgs) == 0 {
		return nil, "", nil
	}
	entries := make([]logEntry, 0, len(msgs))
	for _, msg := range msgs {
		if msg.ID == sinceID {
			continue
		}
		entry, ok := parseRedisMessage(msg, s.pod, s.container)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, msgs[len(msgs)-1].ID, nil
}

func (s *logStream) replay(ctx context.Context, resume logResume, tail int64) []logEntry {
	if resume.sinceID != "" {
		if entries, ok := s.buffer.sinceID(resume.sinceID); ok {
			return entries
		}
		if s.hub.redisEnabled && isRedisID(resume.sinceID) {
			if entries, _, err := s.fetchRedisSince(ctx, resume.sinceID, int(tail)); err == nil && len(entries) > 0 {
				return entries
			}
		}
	}
	if resume.sinceTime != nil {
		entries := s.buffer.sinceTime(*resume.sinceTime)
		if len(entries) > 0 {
			return entries
		}
		if s.hub.redisEnabled {
			startID := redisIDFromTime(*resume.sinceTime)
			if entries, _, err := s.fetchRedisSince(ctx, startID, int(tail)); err == nil && len(entries) > 0 {
				return entries
			}
		}
	}
	if tail > 0 {
		entries := s.buffer.tail(int(tail))
		if len(entries) > 0 {
			return entries
		}
		if s.hub.redisEnabled {
			entries, _, err := s.fetchRedisTail(ctx, int(tail))
			if err == nil && len(entries) > 0 {
				return entries
			}
		}
	}
	return nil
}

type streamSnapshot struct {
	subscribers   int
	dropped       int64
	bufferedLines int
	bufferBytes   int
	leader        bool
	reconnects    int64
	lagMs         int64
}

func (s *logStream) snapshotStats() streamSnapshot {
	s.mu.Lock()
	subs := len(s.subscribers)
	dropped := int64(0)
	for _, sub := range s.subscribers {
		dropped += sub.dropped.Load()
	}
	s.mu.Unlock()
	lines, bytes := s.buffer.snapshot()
	last := s.lastEventAt.Load()
	lagMs := int64(0)
	if last > 0 {
		lastTime := time.Unix(0, last).UTC()
		lagMs = time.Since(lastTime).Milliseconds()
	}
	return streamSnapshot{
		subscribers:   subs,
		dropped:       dropped,
		bufferedLines: lines,
		bufferBytes:   bytes,
		leader:        s.leader,
		reconnects:    s.reconnects.Load(),
		lagMs:         lagMs,
	}
}

func (s *logStream) statusSnapshot() logStreamStatus {
	s.mu.Lock()
	subs := len(s.subscribers)
	s.mu.Unlock()
	lines, bytes := s.buffer.snapshot()
	last := s.lastEventAt.Load()
	lastAt := ""
	lagMs := int64(0)
	if last > 0 {
		lastTime := time.Unix(0, last).UTC()
		lastAt = lastTime.Format(time.RFC3339Nano)
		lagMs = time.Since(lastTime).Milliseconds()
	}
	role := "single"
	if s.hub.redisEnabled {
		if s.leader {
			role = "leader"
		} else {
			role = "follower"
		}
	}
	return logStreamStatus{
		Role:          role,
		RedisEnabled:  s.hub.redisEnabled,
		Leader:        s.leader,
		Reconnects:    s.reconnects.Load(),
		LastEventAt:   lastAt,
		LagMillis:     lagMs,
		Subscribers:   subs,
		BufferedLines: lines,
		BufferBytes:   bytes,
	}
}

func (s *logStream) broadcast(entry logEntry) {
	s.mu.Lock()
	for _, sub := range s.subscribers {
		select {
		case sub.ch <- entry:
		default:
			sub.dropped.Add(1)
		}
	}
	s.mu.Unlock()
}

func (s *logStream) tryAcquireLock() bool {
	if !s.hub.redisEnabled {
		return false
	}
	ok, err := s.hub.redis.SetNX(context.Background(), s.lockKey, s.lockValue, s.hub.redisLockTTL).Result()
	if err != nil {
		return false
	}
	return ok
}

func (s *logStream) refreshLock() bool {
	if !s.hub.redisEnabled {
		return false
	}
	script := redis.NewScript(`if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("PEXPIRE", KEYS[1], ARGV[2]) else return 0 end`)
	res, err := script.Run(context.Background(), s.hub.redis, []string{s.lockKey}, s.lockValue, int64(s.hub.redisLockTTL/time.Millisecond)).Result()
	if err != nil {
		return false
	}
	val, ok := res.(int64)
	return ok && val > 0
}

func (s *logStream) releaseLock() {
	if !s.hub.redisEnabled {
		return
	}
	script := redis.NewScript(`if redis.call("GET", KEYS[1]) == ARGV[1] then return redis.call("DEL", KEYS[1]) else return 0 end`)
	_, _ = script.Run(context.Background(), s.hub.redis, []string{s.lockKey}, s.lockValue).Result()
}

func (b *logBuffer) append(entry logEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	size := estimateEntrySize(entry)
	b.entries = append(b.entries, entry)
	b.bytes += size
	for (b.maxEntries > 0 && len(b.entries) > b.maxEntries) || (b.maxBytes > 0 && b.bytes > b.maxBytes) {
		if len(b.entries) == 0 {
			break
		}
		removed := b.entries[0]
		b.entries = b.entries[1:]
		b.bytes -= estimateEntrySize(removed)
	}
}

func (b *logBuffer) snapshot() (int, int) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.entries), b.bytes
}

func (b *logBuffer) tail(count int) []logEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if count <= 0 || count >= len(b.entries) {
		return append([]logEntry(nil), b.entries...)
	}
	return append([]logEntry(nil), b.entries[len(b.entries)-count:]...)
}

func (b *logBuffer) sinceID(id string) ([]logEntry, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for i, entry := range b.entries {
		if entry.ID == id {
			return append([]logEntry(nil), b.entries[i+1:]...), true
		}
	}
	return nil, false
}

func (b *logBuffer) sinceTime(t time.Time) []logEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := []logEntry{}
	for _, entry := range b.entries {
		if entry.Timestamp == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339Nano, entry.Timestamp)
		if err != nil {
			continue
		}
		if parsed.After(t) || parsed.Equal(t) {
			result = append(result, entry)
		}
	}
	return result
}

func estimateEntrySize(entry logEntry) int {
	return len(entry.Timestamp) + len(entry.Message) + len(entry.PodName) + len(entry.ContainerName) + 16
}

func parseRedisMessage(msg redis.XMessage, defaultPod, defaultContainer string) (logEntry, bool) {
	entry := logEntry{
		ID:            msg.ID,
		Timestamp:     parseRedisString(msg.Values["ts"]),
		Message:       parseRedisString(msg.Values["msg"]),
		PodName:       parseRedisString(msg.Values["pod"]),
		ContainerName: parseRedisString(msg.Values["container"]),
	}
	if entry.PodName == "" {
		entry.PodName = defaultPod
	}
	if entry.ContainerName == "" {
		entry.ContainerName = defaultContainer
	}
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if seqStr := parseRedisString(msg.Values["seq"]); seqStr != "" {
		if seq, err := strconv.ParseUint(seqStr, 10, 64); err == nil {
			entry.Seq = seq
		}
	}
	if entry.Message == "" {
		return logEntry{}, false
	}
	return entry, true
}

func parseRedisString(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int:
		return strconv.Itoa(v)
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func randomID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func isRedisID(id string) bool {
	if id == "" {
		return false
	}
	parts := strings.Split(id, "-")
	if len(parts) != 2 {
		return false
	}
	if _, err := strconv.ParseInt(parts[0], 10, 64); err != nil {
		return false
	}
	if _, err := strconv.ParseInt(parts[1], 10, 64); err != nil {
		return false
	}
	return true
}

func redisIDFromTime(t time.Time) string {
	ms := t.UnixNano() / int64(time.Millisecond)
	if ms < 0 {
		ms = 0
	}
	return fmt.Sprintf("%d-0", ms)
}
