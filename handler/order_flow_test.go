package handler_test

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"golearn/handler"
	"golearn/model"
	"golearn/payment"
	"golearn/repository"
	"golearn/shipping"
)

// These tests exercise the real order handlers against the real in-memory
// repositories. Only the payment gateway, mailer, and shipping provider are
// faked, which is what the handler sees when it talks to Midtrans/Biteship.

type flowApp struct {
	app      *fiber.App
	products repository.ProductRepository
	orders   repository.OrderRepository
	gateway  *countingGateway
	mailer   *fakeMailer
	shipping *fakeShippingProvider
}

type countingGateway struct {
	mu          sync.Mutex
	statusCalls int
	status      *payment.StatusResult
}

func (g *countingGateway) Charge(model.UserOrder) (*payment.ChargeResult, error) {
	return &payment.ChargeResult{Token: "fake-token", RedirectURL: "https://example.test/pay"}, nil
}

func (g *countingGateway) CheckStatus(string) (*payment.StatusResult, error) {
	g.mu.Lock()
	g.statusCalls++
	g.mu.Unlock()

	if g.status == nil {
		return &payment.StatusResult{TransactionStatus: "pending"}, nil
	}
	return g.status, nil
}

func (g *countingGateway) calls() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.statusCalls
}

func newFlowApp(t *testing.T, products repository.ProductRepository, orders repository.OrderRepository, serverKey string) *flowApp {
	t.Helper()

	if products == nil {
		products = repository.NewMemoryProductRepository()
	}
	if orders == nil {
		orders = repository.NewMemoryOrderRepository()
	}

	app := fiber.New()
	gateway := &countingGateway{}
	mailer := &fakeMailer{}
	provider := &fakeShippingProvider{rates: testRates}

	productHandler := handler.NewProductHandler(products)
	orderHandler := handler.NewOrderHandler(products, orders, gateway, mailer, provider, "http://store.test")
	if serverKey != "" {
		orderHandler.SetPaymentServerKey(serverKey)
	}

	app.Get("/products/:id", productHandler.GetProductByID)
	app.Post("/orders", orderHandler.CreateOrder)
	app.Post("/orders/notification", orderHandler.Notification)
	app.Get("/orders/:id", orderHandler.GetOrderByID)
	app.Post("/orders/:id/cancel", orderHandler.CancelOrder)
	app.Post("/admin/orders/:id/shipment", orderHandler.RetryShipment)
	app.Post("/shipping/notification", orderHandler.ShippingNotification)

	return &flowApp{
		app:      app,
		products: products,
		orders:   orders,
		gateway:  gateway,
		mailer:   mailer,
		shipping: provider,
	}
}

// staleReadOrderRepo returns one outdated snapshot before delegating to the
// real repository. It reproduces the window where the handler read the order
// and the sweeper changed it before the status transition ran.
type staleReadOrderRepo struct {
	*repository.MemoryOrderRepository

	mu    sync.Mutex
	stale *model.UserOrder
}

func (r *staleReadOrderRepo) FindByID(id uuid.UUID) (*model.UserOrder, error) {
	r.mu.Lock()
	stale := r.stale
	r.stale = nil
	r.mu.Unlock()

	if stale != nil {
		snapshot := *stale
		return &snapshot, nil
	}
	return r.MemoryOrderRepository.FindByID(id)
}

// gatedProductRepo blocks the first stock decrement until the test releases it,
// so two webhook requests can be interleaved deterministically.
type gatedProductRepo struct {
	*repository.MemoryProductRepository

	arrived chan struct{}
	release chan struct{}
	once    sync.Once
	armed   atomic.Bool
}

func (r *gatedProductRepo) DecreaseStock(requested map[int]int) error {
	if r.armed.Load() {
		r.once.Do(func() {
			r.arrived <- struct{}{}
			<-r.release
		})
	}
	return r.MemoryProductRepository.DecreaseStock(requested)
}

func notifyBody(t *testing.T, app *fiber.App, body any) *http.Response {
	t.Helper()
	return postJSON(t, app, "/orders/notification", body)
}

