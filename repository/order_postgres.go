package repository

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"golearn/model"
)

// PostgresOrderRepository stores orders in Postgres (Supabase). Address, items
// and payment are kept as JSONB to keep the model simple.
type PostgresOrderRepository struct {
	pool *pgxpool.Pool
}

var _ OrderRepository = (*PostgresOrderRepository)(nil)

func NewPostgresOrderRepository(pool *pgxpool.Pool) *PostgresOrderRepository {
	return &PostgresOrderRepository{pool: pool}
}

const orderColumns = `id, username, email, phone, address, items, total, status, shipping_cost, courier, courier_service, snap_token, payment_url, payment, shipment, created_at`

func (r *PostgresOrderRepository) Save(order model.UserOrder) error {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	address, err := json.Marshal(order.Address)
	if err != nil {
		return err
	}
	shipment, err := marshalShipment(order.Shipment)
	if err != nil {
		return err
	}
	items, err := json.Marshal(order.OrderItem)
	if err != nil {
		return err
	}
	payment, err := marshalPayment(order.Payment)
	if err != nil {
		return err
	}

	createdAt := order.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO orders (id, username, email, phone, address, items, total, status, shipping_cost, courier, courier_service, snap_token, payment_url, payment, shipment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		order.ID, order.Username, order.Email, order.Phone, address, items,
		order.Total, order.Status, order.ShippingCost, order.Courier, order.CourierService,
		order.SnapToken, order.PaymentURL, payment, shipment, createdAt)
	return err
}

func (r *PostgresOrderRepository) GetAll(status model.OrderStatus) ([]model.UserOrder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	rows, err := r.pool.Query(ctx,
		`SELECT `+orderColumns+` FROM orders WHERE ($1 = '' OR status = $1) ORDER BY created_at DESC`,
		string(status))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]model.UserOrder, 0)
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, *order)
	}

	return orders, rows.Err()
}

// GetStalePending returns pending orders created before the cutoff. The sweeper
// uses it to release stock held by orders whose payment never completed.
func (r *PostgresOrderRepository) GetStalePending(before time.Time) ([]model.UserOrder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	rows, err := r.pool.Query(ctx,
		`SELECT `+orderColumns+` FROM orders WHERE status = $1 AND created_at < $2 ORDER BY created_at`,
		string(model.OrderStatusPending), before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]model.UserOrder, 0)
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, *order)
	}

	return orders, rows.Err()
}

// GetNeedsRefund returns orders that were paid but can no longer be fulfilled.
func (r *PostgresOrderRepository) GetNeedsRefund() ([]model.UserOrder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	rows, err := r.pool.Query(ctx,
		`SELECT `+orderColumns+` FROM orders WHERE status = $1 ORDER BY created_at`,
		string(model.OrderStatusNeedsRefund))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]model.UserOrder, 0)
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, *order)
	}

	return orders, rows.Err()
}

// GetPaidWithoutShipment returns paid orders with a courier but no provider
// shipment created before the cutoff.
func (r *PostgresOrderRepository) GetPaidWithoutShipment(before time.Time) ([]model.UserOrder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	rows, err := r.pool.Query(ctx,
		`SELECT `+orderColumns+` FROM orders
		 WHERE status = $1
		   AND courier <> ''
		   AND COALESCE(shipment->>'provider_id', '') = ''
		   AND created_at < $2
		 ORDER BY created_at`,
		string(model.OrderStatusPaid), before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]model.UserOrder, 0)
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, *order)
	}

	return orders, rows.Err()
}

func (r *PostgresOrderRepository) FindByID(id uuid.UUID) (*model.UserOrder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	return scanOrder(r.pool.QueryRow(ctx, `SELECT `+orderColumns+` FROM orders WHERE id = $1`, id))
}

// TransitionStatus only updates the row when the current status matches "from",
// which acts as an atomic idempotency guard.
func (r *PostgresOrderRepository) TransitionStatus(id uuid.UUID, from, to model.OrderStatus) (*model.UserOrder, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	order, err := scanOrder(r.pool.QueryRow(ctx,
		`UPDATE orders SET status = $3 WHERE id = $1 AND status = $2 RETURNING `+orderColumns,
		id, from, to))
	if err == nil {
		return order, true, nil
	}
	if !errors.Is(err, ErrOrderNotFound) {
		return nil, false, err
	}

	// nothing was updated: distinguish "not found" from "wrong status"
	current, ferr := r.FindByID(id)
	if ferr != nil {
		return nil, false, ferr
	}
	return current, false, nil
}

// TransitionStatusWithPayment only updates the row when the current status
// matches "from", storing the payment details at the same time.
func (r *PostgresOrderRepository) TransitionStatusWithPayment(id uuid.UUID, from, to model.OrderStatus, info model.PaymentInfo) (*model.UserOrder, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	payment, err := marshalPayment(&info)
	if err != nil {
		return nil, false, err
	}

	order, err := scanOrder(r.pool.QueryRow(ctx,
		`UPDATE orders SET status = $3, payment = $4 WHERE id = $1 AND status = $2 RETURNING `+orderColumns,
		id, from, to, payment))
	if err == nil {
		return order, true, nil
	}
	if !errors.Is(err, ErrOrderNotFound) {
		return nil, false, err
	}

	current, ferr := r.FindByID(id)
	if ferr != nil {
		return nil, false, ferr
	}
	return current, false, nil
}

