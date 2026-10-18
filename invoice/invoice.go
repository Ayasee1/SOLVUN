// Package invoice renders an order as a PDF invoice using Maroto.
package invoice

import (
	"fmt"
	"strings"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/line"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/linestyle"
	"github.com/johnfercher/maroto/v2/pkg/props"

	"golearn/model"
)

// Design tokens shared with the order page and the emails.
var (
	textColor  = &props.Color{Red: 24, Green: 24, Blue: 27}
	mutedColor = &props.Color{Red: 113, Green: 113, Blue: 122}
	lineColor  = &props.Color{Red: 228, Green: 228, Blue: 231}
	okColor    = &props.Color{Red: 21, Green: 128, Blue: 61}
	waitColor  = &props.Color{Red: 180, Green: 83, Blue: 9}
)

var (
	brand      = props.Text{Size: 9, Style: fontstyle.Bold, Color: mutedColor}
	docLabel   = props.Text{Size: 9, Style: fontstyle.Bold, Color: mutedColor, Align: align.Right}
	section    = props.Text{Size: 9, Style: fontstyle.Bold, Color: mutedColor}
	title      = props.Text{Size: 16, Style: fontstyle.Bold, Color: textColor}
	body       = props.Text{Size: 9, Color: textColor}
	muted      = props.Text{Size: 9, Color: mutedColor}
	rightBody  = props.Text{Size: 9, Color: textColor, Align: align.Right}
	rightMuted = props.Text{Size: 9, Color: mutedColor, Align: align.Right}
	totalStyle = props.Text{Size: 11, Style: fontstyle.Bold, Color: textColor, Align: align.Right}
	footer     = props.Text{Size: 8, Color: mutedColor, Align: align.Center}
	divider    = props.Line{Style: linestyle.Solid, Thickness: 0.25, Color: lineColor}
)

