package middleware

import (
	"context"
	"net/http"

	"github.com/DenysonJ/rate-limiter/internal/limiter"
	"github.com/gin-gonic/gin"
)

func RateLimiterMiddleware(limiter *limiter.RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("API_KEY")

		if key == "" {
			key = c.ClientIP()
		}

		if !limiter.Allow(context.Background(), key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"message": "you have reached the maximum number of requests or actions allowed within a certain time frame",
			})
			return
		}
		c.Next()
	}
}
