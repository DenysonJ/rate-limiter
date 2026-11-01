package ratelimiter

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrBackendUnavailable envolve qualquer falha da Strategy, para o chamador
// distinguir com errors.Is.
var ErrBackendUnavailable = errors.New("rate limiter: storage backend unavailable")

type Config struct {
	Limit         int
	Window        time.Duration
	BlockDuration time.Duration
}

type RateLimiter struct {
	strategy      Strategy
	defaultConfig Config
	tokenConfig   map[string]Config
	failOpen      bool
}

type Option func(*RateLimiter)

// WithFailClosed faz o limiter negar a requisição quando o backend falhar
// (padrão é fail-open).
func WithFailClosed() Option {
	return func(rl *RateLimiter) { rl.failOpen = false }
}

func WithTokenConfigs(configs map[string]Config) Option {
	return func(rl *RateLimiter) {
		for key, cfg := range configs {
			rl.tokenConfig[key] = cfg
		}
	}
}

func New(strategy Strategy, defaultConfig Config, opts ...Option) *RateLimiter {
	rl := &RateLimiter{
		strategy:      strategy,
		defaultConfig: defaultConfig,
		tokenConfig:   make(map[string]Config),
		failOpen:      true,
	}
	for _, opt := range opts {
		opt(rl)
	}
	return rl
}

func (rl *RateLimiter) RegisterTokenConfig(key string, cfg Config) {
	rl.tokenConfig[key] = cfg
}

func (rl *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	blocked, blockedErr := rl.strategy.IsBlocked(ctx, key)
	if blockedErr != nil {
		return rl.onBackendError(fmt.Errorf("checking blocked state: %w: %w", ErrBackendUnavailable, blockedErr))
	}
	if blocked {
		return false, nil
	}

	config := rl.defaultConfig
	if tokenConfig := rl.tokenConfig[key]; tokenConfig.Limit > 0 {
		config = tokenConfig
	}

	count, incErr := rl.strategy.Increment(ctx, key, config.Window)
	if incErr != nil {
		return rl.onBackendError(fmt.Errorf("incrementing counter: %w: %w", ErrBackendUnavailable, incErr))
	}

	if count > config.Limit {
		if blockErr := rl.strategy.Block(ctx, key, config.BlockDuration); blockErr != nil {
			return rl.onBackendError(fmt.Errorf("blocking key: %w: %w", ErrBackendUnavailable, blockErr))
		}
		return false, nil
	}

	return true, nil
}

// onBackendError aplica a política de falha e devolve o erro ao chamador.
func (rl *RateLimiter) onBackendError(err error) (bool, error) {
	return rl.failOpen, err
}
