package payment

import "golearn/model"

// ChargeResult is what the frontend needs to open the payment UI.
type ChargeResult struct {
	Token       string
	RedirectURL string
}

// StatusResult is the authoritative payment status from the gateway.
type StatusResult struct {
	TransactionStatus string
	FraudStatus       string

	// amount reported by the gateway, used to verify against the order total
	GrossAmount string

	// payment method details
	PaymentType    string // credit_card, bank_transfer, echannel, qris, gopay, cstore, ...
	Bank           string
	VANumber       string
	BillerCode     string // Mandiri VA ("echannel")
	BillKey        string // Mandiri VA number
	MaskedCard     string
	Store          string
	PaymentCode    string // Alfamart / Indomaret
	TransactionID  string
	SettlementTime string
}

// Gateway abstracts the payment provider so handlers can be tested without
// hitting the real Midtrans API.
type Gateway interface {
	Charge(order model.UserOrder) (*ChargeResult, error)
	CheckStatus(orderID string) (*StatusResult, error)
}
