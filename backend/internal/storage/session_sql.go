package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type SQLSessionStore struct {
	db      *sql.DB
	dialect string
	getStmt string
	putStmt string
	delStmt string
}

func NewSQLSessionStore(db *sql.DB, dialect string) (*SQLSessionStore, error) {
	store := &SQLSessionStore{db: db, dialect: dialect}
	store.initStatements()
	if err := store.ensureSchema(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *SQLSessionStore) initStatements() {
	p1 := s.placeholder(1)
	p2 := s.placeholder(2)
	p3 := s.placeholder(3)

	s.getStmt = fmt.Sprintf("SELECT data, updated_at FROM user_sessions WHERE user_id = %s", p1)
	s.putStmt = fmt.Sprintf("INSERT INTO user_sessions (user_id, data, updated_at) VALUES (%s, %s, %s) ON CONFLICT (user_id) DO UPDATE SET data = EXCLUDED.data, updated_at = EXCLUDED.updated_at", p1, p2, p3)
	s.delStmt = fmt.Sprintf("DELETE FROM user_sessions WHERE user_id = %s", p1)
}

func (s *SQLSessionStore) ensureSchema(ctx context.Context) error {
	schema := `CREATE TABLE IF NOT EXISTS user_sessions (
		user_id TEXT PRIMARY KEY,
		data TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`
	_, err := s.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("create sessions table: %w", err)
	}
	return nil
}

func (s *SQLSessionStore) Get(ctx context.Context, userID string) (*SessionRecord, error) {
	var data string
	var updated string
	if err := s.db.QueryRowContext(ctx, s.getStmt, userID).Scan(&data, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	updatedAt, err := time.Parse(time.RFC3339Nano, updated)
	if err != nil {
		updatedAt = time.Now().UTC()
	}
	return &SessionRecord{Data: []byte(data), UpdatedAt: updatedAt}, nil
}

func (s *SQLSessionStore) Put(ctx context.Context, userID string, data []byte) error {
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, s.putStmt, userID, string(data), updatedAt)
	return err
}

func (s *SQLSessionStore) Delete(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, s.delStmt, userID)
	return err
}

func (s *SQLSessionStore) placeholder(idx int) string {
	if strings.EqualFold(s.dialect, "postgres") {
		return fmt.Sprintf("$%d", idx)
	}
	return "?"
}
