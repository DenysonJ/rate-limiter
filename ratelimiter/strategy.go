package ratelimiter

import (
	"context"
	"time"
)

// Strategy é a porta de armazenamento: conta acessos por janela e bloqueia chaves.
type Strategy interface {
	Increment(ctx context.Context, key string, window time.Duration) (int, error)
	Block(ctx context.Context, key string, ttl time.Duration) error
	IsBlocked(ctx context.Context, key string) (bool, error)
}
