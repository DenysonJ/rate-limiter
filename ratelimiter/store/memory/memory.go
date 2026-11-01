package memory

import (
	"context"
	"sync"
	"time"
)

type counterEntry struct {
	count     int
	expiresAt time.Time
}

// Strategy é um backend in-memory para contagem e bloqueio de chaves.
type Strategy struct {
	mu       sync.Mutex
	counters map[string]counterEntry
	blocks   map[string]time.Time
}

func New() *Strategy {
	return &Strategy{
		counters: make(map[string]counterEntry),
		blocks:   make(map[string]time.Time),
	}
}

func (s *Strategy) Increment(_ context.Context, key string, window time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	entry, ok := s.counters[key]
	// Janela nova: chave inexistente ou janela anterior expirada.
	if !ok || now.After(entry.expiresAt) {
		entry = counterEntry{expiresAt: now.Add(window)}
	}
	entry.count++
	s.counters[key] = entry
	return entry.count, nil
}

func (s *Strategy) Block(_ context.Context, key string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blocks[key] = time.Now().Add(ttl)
	return nil
}

func (s *Strategy) IsBlocked(_ context.Context, key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	expiresAt, ok := s.blocks[key]
	if !ok {
		return false, nil
	}
	// Expiração preguiçosa: limpa ao consultar.
	if time.Now().After(expiresAt) {
		delete(s.blocks, key)
		return false, nil
	}
	return true, nil
}