// SettlePaid claims the order and reserves stock in one transaction. Either the
// order ends up paid with every item reserved, or needs_refund with no product
// touched; a crash can never leave the two halves out of sync.
func (r *PostgresOrderRepository) SettlePaid(id uuid.UUID, from model.OrderStatus, info model.PaymentInfo, stock map[int]int, reserveStock bool) (*model.UserOrder, SettleOutcome, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	payment, err := marshalPayment(&info)
	if err != nil {
		return nil, "", err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, "", err
	}
	// No-op after a successful commit.
	defer func() { _ = tx.Rollback(ctx) }()

	order, err := scanOrder(tx.QueryRow(ctx,
		`UPDATE orders SET status = $3, payment = $4 WHERE id = $1 AND status = $2 RETURNING `+orderColumns,
		id, from, model.OrderStatusPaid, payment))
	if err != nil {
		if !errors.Is(err, ErrOrderNotFound) {
			return nil, "", err
		}
		// Nothing was claimed: report the current state so the caller can retry
		// against the status that actually won.
		_ = tx.Rollback(ctx)
		current, ferr := r.FindByID(id)
		if ferr != nil {
			return nil, "", ferr
		}
		return current, SettleNotClaimed, nil
	}

	if reserveStock {
		// Lock products in a stable order so two settlements cannot deadlock.
		ids := make([]int, 0, len(stock))
		for productID := range stock {
			ids = append(ids, productID)
		}
		sort.Ints(ids)

		for _, productID := range ids {
			var available int
			err := tx.QueryRow(ctx, `SELECT stock FROM products WHERE id = $1 FOR UPDATE`, productID).Scan(&available)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && available < stock[productID]) {
				// Turn the claim into a refund flag inside the same transaction,
				// so no product row has been changed and nothing is half done.
				if _, uerr := tx.Exec(ctx,
					`UPDATE orders SET status = $3 WHERE id = $1 AND status = $2`,
					id, model.OrderStatusPaid, model.OrderStatusNeedsRefund); uerr != nil {
					return nil, "", uerr
				}
				if cerr := tx.Commit(ctx); cerr != nil {
					return nil, "", cerr
				}
				order.Status = model.OrderStatusNeedsRefund
				return order, SettleNeedsRefund, nil
			}
			if err != nil {
				return nil, "", err
			}
		}

		for _, productID := range ids {
			if _, err := tx.Exec(ctx,
				`UPDATE products SET stock = stock - $1, is_available = (stock - $1) > 0, updated_at = now() WHERE id = $2`,
				stock[productID], productID); err != nil {
				return nil, "", err
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, "", err
	}

	return order, SettleClaimed, nil
}

func (r *PostgresOrderRepository) SetShipment(id uuid.UUID, info model.ShipmentInfo) error {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	payload, err := marshalShipment(&info)
	if err != nil {
		return err
	}

	tag, err := r.pool.Exec(ctx, `UPDATE orders SET shipment = $2 WHERE id = $1`, id, payload)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrOrderNotFound
	}
	return nil
}

func (r *PostgresOrderRepository) FindByShipmentProviderID(providerID string) (*model.UserOrder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	return scanOrder(r.pool.QueryRow(ctx,
		`SELECT `+orderColumns+` FROM orders WHERE shipment->>'provider_id' = $1 LIMIT 1`, providerID))
}

func (r *PostgresOrderRepository) Delete(id uuid.UUID) (model.UserOrder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	order, err := scanOrder(r.pool.QueryRow(ctx, `DELETE FROM orders WHERE id = $1 RETURNING `+orderColumns, id))
	if err != nil {
		return model.UserOrder{}, err
	}

	return *order, nil
}

// scanOrder works for both pgx.Row and pgx.Rows.
func scanOrder(row pgx.Row) (*model.UserOrder, error) {
	var (
		order    model.UserOrder
		address  []byte
		items    []byte
		payment  []byte
		shipment []byte
	)

	err := row.Scan(&order.ID, &order.Username, &order.Email, &order.Phone,
		&address, &items, &order.Total, &order.Status,
		&order.ShippingCost, &order.Courier, &order.CourierService,
		&order.SnapToken, &order.PaymentURL, &payment, &shipment, &order.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(address, &order.Address); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(items, &order.OrderItem); err != nil {
		return nil, err
	}
	if len(payment) > 0 {
		var info model.PaymentInfo
		if err := json.Unmarshal(payment, &info); err == nil {
			order.Payment = &info
		}
	}
	if len(shipment) > 0 {
		var info model.ShipmentInfo
		if err := json.Unmarshal(shipment, &info); err == nil {
			order.Shipment = &info
		}
	}

	return &order, nil
}

func marshalShipment(info *model.ShipmentInfo) ([]byte, error) {
	if info == nil {
		return nil, nil
	}
	return json.Marshal(info)
}

func marshalPayment(info *model.PaymentInfo) ([]byte, error) {
	if info == nil {
		return nil, nil
	}
	return json.Marshal(info)
}
