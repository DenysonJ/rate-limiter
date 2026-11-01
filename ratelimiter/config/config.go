package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/DenysonJ/rate-limiter/ratelimiter"
	"github.com/joho/godotenv"
)

const (
	envIPLimit   = "IP_RL_LIMIT"
	envIPWindow  = "IP_RL_WINDOW"
	envIPBlock   = "IP_RL_BLOCK"
	envRedisHost = "REDIS_HOST"
	envRedisPort = "REDIS_PORT"
	envTokens    = "TOKENS_CONFIG"
)

type Settings struct {
	Default      ratelimiter.Config
	TokenConfigs map[string]ratelimiter.Config
	Redis        RedisConfig
}

type RedisConfig struct {
	Host string
	Port string
}

type tokenConfigDTO struct {
	Token  string `json:"token"`
	Limit  int    `json:"limit"`
	Window string `json:"window"`
	Block  string `json:"block"`
}

// Load lê as configurações do ambiente. file é um .env opcional.
func Load(file string) (*Settings, error) {
	_ = godotenv.Load(file) // .env é opcional

	limit, limitErr := strconv.Atoi(os.Getenv(envIPLimit))
	if limitErr != nil {
		return nil, fmt.Errorf("parsing %s: %w", envIPLimit, limitErr)
	}
	window, windowErr := time.ParseDuration(os.Getenv(envIPWindow))
	if windowErr != nil {
		return nil, fmt.Errorf("parsing %s: %w", envIPWindow, windowErr)
	}
	block, blockErr := time.ParseDuration(os.Getenv(envIPBlock))
	if blockErr != nil {
		return nil, fmt.Errorf("parsing %s: %w", envIPBlock, blockErr)
	}

	tokens, tokensErr := parseTokens(os.Getenv(envTokens))
	if tokensErr != nil {
		return nil, tokensErr
	}

	return &Settings{
		Default:      ratelimiter.Config{Limit: limit, Window: window, BlockDuration: block},
		TokenConfigs: tokens,
		Redis:        RedisConfig{Host: os.Getenv(envRedisHost), Port: os.Getenv(envRedisPort)},
	}, nil
}

func parseTokens(raw string) (map[string]ratelimiter.Config, error) {
	configs := make(map[string]ratelimiter.Config)
	if raw == "" {
		return configs, nil
	}

	var dtos []tokenConfigDTO
	if err := json.Unmarshal([]byte(raw), &dtos); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", envTokens, err)
	}

	for _, dto := range dtos {
		window, windowErr := time.ParseDuration(dto.Window)
		if windowErr != nil {
			return nil, fmt.Errorf("parsing window for token %q: %w", dto.Token, windowErr)
		}
		block, blockErr := time.ParseDuration(dto.Block)
		if blockErr != nil {
			return nil, fmt.Errorf("parsing block for token %q: %w", dto.Token, blockErr)
		}
		configs[dto.Token] = ratelimiter.Config{Limit: dto.Limit, Window: window, BlockDuration: block}
	}
	return configs, nil
}
