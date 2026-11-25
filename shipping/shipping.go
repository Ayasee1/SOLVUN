// Package shipping abstracts the logistics provider (Biteship today, any other
// aggregator tomorrow) behind one interface, the same pattern used for payment
// and email so providers can be swapped without touching handlers.
package shipping

import (
	"errors"
	"regexp"
)

// ErrNotConfigured is returned when no shipping provider is configured.
var ErrNotConfigured = errors.New("shipping provider is not configured")

// providerIDPattern is the shape of an id a provider may issue. It is also what
// keeps webhook-supplied ids from being interpolated into a URL path as
// traversal or query syntax.
var providerIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

// ValidProviderID reports whether id is safe to use in a provider URL.
func ValidProviderID(id string) bool {
	return providerIDPattern.MatchString(id)
}

// Origin describes the store warehouse used as the shipment origin.
type Origin struct {
	PostalCode   string
	Lat          float64
	Lon          float64
	ContactName  string
	ContactPhone string
	Address      string
	// CollectionMethod is how the courier gets the parcel: "pickup" (courier
	// comes to the store) or "drop_off". Not every courier supports drop_off.
	CollectionMethod string
}

// RateRequest asks for shipping options for a destination and cart.
type RateRequest struct {
	DestinationPostalCode string
	DestinationLat        float64
	DestinationLon        float64
	// WeightGram is the total package weight in grams (the app's canonical unit).
	WeightGram int
	// Items describe the cart; providers that need item details use them.
	Items    []ShipmentItem
	Couriers []string
}

// Rate is one courier service option.
type Rate struct {
	Courier     string `json:"courier"`      // courier code, e.g. "jne"
	CourierName string `json:"courier_name"` // display name, e.g. "JNE"
	Service     string `json:"service"`      // service code, e.g. "reg"
	ServiceName string `json:"service_name"` // e.g. "Reguler"
	Description string `json:"description,omitempty"`
	Duration    string `json:"duration,omitempty"`
	Price       int    `json:"price"`
}

// ShipmentItem is one line sent to the provider.
type ShipmentItem struct {
	Name       string
	Value      int // declared value in rupiah
	Quantity   int
	WeightGram int
}

// ShipmentRequest creates a real shipment at the provider.
type ShipmentRequest struct {
	ReferenceID  string // our order UUID, used for correlation
	ContactName  string
	ContactPhone string
	ContactEmail string
	Address      string
	PostalCode   string
	Lat          float64
	Lon          float64
	Courier      string
	Service      string
	Insurance    int // declared value to insure, 0 disables
	Items        []ShipmentItem
	Note         string
}

// ShipmentEvent is one entry of the courier tracking history.
type ShipmentEvent struct {
	Status    string `json:"status"`
	Note      string `json:"note,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// Shipment is the provider's created shipment / current tracking state.
type Shipment struct {
	ID         string          `json:"id"`
	WaybillID  string          `json:"waybill_id,omitempty"`
	TrackingID string          `json:"tracking_id,omitempty"`
	Status     string          `json:"status,omitempty"`
	History    []ShipmentEvent `json:"history,omitempty"`
}

// Provider is the contract every shipping provider must satisfy.
type Provider interface {
	Rates(req RateRequest) ([]Rate, error)
	CreateShipment(req ShipmentRequest) (*Shipment, error)
	// Tracking returns the current provider state for a provider order id.
	Tracking(providerOrderID string) (*Shipment, error)
}

// Disabled is used when no provider is configured, so handlers can keep a
// single code path and map ErrNotConfigured to a friendly 503.
type Disabled struct{}

var _ Provider = Disabled{}

func (Disabled) Rates(RateRequest) ([]Rate, error) { return nil, ErrNotConfigured }

func (Disabled) CreateShipment(ShipmentRequest) (*Shipment, error) {
	return nil, ErrNotConfigured
}

func (Disabled) Tracking(string) (*Shipment, error) { return nil, ErrNotConfigured }
