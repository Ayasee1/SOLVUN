// Package sweeper expires orders that were created but never paid, so the stock
// reserved at checkout is returned to the catalog.
package sweeper

import (
	"context"
	"log"
	"time"

	"golearn/model"
	"golearn/repository"
)

const (
	defaultTTL      = 20 * time.Minute
	defaultInterval = time.Minute

	// hardExpireAfter bounds how long an order may stay unverified when the
	// payment provider is unreachable. After this the order is expired anyway:
	// a settlement that arrives later is handled by the late-payment path, which
	// re-reserves the stock or flags the order for a refund.
	hardExpireAfter = 24 * time.Hour
)

// Reconciler asks the payment provider for the real state of a pending order.
// The handler implements it; keeping it an interface lets the sweeper be tested
// without any network.
type Reconciler interface {
	// ReconcilePending reports settled=true when the payment was applied (the
	// order must not be expired) and verified=false when the provider could not
	// be reached.
	ReconcilePending(order model.UserOrder) (settled bool, verified bool)
}

// Sweeper periodically expires stale pending orders.
type Sweeper struct {
	orders     repository.OrderRepository
	products   repository.ProductRepository
	reconciler Reconciler
	ttl        time.Duration
	interval   time.Duration
}

// New builds a sweeper. Non-positive values fall back to safe defaults. The
// reconciler may be nil, in which case stale orders are expired without asking
// the payment provider.
func New(orders repository.OrderRepository, products repository.ProductRepository, reconciler Reconciler, ttl, interval time.Duration) *Sweeper {
	if ttl <= 0 {
		ttl = defaultTTL
	}
	if interval <= 0 {
		interval = defaultInterval
	}

	return &Sweeper{
		orders:     orders,
		products:   products,
		reconciler: reconciler,
		ttl:        ttl,
		interval:   interval,
	}
}

// Run blocks until ctx is cancelled, sweeping once per interval.
func (s *Sweeper) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	log.Printf("order sweeper started (ttl=%s interval=%s)", s.ttl, s.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("order sweeper stopped")
			return
		case <-ticker.C:
			s.Sweep(ctx)
		}
	}
}

// Sweep expires every pending order older than the TTL and releases its stock.
//
// Before expiring, each candidate is checked with the payment provider: a
// webhook can be delayed or lost, and expiring a paid order would release stock
// the customer already bought. Orders that cannot be verified keep their stock
// and are retried on the next tick, up to hardExpireAfter.
//
// It is safe to run alongside payment webhooks: the pending -> expired
// transition is atomic, so an order that was just paid is skipped and stock is
// released at most once.
func (s *Sweeper) Sweep(ctx context.Context) {
	cutoff := time.Now().Add(-s.ttl)

	stale, err := s.orders.GetStalePending(cutoff)
	if err != nil {
		log.Printf("order sweeper: load stale orders: %v", err)
		return
	}

	for _, candidate := range stale {
		if ctx.Err() != nil {
			return
		}

		if s.reconciler != nil {
			settled, verified := s.reconciler.ReconcilePending(candidate)
			if settled {
				// The payment was applied while the webhook was missing.
				continue
			}
			if !verified {
				if age := time.Since(candidate.CreatedAt); age < hardExpireAfter {
					log.Printf("order sweeper: could not verify order %s, keeping its stock reserved", candidate.ID)
					continue
				}
				log.Printf("order sweeper: order %s unverified for %s, expiring it anyway", candidate.ID, hardExpireAfter)
			}
		}

		order, ok, err := s.orders.TransitionStatus(candidate.ID, model.OrderStatusPending, model.OrderStatusExpired)
		if err != nil {
			log.Printf("order sweeper: expire %s: %v", candidate.ID, err)
			continue
		}
		if !ok {
			// paid or cancelled between the query and the transition
			continue
		}

		requested := make(map[int]int, len(order.OrderItem))
		for _, item := range order.OrderItem {
			requested[item.ID] += item.Qty
		}

		if err := s.products.ReleaseStock(requested); err != nil {
			log.Printf("order sweeper: release stock for %s: %v", order.ID, err)
			continue
		}

		log.Printf("order sweeper: expired order %s and released stock", order.ID)
	}
}
