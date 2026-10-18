package handler

import (
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"golearn/invoice"
	"golearn/model"
	"golearn/notification"
	"golearn/payment"
	"golearn/repository"
	"golearn/shipping"
)

// errShippingUnavailable is returned when the requested courier/service is not
// offered by the provider for this destination.
var errShippingUnavailable = errors.New("shipping option is not available")

type OrderHandler struct {
	productRepo  repository.ProductRepository
	orderRepo    repository.OrderRepository
	gateway      payment.Gateway
	mailer       notification.Sender
	shipping     shipping.Provider
	storeBaseURL string
}

func NewOrderHandler(productRepo repository.ProductRepository, orderRepo repository.OrderRepository, gateway payment.Gateway, mailer notification.Sender, shippingProvider shipping.Provider, storeBaseURL string) *OrderHandler {
	return &OrderHandler{
		productRepo:  productRepo,
		orderRepo:    orderRepo,
		gateway:      gateway,
		mailer:       mailer,
		shipping:     shippingProvider,
		storeBaseURL: storeBaseURL,
	}
}

// orderPageURL is the public page the customer can open to follow the order.
func (h *OrderHandler) orderPageURL(id uuid.UUID) string {
	return h.storeBaseURL + "/order/" + id.String()
}

// createOrderRequest is the explicit input accepted from the client. Notably
// there is no shipping_cost, price or total: those are always computed here.
type createOrderRequest struct {
	Username       string        `json:"username"`
	Email          string        `json:"email"`
	Phone          string        `json:"phone"`
	Address        model.Address `json:"address"`
	OrderItem      []model.Item  `json:"order_item"`
	Courier        string        `json:"courier"`
	CourierService string        `json:"courier_service"`
}

// create order handler
func (h *OrderHandler) CreateOrder(c *fiber.Ctx) error {
	var req createOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	order := model.UserOrder{
		Username:  req.Username,
		Email:     req.Email,
		Phone:     req.Phone,
		Address:   req.Address,
		OrderItem: req.OrderItem,
	}

	if len(order.OrderItem) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Order item is empty",
		})
	}

	if order.Username == "" || order.Email == "" || order.Phone == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username, email or phone is empty",
		})
	}

	if order.Address.Street == "" || order.Address.City == "" || order.Address.State == "" || order.Address.ZipCode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Address is empty",
		})
	}

	// coordinates are optional: shipping is quoted from the postal code
	// validate every item and compute prices from the catalog (never trust the frontend)
	requested := map[int]int{}
	shipmentItems := make([]shipping.ShipmentItem, 0, len(order.OrderItem))
	total := 0
	weight := 0
	for idx := range order.OrderItem {
		item := &order.OrderItem[idx]

		product, err := h.productRepo.FindByID(item.ID)
		if err != nil {
			if errors.Is(err, repository.ErrProductNotFound) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": fmt.Sprintf("Product with id %d not found", item.ID),
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to load product",
			})
		}
		if item.Qty <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Invalid quantity for product %s", product.Name),
			})
		}

		item.NameItem = product.Name
		item.Price = product.Price
		item.TotalPrice = product.Price * item.Qty
		item.WeightGram = product.WeightGram
		requested[item.ID] += item.Qty
		total += item.TotalPrice
		weight += product.WeightGram * item.Qty
		shipmentItems = append(shipmentItems, shipping.ShipmentItem{
			Name:       product.Name,
			Value:      product.Price,
			Quantity:   item.Qty,
			WeightGram: product.WeightGram,
		})
	}

	// the courier is the customer's choice, but the cost always comes from the
	// provider (re-fetched here), so a tampered price is impossible.
	if req.Courier != "" {
		rate, err := h.resolveShipping(order.Address, weight, shipmentItems, req.Courier, req.CourierService)
		if err != nil {
			if errors.Is(err, errShippingUnavailable) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": err.Error(),
				})
			}
			log.Printf("shipping lookup failed for %s: %v", order.Address.ZipCode, err)
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
				"error": "Failed to calculate shipping",
			})
		}

		order.Courier = rate.Courier
		order.CourierService = rate.Service
		order.ShippingCost = rate.Price
		total += rate.Price
	}

	// check and decrement stock atomically in the repository
	if err := h.productRepo.DecreaseStock(requested); err != nil {
		if errors.Is(err, repository.ErrProductNotFound) || errors.Is(err, repository.ErrNotEnoughStock) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to reserve stock",
		})
	}

	order.ID = uuid.New()
	order.Total = total
	order.Status = model.OrderStatusPending
	order.CreatedAt = time.Now()

	// start the payment session; the order stays "pending" until the gateway
	// confirms it through the notification webhook.
	charge, err := h.gateway.Charge(order)
	if err != nil {
		log.Printf("payment charge failed for order %s: %v", order.ID, err)
		// payment could not be started -> give the reserved stock back
		_ = h.productRepo.ReleaseStock(requested)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "Failed to create payment",
		})
	}
	order.SnapToken = charge.Token
	order.PaymentURL = charge.RedirectURL

	if err := h.orderRepo.Save(order); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save order",
		})
	}

	// email failures must not break the request
	if err := h.mailer.Send(notification.OrderReceivedEmail(order, h.orderPageURL(order.ID))); err != nil {
		log.Printf("failed to send order email for %s: %v", order.ID, err)
	}

	return c.Status(fiber.StatusCreated).JSON(order)
}

