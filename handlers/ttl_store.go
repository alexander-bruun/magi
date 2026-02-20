package handlers

import (
	"sync"
	"time"
)

// TTLStore is a generic thread-safe in-memory store with automatic TTL-based cleanup.
// It replaces the repeated pattern of sync.RWMutex + map + background cleanup goroutine
// found across multiple middleware files.
type TTLStore[V any] struct {
	mu       sync.RWMutex
	entries  map[string]*V
	ttl      time.Duration
	lastSeen func(*V) time.Time
}

// NewTTLStore creates a new TTLStore and starts a background cleanup goroutine.
// ttl is the duration after which entries expire.
// cleanupInterval controls how often expired entries are purged.
// lastSeenFn extracts the "last active" timestamp from an entry.
func NewTTLStore[V any](ttl, cleanupInterval time.Duration, lastSeenFn func(*V) time.Time) *TTLStore[V] {
	s := &TTLStore[V]{
		entries:  make(map[string]*V),
		ttl:      ttl,
		lastSeen: lastSeenFn,
	}
	go s.cleanup(cleanupInterval)
	return s
}

// Get retrieves an entry by key. Returns nil, false if not found.
func (s *TTLStore[V]) Get(key string) (*V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.entries[key]
	return v, ok
}

// Set stores an entry by key.
func (s *TTLStore[V]) Set(key string, val *V) {
	s.mu.Lock()
	s.entries[key] = val
	s.mu.Unlock()
}

// Delete removes an entry by key.
func (s *TTLStore[V]) Delete(key string) {
	s.mu.Lock()
	delete(s.entries, key)
	s.mu.Unlock()
}

// GetOrCreate retrieves an existing entry or creates a new one using factory.
// The operation is atomic (holds write lock).
func (s *TTLStore[V]) GetOrCreate(key string, factory func() *V) *V {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.entries[key]; ok {
		return v
	}
	v := factory()
	s.entries[key] = v
	return v
}

// Len returns the number of entries in the store.
func (s *TTLStore[V]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// Range iterates over all entries while holding a read lock.
// The callback should not modify the store.
func (s *TTLStore[V]) Range(fn func(key string, val *V) bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for k, v := range s.entries {
		if !fn(k, v) {
			break
		}
	}
}

// Cleanup with write lock — used for custom cleanup in Range
func (s *TTLStore[V]) DeleteWithLock(key string) {
	// Caller must hold no locks — this acquires its own
	s.mu.Lock()
	delete(s.entries, key)
	s.mu.Unlock()
}

func (s *TTLStore[V]) cleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for k, v := range s.entries {
			if now.Sub(s.lastSeen(v)) > s.ttl {
				delete(s.entries, k)
			}
		}
		s.mu.Unlock()
	}
}
