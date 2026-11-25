package handler

import (
	"context"
	"encoding/json"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
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

type orderPageView struct {
	model.UserOrder
	StoreLat            float64
	StoreLon            float64
	DestinationLat      float64
	DestinationLon      float64
	HasMap              bool
	AutoRefresh         bool
	Badge               string
	Tone                string
	Headline            string
	Subline             string
	Steps               []orderStep
	Events              []orderEvent
	ShipmentStatusLabel string
	InvoiceURL          string
}

type orderStep struct {
	Label string
	State string
}

type orderEvent struct {
	Title string
	Note  string
	When  string
	Tone  string
}

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

	// The id alone is not enough: the page carries contact details and payment
	// credentials, so it requires the guest token from the email or checkout.
	if !h.orderTokenValid(c, order) {
		return c.Status(fiber.StatusNotFound).SendString("This order link is not valid. Open the link from your email, or use the tracking page.")
	}

	// Webhooks cannot reach localhost/tunnels that changed. Reconcile pending
	// orders with Midtrans on page load so the tracking page still catches up.
	if order.Status == model.OrderStatusPending {
		h.ReconcilePending(*order)
		if latest, findErr := h.orderRepo.FindByID(id); findErr == nil {
			order = latest
		}
	}

	var buf strings.Builder
	if err := orderPage.Execute(&buf, h.orderView(order)); err != nil {
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

	// Email verification is proof of ownership, so hand the browser a working
	// link. Older orders have no token yet: issue one now.
	if order.AccessToken == "" {
		token, err := newAccessToken()
		if err != nil {
			return c.Redirect("/track?error=1")
		}
		if err := h.orderRepo.SetAccessToken(order.ID, token); err != nil {
			return c.Redirect("/track?error=1")
		}
		order.AccessToken = token
	}

	return c.Redirect("/order/" + order.ID.String() + "?t=" + url.QueryEscape(order.AccessToken))
}

func (h *OrderHandler) orderView(order *model.UserOrder) orderPageView {
	view := orderPageView{
		UserOrder:  *order,
		StoreLat:   h.storeLat,
		StoreLon:   h.storeLon,
		InvoiceURL: h.invoiceURL(*order),
	}
	view.resolveMapDestination(order)

	stage, badge, tone, headline, subline := describeOrder(order)
	view.Badge = badge
	view.Tone = tone
	view.Headline = headline
	view.Subline = subline
	view.Steps = buildSteps(stage)
	view.Events = buildEvents(order)

	if order.Shipment != nil {
		view.ShipmentStatusLabel = shipmentLabel(order.Shipment.Status)
	}

	view.AutoRefresh = order.Status == model.OrderStatusPending ||
		(order.Status == model.OrderStatusPaid && order.Shipment == nil)

	return view
}

func (v *orderPageView) resolveMapDestination(order *model.UserOrder) {
	if v.StoreLat == 0 || v.StoreLon == 0 || strings.TrimSpace(order.Address.City) == "" {
		return
	}

	destLat, destLon := order.Address.Latitude, order.Address.Longitude
	hasDestination := destLat != 0 && destLon != 0

	if lat, lon, ok := geocodeAddress(fullAddress(order.Address)); ok {
		if !hasDestination || distanceKm(destLat, destLon, lat, lon) > 25 {
			destLat, destLon = lat, lon
			hasDestination = true
		}
	} else if lat, lon, ok := geocodeAddress(fallbackAddress(order.Address)); ok {
		if !hasDestination || distanceKm(destLat, destLon, lat, lon) > 25 {
			destLat, destLon = lat, lon
			hasDestination = true
		}
	}

	if hasDestination {
		v.HasMap = true
		v.DestinationLat = destLat
		v.DestinationLon = destLon
	}
}

func fullAddress(address model.Address) string {
	return joinNonEmpty([]string{
		address.StreetLine(), address.City, address.State, address.ZipCode, "Indonesia",
	}, ", ")
}

func fallbackAddress(address model.Address) string {
	return joinNonEmpty([]string{
		address.City, address.State, address.ZipCode, "Indonesia",
	}, ", ")
}

func joinNonEmpty(parts []string, separator string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return strings.Join(out, separator)
}

type geoPoint struct {
	Lat float64
	Lon float64
	OK  bool
}

var geocodeCache = struct {
	sync.RWMutex
	points map[string]geoPoint
}{
	points: make(map[string]geoPoint),
}

// geocodeAddress resolves a free-text address through Nominatim. Results are
// cached so an order page can be refreshed without hammering the service.
func geocodeAddress(query string) (float64, float64, bool) {
	query = strings.TrimSpace(query)
	if query == "" {
		return 0, 0, false
	}

	geocodeCache.RLock()
	point, found := geocodeCache.points[query]
	geocodeCache.RUnlock()
	if found {
		return point.Lat, point.Lon, point.OK
	}

	point = geoPoint{}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	endpoint := "https://nominatim.openstreetmap.org/search?format=jsonv2&limit=1&countrycodes=id&q=" +
		url.QueryEscape(query)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err == nil {
		request.Header.Set("User-Agent", "SolvunTracking/1.0 (contact@solvun.local)")
		request.Header.Set("Accept", "application/json")

		if response, requestErr := http.DefaultClient.Do(request); requestErr == nil {
			defer response.Body.Close()
			if response.StatusCode == http.StatusOK {
				var rows []struct {
					Lat string `json:"lat"`
					Lon string `json:"lon"`
				}
				if decodeErr := json.NewDecoder(response.Body).Decode(&rows); decodeErr == nil && len(rows) > 0 {
					lat, latErr := strconv.ParseFloat(rows[0].Lat, 64)
					lon, lonErr := strconv.ParseFloat(rows[0].Lon, 64)
					if latErr == nil && lonErr == nil {
						point = geoPoint{Lat: lat, Lon: lon, OK: true}
					}
				}
			}
		}
	}

	geocodeCache.Lock()
	geocodeCache.points[query] = point
	geocodeCache.Unlock()

	return point.Lat, point.Lon, point.OK
}

func distanceKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371.0
	toRadians := func(value float64) float64 { return value * math.Pi / 180 }

	deltaLat := toRadians(lat2 - lat1)
	deltaLon := toRadians(lon2 - lon1)
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(toRadians(lat1))*math.Cos(toRadians(lat2))*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)

	return 2 * earthRadius * math.Asin(math.Min(1, math.Sqrt(a)))
}

