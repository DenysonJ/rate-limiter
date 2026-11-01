package ginmw

import (
	"net/http"

	"github.com/DenysonJ/rate-limiter/ratelimiter"
	"github.com/gin-gonic/gin"
)

const defaultKeyHeader = "API_KEY"

var defaultBody = gin.H{
	"message": "you have reached the maximum number of requests or actions allowed within a certain time frame",
}

// KeyFunc deriva a chave de rate-limit a partir do contexto Gin.
type KeyFunc func(c *gin.Context) string

type options struct {
	keyHeader  string
	keyFunc    KeyFunc
	statusCode int
	body       any
}

type Option func(*options)

func WithKeyHeader(header string) Option { return func(o *options) { o.keyHeader = header } }

func WithKeyFunc(f KeyFunc) Option { return func(o *options) { o.keyFunc = f } }

func WithResponse(status int, body any) Option {
	return func(o *options) {
		o.statusCode = status
		o.body = body
	}
}

func Middleware(rl *ratelimiter.RateLimiter, opts ...Option) gin.HandlerFunc {
	o := options{keyHeader: defaultKeyHeader, statusCode: http.StatusTooManyRequests, body: defaultBody}
	for _, opt := range opts {
		opt(&o)
	}

	return func(c *gin.Context) {
		// Erro do backend já está refletido em allowed pela política de falha.
		allowed, _ := rl.Allow(c.Request.Context(), resolveKey(c, &o))
		if !allowed {
			c.AbortWithStatusJSON(o.statusCode, o.body)
			return
		}
		c.Next()
	}
}

func resolveKey(c *gin.Context, o *options) string {
	if o.keyFunc != nil {
		return o.keyFunc(c)
	}
	if key := c.GetHeader(o.keyHeader); key != "" {
		return key
	}
	return c.ClientIP()
}
