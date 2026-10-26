package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DenysonJ/rate-limiter/configuration"
	"github.com/DenysonJ/rate-limiter/internal/infra/http/middleware"
	redisLimiter "github.com/DenysonJ/rate-limiter/internal/infra/redis"
	"github.com/DenysonJ/rate-limiter/internal/limiter"
	"github.com/bytedance/gopkg/util/logger"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func main() {
	r := gin.New()

	configEnv := configuration.InitConfiguration(".env")
	tokenConfigs := configuration.GetTokenConfigs()

	limiterStrategy := redisLimiter.NewRedisLimiter(redis.NewClient(&redis.Options{
		Addr: configEnv.RedisConfig.Host + ":" + configEnv.RedisConfig.Port,
	}))
	limiterService := limiter.NewRateLimiter(limiterStrategy, configEnv.IpConfig, tokenConfigs)

	r.Use(gin.Recovery())
	r.Use(middleware.RateLimiterMiddleware(limiterService))

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello World")
	})

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("Starting server", "port", srv.Addr)
		if listenErr := srv.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			errCh <- listenErr
		}
	}()

	// Wait for interrupt signal or server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case serverErr := <-errCh:
		logger.Fatal("server failed: " + serverErr.Error())
	case <-quit:
		// proceed to graceful shutdown
	}

	logger.Info("shutting down server...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
		return
	}

	logger.Info("server exited properly")
}
