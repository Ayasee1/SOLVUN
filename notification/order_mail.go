package notification

import (
	"fmt"
	"html"
	"strings"

	"golearn/model"
)

// Minimal design tokens shared with the order page and the invoice.
const (
	colText   = "#18181b"
	colMuted  = "#71717a"
	colLine   = "#e4e4e7"
	colOk     = "#15803d"
	colOkBg   = "#f0fdf4"
	colWait   = "#b45309"
	colWaitBg = "#fffbeb"
)

// OrderReceivedEmail tells the customer their order exists and how to pay.
func OrderReceivedEmail(order model.UserOrder, orderPageURL string) Message {
	body := heading("Order received", "Order "+shortID(order.ID.String())) +
		badge("Waiting for payment", colWait, colWaitBg) +
		divider() +
		items(order) +
		divider() +
		paragraph(fmt.Sprintf("Hi %s, we have your order. Complete the payment to confirm it.", html.EscapeString(order.Username))) +
		button(order.PaymentURL, "Pay now") +
		muted(fmt.Sprintf("Or open this link:<br><span style=\"word-break:break-all\">%s</span>", html.EscapeString(order.PaymentURL))) +
		divider() +
		button(orderPageURL, "View order", true)

	return Message{
		To:      order.Email,
		Subject: fmt.Sprintf("Order %s received — waiting for payment", shortID(order.ID.String())),
		HTML:    layout(body),
	}
}

// OrderPaidEmail confirms the payment and lists the order details.
func OrderPaidEmail(order model.UserOrder, orderPageURL string) Message {
	body := heading("Payment received", "Order "+shortID(order.ID.String())) +
		badge("Paid", colOk, colOkBg) +
		divider() +
		items(order)

	details := paymentRows(order)
	if waybill := waybillRow(order); waybill != "" {
		details = append(details, [2]string{"Waybill", waybill})
	}
	if len(details) > 0 {
		body += divider() + rows(details)
	}

	body += divider() +
		paragraph("We will email you again as soon as your parcel is on the way.") +
		button(orderPageURL, "View order")

	return Message{
		To:      order.Email,
		Subject: fmt.Sprintf("Payment received for order %s", shortID(order.ID.String())),
		HTML:    layout(body),
	}
}

// OrderDeliveredEmail tells the customer the parcel arrived.
func OrderDeliveredEmail(order model.UserOrder, orderPageURL string) Message {
	body := heading("Order delivered", "Order "+shortID(order.ID.String())) +
		badge("Delivered", colOk, colOkBg) +
		divider() +
		paragraph(fmt.Sprintf("Hi %s, your order has been delivered. We hope everything is great.", html.EscapeString(order.Username)))

	if waybill := waybillRow(order); waybill != "" {
		body += rows([][2]string{{"Waybill", waybill}})
	}

	body += divider() + button(orderPageURL, "View order")

	return Message{
		To:      order.Email,
		Subject: fmt.Sprintf("Order %s delivered", shortID(order.ID.String())),
		HTML:    layout(body),
	}
}

// --- building blocks -------------------------------------------------------

func heading(title, subtitle string) string {
	return fmt.Sprintf(`
		<h1 style="margin:0;font-size:18px;font-weight:600;letter-spacing:-0.01em;color:%s">%s</h1>
		<p style="margin:4px 0 14px;font-size:13px;color:%s">%s</p>`,
		colText, html.EscapeString(title), colMuted, html.EscapeString(subtitle))
}

func badge(text, fg, bg string) string {
	return fmt.Sprintf(
		`<span style="display:inline-block;padding:4px 10px;border-radius:999px;font-size:12px;font-weight:600;color:%s;background:%s">%s</span>`,
		fg, bg, html.EscapeString(text))
}

func divider() string {
	return fmt.Sprintf(`<hr style="border:0;border-top:1px solid %s;margin:22px 0">`, colLine)
}

func label(text string) string {
	return fmt.Sprintf(
		`<p style="margin:0 0 10px;font-size:11px;font-weight:600;letter-spacing:.08em;text-transform:uppercase;color:%s">%s</p>`,
		colMuted, html.EscapeString(text))
}

func paragraph(text string) string {
	return fmt.Sprintf(`<p style="margin:0 0 16px;font-size:14px;color:%s">%s</p>`, colText, text)
}

func muted(text string) string {
	return fmt.Sprintf(`<p style="margin:0;font-size:12px;color:%s">%s</p>`, colMuted, text)
}

func rows(pairs [][2]string) string {
	var b strings.Builder
	b.WriteString(`<table style="width:100%;border-collapse:collapse;font-size:13px">`)
	for _, pair := range pairs {
		fmt.Fprintf(&b, `
			<tr>
				<td style="padding:4px 0;color:%s">%s</td>
				<td style="padding:4px 0;text-align:right;color:%s">%s</td>
			</tr>`,
			colMuted, html.EscapeString(pair[0]), colText, html.EscapeString(pair[1]))
	}
	b.WriteString(`</table>`)
	return b.String()
}

