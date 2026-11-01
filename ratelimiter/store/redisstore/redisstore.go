package redisstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const blockPrefix = "blocked:"

// Strategy é um backend Redis para contagem e bloqueio de chaves.
type Strategy struct {
	client *redis.Client
}

func New(client *redis.Client) *Strategy {
	return &Strategy{client: client}
}

func (s *Strategy) Increment(ctx context.Context, key string, window time.Duration) (int, error) {
	args := redis.IncrEXIntArgs{
		By: 1,
		Expiration: &redis.ExpirationOption{
			Mode:  redis.ExpirationMode("PX"),
			Value: window.Milliseconds(),
		},
		ENX: true,
	}
	result := s.client.IncrEXInt(ctx, key, args)
	if err := result.Err(); err != nil {
		return 0, fmt.Errorf("incrementing %q: %w", key, err)
	}
	return int(result.Val().Value), nil
}

func (s *Strategy) Block(ctx context.Context, key string, ttl time.Duration) error {
	if err := s.client.Set(ctx, blockPrefix+key, true, ttl).Err(); err != nil {
		return fmt.Errorf("blocking %q: %w", key, err)
	}
	return nil
}

func (s *Strategy) IsBlocked(ctx context.Context, key string) (bool, error) {
	result := s.client.Get(ctx, blockPrefix+key)
	if errors.Is(result.Err(), redis.Nil) {
		return false, nil
	}
	if result.Err() != nil {
		return false, fmt.Errorf("checking block for %q: %w", key, result.Err())
	}
	return result.Bool()
}
