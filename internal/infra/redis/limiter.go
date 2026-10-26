package redis

import (
	"context"
	"errors"
	"time"

	"github.com/bytedance/gopkg/util/logger"
	"github.com/redis/go-redis/v9"
)

type LimiterStrategy struct {
	redisClient *redis.Client
}

func NewRedisLimiter(redis *redis.Client) *LimiterStrategy {
	return &LimiterStrategy{
		redisClient: redis,
	}
}

func (l *LimiterStrategy) Increment(ctx context.Context, key string, window time.Duration) (int, error) {
	incrArgs := redis.IncrEXIntArgs{
		By: 1,
		Expiration: &redis.ExpirationOption{
			Mode:  redis.ExpirationMode("PX"),
			Value: window.Milliseconds(),
		},
		ENX: true,
	}

	incResult := l.redisClient.IncrEXInt(ctx, key, incrArgs)

	return int(incResult.Val().Value), nil
}
func (l *LimiterStrategy) Block(ctx context.Context, key string, ttl time.Duration) error {
	setResult := l.redisClient.Set(ctx, "blocked:"+key, true, ttl)
	logger.Info("Resultado ao bloquear no redis: " + setResult.Val())
	return nil
}
func (l *LimiterStrategy) IsBlocked(ctx context.Context, key string) (bool, error) {
	setResult := l.redisClient.Get(ctx, "blocked:"+key)
	if errors.Is(setResult.Err(), redis.Nil) {
		return false, nil
	}
	if setResult.Err() != nil {
		return true, setResult.Err()
	}

	return setResult.Bool()
}
