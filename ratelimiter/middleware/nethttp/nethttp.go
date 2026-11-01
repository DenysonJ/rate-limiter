package nethttp

import (
	"net"
	"net/http"
	"strings"

	"github.com/DenysonJ/rate-limiter/ratelimiter"
)

const (
	defaultKeyHeader = "API_KEY"
	defaultBody      = `{"message":"you have reached the maximum number of requests or actions allowed within a certain time frame"}`
)

// KeyFunc deriva a chave de rate-limit a partir da requisição.
type KeyFunc func(r *http.Request) string

type options struct {
	keyHeader   string
	keyFunc     KeyFunc
	statusCode  int
	body        string
	contentType string
}

type Option func(*options)

func WithKeyHeader(header string) Option { return func(o *options) { o.keyHeader = header } }

func WithKeyFunc(f KeyFunc) Option { return func(o *options) { o.keyFunc = f } }

func WithResponse(status int, body, contentType string) Option {
	return func(o *options) {
		o.statusCode = status
		o.body = body
		o.contentType = contentType
	}
}

func Handler(rl *ratelimiter.RateLimiter, opts ...Option) func(http.Handler) http.Handler {
	o := options{
		keyHeader:   defaultKeyHeader,
		statusCode:  http.StatusTooManyRequests,
		body:        defaultBody,
		contentType: "application/json",
	}
	for _, opt := range opts {
		opt(&o)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Erro do backend já está refletido em allowed pela política de falha.
			allowed, _ := rl.Allow(r.Context(), resolveKey(r, &o))
			if !allowed {
				w.Header().Set("Content-Type", o.contentType)
				w.WriteHeader(o.statusCode)
				_, _ = w.Write([]byte(o.body))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func resolveKey(r *http.Request, o *options) string {
	if o.keyFunc != nil {
		return o.keyFunc(r)
	}
	if key := r.Header.Get(o.keyHeader); key != "" {
		return key
	}
	return clientIP(r)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
