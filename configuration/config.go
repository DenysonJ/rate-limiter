package configuration

import (
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

func GetTokenConfigs() []limiter.Config {
	return nil
}
