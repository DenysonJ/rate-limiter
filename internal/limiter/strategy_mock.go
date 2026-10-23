package limiter

import (
	"context"
	"sync"
	"time"
)

// counterEntry guarda o contador de uma janela e quando ela expira.
type counterEntry struct {
	count     int
	expiresAt time.Time
}

// MemoryStrategy é um fake in-memory da Strategy, para testes unitários.
type MemoryStrategy struct {
	mu       sync.Mutex
	counters map[string]counterEntry
	blocks   map[string]time.Time
}

func NewMemoryStrategy() *MemoryStrategy {
	return &MemoryStrategy{
		counters: make(map[string]counterEntry),
		blocks:   make(map[string]time.Time),
	}
}

func (m *MemoryStrategy) Increment(ctx context.Context, key string, window time.Duration) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	entry, ok := m.counters[key]

	// Janela nova: chave inexistente OU a janela anterior já expirou.
	if !ok || now.After(entry.expiresAt) {
		entry = counterEntry{count: 0, expiresAt: now.Add(window)}
	}

	entry.count++
	m.counters[key] = entry
	return entry.count, nil
}

func (m *MemoryStrategy) Block(ctx context.Context, key string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.blocks[key] = time.Now().Add(ttl)
	return nil
}

func (m *MemoryStrategy) IsBlocked(ctx context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	expiresAt, ok := m.blocks[key]
	if !ok {
		return false, nil
	}

	// Expiração "preguiçosa": só limpa quando alguém consulta.
	if time.Now().After(expiresAt) {
		delete(m.blocks, key)
		return false, nil
	}
	return true, nil
}
