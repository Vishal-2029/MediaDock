package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrCacheMiss indicates a requested key does not exist in Redis.
var ErrCacheMiss = errors.New("cache key not found")

// RedisClient wraps go-redis with typed helpers for MediaDock.
type RedisClient struct {
	Client *redis.Client
}

// ConnectRedis parses redisURL, opens a client, and verifies connectivity via PING.
func ConnectRedis(redisURL string) (*RedisClient, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	opts.DialTimeout = 5 * time.Second
	opts.ReadTimeout = 5 * time.Second
	opts.WriteTimeout = 5 * time.Second

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &RedisClient{Client: client}, nil
}

// NewFromClient wraps an existing *redis.Client (useful for testing with miniredis).
func NewFromClient(client *redis.Client) *RedisClient {
	return &RedisClient{Client: client}
}

// Ping checks Redis responsiveness.
func (r *RedisClient) Ping(ctx context.Context) error {
	if r == nil || r.Client == nil {
		return errors.New("redis client not initialized")
	}
	return r.Client.Ping(ctx).Err()
}

// Close closes the underlying Redis connection pool.
func (r *RedisClient) Close() error {
	if r == nil || r.Client == nil {
		return nil
	}
	return r.Client.Close()
}

// SetJSON serializes val as JSON and stores it with the given TTL.
func (r *RedisClient) SetJSON(ctx context.Context, key string, val interface{}, ttl time.Duration) error {
	data, err := json.Marshal(val)
	if err != nil {
		return fmt.Errorf("marshal cache payload: %w", err)
	}
	return r.Client.Set(ctx, key, data, ttl).Err()
}

// GetJSON retrieves and unmarshals a JSON value from Redis.
func (r *RedisClient) GetJSON(ctx context.Context, key string, dest interface{}) error {
	raw, err := r.Client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrCacheMiss
		}
		return err
	}
	return json.Unmarshal(raw, dest)
}

// Delete removes one or more keys from Redis.
func (r *RedisClient) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.Client.Del(ctx, keys...).Err()
}

// SetMarker stores a lightweight flag key with expiration (used for token revocation / rate limits).
func (r *RedisClient) SetMarker(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.Client.Set(ctx, key, value, ttl).Err()
}

// Exists reports whether key is present in Redis.
func (r *RedisClient) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.Client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

