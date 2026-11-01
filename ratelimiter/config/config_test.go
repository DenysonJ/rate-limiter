package config_test

import (
	"testing"
	"time"

	"github.com/DenysonJ/rate-limiter/ratelimiter/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Cobre RN-12: helper opcional carrega Config padrão e por token do ambiente.
func TestLoad_ValidEnvironment(t *testing.T) {
	t.Setenv("IP_RL_LIMIT", "10")
	t.Setenv("IP_RL_WINDOW", "2s")
	t.Setenv("IP_RL_BLOCK", "5s")
	t.Setenv("REDIS_HOST", "redis")
	t.Setenv("REDIS_PORT", "6379")
	t.Setenv("TOKENS_CONFIG", `[{"token":"abc","limit":100,"window":"1s","block":"5m"}]`)

	settings, err := config.Load("")

	require.NoError(t, err)
	assert.Equal(t, 10, settings.Default.Limit)
	assert.Equal(t, 2*time.Second, settings.Default.Window)
	assert.Equal(t, 5*time.Second, settings.Default.BlockDuration)
	assert.Equal(t, "redis", settings.Redis.Host)

	require.Contains(t, settings.TokenConfigs, "abc")
	assert.Equal(t, 100, settings.TokenConfigs["abc"].Limit)
	assert.Equal(t, 5*time.Minute, settings.TokenConfigs["abc"].BlockDuration)
}

func TestLoad_InvalidLimitReturnsError(t *testing.T) {
	t.Setenv("IP_RL_LIMIT", "not-a-number")
	t.Setenv("IP_RL_WINDOW", "2s")
	t.Setenv("IP_RL_BLOCK", "5s")

	_, err := config.Load("")

	require.Error(t, err)
}
