package repository

import (
	"sync"

	"github.com/google/uuid"

	"golearn/model"
)

// OrderRepository is the data access contract for orders.
type OrderRepository interface {
	Save(order model.UserOrder) error
	// GetAll returns orders, optionally filtered by status ("" means all).
	GetAll(status model.OrderStatus) ([]model.UserOrder, error)
	FindByID(id uuid.UUID) (*model.UserOrder, error)
	TransitionStatus(id uuid.UUID, from, to model.OrderStatus) (*model.UserOrder, bool, error)
	TransitionStatusWithPayment(id uuid.UUID, from, to model.OrderStatus, info model.PaymentInfo) (*model.UserOrder, bool, error)
	SetShipment(id uuid.UUID, info model.ShipmentInfo) error
	FindByShipmentProviderID(providerID string) (*model.UserOrder, error)
	Delete(id uuid.UUID) (model.UserOrder, error)
}

// MemoryOrderRepository is the in-memory implementation used for dev and tests.
type MemoryOrderRepository struct {
	mu     sync.RWMutex
	orders []model.UserOrder
}

var _ OrderRepository = (*MemoryOrderRepository)(nil)

func NewMemoryOrderRepository() *MemoryOrderRepository {
	return &MemoryOrderRepository{
		orders: []model.UserOrder{},
	}
}

func (r *MemoryOrderRepository) Save(order model.UserOrder) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.orders = append(r.orders, order)
	return nil
}

func (r *MemoryOrderRepository) GetAll(status model.OrderStatus) ([]model.UserOrder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]model.UserOrder, 0, len(r.orders))
	for _, order := range r.orders {
		if status != "" && order.Status != status {
			continue
		}
		out = append(out, order)
	}
	return out, nil
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
