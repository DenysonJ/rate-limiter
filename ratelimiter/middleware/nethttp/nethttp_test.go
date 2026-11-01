package nethttp_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DenysonJ/rate-limiter/ratelimiter"
	"github.com/DenysonJ/rate-limiter/ratelimiter/middleware/nethttp"
	"github.com/DenysonJ/rate-limiter/ratelimiter/store/memory"
	"github.com/stretchr/testify/assert"
)

func newHandler(rl *ratelimiter.RateLimiter, opts ...nethttp.Option) http.Handler {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return nethttp.Handler(rl, opts...)(ok)
}

// Cobre RN-10 happy path: requisição permitida chega ao próximo handler.
func TestHandler_AllowsRequest(t *testing.T) {
	rl := ratelimiter.New(memory.New(), ratelimiter.Config{Limit: 5, Window: time.Second, BlockDuration: time.Second})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("API_KEY", "token-a")

	newHandler(rl).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

// Cobre RN-06 e RN-10: ao exceder o limite responde 429 com o corpo padrão.
func TestHandler_DeniesWithTooManyRequests(t *testing.T) {
	rl := ratelimiter.New(memory.New(), ratelimiter.Config{Limit: 0, Window: time.Second, BlockDuration: time.Second})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("API_KEY", "token-b")

	newHandler(rl).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body, _ := io.ReadAll(rec.Result().Body)
	assert.Contains(t, string(body), "maximum number of requests")
}

// Cobre RN-10: status e corpo configuráveis via WithResponse.
func TestHandler_CustomResponse(t *testing.T) {
	rl := ratelimiter.New(memory.New(), ratelimiter.Config{Limit: 0, Window: time.Second, BlockDuration: time.Second})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("API_KEY", "token-c")

	handler := newHandler(rl, nethttp.WithResponse(http.StatusServiceUnavailable, "slow down", "text/plain"))
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "slow down", rec.Body.String())
}

// Cobre RN-03: a chave é derivada do header configurado (contadores independentes por header).
func TestHandler_KeyFromConfiguredHeader(t *testing.T) {
	rl := ratelimiter.New(memory.New(), ratelimiter.Config{Limit: 1, Window: time.Second, BlockDuration: time.Second})
	handler := newHandler(rl, nethttp.WithKeyHeader("X-Token"))

	do := func(token string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Token", token)
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusOK, do("one"))              // 1º acesso de "one"
	assert.Equal(t, http.StatusOK, do("two"))              // chave diferente, contador próprio
	assert.Equal(t, http.StatusTooManyRequests, do("one")) // 2º acesso de "one" excede
}

// Cobre RN-03: função de extração de chave customizada tem prioridade.
func TestHandler_CustomKeyFunc(t *testing.T) {
	rl := ratelimiter.New(memory.New(), ratelimiter.Config{Limit: 1, Window: time.Second, BlockDuration: time.Second})
	keyFunc := func(_ *http.Request) string { return "fixed" }
	handler := newHandler(rl, nethttp.WithKeyFunc(keyFunc))

	do := func() int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusOK, do())
	assert.Equal(t, http.StatusTooManyRequests, do()) // mesma chave fixa excede
}
