package limiter

import (
	"context"
	"time"

	"github.com/bytedance/gopkg/util/logger"
)

type RateLimiter struct {
	limiterStrategy Limiter
	ipConfig        Config
	tokenConfig     map[string]Config
}

type Config struct {
	Limit         int
	Window        time.Duration
	BlockDuration time.Duration
}

func NewRateLimiter(limiterStrategy Limiter, ipConfig Config, tokenConfigs map[string]Config) *RateLimiter {
	rl := &RateLimiter{
		limiterStrategy: limiterStrategy,
		ipConfig:        ipConfig,
		tokenConfig:     make(map[string]Config),
	}

	for k, v := range tokenConfigs {
		rl.RegisterTokenConfig(k, v)
	}

	return rl
}

func (rl *RateLimiter) RegisterTokenConfig(key string, config Config) {
	rl.tokenConfig[key] = config
}

func (rl *RateLimiter) Allow(ctx context.Context, key string) bool {
	blocked, blockedErr := rl.limiterStrategy.IsBlocked(ctx, key)

	if blockedErr != nil {
		logger.Error("Error consulting if Key is blocked")
	}
	if blocked {
		logger.Info("Key is blocked")
		return false
	}

	config := rl.ipConfig
	if configToken := rl.tokenConfig[key]; configToken.Limit > 0 {
		config = configToken
	}

	count, incErr := rl.limiterStrategy.Increment(ctx, key, config.Window)
	if incErr != nil {
		logger.Error("Error incrementing key access")
		return false
	}
	if count > config.Limit {
		logger.Info("Blocking key, over the limit")
		blockErr := rl.limiterStrategy.Block(ctx, key, config.BlockDuration)
		if blockErr != nil {
			logger.Error("Error blocking")
			return false
		}
		return false
	}

	return true
}
