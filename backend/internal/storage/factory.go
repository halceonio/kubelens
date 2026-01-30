package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/halceonio/kubelens/backend/internal/config"
)

type SessionBackend string

const (
	BackendMemory SessionBackend = "memory"
	BackendRedis  SessionBackend = "redis"
	BackendSQL    SessionBackend = "sql"
)

func NewSessionStoreFromConfig(ctx context.Context, cfg *config.Config) (SessionStore, SessionBackend, error) {
	if cfg.Cache.Enabled && cfg.Cache.RedisURL != "" {
		client, err := NewRedisClientFromURL(ctx, cfg.Cache.RedisURL)
		if err != nil {
			return nil, BackendRedis, fmt.Errorf("redis session store: %w", err)
		}
		return NewRedisSessionStore(client), BackendRedis, nil
	}

	if cfg.Storage.DatabaseURL != "" {
		db, dialect, err := openDatabase(cfg.Storage.DatabaseURL)
		if err != nil {
			return nil, BackendSQL, err
		}
		store, err := NewSQLSessionStore(db, dialect)
		if err != nil {
			return nil, BackendSQL, err
		}
		return store, BackendSQL, nil
	}

	return NewMemorySessionStore(), BackendMemory, nil
}

func openDatabase(databaseURL string) (*sql.DB, string, error) {
	parsed, err := url.Parse(databaseURL)
	if err != nil {
		return nil, "", fmt.Errorf("parse database url: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "postgres", "postgresql":
		db, err := sql.Open("pgx", databaseURL)
		if err != nil {
			return nil, "", err
		}
		if err := db.Ping(); err != nil {
			return nil, "", err
		}
		return db, "postgres", nil
	case "sqlite", "sqlite3", "file":
		dsn, err := sqliteDSN(databaseURL, parsed)
		if err != nil {
			return nil, "", err
		}
		db, err := sql.Open("sqlite", dsn)
		if err != nil {
			return nil, "", err
		}
		if err := db.Ping(); err != nil {
			return nil, "", err
		}
		return db, "sqlite", nil
	default:
		return nil, "", fmt.Errorf("unsupported database scheme: %s", scheme)
	}
}

func sqliteDSN(raw string, parsed *url.URL) (string, error) {
	if strings.HasPrefix(raw, "file:") {
		return raw, nil
	}

	pathPart := parsed.Path
	if parsed.Host != "" {
		pathPart = path.Join("/", parsed.Host, parsed.Path)
	}
	if pathPart == "" {
		return "", errors.New("sqlite path missing")
	}

	dsn := "file:" + pathPart
	if parsed.RawQuery != "" {
		dsn += "?" + parsed.RawQuery
	} else {
		dsn += "?cache=shared&mode=rwc"
	}
	return dsn, nil
}
