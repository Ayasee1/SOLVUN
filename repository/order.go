package repository

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"golearn/model"
)

// Page bounds for admin lists. The handler and the repositories share them so a
// request can never dump the whole table.
const (
	DefaultPageSize = 25
	MaxPageSize     = 100
)

// PageBounds clamps a requested page to safe values.
func PageBounds(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// OrderStats is the dashboard aggregate over every order, computed without
// loading the rows.
type OrderStats struct {
	Total    int
	Revenue  int
	ByStatus map[model.OrderStatus]int
}

// OrderRepository is the data access contract for orders.
type OrderRepository interface {
	Save(order model.UserOrder) error
	// GetAll returns one page of orders, newest first, optionally filtered by
	// status ("" means all), plus the total number of matching orders.
	GetAll(status model.OrderStatus, limit, offset int) ([]model.UserOrder, int, error)
	// Stats returns the dashboard aggregate.
	Stats() (OrderStats, error)
	// GetStalePending returns pending orders created before the cutoff, used by
	// the sweeper to expire orders whose payment never completed.
	GetStalePending(before time.Time) ([]model.UserOrder, error)
	// GetNeedsRefund returns orders whose payment was captured but can no longer
	// be fulfilled, so the operator has to refund them.
	GetNeedsRefund() ([]model.UserOrder, error)
	// GetPaidWithoutShipment returns paid orders with a courier but no provider
	// shipment created before the cutoff. The ops digest reports them.
	GetPaidWithoutShipment(before time.Time) ([]model.UserOrder, error)
	// GetShipmentRetries returns paid orders with a courier and no provider
	// shipment whose last automatic attempt is old enough and that still have
	// attempts left.
	GetShipmentRetries(attemptedBefore time.Time, maxAttempts int) ([]model.UserOrder, error)
	// RecordShipmentAttempt counts one automatic shipment attempt against the
	// order, so a persistent failure cannot be retried forever.
	RecordShipmentAttempt(id uuid.UUID) error
	FindByID(id uuid.UUID) (*model.UserOrder, error)
	TransitionStatus(id uuid.UUID, from, to model.OrderStatus) (*model.UserOrder, bool, error)
	TransitionStatusWithPayment(id uuid.UUID, from, to model.OrderStatus, info model.PaymentInfo) (*model.UserOrder, bool, error)
	// TransitionStatusWithRefund moves an order to "to" (usually refunded) and
	// records the refund atomically.
	TransitionStatusWithRefund(id uuid.UUID, from, to model.OrderStatus, info model.RefundInfo) (*model.UserOrder, bool, error)
	// SettlePaid claims the order for a confirmed payment. When reserveStock is
	// true, the stock is reserved in the same unit of work: the order becomes
	// either paid with all of it reserved, or needs_refund with none of it
	// touched. That is what keeps a crash from leaving a paid order without
	// stock (or stock without a paid order).
	SettlePaid(id uuid.UUID, from model.OrderStatus, info model.PaymentInfo, stock map[int]int, reserveStock bool) (*model.UserOrder, SettleOutcome, error)
	SetAccessToken(id uuid.UUID, token string) error
	SetShipment(id uuid.UUID, info model.ShipmentInfo) error
	FindByShipmentProviderID(providerID string) (*model.UserOrder, error)
	Delete(id uuid.UUID) (model.UserOrder, error)
}

// SettleOutcome reports what SettlePaid did.
type SettleOutcome string

const (
	// SettleClaimed: the order is paid and, when requested, its stock is reserved.
	SettleClaimed SettleOutcome = "claimed"
	// SettleNeedsRefund: the payment was recorded, but the stock was gone, so the
	// order is flagged for a manual refund.
	SettleNeedsRefund SettleOutcome = "needs_refund"
	// SettleNotClaimed: the order was not in the expected status (another
	// notification or the sweeper got there first).
	SettleNotClaimed SettleOutcome = "not_claimed"
)

// MemoryOrderRepository is the in-memory implementation used for dev and tests.
type MemoryOrderRepository struct {
	mu     sync.RWMutex
	orders []model.UserOrder
	// products is set once at startup so settlement can reserve stock. Only the
	// Postgres implementation is atomic; in memory the two steps just run in
	// sequence, which is fine for dev and tests.
	products ProductRepository
}

var _ OrderRepository = (*MemoryOrderRepository)(nil)

func NewMemoryOrderRepository() *MemoryOrderRepository {
	return &MemoryOrderRepository{
		orders: []model.UserOrder{},
	}
}

// SetProducts wires the catalog used by SettlePaid. Call it once at startup.
func (r *MemoryOrderRepository) SetProducts(products ProductRepository) {
	r.products = products
}

func (r *MemoryOrderRepository) Save(order model.UserOrder) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.orders = append(r.orders, order)
	return nil
}

