package sweeper

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"golearn/model"
	"golearn/repository"
)

const (
	productID = 1
	orderQty  = 2
)

// seedPendingOrder reserves orderQty units for a pending order created at the
// given age, and returns the stock level from before the reservation.
func seedPendingOrder(t *testing.T, age time.Duration) (*repository.MemoryProductRepository, *repository.MemoryOrderRepository, model.UserOrder, int) {
	t.Helper()

	products := repository.NewMemoryProductRepository()
	orders := repository.NewMemoryOrderRepository()

	before, err := products.FindByID(productID)
	if err != nil {
		t.Fatalf("find product: %v", err)
	}
	if err := products.DecreaseStock(map[int]int{productID: orderQty}); err != nil {
		t.Fatalf("decrease stock: %v", err)
	}

	order := model.UserOrder{
		ID:        uuid.New(),
		Status:    model.OrderStatusPending,
		CreatedAt: time.Now().Add(-age),
		OrderItem: []model.Item{{ID: productID, Qty: orderQty}},
	}
	if err := orders.Save(order); err != nil {
		t.Fatalf("save order: %v", err)
	}

	return products, orders, order, before.Stock
}

func stockOf(t *testing.T, products *repository.MemoryProductRepository) int {
	t.Helper()

	product, err := products.FindByID(productID)
	if err != nil {
		t.Fatalf("find product: %v", err)
	}
	return product.Stock
}

func statusOf(t *testing.T, orders *repository.MemoryOrderRepository, id uuid.UUID) model.OrderStatus {
	t.Helper()

	order, err := orders.FindByID(id)
	if err != nil {
		t.Fatalf("find order: %v", err)
	}
	return order.Status
}

type fakeReconciler struct {
	settled  bool
	verified bool
	calls    int
}

func (f *fakeReconciler) ReconcilePending(model.UserOrder) (bool, bool) {
	f.calls++
	return f.settled, f.verified
}

func TestSweepExpiresStaleOrderAndReleasesStock(t *testing.T) {
	products, orders, order, initialStock := seedPendingOrder(t, time.Hour)

	s := New(orders, products, nil, 30*time.Minute, time.Minute)
	s.Sweep(context.Background())

	if got := statusOf(t, orders, order.ID); got != model.OrderStatusExpired {
		t.Fatalf("status = %q, want %q", got, model.OrderStatusExpired)
	}
	if got := stockOf(t, products); got != initialStock {
		t.Fatalf("stock = %d, want %d", got, initialStock)
	}

	// a second sweep must not release the stock again
	s.Sweep(context.Background())
	if got := stockOf(t, products); got != initialStock {
		t.Fatalf("stock after second sweep = %d, want %d", got, initialStock)
	}
}

func TestSweepKeepsFreshPendingOrder(t *testing.T) {
	products, orders, order, initialStock := seedPendingOrder(t, 0)

	s := New(orders, products, nil, 30*time.Minute, time.Minute)
	s.Sweep(context.Background())

	if got := statusOf(t, orders, order.ID); got != model.OrderStatusPending {
		t.Fatalf("status = %q, want %q", got, model.OrderStatusPending)
	}
	if got := stockOf(t, products); got != initialStock-orderQty {
		t.Fatalf("stock = %d, want %d (still reserved)", got, initialStock-orderQty)
	}
}

// A delayed webhook: the provider says the payment settled, so the order must be
// left alone for the settlement path instead of being expired.
func TestSweepSkipsOrderSettledByReconciler(t *testing.T) {
	products, orders, order, initialStock := seedPendingOrder(t, time.Hour)

	reconciler := &fakeReconciler{settled: true, verified: true}
	s := New(orders, products, reconciler, 30*time.Minute, time.Minute)
	s.Sweep(context.Background())

	if reconciler.calls != 1 {
		t.Fatalf("reconciler calls = %d, want 1", reconciler.calls)
	}
	if got := statusOf(t, orders, order.ID); got != model.OrderStatusPending {
		t.Fatalf("status = %q, want the order left for the settlement path", got)
	}
	if got := stockOf(t, products); got != initialStock-orderQty {
		t.Fatalf("stock = %d, want %d (still reserved)", got, initialStock-orderQty)
	}
}

func TestSweepKeepsOrderWhenProviderIsUnreachable(t *testing.T) {
	products, orders, order, initialStock := seedPendingOrder(t, time.Hour)

	reconciler := &fakeReconciler{settled: false, verified: false}
	s := New(orders, products, reconciler, 30*time.Minute, time.Minute)
	s.Sweep(context.Background())

	if got := statusOf(t, orders, order.ID); got != model.OrderStatusPending {
		t.Fatalf("status = %q, want pending while unverified", got)
	}
	if got := stockOf(t, products); got != initialStock-orderQty {
		t.Fatalf("stock = %d, want %d (kept)", got, initialStock-orderQty)
	}

	// Once the provider answers again, the normal expiry runs.
	reconciler.verified = true
	s.Sweep(context.Background())

	if got := statusOf(t, orders, order.ID); got != model.OrderStatusExpired {
		t.Fatalf("status after verification = %q, want expired", got)
	}
	if got := stockOf(t, products); got != initialStock {
		t.Fatalf("stock after release = %d, want %d", got, initialStock)
	}
}

// The hard limit keeps a provider outage from pinning stock forever.
func TestSweepExpiresUnverifiedOrderAfterHardLimit(t *testing.T) {
	products, orders, order, initialStock := seedPendingOrder(t, hardExpireAfter+time.Hour)

	reconciler := &fakeReconciler{settled: false, verified: false}
	s := New(orders, products, reconciler, 30*time.Minute, time.Minute)
	s.Sweep(context.Background())

	if got := statusOf(t, orders, order.ID); got != model.OrderStatusExpired {
		t.Fatalf("status = %q, want expired after the hard limit", got)
	}
	if got := stockOf(t, products); got != initialStock {
		t.Fatalf("stock = %d, want %d (released)", got, initialStock)
	}
}
