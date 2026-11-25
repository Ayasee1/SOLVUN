package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Item prices are integers in the smallest currency unit (IDR rupiah).
type Item struct {
	ID         int    `json:"id"`
	NameItem   string `json:"name"`
	Price      int    `json:"price"`
	Qty        int    `json:"qty"`
	TotalPrice int    `json:"total_price"`
	// WeightGram is snapshotted at checkout so the order can be shipped later
	// without re-reading the product catalog.
	WeightGram int `json:"weight_gram"`
}

type Address struct {
	// Recipient is who physically receives the parcel (may differ from the
	// customer who ordered, e.g. when sending a gift).
	RecipientName  string `json:"recipient_name,omitempty"`
	RecipientPhone string `json:"recipient_phone,omitempty"`
	Street         string `json:"street"`
	// Line2 is the apartment, unit or floor (optional).
	Line2     string  `json:"line2,omitempty"`
	City      string  `json:"city"`
	State     string  `json:"state"`
	ZipCode   string  `json:"zip_code"`
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
}

// DeliveryName is the person the courier should look for.
func (o UserOrder) DeliveryName() string {
	if o.Address.RecipientName != "" {
		return o.Address.RecipientName
	}
	return o.Username
}

// DeliveryPhone is the phone the courier should call.
func (o UserOrder) DeliveryPhone() string {
	if o.Address.RecipientPhone != "" {
		return o.Address.RecipientPhone
	}
	return o.Phone
}

// StreetLine joins the street and the optional unit line.
func (a Address) StreetLine() string {
	if a.Line2 == "" {
		return a.Street
	}
	return a.Street + ", " + a.Line2
}

type OrderStatus string

const (
	OrderStatusPending     OrderStatus = "pending"
	OrderStatusPaid        OrderStatus = "paid"
	OrderStatusExpired     OrderStatus = "expired"
	OrderStatusCancelled   OrderStatus = "cancelled"
	OrderStatusNeedsRefund OrderStatus = "needs_refund"
	OrderStatusRefunded    OrderStatus = "refunded"
)

// Valid reports whether s is a known order status.
func (s OrderStatus) Valid() bool {
	switch s {
	case OrderStatusPending, OrderStatusPaid, OrderStatusExpired, OrderStatusCancelled, OrderStatusNeedsRefund, OrderStatusRefunded:
		return true
	}
	return false
}

// PaymentInfo records how the order was paid, filled from the gateway
// notification once the payment is confirmed.
type PaymentInfo struct {
	Type     string `json:"type"`
	Bank     string `json:"bank,omitempty"`
	VANumber string `json:"va_number,omitempty"`
	// Mandiri VA ("echannel") reports a biller code + bill key instead of va_numbers
	BillerCode string `json:"biller_code,omitempty"`
	BillKey    string `json:"bill_key,omitempty"`
	// convenience stores (Alfamart/Indomaret) report a payment code
	Store         string `json:"store,omitempty"`
	PaymentCode   string `json:"payment_code,omitempty"`
	MaskedCard    string `json:"masked_card,omitempty"`
	TransactionID string `json:"transaction_id,omitempty"`
	SettledAt     string `json:"settled_at,omitempty"`
}

// ShipmentInfo is filled once the order is handed to the shipping provider.
type ShipmentInfo struct {
	ProviderID string          `json:"provider_id,omitempty"`
	WaybillID  string          `json:"waybill_id,omitempty"`
	TrackingID string          `json:"tracking_id,omitempty"`
	Status     string          `json:"status,omitempty"`
	History    []ShipmentEvent `json:"history,omitempty"`
}

// RefundInfo records a refund issued through the payment gateway.
type RefundInfo struct {
	// Key is the idempotency key sent to the gateway. Using the order id means a
	// retry can never refund the same payment twice.
	Key        string    `json:"key"`
	Amount     int       `json:"amount"`
	Reason     string    `json:"reason,omitempty"`
	RefundedAt time.Time `json:"refunded_at"`
}

// ShipmentEvent is one courier tracking entry.
type ShipmentEvent struct {
	Status    string `json:"status"`
	Note      string `json:"note,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// MethodLabel turns the gateway's payment_type into something a customer
// understands, e.g. echannel -> "Mandiri Virtual Account".
func (p PaymentInfo) MethodLabel() string {
	switch p.Type {
	case "bank_transfer":
		if p.Bank != "" {
			return "Bank transfer \u00b7 " + strings.ToUpper(p.Bank)
		}
		return "Bank transfer"
	case "echannel":
		return "Mandiri Virtual Account"
	case "cstore":
		if p.Store != "" {
			return titleCase(p.Store)
		}
		return "Convenience store"
	case "qris":
		return "QRIS"
	case "gopay":
		return "GoPay"
	case "shopeepay":
		return "ShopeePay"
	case "credit_card":
		return "Credit card"
	case "akulaku":
		return "Akulaku"
	case "":
		return "-"
	}
	return p.Type
}

func titleCase(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

type UserOrder struct {
	ID        uuid.UUID   `json:"id"`
	Username  string      `json:"username"`
	Email     string      `json:"email"`
	Phone     string      `json:"phone"`
	OrderItem []Item      `json:"order_item"`
	Address   Address     `json:"address"`
	Total     int         `json:"total"`
	Status    OrderStatus `json:"status"`
	// Shipping: the courier is chosen by the customer, but the cost is always
	// recomputed server-side from the shipping provider (never trusted).
	ShippingCost   int           `json:"shipping_cost"`
	Courier        string        `json:"courier,omitempty"`
	CourierService string        `json:"courier_service,omitempty"`
	SnapToken      string        `json:"snap_token,omitempty"`
	PaymentURL     string        `json:"payment_url,omitempty"`
	Payment        *PaymentInfo  `json:"payment,omitempty"`
	Shipment       *ShipmentInfo `json:"shipment,omitempty"`
	Refund         *RefundInfo   `json:"refund,omitempty"`
	// AccessToken is the guest secret that authorises the order page, invoice
	// and cancel endpoint. It is never returned by read APIs.
	AccessToken string `json:"-"`
	// AccessTokenIssued is set only on the create response, so the storefront can
	// build the redirect without a second request.
	AccessTokenIssued string `json:"access_token,omitempty"`
	// ShipmentAttempts counts how many times the API tried to create the provider
	// shipment automatically, so a broken order cannot be retried forever.
	ShipmentAttempts      int        `json:"shipment_attempts,omitempty"`
	ShipmentLastAttemptAt *time.Time `json:"shipment_last_attempt_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
}
