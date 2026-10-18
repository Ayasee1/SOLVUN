package shipping

import "testing"

func TestParseFlatRates(t *testing.T) {
	rates, err := ParseFlatRates("Reguler:15000, Express Same Day:25000")
	if err != nil {
		t.Fatal(err)
	}

	if len(rates) != 2 {
		t.Fatalf("rates = %d, want 2", len(rates))
	}
	if rates[0].Service != "reguler" || rates[0].Price != 15000 {
		t.Errorf("first = %+v", rates[0])
	}
	if rates[1].Service != "express-same-day" || rates[1].ServiceName != "Express Same Day" {
		t.Errorf("second = %+v", rates[1])
	}
}

func TestParseFlatRatesRejectsBadInput(t *testing.T) {
	cases := []string{"Reguler", "Reguler:abc", "Reguler:0", "Reguler:-5", " :1000"}
	for _, input := range cases {
		if _, err := ParseFlatRates(input); err == nil {
			t.Errorf("ParseFlatRates(%q) = nil error, want error", input)
		}
	}

	if _, err := ParseFlatRates(""); err == nil {
		t.Error("empty config should be an error")
	}
}

func TestFlatRateProvider(t *testing.T) {
	provider, err := NewFlatRate("Reguler:15000")
	if err != nil {
		t.Fatal(err)
	}

	rates, err := provider.Rates(RateRequest{DestinationPostalCode: "40123", WeightGram: 3000})
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != 1 || rates[0].Price != 15000 {
		t.Errorf("rates = %+v", rates)
	}

	shipment, err := provider.CreateShipment(ShipmentRequest{ReferenceID: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if shipment.ID != "flat-abc" || shipment.Status != "confirmed" {
		t.Errorf("shipment = %+v", shipment)
	}
}