// resolveShipping asks the provider for options and returns the one the
// customer picked, or errShippingUnavailable when it is not offered.
func (h *OrderHandler) resolveShipping(address model.Address, weight int, items []shipping.ShipmentItem, courier, service string) (shipping.Rate, error) {
	rates, err := h.shipping.Rates(shipping.RateRequest{
		DestinationPostalCode: address.ZipCode,
		DestinationLat:        address.Latitude,
		DestinationLon:        address.Longitude,
		WeightGram:            weight,
		Items:                 items,
	})
	if err != nil {
		return shipping.Rate{}, err
	}

	for _, rate := range rates {
		if rate.Courier == courier && rate.Service == service {
			return rate, nil
		}
	}

	return shipping.Rate{}, fmt.Errorf("%w: %s/%s", errShippingUnavailable, courier, service)
}

// Notification is the Midtrans webhook. Midtrans always re-checks the real
// status itself, so we never trust the payload body beyond the order id.
func (h *OrderHandler) Notification(c *fiber.Ctx) error {
	var payload struct {
		OrderID string `json:"order_id"`
	}
	if err := c.BodyParser(&payload); err != nil || payload.OrderID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid notification",
		})
	}

	id, err := uuid.Parse(payload.OrderID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid order id",
		})
	}

	status, err := h.gateway.CheckStatus(payload.OrderID)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "Failed to verify payment",
		})
	}

	order, err := h.orderRepo.FindByID(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Order not found",
		})
	}

	switch status.TransactionStatus {
	case "capture":
		// "accept" means the fraud check passed; "challenge" stays pending
		if status.FraudStatus == "accept" {
			return h.settlePayment(c, order, status)
		}
	case "settlement":
		return h.settlePayment(c, order, status)
	case "cancel", "expire":
		h.expireOrder(id)
	case "deny", "pending":
		// deny may be retried and pending is still waiting; do nothing
	}

	return c.SendString("ok")
}

