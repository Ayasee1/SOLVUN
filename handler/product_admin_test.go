package handler_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"golearn/handler"
	"golearn/model"
	"golearn/repository"
)

func postProduct(t *testing.T, app *fiber.App, body string) (*http.Response, model.Product) {
	t.Helper()

	req, _ := http.NewRequest(http.MethodPost, "/admin/products", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	var product model.Product
	_ = json.NewDecoder(resp.Body).Decode(&product)
	return resp, product
}

func TestCreateProduct(t *testing.T) {
	app, _ := setupApp()

	resp, created := postProduct(t, app, `{
		"name": "New Thing",
		"description": "Made from the dashboard",
		"images": ["/media/products/a.png"],
		"price": 5000,
		"stock": 3,
		"weight_gram": 250
	}`)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	if created.ID <= 0 {
		t.Fatalf("id = %d, want a positive id", created.ID)
	}
	if created.Name != "New Thing" || created.Price != 5000 || created.Stock != 3 || created.WeightGram != 250 {
		t.Errorf("created = %+v", created)
	}
	if !created.IsAvailable {
		t.Error("is_available should default to true")
	}

	// The new product is in the admin list and (available) in the storefront.
	admin := fetchProducts(t, app, "/admin/products")
	if len(admin) != 7 {
		t.Errorf("admin list = %d products, want 7", len(admin))
	}
}

func TestCreateProductValidation(t *testing.T) {
	app, _ := setupApp()

	cases := map[string]string{
		"empty name":     `{"name":"  ","price":1,"stock":1,"weight_gram":10}`,
		"negative price": `{"name":"X","price":-1,"stock":1,"weight_gram":10}`,
		"zero weight":    `{"name":"X","price":1,"stock":1,"weight_gram":0}`,
		"bad image":      `{"name":"X","price":1,"stock":1,"weight_gram":10,"images":["javascript:alert(1)"]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			resp, _ := postProduct(t, app, body)
			if resp.StatusCode != fiber.StatusBadRequest {
				t.Fatalf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}

func TestUpdateProductContent(t *testing.T) {
	app, _ := setupApp()

	body := `{"name":"Renamed","description":"New copy","weight_gram":321,"images":["/media/products/x.png"]}`
	req, _ := http.NewRequest(http.MethodPatch, "/admin/products/1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var product model.Product
	if err := json.NewDecoder(resp.Body).Decode(&product); err != nil {
		t.Fatal(err)
	}
	if product.Name != "Renamed" || product.Description != "New copy" || product.WeightGram != 321 {
		t.Fatalf("product = %+v", product)
	}
	if len(product.Images) != 1 || product.Images[0] != "/media/products/x.png" {
		t.Fatalf("images = %#v", product.Images)
	}

	bad, _ := http.NewRequest(http.MethodPatch, "/admin/products/1", strings.NewReader(`{"weight_gram":0}`))
	bad.Header.Set("Content-Type", "application/json")
	badResp, err := app.Test(bad)
	if err != nil {
		t.Fatal(err)
	}
	if badResp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("bad weight status = %d, want 400", badResp.StatusCode)
	}
}

func newUploadApp(t *testing.T, dir string) *fiber.App {
	t.Helper()

	productHandler := handler.NewProductHandler(repository.NewMemoryProductRepository())
	productHandler.SetMediaDir(dir)

	app := fiber.New()
	app.Post("/admin/products/images", productHandler.UploadProductImage)
	return app
}

func uploadImage(t *testing.T, app *fiber.App, filename string, data []byte) *http.Response {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPost, "/admin/products/images", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestUploadProductImage(t *testing.T) {
	dir := t.TempDir()
	app := newUploadApp(t, dir)

	// minimal PNG signature; DetectContentType only needs the header
	png := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, make([]byte, 64)...)

	resp := uploadImage(t, app, "photo.png", png)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var payload struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(payload.URL, "/media/products/") || !strings.HasSuffix(payload.URL, ".png") {
		t.Fatalf("url = %q", payload.URL)
	}

	name := strings.TrimPrefix(payload.URL, "/media/products/")
	stored, err := os.Stat(filepath.Join(dir, "products", name))
	if err != nil {
		t.Fatalf("stored file: %v", err)
	}
	if stored.Size() != int64(len(png)) {
		t.Fatalf("size = %d, want %d", stored.Size(), len(png))
	}
	if perm := stored.Mode().Perm(); perm != 0o644 {
		t.Fatalf("mode = %o, want 644 so nginx can read it", perm)
	}

	// A text file is not an image.
	if got := uploadImage(t, app, "notes.txt", []byte("hello world")); got.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("text upload status = %d, want 400", got.StatusCode)
	}
}

func TestUploadDisabledWithoutMediaDir(t *testing.T) {
	app := newUploadApp(t, "")

	png := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, make([]byte, 64)...)
	if got := uploadImage(t, app, "photo.png", png); got.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", got.StatusCode)
	}
}