func (r *MemoryOrderRepository) GetAll(status model.OrderStatus, limit, offset int) ([]model.UserOrder, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	limit, offset = PageBounds(limit, offset)

	filtered := make([]model.UserOrder, 0, len(r.orders))
	for _, order := range r.orders {
		if status != "" && order.Status != status {
			continue
		}
		filtered = append(filtered, order)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})

	total := len(filtered)
	if offset >= total {
		return []model.UserOrder{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return filtered[offset:end], total, nil
}

// Stats aggregates every order for the dashboard.
func (r *MemoryOrderRepository) Stats() (OrderStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := OrderStats{ByStatus: make(map[model.OrderStatus]int)}
	for _, order := range r.orders {
		stats.Total++
		stats.ByStatus[order.Status]++
		if order.Status == model.OrderStatusPaid {
			stats.Revenue += order.Total
		}
	}
	return stats, nil
}

func (r *MemoryOrderRepository) GetStalePending(before time.Time) ([]model.UserOrder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]model.UserOrder, 0)
	for _, order := range r.orders {
		if order.Status == model.OrderStatusPending && order.CreatedAt.Before(before) {
			out = append(out, order)
		}
	}
	return out, nil
}

// GetNeedsRefund returns orders that were paid but cannot be fulfilled.
func (r *MemoryOrderRepository) GetNeedsRefund() ([]model.UserOrder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]model.UserOrder, 0)
	for _, order := range r.orders {
		if order.Status == model.OrderStatusNeedsRefund {
			out = append(out, order)
		}
	}
	return out, nil
}

// GetPaidWithoutShipment returns paid orders that have a courier but no provider
// shipment, created before the cutoff.
func (r *MemoryOrderRepository) GetPaidWithoutShipment(before time.Time) ([]model.UserOrder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]model.UserOrder, 0)
	for _, order := range r.orders {
		if order.Status != model.OrderStatusPaid || order.Courier == "" {
			continue
		}
		if order.Shipment != nil && order.Shipment.ProviderID != "" {
			continue
		}
		if !order.CreatedAt.Before(before) {
			continue
		}
		out = append(out, order)
	}
	return out, nil
}

// GetShipmentRetries returns paid orders that still need a provider shipment
// and are due for another automatic attempt.
func (r *MemoryOrderRepository) GetShipmentRetries(attemptedBefore time.Time, maxAttempts int) ([]model.UserOrder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]model.UserOrder, 0)
	for _, order := range r.orders {
		if order.Status != model.OrderStatusPaid || order.Courier == "" {
			continue
		}
		if order.Shipment != nil && order.Shipment.ProviderID != "" {
			continue
		}
		if order.ShipmentAttempts >= maxAttempts {
			continue
		}
		if order.ShipmentLastAttemptAt != nil && order.ShipmentLastAttemptAt.After(attemptedBefore) {
			continue
		}
		out = append(out, order)
	}
	return out, nil
}

// RecordShipmentAttempt counts one automatic attempt against the order.
func (r *MemoryOrderRepository) RecordShipmentAttempt(id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.orders {
		if r.orders[i].ID != id {
			continue
		}
		now := time.Now()
		r.orders[i].ShipmentAttempts++
		r.orders[i].ShipmentLastAttemptAt = &now
		return nil
	}
	return ErrOrderNotFound
}

func (r *MemoryOrderRepository) FindByID(id uuid.UUID) (*model.UserOrder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := range r.orders {
		if r.orders[i].ID == id {
			order := r.orders[i]
			return &order, nil
		}
	}

	return nil, ErrOrderNotFound
}

