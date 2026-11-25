package session

import (
	"testing"
	"time"
)

// newStoreWithClock returns a store plus a handle to move its clock forward.
func newStoreWithClock(ttl time.Duration) (*Store, func(time.Duration)) {
	store := NewStore(ttl)
	current := time.Now()
	store.now = func() time.Time { return current }
	return store, func(d time.Duration) { current = current.Add(d) }
}

func TestCreateAndValidate(t *testing.T) {
	store := NewStore(time.Hour)

	token, err := store.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if token == "" {
		t.Fatal("Create returned an empty token")
	}
	if !store.Valid(token) {
		t.Fatal("fresh token should be valid")
	}
	if store.Valid("not-a-token") {
		t.Fatal("unknown token should be invalid")
	}
	if store.Valid("") {
		t.Fatal("empty token should be invalid")
	}
}

func TestTokensAreUnique(t *testing.T) {
	store := NewStore(time.Hour)

	seen := make(map[string]bool, 100)
	for range 100 {
		token, err := store.Create()
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if seen[token] {
			t.Fatalf("duplicate token %q", token)
		}
		seen[token] = true
	}
}

func TestOnlyHashesAreStored(t *testing.T) {
	store := NewStore(time.Hour)

	token, err := store.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	store.mu.Lock()
	_, storedRaw := store.sessions[token]
	store.mu.Unlock()

	if storedRaw {
		t.Fatal("store keeps the raw token instead of its hash")
	}
	if !store.Valid(token) {
		t.Fatal("token should validate through its hash")
	}
}

func TestExpiry(t *testing.T) {
	store, advance := newStoreWithClock(30 * time.Minute)

	token, err := store.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	advance(29 * time.Minute)
	if !store.Valid(token) {
		t.Fatal("token should still be valid before expiry")
	}

	advance(2 * time.Minute)
	if store.Valid(token) {
		t.Fatal("token should be invalid after expiry")
	}
	if store.Len() != 0 {
		t.Fatalf("expired token should be dropped, len=%d", store.Len())
	}
}

func TestDelete(t *testing.T) {
	store := NewStore(time.Hour)

	token, err := store.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	store.Delete(token)
	if store.Valid(token) {
		t.Fatal("deleted token should be invalid")
	}

	// Deleting an unknown token must not panic.
	store.Delete("unknown")
}

func TestDeleteAll(t *testing.T) {
	store := NewStore(time.Hour)

	tokens := make([]string, 3)
	for i := range tokens {
		token, err := store.Create()
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		tokens[i] = token
	}

	store.DeleteAll()
	if store.Len() != 0 {
		t.Fatalf("DeleteAll left %d sessions", store.Len())
	}
	for _, token := range tokens {
		if store.Valid(token) {
			t.Fatal("token should be invalid after DeleteAll")
		}
	}
}

func TestCleanup(t *testing.T) {
	store, advance := newStoreWithClock(10 * time.Minute)

	if _, err := store.Create(); err != nil {
		t.Fatalf("Create: %v", err)
	}

	advance(11 * time.Minute)

	if removed := store.Cleanup(); removed != 1 {
		t.Fatalf("Cleanup removed %d sessions, want 1", removed)
	}
	if store.Len() != 0 {
		t.Fatalf("Cleanup left %d sessions", store.Len())
	}
}

func TestDefaultTTL(t *testing.T) {
	store, advance := newStoreWithClock(0)

	token, err := store.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	advance(DefaultTTL - time.Minute)
	if !store.Valid(token) {
		t.Fatal("token should be valid before the default TTL elapses")
	}

	advance(2 * time.Minute)
	if store.Valid(token) {
		t.Fatal("token should be invalid after the default TTL elapses")
	}
}
