package limiter

import (
	"context"
	"time"
)

type Limiter interface {
	Increment(ctx context.Context, key string, window time.Duration) (int, error)
	Block(ctx context.Context, key string, ttl time.Duration) error
	IsBlocked(ctx context.Context, key string) (bool, error)
}