// TransitionStatus atomically moves an order from one status to another.
// The boolean reports whether the transition happened; it is false when the
// order exists but its current status differs from "from" (idempotency guard).
func (r *MemoryOrderRepository) TransitionStatus(id uuid.UUID, from, to model.OrderStatus) (*model.UserOrder, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.orders {
		if r.orders[i].ID != id {
			continue
		}

		order := r.orders[i]
		if order.Status != from {
			return &order, false, nil
		}

		r.orders[i].Status = to
		order.Status = to
		return &order, true, nil
	}

	return nil, false, ErrOrderNotFound
}

// TransitionStatusWithPayment atomically moves an order from "from" to "to"
// and records the payment details, e.g. pending -> paid, expired -> needs_refund.
func (r *MemoryOrderRepository) TransitionStatusWithPayment(id uuid.UUID, from, to model.OrderStatus, info model.PaymentInfo) (*model.UserOrder, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.orders {
		if r.orders[i].ID != id {
			continue
		}

		order := r.orders[i]
		if order.Status != from {
			return &order, false, nil
		}

		r.orders[i].Status = to
		r.orders[i].Payment = &info

		order.Status = to
		order.Payment = &info
		return &order, true, nil
	}

	return nil, false, ErrOrderNotFound
}

// SettlePaid claims the order and then reserves the stock. The in-memory
// version runs both steps under separate locks; only the Postgres version is a
// single transaction, which is what production uses.
func (r *MemoryOrderRepository) SettlePaid(id uuid.UUID, from model.OrderStatus, info model.PaymentInfo, stock map[int]int, reserveStock bool) (*model.UserOrder, SettleOutcome, error) {
	order, confirmed, err := r.TransitionStatusWithPayment(id, from, model.OrderStatusPaid, info)
	if err != nil {
		return nil, "", err
	}
	if !confirmed {
		return order, SettleNotClaimed, nil
	}
	if !reserveStock {
		return order, SettleClaimed, nil
	}
	if r.products == nil {
		return nil, "", errors.New("memory settlement requires a product repository")
	}

	if err := r.products.DecreaseStock(stock); err != nil {
		updated, _, terr := r.TransitionStatusWithPayment(id, model.OrderStatusPaid, model.OrderStatusNeedsRefund, info)
		if terr != nil {
			return nil, "", terr
		}
		return updated, SettleNeedsRefund, nil
	}

	return order, SettleClaimed, nil
}

// TransitionStatusWithRefund moves an order and records the refund atomically.
func (r *MemoryOrderRepository) TransitionStatusWithRefund(id uuid.UUID, from, to model.OrderStatus, info model.RefundInfo) (*model.UserOrder, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.orders {
		if r.orders[i].ID != id {
			continue
		}

		order := r.orders[i]
		if order.Status != from {
			return &order, false, nil
		}

		r.orders[i].Status = to
		r.orders[i].Refund = &info

		order.Status = to
		order.Refund = &info
		return &order, true, nil
	}

	return nil, false, ErrOrderNotFound
}

// SetAccessToken stores the guest access token for an order.
func (r *MemoryOrderRepository) SetAccessToken(id uuid.UUID, token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.orders {
		if r.orders[i].ID == id {
			r.orders[i].AccessToken = token
			return nil
		}
	}
	return ErrOrderNotFound
}

// SetShipment stores the provider shipment details on the order.
func (r *MemoryOrderRepository) SetShipment(id uuid.UUID, info model.ShipmentInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.orders {
		if r.orders[i].ID == id {
			r.orders[i].Shipment = &info
			return nil
		}
	}
	return ErrOrderNotFound
}

// FindByShipmentProviderID locates the order behind a provider shipment id,
// which is how shipping webhooks map back to our orders.
func (r *MemoryOrderRepository) FindByShipmentProviderID(providerID string) (*model.UserOrder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := range r.orders {
		if r.orders[i].Shipment != nil && r.orders[i].Shipment.ProviderID == providerID {
			order := r.orders[i]
			return &order, nil
		}
	}
	return nil, ErrOrderNotFound
}

func (r *MemoryOrderRepository) Delete(id uuid.UUID) (model.UserOrder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.orders {
		if r.orders[i].ID == id {
			order := r.orders[i]
			r.orders = append(r.orders[:i], r.orders[i+1:]...)
			return order, nil
		}
	}

	return model.UserOrder{}, ErrOrderNotFound
}
