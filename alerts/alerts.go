// Package alerts sends the operator a periodic digest of orders that need a
// human: refunds to issue and paid orders whose shipment was never created.
package alerts

import (
	"context"
	"log"
	"time"

	"golearn/model"
	"golearn/notification"
)

const (
	defaultInterval     = 24 * time.Hour
	defaultGrace        = 10 * time.Minute
	defaultInitialDelay = time.Minute
)

// Store is the part of the order repository the digest needs.
type Store interface {
	GetNeedsRefund() ([]model.UserOrder, error)
	GetPaidWithoutShipment(before time.Time) ([]model.UserOrder, error)
}

// Notifier emails a digest on a fixed interval. It stays quiet when nothing
// needs attention, so an empty inbox means everything is fine.
type Notifier struct {
	store        Store
	mailer       notification.Sender
	to           string
	interval     time.Duration
	grace        time.Duration
	initialDelay time.Duration
	now          func() time.Time
}

// New builds a notifier. Non-positive durations fall back to safe defaults.
func New(store Store, mailer notification.Sender, to string, interval, grace time.Duration) *Notifier {
	if interval <= 0 {
		interval = defaultInterval
	}
	if grace <= 0 {
		grace = defaultGrace
	}

	return &Notifier{
		store:        store,
		mailer:       mailer,
		to:           to,
		interval:     interval,
		grace:        grace,
		initialDelay: defaultInitialDelay,
		now:          time.Now,
	}
}

// Run blocks until ctx is cancelled. The first digest goes out shortly after
// startup so a restart with unresolved orders still reminds the operator.
func (n *Notifier) Run(ctx context.Context) {
	log.Printf("ops alerts started (interval=%s)", n.interval)

	select {
	case <-ctx.Done():
		return
	case <-time.After(n.initialDelay):
	}

	n.send()

	ticker := time.NewTicker(n.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("ops alerts stopped")
			return
		case <-ticker.C:
			n.send()
		}
	}
}

func (n *Notifier) send() {
	if err := n.Send(); err != nil {
		log.Printf("ops alerts: %v", err)
	}
}

// Send builds and delivers one digest. Nothing is sent when both lists are empty
// or when the notifier has no recipient.
func (n *Notifier) Send() error {
	if n.to == "" {
		return nil
	}

	refunds, err := n.store.GetNeedsRefund()
	if err != nil {
		return err
	}

	unshipped, err := n.store.GetPaidWithoutShipment(n.now().Add(-n.grace))
	if err != nil {
		return err
	}

	if len(refunds) == 0 && len(unshipped) == 0 {
		return nil
	}

	return n.mailer.Send(notification.OpsDigestEmail(n.to, refunds, unshipped, n.now()))
}
