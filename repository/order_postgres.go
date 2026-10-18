package repository

import (
	"context"
	"encoding/json"
	"errors"
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
