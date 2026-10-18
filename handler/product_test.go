package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"

	"golearn/model"
)

func fetchProducts(t *testing.T, app *fiber.App, path string) []model.Product {
	t.Helper()

	req, _ := http.NewRequest(http.MethodGet, path, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", path, resp.StatusCode)
	}

	var products []model.Product
	if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
		t.Fatal(err)
	}
	return products
}

func TestAdminProductListIncludesHidden(t *testing.T) {
	app, _ := setupApp()

	// hide product 1 through the admin update endpoint
	req, _ := http.NewRequest(http.MethodPatch, "/admin/products/1", bytes.NewReader([]byte(`{"is_available":false}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("hide status = %d, want 200", resp.StatusCode)
	}

	if public := fetchProducts(t, app, "/products"); len(public) != 5 {
		t.Errorf("public list has %d products, want 5 (hidden one excluded)", len(public))
	}
	if admin := fetchProducts(t, app, "/admin/products"); len(admin) != 6 {
		t.Errorf("admin list has %d products, want 6 (includes hidden)", len(admin))
	}
}
