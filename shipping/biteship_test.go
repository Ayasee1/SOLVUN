package shipping

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newDraftServer fakes the draft-order flow: create draft -> read rates -> delete.
func newDraftServer(t *testing.T, ratePayload string) (*httptest.Server, *map[string]any, *bool) {
	t.Helper()

	var draftBody map[string]any
	deleted := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "biteship_test.key" {
			t.Errorf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/draft_orders":
			json.NewDecoder(r.Body).Decode(&draftBody)
			io.WriteString(w, `{"success":true,"id":"draft-1"}`)

		case r.Method == http.MethodGet && r.URL.Path == "/v1/draft_orders/draft-1/rates":
			io.WriteString(w, ratePayload)

		case r.Method == http.MethodDelete && r.URL.Path == "/v1/draft_orders/draft-1":
			deleted = true
			io.WriteString(w, `{"success":true}`)

		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	return server, &draftBody, &deleted
}

func TestBiteshipRatesUsesDraftFlow(t *testing.T) {
	server, draftBody, deleted := newDraftServer(t, `{"success":true,"pricing":[
		{"courier_name":"JNE","courier_code":"jne","courier_service_name":"Reguler","courier_service_code":"reg","duration":"2 - 3 days","price":36000},
		{"courier_name":"AnterAja","courier_code":"anteraja","courier_service_name":"Reguler","courier_service_code":"reg","duration":"2 - 4 days","shipping_fee":44800}
	]}`)

	provider := NewBiteship("biteship_test.key", server.URL, Origin{
		PostalCode:   "55581",
		ContactName:  "Store",
		ContactPhone: "0800000000",
		Address:      "Gudang A",
	})

	rates, err := provider.Rates(RateRequest{
		DestinationPostalCode: "40123",
		WeightGram:            1500,
		Items: []ShipmentItem{
			{Name: "Laptop", Value: 1000, Quantity: 1, WeightGram: 1500},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(rates) != 2 {
		t.Fatalf("rates = %d, want 2", len(rates))
	}
	if rates[0].Courier != "jne" || rates[0].Service != "reg" || rates[0].Price != 36000 {
		t.Errorf("first rate = %+v", rates[0])
	}
	// shipping_fee fallback when price is absent
	if rates[1].Price != 44800 {
		t.Errorf("second rate price = %d, want 44800", rates[1].Price)
	}

	if (*draftBody)["origin_postal_code"] != "55581" {
		t.Errorf("origin = %v", (*draftBody)["origin_postal_code"])
	}
	if (*draftBody)["destination_postal_code"] != "40123" {
		t.Errorf("destination = %v", (*draftBody)["destination_postal_code"])
	}
	if (*draftBody)["delivery_type"] != "now" {
		t.Errorf("delivery_type = %v", (*draftBody)["delivery_type"])
	}

	items, ok := (*draftBody)["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items = %v", (*draftBody)["items"])
	}
	first := items[0].(map[string]any)
	if first["name"] != "Laptop" || first["weight"].(float64) != 1500 {
		t.Errorf("item = %v", first)
	}

	// the temporary draft must be cleaned up
	if !*deleted {
		t.Error("draft order was not deleted after reading rates")
	}
}

func TestBiteshipRatesProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			io.WriteString(w, `{"success":true,"id":"draft-2"}`)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"success":false,"error":"invalid api key"}`)
	}))
	defer server.Close()

	provider := NewBiteship("bad", server.URL, Origin{PostalCode: "55581"})
	if _, err := provider.Rates(RateRequest{DestinationPostalCode: "40123", WeightGram: 1000}); err == nil {
		t.Error("expected an error for a 401 response")
	}
}

func TestBiteshipCreateShipment(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/orders" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&body)

		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"success":true,"id":"bite-1","status":"confirmed","courier":{"waybill_id":"WYB-1","tracking_id":"TRK-1"}}`)
	}))
	defer server.Close()

	provider := NewBiteship("biteship_test.key", server.URL, Origin{
		PostalCode:   "55581",
		ContactName:  "Store",
		ContactPhone: "0800000000",
		Address:      "Gudang A",
	})

	shipment, err := provider.CreateShipment(ShipmentRequest{
		ReferenceID:  "order-uuid",
		ContactName:  "Budi",
		ContactPhone: "0812",
		Address:      "Jl Mawar",
		PostalCode:   "40123",
		Courier:      "jne",
		Service:      "reg",
		Items:        []ShipmentItem{{Name: "Laptop", Value: 1000, Quantity: 2, WeightGram: 1500}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if shipment.ID != "bite-1" || shipment.WaybillID != "WYB-1" || shipment.TrackingID != "TRK-1" {
		t.Errorf("shipment = %+v", shipment)
	}
	if body["reference_id"] != "order-uuid" {
		t.Errorf("reference_id = %v, want the order uuid", body["reference_id"])
	}
	if body["courier_company"] != "jne" || body["courier_type"] != "reg" {
		t.Errorf("courier = %v/%v", body["courier_company"], body["courier_type"])
	}
}
