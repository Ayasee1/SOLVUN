package model

import "time"

// Price is an integer in the smallest currency unit (IDR rupiah).
// Name, Description and Images are content fields (owned by the catalog files);
// Price, Stock and IsAvailable are runtime fields (owned by the database).
type Product struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Images      []string `json:"images,omitempty"`
	Price       int      `json:"price"`
	Stock       int      `json:"stock"`
	IsAvailable bool     `json:"is_available"`
	// WeightGram is the item weight in grams, used to calculate shipping. It is
	// a content field, so it lives in the catalog files.
	WeightGram int       `json:"weight_gram"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ProductUpdate is the (partial) input for updating a product.
// Pointer fields let us tell "not provided" apart from a zero value.
type ProductUpdate struct {
	Name        *string `json:"name"`
	Price       *int    `json:"price"`
	Stock       *int    `json:"stock"`
	IsAvailable *bool   `json:"is_available"`
}

type Catalog struct {
	ID    int       `json:"id"`
	Name  string    `json:"name"`
	Items []Product `json:"items"`
}

// fake data for catalog
var Catalogs = []Catalog{
	{
		ID:   1,
		Name: "Electronics",
		Items: []Product{
			{ID: 1, Name: "Laptop", Price: 1000, Stock: 10, IsAvailable: true, WeightGram: 1500},
			{ID: 2, Name: "Smartphone", Price: 500, Stock: 20, IsAvailable: true, WeightGram: 200},
			{ID: 3, Name: "Tablet", Price: 300, Stock: 30, IsAvailable: true, WeightGram: 500},
		},
	},
	{
		ID:   2,
		Name: "Clothing",
		Items: []Product{
			{ID: 4, Name: "T-Shirt", Price: 50, Stock: 20, IsAvailable: true, WeightGram: 150},
			{ID: 5, Name: "Jeans", Price: 100, Stock: 30, IsAvailable: true, WeightGram: 400},
			{ID: 6, Name: "Jacket", Price: 150, Stock: 40, IsAvailable: true, WeightGram: 600},
		},
	},
}