// Generate returns the invoice PDF bytes for the given order.
func Generate(order model.UserOrder) ([]byte, error) {
	cfg := config.NewBuilder().Build()
	doc := maroto.NewMetricsDecorator(maroto.New(cfg))

	// header: brand + document label
	doc.AddRow(6,
		text.NewCol(6, "GOLEARN STORE", brand),
		text.NewCol(6, "INVOICE", docLabel),
	)

	// order reference + date
	doc.AddRow(10,
		text.NewCol(6, "Order "+shortID(order.ID.String()), title),
		text.NewCol(6, order.CreatedAt.Format("02 Jan 2006 15:04"), props.Text{Size: 9, Color: mutedColor, Align: align.Right}),
	)

	// status
	statusColor := textColor
	switch order.Status {
	case model.OrderStatusPaid:
		statusColor = okColor
	case model.OrderStatusPending:
		statusColor = waitColor
	}
	doc.AddRow(6, text.NewCol(12, strings.ToUpper(statusLabel(order.Status)), props.Text{Size: 9, Style: fontstyle.Bold, Color: statusColor}))

	doc.AddRow(4, line.NewCol(12, divider))

	// items
	doc.AddRow(8,
		text.NewCol(6, "PRODUCT", section),
		text.NewCol(1, "QTY", props.Text{Size: 9, Style: fontstyle.Bold, Color: mutedColor, Align: align.Right}),
		text.NewCol(2, "PRICE", props.Text{Size: 9, Style: fontstyle.Bold, Color: mutedColor, Align: align.Right}),
		text.NewCol(3, "AMOUNT", props.Text{Size: 9, Style: fontstyle.Bold, Color: mutedColor, Align: align.Right}),
	)

	for _, item := range order.OrderItem {
		doc.AddRow(7,
			text.NewCol(6, item.NameItem, body),
			text.NewCol(1, fmt.Sprintf("%d", item.Qty), rightBody),
			text.NewCol(2, rupiah(item.Price), rightMuted),
			text.NewCol(3, rupiah(item.TotalPrice), rightBody),
		)
	}

	doc.AddRow(4, line.NewCol(12, divider))

	// totals
	shippingLabel := "Shipping"
	if order.Courier != "" {
		shippingLabel = fmt.Sprintf("Shipping · %s %s", order.Courier, order.CourierService)
	}
	doc.AddRow(6,
		text.NewCol(9, shippingLabel, rightMuted),
		text.NewCol(3, rupiah(order.ShippingCost), rightBody),
	)
	doc.AddRow(8,
		text.NewCol(9, "Total", totalStyle),
		text.NewCol(3, rupiah(order.Total), totalStyle),
	)

	// payment
	if order.Payment != nil {
		doc.AddRow(4, line.NewCol(12, divider))
		doc.AddRow(7, text.NewCol(12, "PAYMENT", section))

		if order.Payment.Type != "" {
			doc.AddRow(6, text.NewCol(6, "Method", muted), text.NewCol(6, order.Payment.MethodLabel(), rightBody))
		}
		if order.Payment.Bank != "" {
			doc.AddRow(6, text.NewCol(6, "Bank", muted), text.NewCol(6, order.Payment.Bank, rightBody))
		}
		if order.Payment.VANumber != "" {
			doc.AddRow(6, text.NewCol(6, "VA number", muted), text.NewCol(6, order.Payment.VANumber, rightBody))
		}
		if order.Payment.BillerCode != "" {
			doc.AddRow(6, text.NewCol(6, "Biller code", muted), text.NewCol(6, order.Payment.BillerCode, rightBody))
		}
		if order.Payment.BillKey != "" {
			doc.AddRow(6, text.NewCol(6, "VA number", muted), text.NewCol(6, order.Payment.BillKey, rightBody))
		}
		if order.Payment.PaymentCode != "" {
			doc.AddRow(6, text.NewCol(6, "Payment code", muted), text.NewCol(6, order.Payment.PaymentCode, rightBody))
		}
		if order.Payment.TransactionID != "" {
			doc.AddRow(6, text.NewCol(6, "Transaction ID", muted), text.NewCol(6, order.Payment.TransactionID, rightBody))
		}
		if order.Payment.SettledAt != "" {
			doc.AddRow(6, text.NewCol(6, "Settled at", muted), text.NewCol(6, order.Payment.SettledAt, rightBody))
		}
	}

	// shipment
	if order.Shipment != nil && order.Shipment.WaybillID != "" {
		doc.AddRow(4, line.NewCol(12, divider))
		doc.AddRow(7, text.NewCol(12, "SHIPMENT", section))
		doc.AddRow(6, text.NewCol(6, "Waybill", muted), text.NewCol(6, order.Shipment.WaybillID, rightBody))
		doc.AddRow(6, text.NewCol(6, "Status", muted), text.NewCol(6, order.Shipment.Status, rightBody))
	}

	// shipping address
	doc.AddRow(4, line.NewCol(12, divider))
	doc.AddRow(7, text.NewCol(12, "SHIPPING ADDRESS", section))
	doc.AddRow(6, text.NewCol(12, order.DeliveryName(), props.Text{Size: 9, Style: fontstyle.Bold, Color: textColor}))
	doc.AddRow(6, text.NewCol(12, order.Address.StreetLine(), muted))
	doc.AddRow(6, text.NewCol(12, fmt.Sprintf("%s, %s %s", order.Address.City, order.Address.State, order.Address.ZipCode), muted))
	doc.AddRow(6, text.NewCol(12, order.DeliveryPhone(), muted))

	if order.Status == model.OrderStatusPaid && order.Payment != nil && order.Payment.SettledAt != "" {
		doc.AddRow(6, text.NewCol(12, "PAID ON "+order.Payment.SettledAt, props.Text{Size: 9, Style: fontstyle.Bold, Color: okColor}))
	}

	doc.AddRow(10, text.NewCol(12, "This document is your proof of purchase.", footer))
	doc.AddRow(5, text.NewCol(12, "Thank you for your purchase.", footer))

	document, err := doc.Generate()
	if err != nil {
		return nil, err
	}
	return document.GetBytes(), nil
}

func statusLabel(status model.OrderStatus) string {
	switch status {
	case model.OrderStatusPending:
		return "waiting for payment"
	case model.OrderStatusPaid:
		return "paid"
	case model.OrderStatusExpired:
		return "expired"
	case model.OrderStatusCancelled:
		return "cancelled"
	case model.OrderStatusNeedsRefund:
		return "refund required"
	}
	return string(status)
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func rupiah(amount int) string {
	digits := fmt.Sprintf("%d", amount)
	var parts []string
	for len(digits) > 3 {
		parts = append([]string{digits[len(digits)-3:]}, parts...)
		digits = digits[:len(digits)-3]
	}
	parts = append([]string{digits}, parts...)
	return "Rp" + strings.Join(parts, ".")
}
