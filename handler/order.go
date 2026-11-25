package handler

import (
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"net/mail"
	"strconv"
	"sync"
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

// Cart bounds. They keep one request from fanning out into thousands of
// catalog lookups, and stop quantity multiplication from overflowing.
const (
	maxOrderLines = 50
	maxItemQty    = 100
)

// errShippingUnavailable is returned when the requested courier/service is not
// offered by the provider for this destination.
var (
	errShippingUnavailable   = errors.New("shipping option is not available")
	errPaymentAmountMismatch = errors.New("payment amount mismatch")
)

type OrderHandler struct {
	productRepo  repository.ProductRepository
	orderRepo    repository.OrderRepository
	gateway      payment.Gateway
	mailer       notification.Sender
	shipping     shipping.Provider
	storeBaseURL string
	storeLat     float64
	storeLon     float64
	storeAddress string
	// paymentServerKey enables Midtrans signature verification on the webhook.
	// Empty disables it, which keeps local sandbox setups working.
	paymentServerKey string
	// shippingWebhookToken, when set, is required on the shipping webhook.
	shippingWebhookToken string
	// shipmentLocks serialize shipment creation per order. The settlement path,
	// the admin button, and the background retrier all go through createShipment;
	// calling the provider twice would create two shipments. Sharded so the map
	// cannot grow with the order history.
	shipmentLocks [64]sync.Mutex
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

// SetStoreOrigin lets the public order page draw the store-to-customer route.
func (h *OrderHandler) SetStoreOrigin(lat, lon float64, address string) {
	h.storeLat = lat
	h.storeLon = lon
	h.storeAddress = address
}

// SetPaymentServerKey turns on Midtrans webhook signature verification. The key
// is the same Midtrans server key used for API calls.
func (h *OrderHandler) SetPaymentServerKey(key string) {
	h.paymentServerKey = key
}

// SetShippingWebhookToken requires the given token on the shipping webhook, via
// ?token= or the X-Webhook-Token header. Empty leaves the endpoint open, which is
// only acceptable in development.
func (h *OrderHandler) SetShippingWebhookToken(token string) {
	h.shippingWebhookToken = token
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
	if len(order.OrderItem) > maxOrderLines {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Too many order lines (max %d)", maxOrderLines),
		})
	}

	if order.Username == "" || order.Email == "" || order.Phone == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Username, email or phone is empty",
		})
	}

	// Reject anything that is not a plain address: the value ends up in a mail
	// header, where a line break would inject extra headers.
	if _, err := mail.ParseAddress(order.Email); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid email address",
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
		if item.Qty <= 0 || item.Qty > maxItemQty {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": fmt.Sprintf("Quantity for product %s must be between 1 and %d", product.Name, maxItemQty),
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
		log.Printf("failed to save order %s: %v", order.ID, err)
		// the payment session was already created but the order was not stored,
		// so give the reserved stock back instead of leaking it
		_ = h.productRepo.ReleaseStock(requested)
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
		OrderID      string `json:"order_id"`
		StatusCode   string `json:"status_code"`
		GrossAmount  string `json:"gross_amount"`
		SignatureKey string `json:"signature_key"`
	}
	if err := c.BodyParser(&payload); err != nil || payload.OrderID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid notification",
		})
	}

	// Midtrans signs order_id + status_code + gross_amount with the server key.
	// Checking it first rejects forged traffic before it can trigger an outbound
	// API call. Without a configured key (local sandbox) the check is skipped.
	if h.paymentServerKey != "" {
		expected := sha512Hex(payload.OrderID + payload.StatusCode + payload.GrossAmount + h.paymentServerKey)
		if subtle.ConstantTimeCompare([]byte(expected), []byte(payload.SignatureKey)) != 1 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid signature",
			})
		}
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
	if err := h.settlePaidOrder(order, status); err != nil {
		if errors.Is(err, errPaymentAmountMismatch) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Payment amount mismatch",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update order",
		})
	}

	return c.SendString("ok")
}