func button(url, text string, secondary ...bool) string {
	if url == "" {
		return ""
	}

	style := fmt.Sprintf(
		"display:inline-block;padding:11px 18px;border-radius:8px;font-size:13px;font-weight:600;text-decoration:none;background:%s;color:#ffffff",
		colText)
	if len(secondary) > 0 && secondary[0] {
		style = fmt.Sprintf(
			"display:inline-block;padding:11px 18px;border-radius:8px;font-size:13px;font-weight:600;text-decoration:none;background:#ffffff;color:%s;border:1px solid %s",
			colText, colLine)
	}

	return fmt.Sprintf(`<p style="margin:0 0 16px"><a href="%s" style="%s">%s</a></p>`,
		html.EscapeString(url), style, html.EscapeString(text))
}

func items(order model.UserOrder) string {
	var body strings.Builder
	body.WriteString(label("Items"))
	body.WriteString(`<table style="width:100%;border-collapse:collapse;font-size:13px">`)
	body.WriteString(fmt.Sprintf(`
		<tr>
			<th style="text-align:left;padding:0 0 8px;font-size:11px;font-weight:600;letter-spacing:.06em;text-transform:uppercase;color:%s">Product</th>
			<th style="text-align:right;padding:0 0 8px;font-size:11px;font-weight:600;letter-spacing:.06em;text-transform:uppercase;color:%s">Qty</th>
			<th style="text-align:right;padding:0 0 8px;font-size:11px;font-weight:600;letter-spacing:.06em;text-transform:uppercase;color:%s">Amount</th>
		</tr>`, colMuted, colMuted, colMuted))

	for _, item := range order.OrderItem {
		fmt.Fprintf(&body, `
			<tr>
				<td style="padding:10px 0;border-top:1px solid %s;color:%s">%s</td>
				<td style="padding:10px 0;border-top:1px solid %s;text-align:right;color:%s">%d</td>
				<td style="padding:10px 0;border-top:1px solid %s;text-align:right;color:%s">%s</td>
			</tr>`,
			colLine, colText, html.EscapeString(item.NameItem),
			colLine, colText, item.Qty,
			colLine, colText, rupiah(item.TotalPrice))
	}
	body.WriteString(`</table>`)

	shippingLabel := "Shipping"
	if order.Courier != "" {
		shippingLabel = fmt.Sprintf("Shipping · %s %s", order.Courier, order.CourierService)
	}

	fmt.Fprintf(&body, `
		<table style="width:100%%;border-collapse:collapse;font-size:13px;margin-top:12px">
			<tr>
				<td style="padding:4px 0;color:%s">%s</td>
				<td style="padding:4px 0;text-align:right;color:%s">%s</td>
			</tr>
			<tr>
				<td style="padding:12px 0 0;border-top:1px solid %s;font-size:15px;font-weight:600;color:%s">Total</td>
				<td style="padding:12px 0 0;border-top:1px solid %s;text-align:right;font-size:15px;font-weight:600;color:%s">%s</td>
			</tr>
		</table>`,
		colMuted, html.EscapeString(shippingLabel), colText, rupiah(order.ShippingCost),
		colLine, colText, colLine, colText, rupiah(order.Total))

	return body.String()
}

func paymentRows(order model.UserOrder) [][2]string {
	if order.Payment == nil {
		return nil
	}

	rows := [][2]string{}
	if order.Payment.Type != "" {
		rows = append(rows, [2]string{"Payment method", order.Payment.MethodLabel()})
	}
	if order.Payment.Bank != "" {
		rows = append(rows, [2]string{"Bank", order.Payment.Bank})
	}
	if order.Payment.VANumber != "" {
		rows = append(rows, [2]string{"VA number", order.Payment.VANumber})
	}
	if order.Payment.BillerCode != "" {
		rows = append(rows, [2]string{"Biller code", order.Payment.BillerCode})
	}
	if order.Payment.BillKey != "" {
		rows = append(rows, [2]string{"VA number", order.Payment.BillKey})
	}
	if order.Payment.PaymentCode != "" {
		rows = append(rows, [2]string{"Payment code", order.Payment.PaymentCode})
	}
	if order.Payment.Store != "" {
		rows = append(rows, [2]string{"Store", order.Payment.Store})
	}
	if order.Payment.TransactionID != "" {
		rows = append(rows, [2]string{"Transaction ID", order.Payment.TransactionID})
	}
	if order.Payment.SettledAt != "" {
		rows = append(rows, [2]string{"Settled at", order.Payment.SettledAt})
	}
	return rows
}

func waybillRow(order model.UserOrder) string {
	if order.Shipment == nil {
		return ""
	}
	return order.Shipment.WaybillID
}

func layout(body string) string {
	return fmt.Sprintf(`
		<div style="background:#f4f4f5;padding:40px 20px;font-family:system-ui,-apple-system,'Segoe UI',Roboto,sans-serif">
			<div style="max-width:540px;margin:0 auto">
				<p style="margin:0 0 20px;font-size:12px;font-weight:600;letter-spacing:.12em;text-transform:uppercase;color:%s">Golearn Store</p>
				<div style="background:#ffffff;border:1px solid %s;border-radius:12px;padding:28px">
					%s
				</div>
				<p style="margin:24px 0 0;text-align:center;font-size:12px;color:%s">This is an automated message, please do not reply.</p>
			</div>
		</div>`,
		colMuted, colLine, body, colMuted)
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

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
