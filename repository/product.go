package repository

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"golearn/model"
)

var (
	ErrProductNotFound = errors.New("product not found")
	ErrNotEnoughStock  = errors.New("not enough stock")
	ErrOrderNotFound   = errors.New("order not found")
)

// ProductRepository is the data access contract for products.
type ProductRepository interface {
	GetAll() ([]model.Product, error)
	FindByID(id int) (*model.Product, error)
	Delete(id int) (model.Product, error)
	DecreaseStock(requested map[int]int) error
	ReleaseStock(requested map[int]int) error
	UpdateProduct(id int, update model.ProductUpdate) (*model.Product, error)
}

// MemoryProductRepository is the in-memory implementation used for dev and tests.
type MemoryProductRepository struct {
	mu       sync.RWMutex
	products []model.Product
}

var _ ProductRepository = (*MemoryProductRepository)(nil)

func NewMemoryProductRepository() *MemoryProductRepository {
	var products []model.Product
	for _, catalog := range model.Catalogs {
		products = append(products, catalog.Items...)
	}

	return &MemoryProductRepository{products: products}
}

func (r *MemoryProductRepository) GetAll() ([]model.Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]model.Product, len(r.products))
	copy(out, r.products)
	return out, nil
}

func (r *MemoryProductRepository) FindByID(id int) (*model.Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := range r.products {
		if r.products[i].ID == id {
			product := r.products[i]
			return &product, nil
		}
	}

	return nil, ErrProductNotFound
}

func (r *MemoryProductRepository) Delete(id int) (model.Product, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// hard delete: remove the product from the slice
	for i := range r.products {
		if r.products[i].ID == id {
			product := r.products[i]
			r.products = append(r.products[:i], r.products[i+1:]...)
			return product, nil
		}
	}

	return model.Product{}, ErrProductNotFound
}

// DecreaseStock checks availability for every requested product and, only if
// all of them pass, decrements the stock atomically.
func (r *MemoryProductRepository) DecreaseStock(requested map[int]int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, qty := range requested {
		product := r.findByIDLocked(id)
		if product == nil {
			return fmt.Errorf("%w for id %d", ErrProductNotFound, id)
		}
		if product.Stock < qty {
			return fmt.Errorf("%w for product %s", ErrNotEnoughStock, product.Name)
		}
	}

	for id, qty := range requested {
		product := r.findByIDLocked(id)
		product.Stock -= qty
		product.IsAvailable = product.Stock > 0
		product.UpdatedAt = time.Now()
	}

	return nil
}

// ReleaseStock returns reserved stock to the catalog. It is best-effort:
// products that no longer exist are skipped.
func (r *MemoryProductRepository) ReleaseStock(requested map[int]int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, qty := range requested {
		product := r.findByIDLocked(id)
		if product == nil {
			continue
		}
		product.Stock += qty
		product.IsAvailable = product.Stock > 0
		product.UpdatedAt = time.Now()
	}

	return nil
}

// UpdateProduct applies a partial update (only the provided fields) for admins.
func (r *MemoryProductRepository) UpdateProduct(id int, update model.ProductUpdate) (*model.Product, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	product := r.findByIDLocked(id)
	if product == nil {
		return nil, fmt.Errorf("%w for id %d", ErrProductNotFound, id)
	}

	if update.Name != nil {
		product.Name = *update.Name
	}
	if update.Price != nil {
		product.Price = *update.Price
	}
	if update.Stock != nil {
		product.Stock = *update.Stock
		product.IsAvailable = *update.Stock > 0
	}
	if update.IsAvailable != nil {
		product.IsAvailable = *update.IsAvailable
	}
	product.UpdatedAt = time.Now()

	result := *product
	return &result, nil
}

// findByIDLocked must only be called while holding r.mu.
func (r *MemoryProductRepository) findByIDLocked(id int) *model.Product {
	for i := range r.products {
		if r.products[i].ID == id {
			return &r.products[i]
		}
	}
	return nil
}