func (h *OrderHandler) settlePaidOrder(order *model.UserOrder, status *payment.StatusResult) error {
	// Midtrans reports the amount as a decimal string (e.g. "1000.00"),
	// even though IDR has no cents.
	amount, err := strconv.ParseFloat(status.GrossAmount, 64)
	if err != nil || int64(math.Round(amount)) != int64(order.Total) {
		return errPaymentAmountMismatch
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

	// A duplicate notification for an order that is already settled.
	if order.Status == model.OrderStatusPaid || order.Status == model.OrderStatusNeedsRefund {
		return nil
	}

	// Claim the order and reserve stock in one unit of work. The claim only
	// succeeds for one of several concurrent notifications, and a crash cannot
	// leave the order paid without its stock (or the reverse).
	from := order.Status
	reserveStock := from == model.OrderStatusExpired || from == model.OrderStatusCancelled

	claimed, outcome, err := h.orderRepo.SettlePaid(order.ID, from, info, requestedItems(order), reserveStock)
	if err != nil {
		return err
	}

	if outcome == repository.SettleNotClaimed {
		// The status changed between our read and the claim: re-read and retry
		// once against whatever won (usually the sweeper expiring the order).
		current, findErr := h.orderRepo.FindByID(order.ID)
		if findErr != nil {
			return findErr
		}

		switch current.Status {
		case model.OrderStatusPaid, model.OrderStatusNeedsRefund:
			return nil
		case model.OrderStatusExpired, model.OrderStatusCancelled:
			claimed, outcome, err = h.orderRepo.SettlePaid(
				order.ID, current.Status, info, requestedItems(current), true)
			if err != nil {
				return err
			}
			if outcome == repository.SettleNotClaimed {
				return nil
			}
		default:
			return nil
		}
	}

	if outcome == repository.SettleNeedsRefund {
		// The payment is recorded and the order flagged; someone refunds it.
		return nil
	}

	if shipment := h.createShipment(*claimed); shipment != nil {
		claimed.Shipment = shipment
	}
	h.sendPaidEmail(*claimed)

	return nil
}

// requestedItems sums the quantity per product for stock operations.
func requestedItems(order *model.UserOrder) map[int]int {
	requested := make(map[int]int, len(order.OrderItem))
	for _, item := range order.OrderItem {
		requested[item.ID] += item.Qty
	}
	return requested
}

// sha512Hex is the format Midtrans uses for signature_key.
func sha512Hex(value string) string {
	sum := sha512.Sum512([]byte(value))
	return hex.EncodeToString(sum[:])
}

// ReconcilePending asks the payment provider for the authoritative state of a
// pending order and applies it. settled reports that the order is no longer
// pending; verified is false when the provider could not be reached, which the
// sweeper uses to keep the stock reserved instead of guessing.
func (h *OrderHandler) ReconcilePending(order model.UserOrder) (settled bool, verified bool) {
	if order.Status != model.OrderStatusPending {
		return true, true
	}

	status, err := h.gateway.CheckStatus(order.ID.String())
	if err != nil {
		log.Printf("order %s: could not verify payment: %v", order.ID, err)
		return false, false
	}

	switch status.TransactionStatus {
	case "capture":
		if status.FraudStatus == "accept" {
			h.settleReconciled(&order, status)
			return true, true
		}
		return false, true
	case "settlement":
		h.settleReconciled(&order, status)
		return true, true
	case "cancel", "expire":
		h.expireOrder(order.ID)
		return true, true
	}

	// pending or deny: the transaction is still open (or retryable), so let the
	// TTL decide.
	return false, true
}

func (h *OrderHandler) settleReconciled(order *model.UserOrder, status *payment.StatusResult) {
	if err := h.settlePaidOrder(order, status); err != nil {
		log.Printf("order %s: payment reconciliation failed: %v", order.ID, err)
	}
}

// createShipment hands the paid order to the shipping provider. It is
// idempotent per order: a concurrent caller (settlement, admin retry, background
// retrier) either sees the shipment that already exists or waits for the one
// being created. Failures are logged and leave the order without a waybill so it
// can be retried later.
func (h *OrderHandler) createShipment(order model.UserOrder) *model.ShipmentInfo {
	if order.Courier == "" {
		return nil
	}

	unlock := h.lockShipment(order.ID)
	defer unlock()

	// Re-read inside the lock: another caller may have created it already.
	if current, err := h.orderRepo.FindByID(order.ID); err == nil {
		if current.Shipment != nil && current.Shipment.ProviderID != "" {
			return current.Shipment
		}
		order = *current
	}
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

// RetryShipmentFor creates the provider shipment for a paid order that has none.
// The background retrier calls it; the admin endpoint and the settlement path
// share the same idempotent createShipment.
func (h *OrderHandler) RetryShipmentFor(order model.UserOrder) bool {
	if order.Status != model.OrderStatusPaid || order.Courier == "" {
		return false
	}
	if order.Shipment != nil && order.Shipment.ProviderID != "" {
		return true
	}
	return h.createShipment(order) != nil
}

// lockShipment serializes shipment creation for one order without keeping a map
// that grows with the order history.
func (h *OrderHandler) lockShipment(id uuid.UUID) func() {
	slot := &h.shipmentLocks[int(id[0])%len(h.shipmentLocks)]
	slot.Lock()
	return slot.Unlock
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
	// Biteship does not sign its webhooks, so a shared secret is the gate. The
	// token travels in the URL because the dashboard only lets us set the URL.
	if h.shippingWebhookToken != "" {
		provided := c.Query("token")
		if provided == "" {
			provided = c.Get("X-Webhook-Token")
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(h.shippingWebhookToken)) != 1 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid webhook token",
			})
		}
	}

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

	// The id is interpolated into the provider URL, so only accept the shape a
	// provider actually issues.
	if !shipping.ValidProviderID(payload.OrderID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid shipment id",
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

// RetryShipment (admin) creates the provider shipment for a paid order that does
// not have one yet, so a provider outage does not leave the order stuck.
func (h *OrderHandler) RetryShipment(c *fiber.Ctx) error {
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

	if order.Status != model.OrderStatusPaid {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": fmt.Sprintf("Only paid orders can be shipped (current status: %s)", order.Status),
		})
	}
	if order.Shipment != nil && order.Shipment.ProviderID != "" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Order already has a shipment",
		})
	}
	if order.Courier == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Order has no courier",
		})
	}

	info := h.createShipment(*order)
	if info == nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error": "Shipping provider rejected the shipment",
		})
	}

	order.Shipment = info
	return c.JSON(order)
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

	if err := h.productRepo.ReleaseStock(requestedItems(order)); err != nil {
		log.Printf("failed to release stock for expired order %s: %v", id, err)
	}
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

	requested := requestedItems(order)
	if err := h.productRepo.ReleaseStock(requested); err != nil {
		log.Printf("failed to release stock for cancelled order %s: %v", order.ID, err)
	}

	return c.JSON(order)
}
