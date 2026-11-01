package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DenysonJ/rate-limiter/ratelimiter"
	"github.com/DenysonJ/rate-limiter/ratelimiter/config"
	"github.com/DenysonJ/rate-limiter/ratelimiter/middleware/ginmw"
	"github.com/DenysonJ/rate-limiter/ratelimiter/store/redisstore"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func main() {
	settings, settingsErr := config.Load(".env")
	if settingsErr != nil {
		slog.Error("loading configuration", "error", settingsErr)
		os.Exit(1)
	}

	strategy := redisstore.New(redis.NewClient(&redis.Options{
		Addr: settings.Redis.Host + ":" + settings.Redis.Port,
	}))
	limiter := ratelimiter.New(strategy, settings.Default, ratelimiter.WithTokenConfigs(settings.TokenConfigs))

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(ginmw.Middleware(limiter))
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello World")
	})

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("starting server", "port", srv.Addr)
		if listenErr := srv.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			errCh <- listenErr
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case serverErr := <-errCh:
		slog.Error("server failed", "error", serverErr)
		os.Exit(1)
	case <-quit:
	}

	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
		slog.Error("graceful shutdown failed", "error", shutdownErr)
		return
	}

	slog.Info("server exited properly")
}