func postJSON(t *testing.T, app *fiber.App, path string, body any) *http.Response {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// notifyLong disables the 1s app.Test timeout, which the gated repository
// deliberately exceeds.
func notifyLong(t *testing.T, app *fiber.App, id uuid.UUID) *http.Response {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"order_id": id.String()})
	req, _ := http.NewRequest(http.MethodPost, "/orders/notification", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Errorf("notification: %v", err)
		return &http.Response{StatusCode: 0, Body: http.NoBody}
	}
	return resp
}

func settlement(grossAmount string) *payment.StatusResult {
	return &payment.StatusResult{
		TransactionStatus: "settlement",
		FraudStatus:       "accept",
		GrossAmount:       grossAmount,
		PaymentType:       "qris",
		TransactionID:     "tx-flow",
	}
}

// Case 1: the sweeper expires the order between the handler's read and the
// status update. The payment is real, so the order must end up paid (stock is
// available again) or needs_refund (stock is gone) - never silently expired.
func TestFlowSettlementRacingTheSweeper(t *testing.T) {
	realOrders := repository.NewMemoryOrderRepository()
	orders := &staleReadOrderRepo{MemoryOrderRepository: realOrders}
	flow := newFlowApp(t, nil, orders, "")

	order := validOrder(model.Item{ID: 1, Qty: 2})
	order.Courier = "jne"
	order.CourierService = "reg"
	_, created := postOrderModel(t, flow.app, order)
	if created.ID == uuid.Nil {
		t.Fatal("order was not created")
	}

	// The sweeper expires the order and releases its stock.
	if _, _, err := realOrders.TransitionStatus(created.ID, model.OrderStatusPending, model.OrderStatusExpired); err != nil {
		t.Fatalf("expire: %v", err)
	}
	if err := flow.products.ReleaseStock(map[int]int{1: 2}); err != nil {
		t.Fatalf("release stock: %v", err)
	}

	// The handler had already read the order while it was still pending.
	stale := created
	orders.mu.Lock()
	orders.stale = &stale
	orders.mu.Unlock()

	// 2 x 1000 for the items plus 25000 for shipping.
	flow.gateway.status = settlement("27000.00")

	resp := notify(t, flow.app, created.ID)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("notification status = %d, want 200", resp.StatusCode)
	}

	stored, err := realOrders.FindByID(created.ID)
	if err != nil {
		t.Fatalf("find order: %v", err)
	}

	if stored.Status != model.OrderStatusPaid {
		t.Fatalf("status = %q, want %q (payment captured but order left behind)", stored.Status, model.OrderStatusPaid)
	}
	if stored.Payment == nil {
		t.Fatal("payment info was not recorded")
	}
	if stock := stockOf(t, flow.app, 1); stock != 8 {
		t.Errorf("stock = %d, want 8 (re-reserved once)", stock)
	}
	if len(flow.shipping.shipmentReqs) != 1 {
		t.Errorf("shipments created = %d, want 1", len(flow.shipping.shipmentReqs))
	}
	if len(flow.mailer.sent) == 0 {
		t.Error("no paid email was sent")
	}
}

// Case 2: two settlement notifications arrive for an order that was already
// cancelled. Only one may re-reserve stock.
func TestFlowDuplicateLatePaymentReservesStockOnce(t *testing.T) {
	gated := &gatedProductRepo{
		MemoryProductRepository: repository.NewMemoryProductRepository(),
		arrived:                 make(chan struct{}, 1),
		release:                 make(chan struct{}),
	}
	flow := newFlowApp(t, gated, nil, "")

	_, created := postOrderModel(t, flow.app, validOrder(model.Item{ID: 1, Qty: 2}))

	cancelReq, _ := http.NewRequest(http.MethodPost, "/orders/"+created.ID.String()+"/cancel", nil)
	if resp, err := flow.app.Test(cancelReq); err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("cancel failed: %v", err)
	}
	// stock is back to 10
	if stock := stockOf(t, flow.app, 1); stock != 10 {
		t.Fatalf("stock after cancel = %d, want 10", stock)
	}

	flow.gateway.status = settlement("2000.00")
	gated.armed.Store(true)

	var wg sync.WaitGroup
	send := func() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := notifyLong(t, flow.app, created.ID)
			if resp.StatusCode != fiber.StatusOK {
				t.Errorf("notification status = %d, want 200", resp.StatusCode)
			}
		}()
	}

	// First request reads the cancelled order and blocks right before it
	// decrements stock.
	send()
	<-gated.arrived

	// Second request handles the same order concurrently.
	send()

	// Let the first request continue.
	close(gated.release)
	wg.Wait()

	if stock := stockOf(t, flow.app, 1); stock != 8 {
		t.Fatalf("stock = %d, want 8 (reserved once, not %d)", stock, 10-stock)
	}

	stored, err := flow.orders.FindByID(created.ID)
	if err != nil {
		t.Fatalf("find order: %v", err)
	}
	if stored.Status != model.OrderStatusPaid {
		t.Fatalf("status = %q, want paid", stored.Status)
	}
}

