// Package retrier retries shipment creation for paid orders whose provider call
// failed, a bounded number of times so a broken order cannot loop forever.
package retrier

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"

	"golearn/model"
)

const (
	defaultMaxAttempts  = 5
	defaultInterval     = 10 * time.Minute
	defaultInitialDelay = time.Minute
)

// Store is the part of the order repository the retrier needs.
type Store interface {
	GetShipmentRetries(attemptedBefore time.Time, maxAttempts int) ([]model.UserOrder, error)
	RecordShipmentAttempt(id uuid.UUID) error
}

// Shipper creates the missing shipment. It returns true when a shipment exists
// afterwards.
type Shipper interface {
	RetryShipmentFor(order model.UserOrder) bool
}

// Retrier periodically retries orders that are paid but have no shipment.
type Retrier struct {
	store        Store
	shipper      Shipper
	maxAttempts  int
	backoff      time.Duration
	interval     time.Duration
	initialDelay time.Duration
	now          func() time.Time
}

// New builds a retrier. Non-positive values fall back to safe defaults.
func New(store Store, shipper Shipper, maxAttempts int, interval time.Duration) *Retrier {
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}
	if interval <= 0 {
		interval = defaultInterval
	}

	return &Retrier{
		store:        store,
		shipper:      shipper,
		maxAttempts:  maxAttempts,
		backoff:      interval,
		interval:     interval,
		initialDelay: defaultInitialDelay,
		now:          time.Now,
	}
}

// Run blocks until ctx is cancelled.
func (r *Retrier) Run(ctx context.Context) {
	log.Printf("shipment retrier started (interval=%s max_attempts=%d)", r.interval, r.maxAttempts)

	select {
	case <-ctx.Done():
		return
	case <-time.After(r.initialDelay):
	}

	r.Sweep(ctx)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("shipment retrier stopped")
			return
		case <-ticker.C:
			r.Sweep(ctx)
		}
	}
}

// Sweep attempts every order that is due. An attempt is recorded before the
// provider call, so a crash in the middle still consumes it; a persistent
// failure stops after maxAttempts and is left to the ops digest.
func (r *Retrier) Sweep(ctx context.Context) {
	orders, err := r.store.GetShipmentRetries(r.now().Add(-r.backoff), r.maxAttempts)
	if err != nil {
		log.Printf("shipment retrier: load orders: %v", err)
		return
	}

	for _, order := range orders {
		if ctx.Err() != nil {
			return
		}

		if err := r.store.RecordShipmentAttempt(order.ID); err != nil {
			log.Printf("shipment retrier: record attempt for %s: %v", order.ID, err)
			continue
		}

		attempt := order.ShipmentAttempts + 1
		if r.shipper.RetryShipmentFor(order) {
			log.Printf("shipment retrier: created shipment for order %s (attempt %d)", order.ID, attempt)
			continue
		}
		log.Printf("shipment retrier: attempt %d/%d failed for order %s", attempt, r.maxAttempts, order.ID)
	}
}
