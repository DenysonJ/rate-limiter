package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/DenysonJ/rate-limiter/ratelimiter"
	"github.com/DenysonJ/rate-limiter/ratelimiter/store/memory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Cobre RN-11: a estratégia in-memory satisfaz a porta do core.
var _ ratelimiter.Strategy = (*memory.Strategy)(nil)

func TestStrategy_IncrementCountsWithinWindowAndResets(t *testing.T) {
	s := memory.New()
	ctx := context.Background()

	first, err := s.Increment(ctx, "k", 20*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 1, first)

	second, err := s.Increment(ctx, "k", 20*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 2, second)

	time.Sleep(30 * time.Millisecond)
	afterReset, err := s.Increment(ctx, "k", 20*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 1, afterReset)
}

func TestStrategy_BlockAndIsBlockedExpire(t *testing.T) {
	s := memory.New()
	ctx := context.Background()

	require.NoError(t, s.Block(ctx, "k", 20*time.Millisecond))

	blocked, err := s.IsBlocked(ctx, "k")
	require.NoError(t, err)
	assert.True(t, blocked)

	time.Sleep(30 * time.Millisecond)
	blocked, err = s.IsBlocked(ctx, "k")
	require.NoError(t, err)
	assert.False(t, blocked)
}
