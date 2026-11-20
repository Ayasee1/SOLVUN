package sweeper

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"golearn/model"
	"golearn/repository"
)

func TestSweepExpiresStaleOrderAndReleasesStock(t *testing.T) {
	products := repository.NewMemoryProductRepository()
	orders := repository.NewMemoryOrderRepository()

	const productID = 1
	before, err := products.FindByID(productID)
	if err != nil {
		t.Fatalf("find product: %v", err)
	}

	if err := products.DecreaseStock(map[int]int{productID: 2}); err != nil {
		t.Fatalf("decrease stock: %v", err)
	}

	order := model.UserOrder{
		ID:        uuid.New(),
		Status:    model.OrderStatusPending,
		CreatedAt: time.Now().Add(-time.Hour),
		OrderItem: []model.Item{{ID: productID, Qty: 2}},
	}
	if err := orders.Save(order); err != nil {
		t.Fatalf("save order: %v", err)
	}

	s := New(orders, products, 30*time.Minute, time.Minute)
	s.Sweep(context.Background())

	stored, err := orders.FindByID(order.ID)
	if err != nil {
		t.Fatalf("find order: %v", err)
	}
	if stored.Status != model.OrderStatusExpired {
		t.Fatalf("status = %q, want %q", stored.Status, model.OrderStatusExpired)
	}

	after, err := products.FindByID(productID)
	if err != nil {
		t.Fatalf("find product after sweep: %v", err)
	}
	if after.Stock != before.Stock {
		t.Fatalf("stock = %d, want %d", after.Stock, before.Stock)
	}

	// a second sweep must not release the stock again
	s.Sweep(context.Background())
	again, err := products.FindByID(productID)
	if err != nil {
		t.Fatalf("find product after second sweep: %v", err)
	}
	if again.Stock != before.Stock {
		t.Fatalf("stock after second sweep = %d, want %d", again.Stock, before.Stock)
	}
}

func TestSweepKeepsFreshPendingOrder(t *testing.T) {
	products := repository.NewMemoryProductRepository()
	orders := repository.NewMemoryOrderRepository()

	const productID = 1
	if err := products.DecreaseStock(map[int]int{productID: 1}); err != nil {
		t.Fatalf("decrease stock: %v", err)
	}
	reserved, err := products.FindByID(productID)
	if err != nil {
		t.Fatalf("find product: %v", err)
	}

	order := model.UserOrder{
		ID:        uuid.New(),
		Status:    model.OrderStatusPending,
		CreatedAt: time.Now(),
		OrderItem: []model.Item{{ID: productID, Qty: 1}},
	}
	if err := orders.Save(order); err != nil {
		t.Fatalf("save order: %v", err)
	}

	s := New(orders, products, 30*time.Minute, time.Minute)
	s.Sweep(context.Background())

	stored, err := orders.FindByID(order.ID)
	if err != nil {
		t.Fatalf("find order: %v", err)
	}
	if stored.Status != model.OrderStatusPending {
		t.Fatalf("status = %q, want %q", stored.Status, model.OrderStatusPending)
	}

	after, err := products.FindByID(productID)
	if err != nil {
		t.Fatalf("find product: %v", err)
	}
	if after.Stock != reserved.Stock {
		t.Fatalf("stock = %d, want %d", after.Stock, reserved.Stock)
	}
}
