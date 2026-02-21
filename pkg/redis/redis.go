// Package redis provides a Redis client wrapper and utilities.
package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// NewClient creates a new Redis client and verifies connectivity.
func NewClient(ctx context.Context, addr, password string, db int) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	log.Info().Str("addr", addr).Msg("connected to Redis")
	return client, nil
}

// DistLock implements a simple Redis-based distributed lock using SETNX with TTL.
// Suitable for leader election or short-lived mutual exclusion across multiple processes.
type DistLock struct {
	client *redis.Client
	key    string
	value  string
	ttl    time.Duration
}

// NewDistLock creates a new distributed lock with the given key and TTL.
func NewDistLock(client *redis.Client, key string, ttl time.Duration) *DistLock {
	return &DistLock{
		client: client,
		key:    "lock:" + key,
		value:  uuid.New().String(), // unique per lock holder
		ttl:    ttl,
	}
}

// Acquire attempts to acquire the lock. Returns true if successful.
func (l *DistLock) Acquire(ctx context.Context) (bool, error) {
	ok, err := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
	if err != nil {
		return false, fmt.Errorf("acquire lock: %w", err)
	}
	return ok, nil
}

// Release releases the lock, but only if we still own it (compare-and-delete).
// Uses a Lua script for atomicity.
func (l *DistLock) Release(ctx context.Context) error {
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		end
		return 0
	`)
	_, err := script.Run(ctx, l.client, []string{l.key}, l.value).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("release lock: %w", err)
	}
	return nil
}

// AcquireWithRetry attempts to acquire the lock with retries.
func (l *DistLock) AcquireWithRetry(ctx context.Context, maxRetries int, backoff time.Duration) (bool, error) {
	for i := 0; i < maxRetries; i++ {
		acquired, err := l.Acquire(ctx)
		if err != nil {
			return false, err
		}
		if acquired {
			return true, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(backoff):
			// retry
		}
	}
	return false, nil
}
