package retrier

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"golearn/model"
)

type fakeStore struct {
	mu       sync.Mutex
	orders   []model.UserOrder
	err      error
	recorded []uuid.UUID
	cutoff   time.Time
	max      int
}

func (s *fakeStore) GetShipmentRetries(attemptedBefore time.Time, maxAttempts int) ([]model.UserOrder, error) {
	s.cutoff = attemptedBefore
	s.max = maxAttempts
	return s.orders, s.err
}

func (s *fakeStore) RecordShipmentAttempt(id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recorded = append(s.recorded, id)
	return nil
}

func (s *fakeStore) attemptCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.recorded)
}

type fakeShipper struct {
	results map[uuid.UUID]bool
	calls   []uuid.UUID
}

func (s *fakeShipper) RetryShipmentFor(order model.UserOrder) bool {
	s.calls = append(s.calls, order.ID)
	return s.results[order.ID]
}

func paidOrder() model.UserOrder {
	return model.UserOrder{
		ID:      uuid.New(),
		Status:  model.OrderStatusPaid,
		Courier: "jne",
	}
}

func TestSweepAttemptsDueOrders(t *testing.T) {
	first, second := paidOrder(), paidOrder()
	store := &fakeStore{orders: []model.UserOrder{first, second}}
	shipper := &fakeShipper{results: map[uuid.UUID]bool{first.ID: true}}

	r := New(store, shipper, 3, 10*time.Minute)
	r.Sweep(context.Background())

	if len(store.recorded) != 2 {
		t.Fatalf("recorded = %d attempts, want 2", len(store.recorded))
	}
	if len(shipper.calls) != 2 {
		t.Fatalf("shipper calls = %d, want 2", len(shipper.calls))
	}
	if store.max != 3 {
		t.Fatalf("max attempts passed to the store = %d, want 3", store.max)
	}
	wantCutoff := time.Now().Add(-10 * time.Minute)
	if delta := store.cutoff.Sub(wantCutoff); delta > time.Minute || delta < -time.Minute {
		t.Fatalf("cutoff = %v, want about %v", store.cutoff, wantCutoff)
	}
}

func TestSweepRecordsBeforeCallingTheProvider(t *testing.T) {
	order := paidOrder()
	store := &fakeStore{orders: []model.UserOrder{order}}
	shipper := &fakeShipper{results: map[uuid.UUID]bool{}}

	// The fake shipper fails the test if the attempt was not recorded first.
	shipperCheck := &orderedShipper{store: store, orderID: order.ID}

	r := New(store, shipperCheck, 5, time.Minute)
	r.Sweep(context.Background())

	if len(shipper.calls) != 0 {
		t.Fatal("unexpected use of the plain fake shipper")
	}
	if !shipperCheck.calledAfterRecord {
		t.Fatal("provider was called before the attempt was recorded")
	}
}

type orderedShipper struct {
	store             *fakeStore
	orderID           uuid.UUID
	calledAfterRecord bool
}

func (s *orderedShipper) RetryShipmentFor(model.UserOrder) bool {
	s.calledAfterRecord = len(s.store.recorded) == 1 && s.store.recorded[0] == s.orderID
	return true
}

func TestSweepStopsOnStoreError(t *testing.T) {
	shipper := &fakeShipper{results: map[uuid.UUID]bool{}}
	store := &fakeStore{err: errors.New("db down")}

	r := New(store, shipper, 5, time.Minute)
	r.Sweep(context.Background())

	if len(shipper.calls) != 0 {
		t.Fatalf("shipper calls = %d, want 0", len(shipper.calls))
	}
}

func TestSweepStopsWhenContextIsCancelled(t *testing.T) {
	first, second := paidOrder(), paidOrder()
	store := &fakeStore{orders: []model.UserOrder{first, second}}
	shipper := &fakeShipper{results: map[uuid.UUID]bool{}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := New(store, shipper, 5, time.Minute)
	r.Sweep(ctx)

	if len(store.recorded) != 0 {
		t.Fatalf("recorded = %d attempts, want 0 after cancellation", len(store.recorded))
	}
}

func TestSweepRunsAfterTheInitialDelay(t *testing.T) {
	store := &fakeStore{orders: []model.UserOrder{paidOrder()}}
	shipper := &fakeShipper{results: map[uuid.UUID]bool{}}

	r := New(store, shipper, 5, time.Hour)
	r.initialDelay = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for store.attemptCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	if got := store.attemptCount(); got != 1 {
		t.Fatalf("recorded = %d attempts, want 1", got)
	}
}