// Case 3: a forged notification must not reach the payment gateway when a
// server key is configured.
func TestFlowForgedNotificationIsRejected(t *testing.T) {
	const serverKey = "server-key-for-tests"
	flow := newFlowApp(t, nil, nil, serverKey)

	_, created := postOrderModel(t, flow.app, validOrder(model.Item{ID: 1, Qty: 2}))
	flow.gateway.status = settlement("2000.00")

	forged := notifyBody(t, flow.app, map[string]string{"order_id": created.ID.String()})
	if forged.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("unsigned notification status = %d, want 401", forged.StatusCode)
	}

	badSignature := notifyBody(t, flow.app, map[string]string{
		"order_id":      created.ID.String(),
		"status_code":   "200",
		"gross_amount":  "2000.00",
		"signature_key": "not-the-right-signature",
	})
	if badSignature.StatusCode != fiber.StatusUnauthorized {
		t.Errorf("bad signature status = %d, want 401", badSignature.StatusCode)
	}

	if got := flow.gateway.calls(); got != 0 {
		t.Errorf("gateway was called %d times for forged notifications, want 0", got)
	}

	valid := notifyBody(t, flow.app, map[string]string{
		"order_id":      created.ID.String(),
		"status_code":   "200",
		"gross_amount":  "2000.00",
		"signature_key": midtransSignature(created.ID.String(), "200", "2000.00", serverKey),
	})
	if valid.StatusCode != fiber.StatusOK {
		t.Fatalf("signed notification status = %d, want 200", valid.StatusCode)
	}
	if got := flow.gateway.calls(); got != 1 {
		t.Errorf("gateway calls = %d, want 1", got)
	}
}

func midtransSignature(orderID, statusCode, grossAmount, serverKey string) string {
	sum := sha512.Sum512([]byte(orderID + statusCode + grossAmount + serverKey))
	return hex.EncodeToString(sum[:])
}

// Case 4: an email with a line break must not reach the mailer.
func TestFlowOrderRejectsEmailHeaderInjection(t *testing.T) {
	flow := newFlowApp(t, nil, nil, "")

	order := validOrder(model.Item{ID: 1, Qty: 1})
	order.Email = "b@mail.com\r\nBcc: victim@example.com"

	resp, created := postOrderModel(t, flow.app, order)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a CRLF in the email", resp.StatusCode)
	}
	if created.ID != uuid.Nil {
		t.Error("order was created despite the invalid email")
	}
	if len(flow.mailer.sent) != 0 {
		t.Errorf("mailer sent %d messages, want 0", len(flow.mailer.sent))
	}
}

// Case 5: cart bounds. A single request must not fan out into thousands of
// catalog lookups, and a quantity must stay in a sane range.
func TestFlowOrderRejectsOversizedCart(t *testing.T) {
	// Plenty of stock, so only the cart limits can reject the request.
	products := repository.NewMemoryProductRepository()
	big := 1_000_000
	if _, err := products.UpdateProduct(1, model.ProductUpdate{Stock: &big}); err != nil {
		t.Fatalf("seed stock: %v", err)
	}
	flow := newFlowApp(t, products, nil, "")

	t.Run("too many lines", func(t *testing.T) {
		order := validOrder(model.Item{ID: 1, Qty: 1})
		order.OrderItem = nil
		for range 200 {
			order.OrderItem = append(order.OrderItem, model.Item{ID: 1, Qty: 1})
		}

		resp, _ := postOrderModel(t, flow.app, order)
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
	})

	t.Run("quantity too large", func(t *testing.T) {
		resp, _ := postOrderModel(t, flow.app, validOrder(model.Item{ID: 1, Qty: 100_000}))
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
	})
}

