package ginmw_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DenysonJ/rate-limiter/ratelimiter"
	"github.com/DenysonJ/rate-limiter/ratelimiter/middleware/ginmw"
	"github.com/DenysonJ/rate-limiter/ratelimiter/store/memory"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newRouter(rl *ratelimiter.RateLimiter, opts ...ginmw.Option) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ginmw.Middleware(rl, opts...))
	r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

// Cobre RN-10 happy path para o adapter Gin.
func TestMiddleware_AllowsRequest(t *testing.T) {
	rl := ratelimiter.New(memory.New(), ratelimiter.Config{Limit: 5, Window: time.Second, BlockDuration: time.Second})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("API_KEY", "gin-a")

	newRouter(rl).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

// Cobre RN-06 e RN-10: Gin responde 429 JSON ao exceder o limite.
func TestMiddleware_DeniesWithTooManyRequests(t *testing.T) {
	rl := ratelimiter.New(memory.New(), ratelimiter.Config{Limit: 0, Window: time.Second, BlockDuration: time.Second})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("API_KEY", "gin-b")

	newRouter(rl).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Contains(t, rec.Body.String(), "maximum number of requests")
}