// settlePayment verifies the paid amount and confirms the order. A payment can
// arrive after the order expired (the sweeper released the stock), so for those
// cases it tries to reserve the stock again, or flags the order for refund.
func (h *OrderHandler) settlePayment(c *fiber.Ctx, order *model.UserOrder, status *payment.StatusResult) error {
	// Midtrans reports the amount as a decimal string (e.g. "1000.00"),
	// even though IDR has no cents.
	amount, err := strconv.ParseFloat(status.GrossAmount, 64)
	if err != nil || int64(math.Round(amount)) != int64(order.Total) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Payment amount mismatch",
		})
	}

	info := model.PaymentInfo{
		Type:          status.PaymentType,
		Bank:          status.Bank,
		VANumber:      status.VANumber,
		BillerCode:    status.BillerCode,
		BillKey:       status.BillKey,
		MaskedCard:    status.MaskedCard,
		Store:         status.Store,
		PaymentCode:   status.PaymentCode,
		TransactionID: status.TransactionID,
		SettledAt:     status.SettlementTime,
	}

	switch order.Status {
	case model.OrderStatusPaid, model.OrderStatusNeedsRefund:
		// duplicate notification: already handled (idempotent)
		return c.SendString("ok")

	case model.OrderStatusExpired, model.OrderStatusCancelled:
		// late payment: stock was released, try to take it back
		requested := map[int]int{}
		for _, item := range order.OrderItem {
			requested[item.ID] += item.Qty
		}
		if err := h.productRepo.DecreaseStock(requested); err != nil {
			// the order can no longer be fulfilled -> the customer must be refunded
			if _, _, err := h.orderRepo.TransitionStatusWithPayment(order.ID, order.Status, model.OrderStatusNeedsRefund, info); err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to update order",
				})
			}
			return c.SendString("ok")
		}
	}

	// stock is reserved (or just re-reserved) -> confirm the payment
	updated, confirmed, err := h.orderRepo.TransitionStatusWithPayment(order.ID, order.Status, model.OrderStatusPaid, info)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update order",
		})
	}

	// only when this call actually confirmed the order (idempotent webhooks)
	if confirmed {
		if shipment := h.createShipment(*updated); shipment != nil {
			updated.Shipment = shipment
		}
		h.sendPaidEmail(*updated)
	}

	return c.SendString("ok")
}

// createShipment hands the paid order to the shipping provider. Failures are
// logged and leave the order without a waybill so it can be retried later.
func (h *OrderHandler) createShipment(order model.UserOrder) *model.ShipmentInfo {
	if order.Courier == "" {
		return nil
	}

	items := make([]shipping.ShipmentItem, 0, len(order.OrderItem))
	for _, item := range order.OrderItem {
		items = append(items, shipping.ShipmentItem{
			Name:       item.NameItem,
			Value:      item.Price,
			Quantity:   item.Qty,
			WeightGram: item.WeightGram,
		})
	}

	shipment, err := h.shipping.CreateShipment(shipping.ShipmentRequest{
		ReferenceID:  order.ID.String(),
		ContactName:  order.DeliveryName(),
		ContactPhone: order.DeliveryPhone(),
		ContactEmail: order.Email,
		Address:      order.Address.StreetLine(),
		PostalCode:   order.Address.ZipCode,
		Lat:          order.Address.Latitude,
		Lon:          order.Address.Longitude,
		Courier:      order.Courier,
		Service:      order.CourierService,
		Items:        items,
		Note:         "Order " + order.ID.String()[:8],
	})
	if err != nil {
		log.Printf("failed to create shipment for order %s: %v", order.ID, err)
		return nil
	}

	info := &model.ShipmentInfo{
		ProviderID: shipment.ID,
		WaybillID:  shipment.WaybillID,
		TrackingID: shipment.TrackingID,
		Status:     shipment.Status,
	}
	if err := h.orderRepo.SetShipment(order.ID, *info); err != nil {
		log.Printf("failed to store shipment for order %s: %v", order.ID, err)
	}

	log.Printf("shipment created for order %s: %s (%s)", order.ID, shipment.ID, shipment.WaybillID)
	return info
}

// sendPaidEmail sends the payment confirmation with the invoice PDF attached.
func (h *OrderHandler) sendPaidEmail(order model.UserOrder) {
	msg := notification.OrderPaidEmail(order, h.orderPageURL(order.ID))

	if pdf, err := invoice.Generate(order); err != nil {
		log.Printf("failed to build invoice for %s: %v", order.ID, err)
	} else {
		msg.Attachments = append(msg.Attachments, notification.Attachment{
			Filename:    fmt.Sprintf("invoice-%s.pdf", order.ID.String()[:8]),
			ContentType: "application/pdf",
			Data:        pdf,
		})
	}

	if err := h.mailer.Send(msg); err != nil {
		log.Printf("failed to send paid email for %s: %v", order.ID, err)
	}
}

