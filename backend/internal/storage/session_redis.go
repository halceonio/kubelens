package storage

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisSessionStore struct {
	client    *redis.Client
	keyPrefix string
}

func NewRedisSessionStore(client *redis.Client) *RedisSessionStore {
	return &RedisSessionStore{client: client, keyPrefix: "kubelens:session:"}
}

func (r *RedisSessionStore) Get(ctx context.Context, userID string) (*SessionRecord, error) {
	key := r.keyPrefix + userID
	values, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, ErrNotFound
	}
	data := values["data"]
	if data == "" {
		return nil, ErrNotFound
	}
	updatedAt := time.Now().UTC()
	if ts := values["updated_at"]; ts != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			updatedAt = parsed
		}
	}
	return &SessionRecord{Data: []byte(data), UpdatedAt: updatedAt}, nil
}

func (r *RedisSessionStore) Put(ctx context.Context, userID string, data []byte) error {
	key := r.keyPrefix + userID
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if err := r.client.HSet(ctx, key, map[string]any{
		"data":       string(data),
		"updated_at": updatedAt,
	}).Err(); err != nil {
		return err
	}
	return nil
}

func (r *RedisSessionStore) Delete(ctx context.Context, userID string) error {
	key := r.keyPrefix + userID
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return err
	}
	return nil
}

func NewRedisClientFromURL(ctx context.Context, redisURL string) (*redis.Client, error) {
	if redisURL == "" {
		return nil, errors.New("redis url is empty")
	}
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return client, nil
}
