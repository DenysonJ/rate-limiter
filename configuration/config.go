package configuration

import (
	"encoding/json"
	"os"
	"strconv"
	"time"

	"github.com/DenysonJ/rate-limiter/internal/limiter"
	"github.com/bytedance/gopkg/util/logger"
	"github.com/joho/godotenv"
)

type ConfigEnv struct {
	IpConfig    limiter.Config
	RedisConfig RedisConfig
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

const IpConfigWindow = "IP_RL_WINDOW"
const IpBlockWindow = "IP_RL_BLOCK"
const IpLimit = "IP_RL_LIMIT"
const RedisHost = "REDIS_HOST"
const RedisPort = "REDIS_PORT"

func InitConfiguration(file string) *ConfigEnv {
	if err := godotenv.Load(file); err != nil {
		logger.Info("No .env file found; using environment variables")
	}

	ipLimit, limitErr := strconv.Atoi(os.Getenv(IpLimit))
	if limitErr != nil {
		logger.Fatal("Error loading ip limit")
	}

	ipWindowDuration, ipWindowErr := time.ParseDuration(os.Getenv(IpConfigWindow))
	if ipWindowErr != nil {
		logger.Fatal("Error loading ip window")
	}

	ipBlockDuration, ipBlockErr := time.ParseDuration(os.Getenv(IpBlockWindow))
	if ipBlockErr != nil {
		logger.Fatal("Error loading ip window")
	}

	return &ConfigEnv{
		IpConfig: limiter.Config{
			Limit:         ipLimit,
			Window:        ipWindowDuration,
			BlockDuration: ipBlockDuration,
		},
		RedisConfig: RedisConfig{
			Host: os.Getenv(RedisHost),
			Port: os.Getenv(RedisPort),
		},
	}
}

func GetTokenConfigs() map[string]limiter.Config {
	var dtos []tokenConfigDTO
	unmarshalErr := json.Unmarshal([]byte(os.Getenv("TOKENS_CONFIG")), &dtos)
	if unmarshalErr != nil {
		logger.Error("Error loading token configs")
	}

	var configs = make(map[string]limiter.Config)
	for _, config := range dtos {
		windowDuration, windowErr := time.ParseDuration(config.Window)
		if windowErr != nil {
			logger.Error("Error loading token window", windowErr)
		}

		blockDuration, blockErr := time.ParseDuration(config.Block)
		if blockErr != nil {
			logger.Error("Error loading ip window", blockErr)
		}
		configs[config.Token] = limiter.Config{
			Limit:         config.Limit,
			Window:        windowDuration,
			BlockDuration: blockDuration,
		}
	}
	return nil
}
