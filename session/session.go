// Package session keeps admin session tokens in memory.
//
// Only a SHA-256 hash of each token is stored, so a memory dump does not expose
// usable credentials. Sessions are intentionally ephemeral: restarting the API
// signs the operator out.
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"sync"
	"time"
)

// DefaultTTL is used when NewStore receives a non-positive lifetime.
const DefaultTTL = 12 * time.Hour

// Store is a concurrency-safe set of live session tokens.
type Store struct {
	mu       sync.Mutex
	sessions map[string]time.Time // token hash -> expiry
	ttl      time.Duration
	now      func() time.Time
}

// NewStore returns a store whose tokens live for ttl.
func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	return &Store{
		sessions: make(map[string]time.Time),
		ttl:      ttl,
		now:      time.Now,
	}
}

// Create issues a new token and returns it. Only its hash is retained.
func (s *Store) Create() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}

	token := base64.RawURLEncoding.EncodeToString(raw)

	s.mu.Lock()
	s.sessions[hashToken(token)] = s.now().Add(s.ttl)
	s.mu.Unlock()

	return token, nil
}

// Valid reports whether token refers to a live session. Expired tokens are
// dropped as a side effect.
func (s *Store) Valid(token string) bool {
	if token == "" {
		return false
	}

	key := hashToken(token)

	s.mu.Lock()
	defer s.mu.Unlock()

	expires, ok := s.sessions[key]
	if !ok {
		return false
	}
	if !s.now().Before(expires) {
		delete(s.sessions, key)
		return false
	}

	return true
}

// Delete revokes a single session. Unknown tokens are ignored.
func (s *Store) Delete(token string) {
	if token == "" {
		return
	}

	s.mu.Lock()
	delete(s.sessions, hashToken(token))
	s.mu.Unlock()
}

// DeleteAll revokes every session, for example after a credential change.
func (s *Store) DeleteAll() {
	s.mu.Lock()
	s.sessions = make(map[string]time.Time)
	s.mu.Unlock()
}

// Cleanup drops expired sessions and returns how many were removed.
func (s *Store) Cleanup() int {
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	for key, expires := range s.sessions {
		if !now.Before(expires) {
			delete(s.sessions, key)
			removed++
		}
	}

	return removed
}

// Len reports how many sessions are stored, including not-yet-collected ones.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.sessions)
}

// RunCleanup collects expired sessions until ctx is cancelled.
func (s *Store) RunCleanup(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Cleanup()
		}
	}
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
