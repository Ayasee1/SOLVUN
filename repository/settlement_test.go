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

func TestMemoryOrderRepositoryOpsQueries(t *testing.T) {
	orders := NewMemoryOrderRepository()
	now := time.Now()

	newOrder := func(status model.OrderStatus, courier string, age time.Duration) model.UserOrder {
		o := model.UserOrder{
			ID:        uuid.New(),
			Status:    status,
			Courier:   courier,
			Total:     1000,
			CreatedAt: now.Add(-age),
			OrderItem: []model.Item{{ID: 1, Qty: 1}},
		}
		if err := orders.Save(o); err != nil {
			t.Fatalf("save: %v", err)
		}
		return o
	}

	refund := newOrder(model.OrderStatusNeedsRefund, "", time.Hour)
	unshippedOld := newOrder(model.OrderStatusPaid, "jne", time.Hour)
	newOrder(model.OrderStatusPaid, "jne", time.Second) // inside the grace period
	shipped := newOrder(model.OrderStatusPaid, "jne", time.Hour)
	newOrder(model.OrderStatusPending, "", time.Hour)

	if err := orders.SetShipment(shipped.ID, model.ShipmentInfo{ProviderID: "bite-1"}); err != nil {
		t.Fatalf("set shipment: %v", err)
	}

	refunds, err := orders.GetNeedsRefund()
	if err != nil {
		t.Fatalf("GetNeedsRefund: %v", err)
	}
	if len(refunds) != 1 || refunds[0].ID != refund.ID {
		t.Fatalf("refunds = %+v, want only %s", refunds, refund.ID)
	}

	unshipped, err := orders.GetPaidWithoutShipment(now.Add(-10 * time.Minute))
	if err != nil {
		t.Fatalf("GetPaidWithoutShipment: %v", err)
	}
	if len(unshipped) != 1 || unshipped[0].ID != unshippedOld.ID {
		t.Fatalf("unshipped = %+v, want only %s", unshipped, unshippedOld.ID)
	}
}

func TestPostgresOrderRepositoryOpsQueries(t *testing.T) {
	_, repo := postgresSettleRepo(t)
	now := time.Now()

	seed := func(status model.OrderStatus, courier string, age time.Duration, shipment bool) model.UserOrder {
		order := seedPostgresOrder(t, repo, status, []model.Item{{ID: 1, Qty: 1}}, age)
		if courier != "" {
			if _, err := repo.pool.Exec(context.Background(),
				`UPDATE orders SET courier = $2 WHERE id = $1`, order.ID, courier); err != nil {
				t.Fatalf("set courier: %v", err)
			}
		}
		if shipment {
			if err := repo.SetShipment(order.ID, model.ShipmentInfo{ProviderID: "bite-1"}); err != nil {
				t.Fatalf("set shipment: %v", err)
			}
		}
		return order
	}

	refund := seed(model.OrderStatusNeedsRefund, "", time.Hour, false)
	unshippedOld := seed(model.OrderStatusPaid, "jne", time.Hour, false)
	seed(model.OrderStatusPaid, "jne", time.Second, false) // inside the grace period
	seed(model.OrderStatusPaid, "jne", time.Hour, true)    // already shipped
	seed(model.OrderStatusPending, "", time.Hour, false)

	refunds, err := repo.GetNeedsRefund()
	if err != nil {
		t.Fatalf("GetNeedsRefund: %v", err)
	}
	if len(refunds) != 1 || refunds[0].ID != refund.ID {
		t.Fatalf("refunds = %+v, want only %s", refunds, refund.ID)
	}

	unshipped, err := repo.GetPaidWithoutShipment(now.Add(-10 * time.Minute))
	if err != nil {
		t.Fatalf("GetPaidWithoutShipment: %v", err)
	}
	if len(unshipped) != 1 || unshipped[0].ID != unshippedOld.ID {
		t.Fatalf("unshipped = %+v, want only %s", unshipped, unshippedOld.ID)
	}
}

