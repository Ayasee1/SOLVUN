package handler

import (
	"html/template"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"golearn/model"
)

var orderPage = template.Must(template.New("order").Funcs(template.FuncMap{
	"rupiah":      rupiahID,
	"short":       shortID,
	"statusLabel": statusLabel,
	"statusClass": statusClass,
	"date":        humanDate,
}).Parse(orderPageHTML))

var trackPage = template.Must(template.New("track").Parse(trackPageHTML))

// OrderPageView renders the public order status page. It is what the customer
// lands on after paying (Snap finish redirect) and what the emails link to.
func (h *OrderHandler) OrderPageView(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid order id")
	}

	order, err := h.orderRepo.FindByID(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Order not found")
	}

	var buf strings.Builder
	if err := orderPage.Execute(&buf, order); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to render page")
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(buf.String())
}

// TrackPageView renders the "find my order" form (GET) and handles its submit
// (POST). Guests have no account, so the order id plus the email on the order
// act as the proof of ownership.
func (h *OrderHandler) TrackPageView(c *fiber.Ctx) error {
	if c.Method() == fiber.MethodPost {
		return h.trackOrder(c)
	}

	c.Set("Content-Type", "text/html; charset=utf-8")
	return trackPage.Execute(c.Response().BodyWriter(), fiber.Map{
		"Error": c.Query("error") != "",
	})
}

func (h *OrderHandler) trackOrder(c *fiber.Ctx) error {
	var form struct {
		OrderID string `form:"order_id"`
		Email   string `form:"email"`
	}
	if err := c.BodyParser(&form); err != nil {
		return c.Redirect("/track?error=1")
	}

	id, err := uuid.Parse(strings.TrimSpace(form.OrderID))
	if err != nil {
		return c.Redirect("/track?error=1")
	}

	order, err := h.orderRepo.FindByID(id)
	if err != nil || !strings.EqualFold(strings.TrimSpace(order.Email), strings.TrimSpace(form.Email)) {
		return c.Redirect("/track?error=1")
	}

	return c.Redirect("/order/" + order.ID.String())
}

func rupiahID(amount int) string {
	digits := strconv.Itoa(amount)
	negative := strings.HasPrefix(digits, "-")
	if negative {
		digits = digits[1:]
	}

	var parts []string
	for len(digits) > 3 {
		parts = append([]string{digits[len(digits)-3:]}, parts...)
		digits = digits[:len(digits)-3]
	}
	parts = append([]string{digits}, parts...)

	prefix := "Rp"
	if negative {
		prefix = "-Rp"
	}
	return prefix + strings.Join(parts, ".")
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func statusLabel(status model.OrderStatus) string {
	switch status {
	case model.OrderStatusPending:
		return "Waiting for payment"
	case model.OrderStatusPaid:
		return "Paid"
	case model.OrderStatusExpired:
		return "Expired"
	case model.OrderStatusCancelled:
		return "Cancelled"
	case model.OrderStatusNeedsRefund:
		return "Refund required"
	}
	return string(status)
}

func statusClass(status model.OrderStatus) string {
	switch status {
	case model.OrderStatusPaid:
		return "ok"
	case model.OrderStatusNeedsRefund:
		return "bad"
	case model.OrderStatusPending:
		return "wait"
	}
	return "muted"
}

func humanDate(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.Format("02 Jan 2006 15:04")
}
