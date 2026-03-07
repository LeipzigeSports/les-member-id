// Package store contains implementations of backend services for the member ID services to function.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrKeyStoreOperation is a generic error that an expiring key storage backend may return.
	ErrKeyStoreOperation = errors.New("failed to perform backend operation")
	// ErrKeyNotPresent is returned if a storage backend cannot find a requested key.
	ErrKeyNotPresent = errors.New("key not present in backend")
)

const durationInvalid = time.Second * -1

// ExpiringKeyStore defines functions that must be supported by a storage backend which is capable of storing
// keys and values that must expire after a set amount of time.
type ExpiringKeyStore interface {
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	GetWithTTL(ctx context.Context, key string) (string, time.Duration, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
}

//nolint:revive
type RedisExpiringKeyStore struct {
	client    *redis.Client
	keyPrefix string
}

// NewRedisExpiringKeyStore returns an implementation of an expiring key store based on Redis.
func NewRedisExpiringKeyStore(client *redis.Client, keyPrefix string) *RedisExpiringKeyStore {
	return &RedisExpiringKeyStore{
		client:    client,
		keyPrefix: keyPrefix,
	}
}

// Set assigns a value to a key in the Redis backend with a defined expiration time.
func (__this *RedisExpiringKeyStore) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	redisKey := __this.buildKey(key)

	err := __this.client.Set(ctx, redisKey, value, ttl).Err()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrKeyStoreOperation, err)
	}

	return nil
}

// Get returns the value of a key in the Redis backend.
func (__this *RedisExpiringKeyStore) Get(ctx context.Context, key string) (string, error) {
	redisKey := __this.buildKey(key)

	value, err := __this.client.Get(ctx, redisKey).Result()
	if err != nil {
		// nil = key doesn't exist
		if errors.Is(err, redis.Nil) {
			return "", ErrKeyNotPresent
		}

		// otherwise return error
		return "", fmt.Errorf("%w: %w", ErrKeyStoreOperation, err)
	}

	return value, nil
}

// GetWithTTL returns the value assigned to a key in the Redis backend and the seconds until the key expires.
func (__this *RedisExpiringKeyStore) GetWithTTL(ctx context.Context, key string) (string, time.Duration, error) {
	redisKey := __this.buildKey(key)

	pipe := __this.client.Pipeline()

	getCmd := pipe.Get(ctx, redisKey)
	ttlCmd := pipe.TTL(ctx, redisKey)

	_, err := pipe.Exec(ctx)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", durationInvalid, ErrKeyNotPresent
		}

		return "", durationInvalid, fmt.Errorf("%w: %w", ErrKeyStoreOperation, err)
	}

	return getCmd.Val(), ttlCmd.Val(), nil
}

// TTL returns the amount of seconds until a key expires in the Redis backend.
func (__this *RedisExpiringKeyStore) TTL(ctx context.Context, key string) (time.Duration, error) {
	redisKey := __this.buildKey(key)

	duration, err := __this.client.TTL(ctx, redisKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return durationInvalid, ErrKeyNotPresent
		}

		return durationInvalid, fmt.Errorf("%w: %w", ErrKeyStoreOperation, err)
	}

	return duration, nil
}

func (__this *RedisExpiringKeyStore) buildKey(key string) string {
	return fmt.Sprintf("%s:%s", __this.keyPrefix, key)
}
