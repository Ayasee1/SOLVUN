package repository

import (
	"context"
	"errors"
	"testing"

	"golearn/db"
	"golearn/model"
)

func TestPostgresCreateAndUpdateProduct(t *testing.T) {
	pool, _ := postgresSettleRepo(t)
	repo := NewPostgresProductRepository(pool)

	created, err := repo.Create(model.ProductInput{
		Name:        "New",
		Description: "from the dashboard",
		Images:      []string{"/media/products/a.png"},
		Price:       2500,
		Stock:       4,
		WeightGram:  120,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID != 3 {
		t.Fatalf("id = %d, want 3 (sequence after the seeded ids)", created.ID)
	}
	if !created.IsAvailable || created.Name != "New" || created.Price != 2500 || created.WeightGram != 120 {
		t.Fatalf("created = %+v", created)
	}

	name, description, weight := "Renamed", "new copy", 300
	images := []string{"/media/products/b.png"}
	if _, err := repo.UpdateProduct(created.ID, model.ProductUpdate{
		Name:        &name,
		Description: &description,
		Images:      &images,
		WeightGram:  &weight,
	}); err != nil {
		t.Fatalf("UpdateProduct: %v", err)
	}

	stored, err := repo.FindByID(created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if stored.Name != name || stored.Description != description || stored.WeightGram != weight {
		t.Fatalf("stored = %+v", stored)
	}
	if len(stored.Images) != 1 || stored.Images[0] != images[0] {
		t.Fatalf("images = %#v", stored.Images)
	}
}

func TestPostgresSeedOnlyWhenEmpty(t *testing.T) {
	pool, _ := postgresSettleRepo(t)
	repo := NewPostgresProductRepository(pool)
	ctx := context.Background()

	catalog := []model.Catalog{{
		ID:   1,
		Name: "c",
		Items: []model.Product{
			{ID: 101, Name: "From Catalog", Price: 1, Stock: 1, IsAvailable: true, WeightGram: 10},
		},
	}}

	// The table already has rows, so the seed must not touch it.
	if err := db.Seed(ctx, pool, catalog); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if _, err := repo.FindByID(101); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("seed inserted into a non-empty table: %v", err)
	}

	// Empty table: the catalog is used as the starting point.
	if _, err := pool.Exec(ctx, `TRUNCATE products`); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if err := db.Seed(ctx, pool, catalog); err != nil {
		t.Fatalf("Seed empty: %v", err)
	}
	seeded, err := repo.FindByID(101)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if seeded.Name != "From Catalog" {
		t.Fatalf("name = %q", seeded.Name)
	}

	// Dashboard edits survive the next startup.
	edited := "Edited"
	if _, err := repo.UpdateProduct(101, model.ProductUpdate{Name: &edited}); err != nil {
		t.Fatalf("UpdateProduct: %v", err)
	}
	if err := db.Seed(ctx, pool, catalog); err != nil {
		t.Fatalf("Seed again: %v", err)
	}
	stored, err := repo.FindByID(101)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if stored.Name != "Edited" {
		t.Fatalf("name = %q, want the dashboard edit to survive", stored.Name)
	}

	// The sequence follows the highest seeded id, so the next product is 102.
	created, err := repo.Create(model.ProductInput{Name: "Next", Price: 1, Stock: 1, WeightGram: 1})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID != 102 {
		t.Fatalf("id = %d, want 102", created.ID)
	}
}
