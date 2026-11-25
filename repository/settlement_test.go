package repository

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"golearn/db"
	"golearn/model"
)

func newSettleTestRepos(t *testing.T) (*MemoryProductRepository, *MemoryOrderRepository, model.UserOrder) {
	t.Helper()

	products := NewMemoryProductRepository()
	orders := NewMemoryOrderRepository()
	orders.SetProducts(products)

	// product 1 starts with 10 units; reserve 2 for a pending order
	if err := products.DecreaseStock(map[int]int{1: 2}); err != nil {
		t.Fatalf("reserve: %v", err)
	}

	order := model.UserOrder{
		ID:        uuid.New(),
		Status:    model.OrderStatusPending,
		Total:     2000,
		CreatedAt: time.Now(),
		OrderItem: []model.Item{{ID: 1, Qty: 2}},
	}
	if err := orders.Save(order); err != nil {
		t.Fatalf("save: %v", err)
	}

	return products, orders, order
}

func paymentInfo() model.PaymentInfo {
	return model.PaymentInfo{Type: "qris", TransactionID: "tx-1"}
}

func TestSettlePaidPendingOrder(t *testing.T) {
	products, orders, order := newSettleTestRepos(t)

	settled, outcome, err := orders.SettlePaid(order.ID, model.OrderStatusPending, paymentInfo(), map[int]int{1: 2}, false)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if outcome != SettleClaimed {
		t.Fatalf("outcome = %q, want claimed", outcome)
	}
	if settled.Status != model.OrderStatusPaid {
		t.Fatalf("status = %q, want paid", settled.Status)
	}
	if settled.Payment == nil {
		t.Fatal("payment info was not recorded")
	}

	// A pending order already reserved its stock at checkout.
	if stock := mustFind(t, products, 1).Stock; stock != 8 {
		t.Fatalf("stock = %d, want 8 (unchanged)", stock)
	}
}

func TestSettlePaidLatePaymentReservesStock(t *testing.T) {
	products, orders, order := newSettleTestRepos(t)

	// The sweeper released the stock when the order expired.
	if _, _, err := orders.TransitionStatus(order.ID, model.OrderStatusPending, model.OrderStatusExpired); err != nil {
		t.Fatalf("expire: %v", err)
	}
	if err := products.ReleaseStock(map[int]int{1: 2}); err != nil {
		t.Fatalf("release: %v", err)
	}

	_, outcome, err := orders.SettlePaid(order.ID, model.OrderStatusExpired, paymentInfo(), map[int]int{1: 2}, true)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if outcome != SettleClaimed {
		t.Fatalf("outcome = %q, want claimed", outcome)
	}
	if stock := mustFind(t, products, 1).Stock; stock != 8 {
		t.Fatalf("stock = %d, want 8 (re-reserved)", stock)
	}
}

func TestSettlePaidWithoutStockNeedsRefund(t *testing.T) {
	products, orders, order := newSettleTestRepos(t)

	if _, _, err := orders.TransitionStatus(order.ID, model.OrderStatusPending, model.OrderStatusExpired); err != nil {
		t.Fatalf("expire: %v", err)
	}
	// Someone else takes all the stock.
	if err := products.DecreaseStock(map[int]int{1: 8}); err != nil {
		t.Fatalf("consume stock: %v", err)
	}

	settled, outcome, err := orders.SettlePaid(order.ID, model.OrderStatusExpired, paymentInfo(), map[int]int{1: 2}, true)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if outcome != SettleNeedsRefund {
		t.Fatalf("outcome = %q, want needs_refund", outcome)
	}
	if settled.Status != model.OrderStatusNeedsRefund {
		t.Fatalf("status = %q, want needs_refund", settled.Status)
	}
	if settled.Payment == nil {
		t.Fatal("payment info must be recorded even when refunding")
	}
	if stock := mustFind(t, products, 1).Stock; stock != 0 {
		t.Fatalf("stock = %d, want 0 (untouched by the failed settlement)", stock)
	}
}

func TestSettlePaidNotClaimed(t *testing.T) {
	products, orders, order := newSettleTestRepos(t)

	// The order is pending, but the caller thinks it is expired.
	settled, outcome, err := orders.SettlePaid(order.ID, model.OrderStatusExpired, paymentInfo(), map[int]int{1: 2}, true)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if outcome != SettleNotClaimed {
		t.Fatalf("outcome = %q, want not_claimed", outcome)
	}
	if settled.Status != model.OrderStatusPending {
		t.Fatalf("status = %q, want pending", settled.Status)
	}
	if stock := mustFind(t, products, 1).Stock; stock != 8 {
		t.Fatalf("stock = %d, want 8 (unchanged)", stock)
	}
}

