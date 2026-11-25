package alerts

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"golearn/model"
	"golearn/notification"
)

type fakeStore struct {
	refunds   []model.UserOrder
	unshipped []model.UserOrder
	err       error
	cutoff    time.Time
}

func (s *fakeStore) GetNeedsRefund() ([]model.UserOrder, error) {
	return s.refunds, s.err
}

func (s *fakeStore) GetPaidWithoutShipment(before time.Time) ([]model.UserOrder, error) {
	s.cutoff = before
	return s.unshipped, s.err
}

type fakeMailer struct {
	mu   sync.Mutex
	sent []notification.Message
	err  error
}

func (m *fakeMailer) Send(msg notification.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, msg)
	return nil
}

func (m *fakeMailer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

func order(status model.OrderStatus, total int) model.UserOrder {
	return model.UserOrder{
		ID:        uuid.New(),
		Status:    status,
		Total:     total,
		Courier:   "jne",
		CreatedAt: time.Now(),
		Payment:   &model.PaymentInfo{Type: "qris", TransactionID: "tx-" + string(status)},
	}
}

func fixedNow() time.Time { return time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC) }

func TestSendDigestListsFlaggedOrders(t *testing.T) {
	refund := order(model.OrderStatusNeedsRefund, 25000)
	unshipped := order(model.OrderStatusPaid, 30000)

	store := &fakeStore{refunds: []model.UserOrder{refund}, unshipped: []model.UserOrder{unshipped}}
	mailer := &fakeMailer{}
	n := New(store, mailer, "ops@solvun.test", time.Hour, 10*time.Minute)
	n.now = fixedNow

	if err := n.Send(); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("messages = %d, want 1", len(mailer.sent))
	}

	msg := mailer.sent[0]
	if msg.To != "ops@solvun.test" {
		t.Fatalf("to = %q", msg.To)
	}
	if !strings.Contains(msg.Subject, "2 order(s)") {
		t.Fatalf("subject = %q", msg.Subject)
	}
	for _, want := range []string{
		refund.ID.String()[:8],
		unshipped.ID.String()[:8],
		"Needs refund",
		"Paid, no shipment",
		"jne",
	} {
		if !strings.Contains(msg.HTML, want) {
			t.Errorf("digest does not contain %q", want)
		}
	}

	// The cutoff respects the grace period.
	if want := fixedNow().Add(-10 * time.Minute); !store.cutoff.Equal(want) {
		t.Errorf("cutoff = %v, want %v", store.cutoff, want)
	}
}

func TestSendIsQuietWhenNothingNeedsAttention(t *testing.T) {
	mailer := &fakeMailer{}
	n := New(&fakeStore{}, mailer, "ops@solvun.test", time.Hour, 0)

	if err := n.Send(); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(mailer.sent) != 0 {
		t.Fatalf("messages = %d, want 0", len(mailer.sent))
	}
}

func TestSendWithoutRecipientDoesNothing(t *testing.T) {
	mailer := &fakeMailer{}
	n := New(&fakeStore{refunds: []model.UserOrder{order(model.OrderStatusNeedsRefund, 1000)}}, mailer, "", time.Hour, 0)

	if err := n.Send(); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(mailer.sent) != 0 {
		t.Fatalf("messages = %d, want 0", len(mailer.sent))
	}
}

func TestSendSurfacesStoreErrors(t *testing.T) {
	n := New(&fakeStore{err: errors.New("db down")}, &fakeMailer{}, "ops@solvun.test", time.Hour, 0)

	if err := n.Send(); err == nil {
		t.Fatal("want an error")
	}
}

func TestDigestEscapesCourierHTML(t *testing.T) {
	unshipped := order(model.OrderStatusPaid, 1000)
	unshipped.Courier = `<script>alert(1)</script>`

	mailer := &fakeMailer{}
	n := New(&fakeStore{unshipped: []model.UserOrder{unshipped}}, mailer, "ops@solvun.test", time.Hour, 0)

	if err := n.Send(); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if strings.Contains(mailer.sent[0].HTML, "<script>") {
		t.Fatal("courier value was not escaped")
	}
}

func TestRunSendsAfterTheInitialDelayAndStops(t *testing.T) {
	mailer := &fakeMailer{}
	n := New(&fakeStore{refunds: []model.UserOrder{order(model.OrderStatusNeedsRefund, 1000)}},
		mailer, "ops@solvun.test", time.Hour, 0)
	n.initialDelay = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		n.Run(ctx)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for mailer.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	if got := mailer.count(); got != 1 {
		t.Fatalf("messages = %d, want 1", got)
	}
}
