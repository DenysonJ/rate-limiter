package ratelimiter_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DenysonJ/rate-limiter/ratelimiter"
	"github.com/DenysonJ/rate-limiter/ratelimiter/store/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubStrategy injeta erros e resultados controlados para exercitar as políticas.
type stubStrategy struct {
	count        int
	blocked      bool
	incErr       error
	blockErr     error
	isBlockedErr error
	gotCtx       context.Context
}

func (s *stubStrategy) Increment(ctx context.Context, _ string, _ time.Duration) (int, error) {
	s.gotCtx = ctx
	return s.count, s.incErr
}

func (s *stubStrategy) Block(_ context.Context, _ string, _ time.Duration) error {
	return s.blockErr
}

func (s *stubStrategy) IsBlocked(ctx context.Context, _ string) (bool, error) {
	s.gotCtx = ctx
	return s.blocked, s.isBlockedErr
}

func defaultCfg() ratelimiter.Config {
	return ratelimiter.Config{Limit: 5, Window: 5 * time.Second, BlockDuration: 5 * time.Second}
}

// Cobre RN-04, RN-05, RN-06, RN-07 com o backend in-memory.
func TestAllow_WithMemoryStrategy(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		config ratelimiter.Config
		access int
		want   bool
	}{
		{
			name:   "GIVEN a new key WHEN checked THEN it is allowed (RN-05)",
			key:    "new",
			config: ratelimiter.Config{},
			access: 1,
			want:   true,
		},
		{
			name:   "GIVEN a key over its token limit WHEN checked THEN it is denied (RN-06)",
			key:    "over",
			config: ratelimiter.Config{Limit: 2, Window: 5 * time.Second, BlockDuration: 5 * time.Second},
			access: 3,
			want:   false,
		},
		{
			name:   "GIVEN a key under the default IP limit WHEN checked THEN it is allowed (RN-04)",
			key:    "under-default",
			config: ratelimiter.Config{},
			access: 5,
			want:   true,
		},
		{
			name:   "GIVEN a key under its higher token limit WHEN checked THEN it is allowed (RN-04)",
			key:    "under-token",
			config: ratelimiter.Config{Limit: 6, Window: 5 * time.Second, BlockDuration: 5 * time.Second},
			access: 6,
			want:   true,
		},
		{
			name:   "GIVEN a key over IP limit but under token limit WHEN checked THEN it is allowed (RN-04)",
			key:    "token-wins",
			config: ratelimiter.Config{Limit: 10, Window: 5 * time.Second, BlockDuration: 5 * time.Second},
			access: 6,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := ratelimiter.New(memory.New(), defaultCfg())
			rl.RegisterTokenConfig(tt.key, tt.config)

			var allowed bool
			for i := 0; i < tt.access; i++ {
				var allowErr error
				allowed, allowErr = rl.Allow(context.Background(), tt.key)
				require.NoError(t, allowErr)
			}

			assert.Equal(t, tt.want, allowed)
		})
	}
}

// Cobre RN-07: chave bloqueada é negada sem incrementar o contador.
func TestAllow_BlockedKeyDeniedWithoutIncrement(t *testing.T) {
	strategy := &stubStrategy{blocked: true, incErr: errors.New("increment must not be called")}
	rl := ratelimiter.New(strategy, defaultCfg())

	allowed, err := rl.Allow(context.Background(), "blocked")

	require.NoError(t, err) // sem erro prova que Increment não foi chamado
	assert.False(t, allowed)
}

// Cobre RN-08 e RN-09: fail-open (padrão) permite e propaga o erro do backend.
func TestAllow_FailOpenOnBackendError(t *testing.T) {
	strategy := &stubStrategy{isBlockedErr: errors.New("redis down")}
	rl := ratelimiter.New(strategy, defaultCfg())

	allowed, err := rl.Allow(context.Background(), "key")

	assert.True(t, allowed)
	assert.ErrorIs(t, err, ratelimiter.ErrBackendUnavailable)
}

// Cobre RN-08: fail-closed nega e propaga o erro do backend.
func TestAllow_FailClosedOnBackendError(t *testing.T) {
	strategy := &stubStrategy{incErr: errors.New("redis down")}
	rl := ratelimiter.New(strategy, defaultCfg(), ratelimiter.WithFailClosed())

	allowed, err := rl.Allow(context.Background(), "key")

	assert.False(t, allowed)
	assert.ErrorIs(t, err, ratelimiter.ErrBackendUnavailable)
}

// Cobre RN-04 e RN-12: WithTokenConfigs via construtor sem variáveis de ambiente.
func TestNew_WithTokenConfigs(t *testing.T) {
	strategy := memory.New()
	rl := ratelimiter.New(strategy, ratelimiter.Config{Limit: 1, Window: time.Second, BlockDuration: time.Second},
		ratelimiter.WithTokenConfigs(map[string]ratelimiter.Config{
			"vip": {Limit: 3, Window: time.Second, BlockDuration: time.Second},
		}))

	var allowed bool
	for i := 0; i < 3; i++ {
		allowed, _ = rl.Allow(context.Background(), "vip")
	}

	assert.True(t, allowed) // 3 acessos sob o limite do token
}

// Cobre RN-13: o context do chamador é propagado à Strategy.
func TestAllow_PropagatesContext(t *testing.T) {
	type ctxKey struct{}
	strategy := &stubStrategy{}
	rl := ratelimiter.New(strategy, defaultCfg())
	ctx := context.WithValue(context.Background(), ctxKey{}, "value")

	_, err := rl.Allow(ctx, "key")

	require.NoError(t, err)
	assert.Equal(t, "value", strategy.gotCtx.Value(ctxKey{}))
}
