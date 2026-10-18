package repository

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"golearn/model"
)

func newTestRepo() *MemoryProductRepository {
	return &MemoryProductRepository{
		products: []model.Product{
			{ID: 1, Name: "Laptop", Price: 1000, Stock: 5},
			{ID: 2, Name: "Phone", Price: 500, Stock: 0},
		},
	}
}

func mustFind(t *testing.T, r *MemoryProductRepository, id int) *model.Product {
	t.Helper()

	p, err := r.FindByID(id)
	if err != nil {
		t.Fatalf("FindByID(%d) unexpected error: %v", id, err)
	}
	return p
}

func TestDecreaseStock(t *testing.T) {
	t.Run("success decreases stock", func(t *testing.T) {
		r := newTestRepo()

		if err := r.DecreaseStock(map[int]int{1: 3}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if p := mustFind(t, r, 1); p.Stock != 2 {
			t.Errorf("stock = %d, want 2", p.Stock)
		}
	})

	t.Run("not enough stock returns error and keeps stock", func(t *testing.T) {
		r := newTestRepo()

		err := r.DecreaseStock(map[int]int{1: 100})
		if !errors.Is(err, ErrNotEnoughStock) {
			t.Fatalf("err = %v, want ErrNotEnoughStock", err)
		}

		if p := mustFind(t, r, 1); p.Stock != 5 {
			t.Errorf("stock = %d, want 5 (unchanged)", p.Stock)
		}
	})

	t.Run("unknown product returns error", func(t *testing.T) {
		r := newTestRepo()

		err := r.DecreaseStock(map[int]int{999: 1})
		if !errors.Is(err, ErrProductNotFound) {
			t.Fatalf("err = %v, want ErrProductNotFound", err)
		}
	})

	t.Run("atomic: no item is changed when another fails", func(t *testing.T) {
		r := newTestRepo()

		// id 1 is fine, id 2 has no stock -> whole operation must fail
		err := r.DecreaseStock(map[int]int{1: 1, 2: 1})
		if !errors.Is(err, ErrNotEnoughStock) {
			t.Fatalf("err = %v, want ErrNotEnoughStock", err)
		}

		if p := mustFind(t, r, 1); p.Stock != 5 {
			t.Errorf("id1 stock = %d, want 5 (must not change)", p.Stock)
		}
	})
}

func TestDecreaseStockConcurrentNoOversell(t *testing.T) {
	r := &MemoryProductRepository{
		products: []model.Product{{ID: 1, Name: "Limited", Price: 10, Stock: 100}},
	}

	const attempts = 200
	var wg sync.WaitGroup
	var success int32

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.DecreaseStock(map[int]int{1: 1}); err == nil {
				atomic.AddInt32(&success, 1)
			}
		}()
	}
	wg.Wait()

	if success != 100 {
		t.Errorf("successful decrements = %d, want 100", success)
	}
	if p := mustFind(t, r, 1); p.Stock != 0 {
		t.Errorf("final stock = %d, want 0", p.Stock)
	}
}

func TestDelete(t *testing.T) {
	r := newTestRepo()

	p, err := r.Delete(1)
	if err != nil {
		t.Fatalf("Delete(1) unexpected error: %v", err)
	}
	if p.ID != 1 {
		t.Errorf("deleted product id = %d, want 1", p.ID)
	}
	if _, err := r.FindByID(1); !errors.Is(err, ErrProductNotFound) {
		t.Errorf("FindByID(1) err = %v, want ErrProductNotFound", err)
	}
	if _, err := r.Delete(999); !errors.Is(err, ErrProductNotFound) {
		t.Errorf("Delete(999) err = %v, want ErrProductNotFound", err)
	}
}

func TestGetAllReturnsCopy(t *testing.T) {
	r := newTestRepo()

	got, err := r.GetAll()
	if err != nil {
		t.Fatal(err)
	}
	got[0].Stock = 999

	if p := mustFind(t, r, 1); p.Stock == 999 {
		t.Error("GetAll leaked the internal slice (mutation affected the repo)")
	}
}

func TestFindByIDNotFound(t *testing.T) {
	r := newTestRepo()

	if _, err := r.FindByID(999); !errors.Is(err, ErrProductNotFound) {
		t.Errorf("err = %v, want ErrProductNotFound", err)
	}
}

func TestDecreaseStockMarksUnavailableWhenEmpty(t *testing.T) {
	r := newTestRepo() // product 1 has stock 5

	if err := r.DecreaseStock(map[int]int{1: 5}); err != nil {
		t.Fatal(err)
	}

	p := mustFind(t, r, 1)
	if p.Stock != 0 {
		t.Errorf("stock = %d, want 0", p.Stock)
	}
	if p.IsAvailable {
		t.Error("IsAvailable = true, want false when stock is 0")
	}
	if p.UpdatedAt.IsZero() {
		t.Error("UpdatedAt was not set")
	}
}

func TestReleaseStock(t *testing.T) {
	r := newTestRepo() // product 1 has stock 5

	if err := r.DecreaseStock(map[int]int{1: 5}); err != nil {
		t.Fatal(err)
	}
	if p := mustFind(t, r, 1); p.IsAvailable {
		t.Fatal("precondition failed: product should be unavailable with 0 stock")
	}

	if err := r.ReleaseStock(map[int]int{1: 3}); err != nil {
		t.Fatal(err)
	}

	p := mustFind(t, r, 1)
	if p.Stock != 3 {
		t.Errorf("stock = %d, want 3", p.Stock)
	}
	if !p.IsAvailable {
		t.Error("IsAvailable = false, want true after restock")
	}

	// releasing a product that no longer exists must not error
	if err := r.ReleaseStock(map[int]int{999: 1}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdateProductPartial(t *testing.T) {
	r := newTestRepo()
	r.products[0].IsAvailable = true

	price := 1234
	updated, err := r.UpdateProduct(1, model.ProductUpdate{Price: &price})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.Price != 1234 {
		t.Errorf("price = %v, want 1234", updated.Price)
	}
	if updated.Name != "Laptop" {
		t.Errorf("name = %q, want Laptop (must be preserved)", updated.Name)
	}
	if !updated.IsAvailable {
		t.Error("IsAvailable flipped to false; untouched field must be preserved")
	}
	if updated.UpdatedAt.IsZero() {
		t.Error("UpdatedAt was not set")
	}

	if _, err := r.UpdateProduct(999, model.ProductUpdate{}); !errors.Is(err, ErrProductNotFound) {
		t.Errorf("err = %v, want ErrProductNotFound", err)
	}
}
