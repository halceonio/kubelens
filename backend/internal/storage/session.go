package storage

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrNotFound = errors.New("session not found")

type SessionRecord struct {
	Data      []byte
	UpdatedAt time.Time
}

type SessionStore interface {
	Get(ctx context.Context, userID string) (*SessionRecord, error)
	Put(ctx context.Context, userID string, data []byte) error
	Delete(ctx context.Context, userID string) error
}

type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]SessionRecord
}

func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{sessions: make(map[string]SessionRecord)}
}

func (m *MemorySessionStore) Get(_ context.Context, userID string) (*SessionRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.sessions[userID]
	if !ok {
		return nil, ErrNotFound
	}
	copyData := make([]byte, len(rec.Data))
	copy(copyData, rec.Data)
	return &SessionRecord{Data: copyData, UpdatedAt: rec.UpdatedAt}, nil
}

func (m *MemorySessionStore) Put(_ context.Context, userID string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copyData := make([]byte, len(data))
	copy(copyData, data)
	m.sessions[userID] = SessionRecord{Data: copyData, UpdatedAt: time.Now().UTC()}
	return nil
}

func (m *MemorySessionStore) Delete(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, userID)
	return nil
}