func describeOrder(order *model.UserOrder) (stage int, badge, tone, headline, subline string) {
	switch order.Status {
	case model.OrderStatusPending:
		return 1, "Waiting for payment", "wait", "Waiting for payment", "Complete your payment to begin the journey."
	case model.OrderStatusExpired:
		return -1, "Expired", "muted", "Payment expired", "This order was not paid in time."
	case model.OrderStatusCancelled:
		return -1, "Cancelled", "muted", "Order cancelled", "This order has been cancelled."
	case model.OrderStatusNeedsRefund:
		return -1, "Refund required", "bad", "Refund required", "We could not fulfil this order. Our team will contact you."
	}

	if order.Shipment == nil {
		return 2, "Paid", "ok", "Preparing your order", "We are packing your pieces for pickup."
	}

	courier := strings.ToUpper(strings.TrimSpace(order.Courier))
	courierNote := "Your parcel is ready for pickup."
	if courier != "" {
		courierNote = "Your parcel is ready for pickup by " + courier + "."
	}

	switch normalizeShipmentStatus(order.Shipment.Status) {
	case "confirmed", "allocated":
		return 2, "Courier assigned", "ok", "Courier assigned", courierNote
	case "picking_up":
		return 3, "In transit", "ok", "On the way", "The courier is picking up your parcel."
	case "picked":
		return 3, "In transit", "ok", "On the way", "Your parcel is moving through the network."
	case "dropping_off":
		return 3, "Out for delivery", "ok", "Out for delivery", "The courier is heading to your address."
	case "delivered":
		return 4, "Delivered", "ok", "Delivered", "Enjoy your pieces."
	case "cancelled", "rejected", "returned":
		return -1, "Delivery issue", "bad", "Delivery issue", "The courier could not complete this delivery."
	default:
		return 2, "Paid", "ok", "In progress", "Your parcel is being handled by the courier."
	}
}

func buildSteps(stage int) []orderStep {
	labels := []string{"Order placed", "Payment", "Courier", "In transit", "Delivered"}
	steps := make([]orderStep, len(labels))

	for index, label := range labels {
		state := "pending"
		if stage >= 0 {
			switch {
			case index < stage:
				state = "done"
			case index == stage:
				state = "current"
			}
		}
		steps[index] = orderStep{Label: label, State: state}
	}

	return steps
}

func buildEvents(order *model.UserOrder) []orderEvent {
	events := []orderEvent{{
		Title: "Order placed",
		When:  humanDate(order.CreatedAt),
		Tone:  "done",
	}}

	if order.Payment != nil {
		events = append(events, orderEvent{
			Title: "Payment confirmed",
			Note:  order.Payment.MethodLabel(),
			When:  order.Payment.SettledAt,
			Tone:  "done",
		})
	}

	if order.Shipment != nil {
		if len(order.Shipment.History) > 0 {
			for index, event := range order.Shipment.History {
				tone := "done"
				if index == 0 {
					tone = "current"
				}
				events = append(events, orderEvent{
					Title: shipmentLabel(event.Status),
					Note:  event.Note,
					When:  event.UpdatedAt,
					Tone:  tone,
				})
			}
		} else {
			note := ""
			if order.Courier != "" {
				note = "Courier " + order.Courier + " " + order.CourierService
			}
			events = append(events, orderEvent{
				Title: shipmentLabel(order.Shipment.Status),
				Note:  note,
				Tone:  "current",
			})
		}
	} else {
		switch order.Status {
		case model.OrderStatusPending:
			events = append(events, orderEvent{Title: "Awaiting payment", Tone: "current"})
		case model.OrderStatusPaid:
			events = append(events, orderEvent{Title: "Preparing your order", Tone: "current"})
		}
	}

	switch order.Status {
	case model.OrderStatusExpired:
		events = append(events, orderEvent{Title: "Payment expired", Tone: "muted"})
	case model.OrderStatusCancelled:
		events = append(events, orderEvent{Title: "Order cancelled", Tone: "muted"})
	case model.OrderStatusNeedsRefund:
		events = append(events, orderEvent{Title: "Refund required", Tone: "bad"})
	}

	return events
}

func normalizeShipmentStatus(status string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(status)), " ", "_")
}

func shipmentLabel(status string) string {
	normalized := normalizeShipmentStatus(status)

	switch normalized {
	case "":
		return "—"
	case "confirmed", "allocated":
		return "Courier assigned"
	case "picking_up":
		return "Picking up"
	case "picked":
		return "Picked up"
	case "dropping_off":
		return "On the way"
	case "out_for_delivery":
		return "Out for delivery"
	case "delivered":
		return "Delivered"
	case "cancelled":
		return "Cancelled"
	case "rejected":
		return "Rejected"
	case "returned":
		return "Returned"
	}

	if normalized == "" {
		return normalized
	}
	return strings.ToUpper(normalized[:1]) + normalized[1:]
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