// ---------------------------------------------------------------------------
// Postgres: the same contract, but atomic. Skipped unless TEST_DATABASE_URL is
// set (CI provides a service container).

func postgresSettleRepo(t *testing.T) (*pgxpool.Pool, *PostgresOrderRepository) {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	pool, err := db.NewPool(context.Background(), dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := db.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `TRUNCATE orders, products`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO products (id, name, price, stock, weight_gram)
		VALUES (1, 'A', 1000, 10, 100), (2, 'B', 500, 0, 100)`); err != nil {
		t.Fatalf("seed products: %v", err)
	}

	return pool, NewPostgresOrderRepository(pool)
}

func seedPostgresOrder(t *testing.T, repo *PostgresOrderRepository, status model.OrderStatus, items []model.Item, age time.Duration) model.UserOrder {
	t.Helper()

	order := model.UserOrder{
		ID:        uuid.New(),
		Username:  "budi",
		Email:     "b@mail.com",
		Phone:     "0812",
		Address:   model.Address{Street: "Jl Mawar", City: "Bandung", State: "Jabar", ZipCode: "40123"},
		OrderItem: items,
		Total:     2000,
		Status:    status,
		CreatedAt: time.Now().Add(-age),
	}
	if err := repo.Save(order); err != nil {
		t.Fatalf("save order: %v", err)
	}
	return order
}

func postgresStock(t *testing.T, pool *pgxpool.Pool, id int) int {
	t.Helper()

	var stock int
	if err := pool.QueryRow(context.Background(), `SELECT stock FROM products WHERE id = $1`, id).Scan(&stock); err != nil {
		t.Fatalf("read stock: %v", err)
	}
	return stock
}

func TestPostgresSettlePaidClaimsOnce(t *testing.T) {
	pool, repo := postgresSettleRepo(t)
	order := seedPostgresOrder(t, repo, model.OrderStatusExpired, []model.Item{{ID: 1, Qty: 2}}, time.Minute)

	const workers = 8
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		claimed int
		refunds int
	)

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, outcome, err := repo.SettlePaid(order.ID, model.OrderStatusExpired, paymentInfo(), map[int]int{1: 2}, true)
			if err != nil {
				t.Errorf("settle: %v", err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			switch outcome {
			case SettleClaimed:
				claimed++
			case SettleNeedsRefund:
				refunds++
			}
		}()
	}
	wg.Wait()

	if claimed != 1 {
		t.Fatalf("claimed = %d, want exactly 1 of %d concurrent settlements", claimed, workers)
	}
	if refunds != 0 {
		t.Fatalf("refunds = %d, want 0", refunds)
	}
	if stock := postgresStock(t, pool, 1); stock != 8 {
		t.Fatalf("stock = %d, want 8 (reserved exactly once)", stock)
	}
}

func TestPostgresSettlePaidNeedsRefundLeavesStockUntouched(t *testing.T) {
	pool, repo := postgresSettleRepo(t)
	// Product 1 has enough, product 2 has none.
	order := seedPostgresOrder(t, repo, model.OrderStatusExpired,
		[]model.Item{{ID: 1, Qty: 2}, {ID: 2, Qty: 1}}, time.Minute)

	settled, outcome, err := repo.SettlePaid(order.ID, model.OrderStatusExpired, paymentInfo(), map[int]int{1: 2, 2: 1}, true)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if outcome != SettleNeedsRefund {
		t.Fatalf("outcome = %q, want needs_refund", outcome)
	}
	if settled.Status != model.OrderStatusNeedsRefund {
		t.Fatalf("status = %q, want needs_refund", settled.Status)
	}
	if settled.Payment == nil {
		t.Fatal("payment info must survive a refund flag")
	}
	// The whole reservation is rolled back, including the item that had stock.
	if stock := postgresStock(t, pool, 1); stock != 10 {
		t.Fatalf("product 1 stock = %d, want 10 (no partial reservation)", stock)
	}
	if stock := postgresStock(t, pool, 2); stock != 0 {
		t.Fatalf("product 2 stock = %d, want 0", stock)
	}
}

func TestPostgresSettlePaidRejectsWrongStatus(t *testing.T) {
	pool, repo := postgresSettleRepo(t)
	order := seedPostgresOrder(t, repo, model.OrderStatusPending, []model.Item{{ID: 1, Qty: 2}}, time.Minute)

	_, outcome, err := repo.SettlePaid(order.ID, model.OrderStatusExpired, paymentInfo(), map[int]int{1: 2}, true)
	if err != nil {
		t.Fatalf("settle: %v", err)
	}
	if outcome != SettleNotClaimed {
		t.Fatalf("outcome = %q, want not_claimed", outcome)
	}
	if stock := postgresStock(t, pool, 1); stock != 10 {
		t.Fatalf("stock = %d, want 10", stock)
	}
}
