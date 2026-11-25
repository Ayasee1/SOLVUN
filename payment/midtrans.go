package payment

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"github.com/midtrans/midtrans-go/snap"

	"golearn/model"
)

// MidtransGateway implements Gateway using Midtrans Snap (payment UI) and
// Core API (status check).
type MidtransGateway struct {
	snap          snap.Client
	core          coreapi.Client
	expiryMinutes int
	storeBaseURL  string
}

func NewMidtransGateway(serverKey, env string, expiryMinutes int, storeBaseURL string) *MidtransGateway {
	e := midtrans.Sandbox
	if env == "production" {
		e = midtrans.Production
	}

	var s snap.Client
	s.New(serverKey, e)
	quietLogger(s.HttpClient)

	var c coreapi.Client
	c.New(serverKey, e)
	quietLogger(c.HttpClient)

	return &MidtransGateway{snap: s, core: c, expiryMinutes: expiryMinutes, storeBaseURL: storeBaseURL}
}

// quietLogger stops the SDK from logging request headers. In sandbox the SDK
// hardcodes LogDebug, which prints the Authorization header containing the
// base64 server key.
func quietLogger(client midtrans.HttpClient) {
	if hc, ok := client.(*midtrans.HttpClientImplementation); ok {
		hc.Logger = &midtrans.LoggerImplementation{LogLevel: midtrans.LogError}
	}
}

func (g *MidtransGateway) Charge(order model.UserOrder) (*ChargeResult, error) {
	items := make([]midtrans.ItemDetails, 0, len(order.OrderItem)+1)
	for _, item := range order.OrderItem {
		items = append(items, midtrans.ItemDetails{
			ID:    strconv.Itoa(item.ID),
			Name:  item.NameItem,
			Price: int64(item.Price),
			Qty:   int32(item.Qty),
		})
	}

	// Midtrans requires gross_amount to equal the sum of item_details, so the
	// shipping cost must be part of the items too.
	if order.ShippingCost > 0 {
		name := "Shipping"
		if order.Courier != "" {
			name = fmt.Sprintf("Shipping (%s %s)", order.Courier, order.CourierService)
		}
		items = append(items, midtrans.ItemDetails{
			ID:    "shipping",
			Name:  name,
			Price: int64(order.ShippingCost),
			Qty:   1,
		})
	}

	req := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  order.ID.String(),
			GrossAmt: int64(order.Total),
		},
		CustomerDetail: &midtrans.CustomerDetails{
			FName: order.Username,
			Email: order.Email,
			Phone: order.Phone,
		},
		Items: &items,
	}
	if g.expiryMinutes > 0 {
		req.Expiry = &snap.ExpiryDetails{Unit: "minute", Duration: int64(g.expiryMinutes)}
	}
	if g.storeBaseURL != "" {
		// send the customer back to our own order page after paying
		req.Callbacks = &snap.Callbacks{
			Finish: g.storeBaseURL + "/order/" + order.ID.String(),
		}
	}

	resp, mErr := g.snap.CreateTransaction(req)
	if mErr != nil {
		return nil, errors.New(mErr.GetMessage())
	}

	return &ChargeResult{
		Token:       resp.Token,
		RedirectURL: resp.RedirectURL,
	}, nil
}

// Refund returns the money for a captured payment. The refund key is the order
// id, which makes the call idempotent: Midtrans will not refund the same payment
// twice if the request is retried.
func (g *MidtransGateway) Refund(orderID string, amount int, reason string) error {
	_, mErr := g.core.RefundTransaction(orderID, &coreapi.RefundReq{
		RefundKey: orderID,
		Amount:    int64(amount),
		Reason:    reason,
	})
	if mErr != nil {
		return errors.New(mErr.GetMessage())
	}
	return nil
}

func (g *MidtransGateway) CheckStatus(orderID string) (*StatusResult, error) {
	resp, mErr := g.core.CheckTransaction(orderID)
	if mErr != nil {
		return nil, errors.New(mErr.GetMessage())
	}

	// Permata reports its VA in a dedicated field; other banks use va_numbers.
	bank := resp.Bank
	vaNumber := resp.PermataVaNumber
	if len(resp.VaNumbers) > 0 {
		if bank == "" {
			bank = resp.VaNumbers[0].Bank
		}
		if vaNumber == "" {
			vaNumber = resp.VaNumbers[0].VANumber
		}
	}

	return &StatusResult{
		TransactionStatus: resp.TransactionStatus,
		FraudStatus:       resp.FraudStatus,
		GrossAmount:       resp.GrossAmount,
		PaymentType:       resp.PaymentType,
		Bank:              bank,
		VANumber:          vaNumber,
		BillerCode:        resp.BillerCode,
		BillKey:           resp.BillKey,
		MaskedCard:        resp.MaskedCard,
		Store:             resp.Store,
		PaymentCode:       resp.PaymentCode,
		TransactionID:     resp.TransactionID,
		SettlementTime:    resp.SettlementTime,
	}, nil
}