// Case 6: the shipping webhook forwards the provider id into a URL. A value
// containing path separators must be rejected, not appended raw.
func TestFlowShippingWebhookRejectsTraversalID(t *testing.T) {
	provider := &recordingShipping{}
	orders := repository.NewMemoryOrderRepository()
	app := fiber.New()
	orderHandler := handler.NewOrderHandler(repository.NewMemoryProductRepository(), orders, &countingGateway{}, &fakeMailer{}, provider, "http://store.test")
	app.Post("/shipping/notification", orderHandler.ShippingNotification)

	resp := postJSON(t, app, "/shipping/notification", map[string]string{"order_id": "../../v1/orders"})
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a traversal id", resp.StatusCode)
	}
	if provider.called != "" {
		t.Fatalf("provider was called with %q, want no call", provider.called)
	}
}

type recordingShipping struct {
	called string
}

func (r *recordingShipping) Rates(shipping.RateRequest) ([]shipping.Rate, error) {
	return nil, nil
}

func (r *recordingShipping) CreateShipment(shipping.ShipmentRequest) (*shipping.Shipment, error) {
	return nil, nil
}

func (r *recordingShipping) Tracking(id string) (*shipping.Shipment, error) {
	r.called = id
	return &shipping.Shipment{ID: id, Status: "confirmed"}, nil
}

// Case 7: a paid order whose shipment creation failed can be retried from the
// dashboard instead of being stuck forever.
func TestFlowAdminRetryCreatesMissingShipment(t *testing.T) {
	flow := newFlowApp(t, nil, nil, "")
	flow.shipping.shipmentErr = errors.New("provider is down")

	order := validOrder(model.Item{ID: 1, Qty: 2})
	order.Courier = "jne"
	order.CourierService = "reg"

	_, created := postOrderModel(t, flow.app, order)
	flow.gateway.status = settlement("27000.00")
	if resp := notify(t, flow.app, created.ID); resp.StatusCode != fiber.StatusOK {
		t.Fatalf("notification status = %d, want 200", resp.StatusCode)
	}

	stored, err := flow.orders.FindByID(created.ID)
	if err != nil {
		t.Fatalf("find order: %v", err)
	}
	if stored.Status != model.OrderStatusPaid {
		t.Fatalf("status = %q, want paid", stored.Status)
	}
	if stored.Shipment != nil {
		t.Fatal("shipment should be missing while the provider is down")
	}

	// The provider recovers and the operator presses "Create shipment".
	flow.shipping.shipmentErr = nil
	path := "/admin/orders/" + created.ID.String() + "/shipment"
	resp := postJSON(t, flow.app, path, nil)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("retry status = %d, want 200", resp.StatusCode)
	}

	var updated model.UserOrder
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("decode order: %v", err)
	}
	if updated.Shipment == nil || updated.Shipment.WaybillID == "" {
		t.Fatalf("shipment = %+v, want a waybill", updated.Shipment)
	}

	stored, err = flow.orders.FindByID(created.ID)
	if err != nil {
		t.Fatalf("find order: %v", err)
	}
	if stored.Shipment == nil || stored.Shipment.WaybillID == "" {
		t.Fatal("shipment was not persisted")
	}

	// A second retry must not create a duplicate provider shipment.
	if again := postJSON(t, flow.app, path, nil); again.StatusCode != fiber.StatusConflict {
		t.Fatalf("second retry status = %d, want 409", again.StatusCode)
	}
	if got := len(flow.shipping.shipmentReqs); got != 2 {
		t.Fatalf("provider calls = %d, want 2 (one failed, one retry)", got)
	}
}

func stockOf(t *testing.T, app *fiber.App, productID int) int {
	t.Helper()

	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d", productID), nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("product request: %v", err)
	}
	defer resp.Body.Close()

	var product model.Product
	if err := json.NewDecoder(resp.Body).Decode(&product); err != nil {
		t.Fatalf("decode product: %v", err)
	}
	return product.Stock
}