// ShippingNotification is the Biteship webhook for shipment status updates.
// The payload is never trusted: the real state is re-read from the provider,
// the same pattern used for Midtrans.
func (h *OrderHandler) ShippingNotification(c *fiber.Ctx) error {
	var payload struct {
		Event     string `json:"event"`
		OrderID   string `json:"order_id"`
		Status    string `json:"status"`
		WaybillID string `json:"courier_waybill_id"`
	}
	if err := c.BodyParser(&payload); err != nil || payload.OrderID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid notification",
		})
	}

	shipment, err := h.shipping.Tracking(payload.OrderID)
	if err != nil {
		if errors.Is(err, shipping.ErrNotConfigured) {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": "Shipping is not configured",
			})
		}
		log.Printf("shipping tracking failed for %s: %v", payload.OrderID, err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "Failed to verify shipment",
		})
	}

	order, err := h.orderRepo.FindByShipmentProviderID(payload.OrderID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Order not found for shipment",
		})
	}

	previousStatus := ""
	if order.Shipment != nil {
		previousStatus = order.Shipment.Status
	}

	info := model.ShipmentInfo{
		ProviderID: payload.OrderID,
		WaybillID:  shipment.WaybillID,
		TrackingID: shipment.TrackingID,
		Status:     shipment.Status,
		History:    toShipmentEvents(shipment.History),
	}
	if err := h.orderRepo.SetShipment(order.ID, info); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update shipment",
		})
	}

	// notify the customer once, when the parcel is delivered
	if previousStatus != "delivered" && shipment.Status == "delivered" {
		order.Shipment = &info
		if err := h.mailer.Send(notification.OrderDeliveredEmail(*order, h.orderPageURL(order.ID))); err != nil {
			log.Printf("failed to send delivered email for %s: %v", order.ID, err)
		}
	}

	return c.SendString("ok")
}

func toShipmentEvents(events []shipping.ShipmentEvent) []model.ShipmentEvent {
	out := make([]model.ShipmentEvent, 0, len(events))
	for _, event := range events {
		out = append(out, model.ShipmentEvent{
			Status:    event.Status,
			Note:      event.Note,
			UpdatedAt: event.UpdatedAt,
		})
	}
	return out
}

// DownloadInvoice serves the invoice PDF (guest link, protected by the order UUID).
func (h *OrderHandler) DownloadInvoice(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid order id",
		})
	}

	order, err := h.orderRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Order not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Internal server error",
		})
	}

	pdf, err := invoice.Generate(*order)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to build invoice",
		})
	}

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf("inline; filename=\"invoice-%s.pdf\"", id.String()[:8]))
	return c.Send(pdf)
}

func (h *OrderHandler) expireOrder(id uuid.UUID) {
	order, ok, err := h.orderRepo.TransitionStatus(id, model.OrderStatusPending, model.OrderStatusExpired)
	if err != nil || !ok {
		return
	}

	requested := map[int]int{}
	for _, item := range order.OrderItem {
		requested[item.ID] += item.Qty
	}
	_ = h.productRepo.ReleaseStock(requested)
}

// GetOrders lists orders (admin), optionally filtered by ?status=
func (h *OrderHandler) GetOrders(c *fiber.Ctx) error {
	status := model.OrderStatus(c.Query("status"))
	if status != "" && !status.Valid() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid status",
		})
	}

	orders, err := h.orderRepo.GetAll(status)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to load orders",
		})
	}

	return c.JSON(orders)
}

// get order by id handler
func (h *OrderHandler) GetOrderByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid order id",
		})
	}

	order, err := h.orderRepo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Order not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Internal server error",
		})
	}

	return c.JSON(order)
}

// cancel order handler: only pending orders can be cancelled, and the stock
// held by the order is released back to the catalog.
func (h *OrderHandler) CancelOrder(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid order id",
		})
	}

	// atomic transition acts as the idempotency guard: a second cancel finds the
	// order in "cancelled" state and must not release stock twice.
	order, ok, err := h.orderRepo.TransitionStatus(id, model.OrderStatusPending, model.OrderStatusCancelled)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Order not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Internal server error",
		})
	}
	if !ok {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": fmt.Sprintf("Order cannot be cancelled (current status: %s)", order.Status),
		})
	}

	requested := map[int]int{}
	for _, item := range order.OrderItem {
		requested[item.ID] += item.Qty
	}
	_ = h.productRepo.ReleaseStock(requested)

	return c.JSON(order)
}