func TestMemoryOrderRepositoryShipmentRetries(t *testing.T) {
	orders := NewMemoryOrderRepository()
	now := time.Now()

	due := model.UserOrder{ID: uuid.New(), Status: model.OrderStatusPaid, Courier: "jne", CreatedAt: now.Add(-time.Hour)}
	tooMany := model.UserOrder{ID: uuid.New(), Status: model.OrderStatusPaid, Courier: "jne", CreatedAt: now.Add(-time.Hour), ShipmentAttempts: 5}
	recent := model.UserOrder{ID: uuid.New(), Status: model.OrderStatusPaid, Courier: "jne", CreatedAt: now.Add(-time.Hour), ShipmentLastAttemptAt: &now}
	shipped := model.UserOrder{ID: uuid.New(), Status: model.OrderStatusPaid, Courier: "jne", CreatedAt: now.Add(-time.Hour)}
	pending := model.UserOrder{ID: uuid.New(), Status: model.OrderStatusPending, Courier: "jne", CreatedAt: now.Add(-time.Hour)}

	for _, order := range []model.UserOrder{due, tooMany, recent, shipped, pending} {
		if err := orders.Save(order); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	if err := orders.SetShipment(shipped.ID, model.ShipmentInfo{ProviderID: "bite-1"}); err != nil {
		t.Fatalf("set shipment: %v", err)
	}

	retries, err := orders.GetShipmentRetries(now.Add(-10*time.Minute), 5)
	if err != nil {
		t.Fatalf("GetShipmentRetries: %v", err)
	}
	if len(retries) != 1 || retries[0].ID != due.ID {
		t.Fatalf("retries = %+v, want only %s", retries, due.ID)
	}

	if err := orders.RecordShipmentAttempt(due.ID); err != nil {
		t.Fatalf("record attempt: %v", err)
	}
	stored, err := orders.FindByID(due.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if stored.ShipmentAttempts != 1 || stored.ShipmentLastAttemptAt == nil {
		t.Fatalf("attempts = %d last = %v, want 1 and a timestamp", stored.ShipmentAttempts, stored.ShipmentLastAttemptAt)
	}

	// The attempt just recorded puts the order into backoff.
	retries, err = orders.GetShipmentRetries(now.Add(-10*time.Minute), 5)
	if err != nil {
		t.Fatalf("GetShipmentRetries: %v", err)
	}
	if len(retries) != 0 {
		t.Fatalf("retries = %d, want 0 while in backoff", len(retries))
	}
}

func TestPostgresOrderRepositoryShipmentRetries(t *testing.T) {
	pool, repo := postgresSettleRepo(t)
	now := time.Now()

	seed := func(status model.OrderStatus, courier string, attempts int, lastAttempt *time.Time) model.UserOrder {
		order := seedPostgresOrder(t, repo, status, []model.Item{{ID: 1, Qty: 1}}, time.Hour)
		if _, err := pool.Exec(context.Background(),
			`UPDATE orders SET courier = $2, shipment_attempts = $3, shipment_last_attempt_at = $4 WHERE id = $1`,
			order.ID, courier, attempts, lastAttempt); err != nil {
			t.Fatalf("seed order: %v", err)
		}
		return order
	}

	due := seed(model.OrderStatusPaid, "jne", 0, nil)
	seed(model.OrderStatusPaid, "jne", 5, nil)  // attempts exhausted
	seed(model.OrderStatusPaid, "jne", 0, &now) // in backoff
	shipped := seed(model.OrderStatusPaid, "jne", 0, nil)
	seed(model.OrderStatusPending, "jne", 0, nil) // not paid
	if err := repo.SetShipment(shipped.ID, model.ShipmentInfo{ProviderID: "bite-1"}); err != nil {
		t.Fatalf("set shipment: %v", err)
	}

	retries, err := repo.GetShipmentRetries(now.Add(-10*time.Minute), 5)
	if err != nil {
		t.Fatalf("GetShipmentRetries: %v", err)
	}
	if len(retries) != 1 || retries[0].ID != due.ID {
		t.Fatalf("retries = %+v, want only %s", retries, due.ID)
	}

	if err := repo.RecordShipmentAttempt(due.ID); err != nil {
		t.Fatalf("record attempt: %v", err)
	}
	stored, err := repo.FindByID(due.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if stored.ShipmentAttempts != 1 || stored.ShipmentLastAttemptAt == nil {
		t.Fatalf("attempts = %d last = %v, want 1 and a timestamp", stored.ShipmentAttempts, stored.ShipmentLastAttemptAt)
	}
}

func TestMemoryTransitionStatusWithRefund(t *testing.T) {
	orders := NewMemoryOrderRepository()
	order := model.UserOrder{ID: uuid.New(), Status: model.OrderStatusNeedsRefund, Total: 1000}
	if err := orders.Save(order); err != nil {
		t.Fatalf("save: %v", err)
	}

	info := model.RefundInfo{Key: order.ID.String(), Amount: 1000, RefundedAt: time.Now()}
	updated, ok, err := orders.TransitionStatusWithRefund(order.ID, model.OrderStatusNeedsRefund, model.OrderStatusRefunded, info)
	if err != nil {
		t.Fatalf("transition: %v", err)
	}
	if !ok || updated.Status != model.OrderStatusRefunded || updated.Refund == nil {
		t.Fatalf("updated = %+v, ok = %v", updated, ok)
	}

	// A second transition from needs_refund must not apply.
	if _, ok, err := orders.TransitionStatusWithRefund(order.ID, model.OrderStatusNeedsRefund, model.OrderStatusRefunded, info); err != nil || ok {
		t.Fatalf("second transition ok = %v, err = %v, want false/nil", ok, err)
	}
}

func TestPostgresTransitionStatusWithRefund(t *testing.T) {
	_, repo := postgresSettleRepo(t)
	order := seedPostgresOrder(t, repo, model.OrderStatusNeedsRefund, []model.Item{{ID: 1, Qty: 1}}, time.Minute)

	info := model.RefundInfo{Key: order.ID.String(), Amount: 2000, RefundedAt: time.Now()}
	updated, ok, err := repo.TransitionStatusWithRefund(order.ID, model.OrderStatusNeedsRefund, model.OrderStatusRefunded, info)
	if err != nil {
		t.Fatalf("transition: %v", err)
	}
	if !ok || updated.Status != model.OrderStatusRefunded || updated.Refund == nil || updated.Refund.Amount != 2000 {
		t.Fatalf("updated = %+v, ok = %v", updated, ok)
	}

	stored, err := repo.FindByID(order.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if stored.Status != model.OrderStatusRefunded || stored.Refund == nil || stored.Refund.Amount != 2000 {
		t.Fatalf("stored = %+v", stored)
	}
}

func TestPostgresOrderPaginationAndStats(t *testing.T) {
	_, repo := postgresSettleRepo(t)

	oldest := seedPostgresOrder(t, repo, model.OrderStatusPending, []model.Item{{ID: 1, Qty: 1}}, 3*time.Minute)
	middle := seedPostgresOrder(t, repo, model.OrderStatusPaid, []model.Item{{ID: 1, Qty: 1}}, 2*time.Minute)
	newest := seedPostgresOrder(t, repo, model.OrderStatusCancelled, []model.Item{{ID: 1, Qty: 1}}, time.Minute)

	first, total, err := repo.GetAll("", 2, 0)
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if total != 3 || len(first) != 2 {
		t.Fatalf("page 1 = %d items, total %d, want 2/3", len(first), total)
	}
	if first[0].ID != newest.ID || first[1].ID != middle.ID {
		t.Fatalf("page 1 order = %s,%s, want newest then middle", first[0].ID, first[1].ID)
	}

	second, total, err := repo.GetAll("", 2, 2)
	if err != nil {
		t.Fatalf("GetAll page 2: %v", err)
	}
	if total != 3 || len(second) != 1 || second[0].ID != oldest.ID {
		t.Fatalf("page 2 = %+v, total %d", second, total)
	}

	paid, totalPaid, err := repo.GetAll(model.OrderStatusPaid, 10, 0)
	if err != nil {
		t.Fatalf("GetAll paid: %v", err)
	}
	if totalPaid != 1 || len(paid) != 1 || paid[0].ID != middle.ID {
		t.Fatalf("paid page = %+v, total %d", paid, totalPaid)
	}

	stats, err := repo.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.Total != 3 {
		t.Fatalf("total = %d, want 3", stats.Total)
	}
	if stats.ByStatus[model.OrderStatusPending] != 1 || stats.ByStatus[model.OrderStatusPaid] != 1 || stats.ByStatus[model.OrderStatusCancelled] != 1 {
		t.Fatalf("by status = %+v", stats.ByStatus)
	}
	if stats.Revenue != 2000 {
		t.Fatalf("revenue = %d, want 2000", stats.Revenue)
	}
}
