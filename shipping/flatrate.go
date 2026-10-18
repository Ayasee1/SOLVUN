package shipping

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// FlatRate is a development provider that returns fixed shipping options from
// config, so the checkout flow can be built and tested without calling a paid
// provider. Swap it for Biteship by changing SHIPPING_PROVIDER.
type FlatRate struct {
	options []Rate
}

var _ Provider = (*FlatRate)(nil)

// NewFlatRate parses "Reguler:15000,Express:25000" into shipping options.
func NewFlatRate(raw string) (*FlatRate, error) {
	options, err := ParseFlatRates(raw)
	if err != nil {
		return nil, err
	}
	return &FlatRate{options: options}, nil
}

func ParseFlatRates(raw string) ([]Rate, error) {
	rates := make([]Rate, 0)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		name, priceText, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("invalid flat rate %q, want name:price", part)
		}

		price, err := strconv.Atoi(strings.TrimSpace(priceText))
		if err != nil || price <= 0 {
			return nil, fmt.Errorf("invalid price in flat rate %q", part)
		}

		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("empty name in flat rate %q", part)
		}

		rates = append(rates, Rate{
			Courier:     "flat",
			CourierName: "Flat Rate",
			Service:     slug(name),
			ServiceName: name,
			Description: "Development flat rate",
			Duration:    "-",
			Price:       price,
		})
	}

	if len(rates) == 0 {
		return nil, errors.New("no flat rates configured")
	}
	return rates, nil
}

func (f *FlatRate) Rates(RateRequest) ([]Rate, error) {
	return f.options, nil
}

// CreateShipment returns a synthetic shipment (no waybill), clearly marked as
// development data with a "flat-" prefix.
func (f *FlatRate) CreateShipment(req ShipmentRequest) (*Shipment, error) {
	return &Shipment{
		ID:     "flat-" + req.ReferenceID,
		Status: "confirmed",
	}, nil
}

// Tracking returns a synthetic state for development.
func (f *FlatRate) Tracking(providerOrderID string) (*Shipment, error) {
	return &Shipment{
		ID:     providerOrderID,
		Status: "confirmed",
		History: []ShipmentEvent{
			{Status: "confirmed", Note: "Development flat rate shipment"},
		},
	}, nil
}

func slug(value string) string {
	return strings.ReplaceAll(strings.ToLower(value), " ", "-")
}
