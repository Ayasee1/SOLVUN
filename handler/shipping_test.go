package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"

	"golearn/handler"
	"golearn/repository"
	"golearn/shipping"
)

type fakeShipping struct {
	lastReq shipping.RateRequest
	rates   []shipping.Rate
	err     error
}

func (f *fakeShipping) Rates(req shipping.RateRequest) ([]shipping.Rate, error) {
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.rates, nil
}

func (f *fakeShipping) CreateShipment(shipping.ShipmentRequest) (*shipping.Shipment, error) {
	return nil, nil
}

func (f *fakeShipping) Tracking(string) (*shipping.Shipment, error) {
	return &shipping.Shipment{Status: "confirmed"}, nil
}

func setupShippingApp(provider shipping.Provider) *fiber.App {
	app := fiber.New()

	productRepo := repository.NewMemoryProductRepository()
	shippingHandler := handler.NewShippingHandler(provider, productRepo)

	app.Post("/shipping/rates", shippingHandler.Rates)

	return app
}

func postRates(t *testing.T, app *fiber.App, body string) (*http.Response, map[string]any) {
	t.Helper()

	req, _ := http.NewRequest(http.MethodPost, "/shipping/rates", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	json.NewDecoder(resp.Body).Decode(&payload)
	return resp, payload
}

func TestShippingRatesComputesWeightServerSide(t *testing.T) {
	fake := &fakeShipping{rates: []shipping.Rate{{Courier: "jne", Service: "reg", Price: 15000}}}
	app := setupShippingApp(fake)

	resp, payload := postRates(t, app,
		`{"destination_postal_code":"40123","items":[{"product_id":1,"qty":2}]}`)

	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// product 1 weighs 1500g, qty 2 -> 3000g
	if fake.lastReq.WeightGram != 3000 {
		t.Errorf("provider weight = %d, want 3000", fake.lastReq.WeightGram)
	}
	if fake.lastReq.DestinationPostalCode != "40123" {
		t.Errorf("destination = %q", fake.lastReq.DestinationPostalCode)
	}
	if payload["weight_gram"].(float64) != 3000 {
		t.Errorf("response weight_gram = %v, want 3000", payload["weight_gram"])
	}
}

func TestShippingRatesRejectsUnknownProduct(t *testing.T) {
	app := setupShippingApp(&fakeShipping{})

	resp, _ := postRates(t, app, `{"destination_postal_code":"40123","items":[{"product_id":999,"qty":1}]}`)

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestShippingRatesRejectsEmptyCart(t *testing.T) {
	app := setupShippingApp(&fakeShipping{})

	resp, _ := postRates(t, app, `{"destination_postal_code":"40123","items":[]}`)

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestShippingRatesDisabledProvider(t *testing.T) {
	app := setupShippingApp(shipping.Disabled{})

	resp, _ := postRates(t, app, `{"destination_postal_code":"40123","items":[{"product_id":1,"qty":1}]}`)

	if resp.StatusCode != fiber.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}
