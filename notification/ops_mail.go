package notification

import (
	"fmt"
	"html"
	"strings"
	"time"

	"golearn/model"
)

// OpsDigestEmail summarises the orders that need a human: payments to refund and
// paid orders whose shipment was never created.
func OpsDigestEmail(to string, refunds, unshipped []model.UserOrder, now time.Time) Message {
	body := heading("Orders that need attention", now.Format("02 Jan 2006 15:04"))
	body += paragraph(fmt.Sprintf(
		"%d refund(s) to issue and %d paid order(s) without a shipment.",
		len(refunds), len(unshipped)))

	if len(refunds) > 0 {
		body += divider() + label("Needs refund")
		body += orderTable(refunds, func(order model.UserOrder) [3]string {
			method := "-"
			if order.Payment != nil {
				method = order.Payment.MethodLabel()
				if order.Payment.TransactionID != "" {
					method += " · " + order.Payment.TransactionID
				}
			}
			return [3]string{shortID(order.ID.String()), rupiah(order.Total), method}
		})
	}

	if len(unshipped) > 0 {
		body += divider() + label("Paid, no shipment")
		body += orderTable(unshipped, func(order model.UserOrder) [3]string {
			courier := strings.TrimSpace(order.Courier + " " + order.CourierService)
			return [3]string{shortID(order.ID.String()), rupiah(order.Total), courier}
		})
	}

	return Message{
		To:      to,
		Subject: fmt.Sprintf("%d order(s) need attention", len(refunds)+len(unshipped)),
		HTML:    layout(body),
	}
}

func orderTable(orders []model.UserOrder, cells func(model.UserOrder) [3]string) string {
	var b strings.Builder
	b.WriteString(`<table style="width:100%;border-collapse:collapse;font-size:13px">`)
	fmt.Fprintf(&b, `
		<tr>
			<th style="text-align:left;padding:0 0 8px;font-size:11px;font-weight:600;letter-spacing:.06em;text-transform:uppercase;color:%s">Order</th>
			<th style="text-align:right;padding:0 0 8px;font-size:11px;font-weight:600;letter-spacing:.06em;text-transform:uppercase;color:%s">Total</th>
			<th style="text-align:right;padding:0 0 8px;font-size:11px;font-weight:600;letter-spacing:.06em;text-transform:uppercase;color:%s">Details</th>
		</tr>`, colMuted, colMuted, colMuted)

	for _, order := range orders {
		cell := cells(order)
		fmt.Fprintf(&b, `
			<tr>
				<td style="padding:8px 0;border-top:1px solid %s;font-family:ui-monospace,Menlo,monospace;color:%s">%s</td>
				<td style="padding:8px 0;border-top:1px solid %s;text-align:right;color:%s">%s</td>
				<td style="padding:8px 0;border-top:1px solid %s;text-align:right;color:%s">%s</td>
			</tr>`,
			colLine, colText, html.EscapeString(cell[0]),
			colLine, colText, html.EscapeString(cell[1]),
			colLine, colText, html.EscapeString(cell[2]))
	}

	b.WriteString(`</table>`)
	return b.String()
}
