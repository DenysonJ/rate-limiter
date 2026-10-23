package limiter

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_Allow(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		config Config
		access int
		want   bool
	}{
		{
			name:   "given a new key when the rate limiter checks it then it allows it",
			key:    "test",
			config: Config{},
			access: 1,
			want:   true,
		},
		{
			name: "given a key exceeds the limit when the rate limiter checks it then it deny the access",
			key:  "test-2",
			config: Config{
				Limit:         2,
				Window:        5 * time.Second,
				BlockDuration: 5 * time.Second,
			},
			access: 3,
			want:   false,
		},
		{
			name:   "given a key access the ip limit when the rate limiter checks it then it allow the access",
			key:    "test-3",
			config: Config{},
			access: 5,
			want:   true,
		},
		{
			name: "given a key access the token limit when the rate limiter checks it then it allow the access",
			key:  "test-2",
			config: Config{
				Limit:         6,
				Window:        5 * time.Second,
				BlockDuration: 5 * time.Second,
			},
			access: 5,
			want:   true,
		},
		{
			name: "given a key exceeds the limit ip limit but not the token limit when the rate limiter checks it then it allow the access",
			key:  "test-5",
			config: Config{
				Limit:         10,
				Window:        5 * time.Second,
				BlockDuration: 5 * time.Second,
			},
			access: 6,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memory := NewMemoryStrategy()
			rateL := NewRateLimiter(memory, Config{
				Limit:         5,
				Window:        5 * time.Second,
				BlockDuration: 5 * time.Second,
			})
			rateL.RegisterTokenConfig(tt.key, tt.config)

			for range tt.access - 1 {
				rateL.Allow(context.Background(), tt.key)
			}

			allow := rateL.Allow(context.Background(), tt.key)

			assert.Equal(t, tt.want, allow)
		})
	}
}
