package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"golearn/handler"
	"golearn/model"
	"golearn/notification"
	"golearn/payment"
	"golearn/repository"
	"golearn/shipping"
)

type fakeMailer struct {
	sent []notification.Message
}

func (f *fakeMailer) Send(msg notification.Message) error {
	f.sent = append(f.sent, msg)
	return nil
}

type fakeGateway struct {
	chargeErr   error
	status      *payment.StatusResult
	statusErr   error
	charged     *model.UserOrder
	refundErr   error
	refundCalls int
	expireErr   error
	expireCalls int
}

type fakeShippingProvider struct {
	rates        []shipping.Rate
	err          error
	shipmentErr  error
	tracking     *shipping.Shipment
	trackingErr  error
	last         shipping.RateRequest
	shipmentReqs []shipping.ShipmentRequest
}

func (f *fakeShippingProvider) Rates(req shipping.RateRequest) ([]shipping.Rate, error) {
	f.last = req
	if f.err != nil {
		return nil, f.err
	}
	return f.rates, nil
}

func (f *fakeShippingProvider) Tracking(providerOrderID string) (*shipping.Shipment, error) {
	if f.trackingErr != nil {
		return nil, f.trackingErr
	}
	if f.tracking == nil {
		return &shipping.Shipment{ID: providerOrderID, Status: "confirmed"}, nil
	}
	return f.tracking, nil
}

func (f *fakeShippingProvider) CreateShipment(req shipping.ShipmentRequest) (*shipping.Shipment, error) {
	f.shipmentReqs = append(f.shipmentReqs, req)
	if f.shipmentErr != nil {
		return nil, f.shipmentErr
	}
	return &shipping.Shipment{
		ID:         "bite-" + req.ReferenceID[:8],
		WaybillID:  "WYB-123",
		TrackingID: "TRK-123",
		Status:     "confirmed",
	}, nil
}

var testRates = []shipping.Rate{
	{Courier: "jne", CourierName: "JNE", Service: "reg", ServiceName: "Reguler", Price: 25000},
	{Courier: "jne", Service: "yes", ServiceName: "YES", Price: 84000},
}

func (f *fakeGateway) Charge(order model.UserOrder) (*payment.ChargeResult, error) {
	f.charged = &order
	if f.chargeErr != nil {
		return nil, f.chargeErr
	}
	return &payment.ChargeResult{Token: "fake-token", RedirectURL: "https://example.test/pay"}, nil
}

func (f *fakeGateway) Refund(orderID string, amount int, reason string) error {
	f.refundCalls++
	return f.refundErr
}

func (f *fakeGateway) Expire(orderID string) error {
	f.expireCalls++
	return f.expireErr
}

func (f *fakeGateway) CheckStatus(orderID string) (*payment.StatusResult, error) {
	if f.statusErr != nil {
		return nil, f.statusErr
	}
	if f.status == nil {
		return &payment.StatusResult{TransactionStatus: "pending"}, nil
	}
	return f.status, nil
}

func setupApp() (*fiber.App, *fakeGateway) {
	app, gw, _ := buildApp()
	return app, gw
}

func buildApp() (*fiber.App, *fakeGateway, *fakeMailer) {
	return buildAppWithShipping(&fakeShippingProvider{rates: testRates})
}

func buildAppWithShipping(provider shipping.Provider) (*fiber.App, *fakeGateway, *fakeMailer) {
	app := fiber.New()

	productRepo := repository.NewMemoryProductRepository()
	orderRepo := repository.NewMemoryOrderRepository()
	orderRepo.SetProducts(productRepo)
	gw := &fakeGateway{}
	mailer := &fakeMailer{}

	productHandler := handler.NewProductHandler(productRepo)
	orderHandler := handler.NewOrderHandler(productRepo, orderRepo, gw, mailer, provider, "http://store.test")

	app.Get("/products", productHandler.GetProducts)
	app.Get("/products/:id", productHandler.GetProductByID)
	app.Delete("/products/:id", productHandler.DeleteProduct)
	app.Get("/admin/products", productHandler.GetAllProducts)
	app.Patch("/admin/products/:id", productHandler.UpdateProduct)
	app.Post("/orders", orderHandler.CreateOrder)
	app.Post("/orders/notification", orderHandler.Notification)
	app.Get("/orders", orderHandler.GetOrders)
	app.Get("/orders/:id", orderHandler.GetOrderByID)
	app.Get("/admin/orders/:id", orderHandler.AdminGetOrderByID)
	app.Get("/orders/:id/invoice.pdf", orderHandler.DownloadInvoice)
	app.Post("/orders/:id/cancel", orderHandler.CancelOrder)
	app.Post("/shipping/notification", orderHandler.ShippingNotification)
	app.Get("/stats", orderHandler.GetStats)
	app.Get("/order/:id", orderHandler.OrderPageView)
	app.Get("/track", orderHandler.TrackPageView)
	app.Post("/track", orderHandler.TrackPageView)

	return app, gw, mailer
}

func orderBody(t *testing.T, item model.Item) *bytes.Reader {
	t.Helper()

	order := model.UserOrder{
		Username:  "budi",
		Email:     "b@mail.com",
		Phone:     "0812",
		Address:   model.Address{Street: "Jl Mawar", City: "Bandung", State: "Jabar", ZipCode: "40123", Longitude: 107.6, Latitude: -6.9},
		OrderItem: []model.Item{item},
	}

	b, err := json.Marshal(order)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(b)
}

func validOrder(item model.Item) model.UserOrder {
	return model.UserOrder{
		Username:  "budi",
		Email:     "b@mail.com",
		Phone:     "0812",
		Address:   model.Address{Street: "Jl Mawar", City: "Bandung", State: "Jabar", ZipCode: "40123", Longitude: 107.6, Latitude: -6.9},
		OrderItem: []model.Item{item},
	}
}

func postOrderModel(t *testing.T, app *fiber.App, order model.UserOrder) (*http.Response, model.UserOrder) {
	t.Helper()

	body, err := json.Marshal(order)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	var created model.UserOrder
	json.NewDecoder(resp.Body).Decode(&created)
	return resp, created
}

func postOrder(t *testing.T, app *fiber.App, item model.Item) (*http.Response, model.UserOrder) {
	t.Helper()

	req, _ := http.NewRequest(http.MethodPost, "/orders", orderBody(t, item))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	var order model.UserOrder
	json.NewDecoder(resp.Body).Decode(&order)
	return resp, order
}

func getOrder(t *testing.T, app *fiber.App, order model.UserOrder) model.UserOrder {
	t.Helper()

	path := "/admin/orders/" + order.ID.String()
	req, _ := http.NewRequest(http.MethodGet, path, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	var got model.UserOrder
	json.NewDecoder(resp.Body).Decode(&got)
	return got
}

func cancelURL(order model.UserOrder) string {
	return "/orders/" + order.ID.String() + "/cancel?t=" + url.QueryEscape(order.AccessTokenIssued)
}

func notify(t *testing.T, app *fiber.App, id uuid.UUID) *http.Response {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"order_id": id.String()})
	req, _ := http.NewRequest(http.MethodPost, "/orders/notification", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestCreateOrderSuccess(t *testing.T) {
	app, _ := setupApp()

	resp, order := postOrder(t, app, model.Item{ID: 1, Qty: 2})

	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	if order.ID == uuid.Nil {
		t.Error("order ID was not generated")
	}
	if order.OrderItem[0].Price != 1000 {
		t.Errorf("price = %v, want 1000 (must come from catalog)", order.OrderItem[0].Price)
	}
	if order.OrderItem[0].TotalPrice != 2000 {
		t.Errorf("total_price = %v, want 2000", order.OrderItem[0].TotalPrice)
	}
	if order.Total != 2000 {
		t.Errorf("total = %v, want 2000", order.Total)
	}
	if order.Status != model.OrderStatusPending {
		t.Errorf("status = %q, want %q", order.Status, model.OrderStatusPending)
	}
	if order.SnapToken != "fake-token" {
		t.Errorf("snap_token = %q, want fake-token", order.SnapToken)
	}
}

func TestCreateOrderIgnoresClientTotal(t *testing.T) {
	app, _ := setupApp()

	order := model.UserOrder{
		Username:  "budi",
		Email:     "b@mail.com",
		Phone:     "0812",
		Address:   model.Address{Street: "Jl Mawar", City: "Bandung", State: "Jabar", ZipCode: "40123", Longitude: 107.6, Latitude: -6.9},
		OrderItem: []model.Item{{ID: 1, Qty: 1}},
		Total:     1,
	}
	body, _ := json.Marshal(order)
	req, _ := http.NewRequest(http.MethodPost, "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	var created model.UserOrder
	json.NewDecoder(resp.Body).Decode(&created)
	if created.Total != 1000 {
		t.Errorf("total = %v, want 1000 (server must ignore client total)", created.Total)
	}
}

func TestCreateOrderIgnoresClientPrice(t *testing.T) {
	app, _ := setupApp()

	// client tries to send a fake price of 1
	resp, order := postOrder(t, app, model.Item{ID: 1, Price: 1, Qty: 1})

	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	if order.OrderItem[0].Price != 1000 {
		t.Errorf("price = %v, want 1000 (server must ignore client price)", order.OrderItem[0].Price)
	}
}

func TestCreateOrderInsufficientStock(t *testing.T) {
	app, _ := setupApp()

	resp, _ := postOrder(t, app, model.Item{ID: 1, Qty: 99999})

	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreateOrderReleasesStockWhenPaymentFails(t *testing.T) {
	app, gw := setupApp()
	gw.chargeErr = errors.New("midtrans down")

	resp, _ := postOrder(t, app, model.Item{ID: 1, Qty: 2})

	if resp.StatusCode != fiber.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
	if got := productStock(t, app, 1); got != 10 {
		t.Errorf("stock = %d, want 10 (reserved stock must be released)", got)
	}
}

func TestGetOrderByID(t *testing.T) {
	app, _ := setupApp()

	_, created := postOrder(t, app, model.Item{ID: 1, Qty: 1})

	// The id alone reveals nothing.
	req, _ := http.NewRequest(http.MethodGet, "/orders/"+created.ID.String(), nil)
	anonymous, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if anonymous.StatusCode != fiber.StatusNotFound {
		t.Fatalf("anonymous status = %d, want 404", anonymous.StatusCode)
	}

	// With the guest token only the minimal status is returned.
	path := "/orders/" + created.ID.String() + "?t=" + url.QueryEscape(created.AccessTokenIssued)
	req, _ = http.NewRequest(http.MethodGet, path, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var got struct {
		ID     uuid.UUID         `json:"id"`
		Status model.OrderStatus `json:"status"`
		Total  int               `json:"total"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != created.ID || got.Status != model.OrderStatusPending || got.Total != created.Total {
		t.Errorf("got = %+v", got)
	}

	for _, leaked := range []string{created.Email, created.Phone, created.Address.Street} {
		if strings.Contains(string(body), leaked) {
			t.Errorf("public order response leaks %q", leaked)
		}
	}
}

func TestGetOrdersFilterByStatus(t *testing.T) {
	app, _ := setupApp()

	_, first := postOrder(t, app, model.Item{ID: 1, Qty: 1})
	postOrder(t, app, model.Item{ID: 1, Qty: 1})

	cancelReq, _ := http.NewRequest(http.MethodPost, cancelURL(first), nil)
	if _, err := app.Test(cancelReq); err != nil {
		t.Fatal(err)
	}

	type page struct {
		Items  []model.UserOrder `json:"items"`
		Total  int               `json:"total"`
		Limit  int               `json:"limit"`
		Offset int               `json:"offset"`
	}

	fetch := func(t *testing.T, url string) page {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", url, resp.StatusCode)
		}
		var got page
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		return got
	}

	all := fetch(t, "/orders")
	if all.Total != 2 || len(all.Items) != 2 || all.Limit != 25 || all.Offset != 0 {
		t.Errorf("GET /orders = %+v, want 2 items, total 2, limit 25", all)
	}

	pending := fetch(t, "/orders?status=pending")
	if pending.Total != 1 || len(pending.Items) != 1 || pending.Items[0].Status != model.OrderStatusPending {
		t.Errorf("pending filter returned %+v, want 1 pending order", pending)
	}

	cancelled := fetch(t, "/orders?status=cancelled")
	if cancelled.Total != 1 || len(cancelled.Items) != 1 || cancelled.Items[0].ID != first.ID {
		t.Errorf("cancelled filter returned %+v, want the cancelled order", cancelled)
	}

	// Newest first: the second order was created last.
	second := fetch(t, "/orders?limit=1&offset=0")
	if second.Total != 2 || len(second.Items) != 1 || second.Items[0].ID == first.ID {
		t.Errorf("first page = %+v, want the newest order", second)
	}
	last := fetch(t, "/orders?limit=1&offset=1")
	if last.Total != 2 || len(last.Items) != 1 || last.Items[0].ID != first.ID {
		t.Errorf("second page = %+v, want the cancelled order", last)
	}

	// Out-of-range pages are empty but keep the total.
	beyond := fetch(t, "/orders?limit=1&offset=99")
	if beyond.Total != 2 || len(beyond.Items) != 0 {
		t.Errorf("beyond the last page = %+v, want 0 items and total 2", beyond)
	}

	// Bad bounds are clamped, not rejected.
	clamped := fetch(t, "/orders?limit=999&offset=-5")
	if clamped.Limit != 100 || clamped.Offset != 0 {
		t.Errorf("clamped = limit %d offset %d, want 100/0", clamped.Limit, clamped.Offset)
	}
}

func TestGetOrderStats(t *testing.T) {
	app, _ := setupApp()

	_, first := postOrder(t, app, model.Item{ID: 1, Qty: 1})
	postOrder(t, app, model.Item{ID: 1, Qty: 1})
	cancelReq, _ := http.NewRequest(http.MethodPost, cancelURL(first), nil)
	if _, err := app.Test(cancelReq); err != nil {
		t.Fatal(err)
	}

	req, _ := http.NewRequest(http.MethodGet, "/stats", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var stats struct {
		OrdersTotal int            `json:"orders_total"`
		Revenue     int            `json:"revenue"`
		ByStatus    map[string]int `json:"by_status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		t.Fatal(err)
	}
	if stats.OrdersTotal != 2 {
		t.Errorf("orders_total = %d, want 2", stats.OrdersTotal)
	}
	if stats.ByStatus["pending"] != 1 || stats.ByStatus["cancelled"] != 1 {
		t.Errorf("by_status = %+v", stats.ByStatus)
	}
	if stats.Revenue != 0 {
		t.Errorf("revenue = %d, want 0", stats.Revenue)
	}
}

func TestGetOrderByIDNotFound(t *testing.T) {
	app, _ := setupApp()

	req, _ := http.NewRequest(http.MethodGet, "/orders/00000000-0000-0000-0000-000000000000", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func productStock(t *testing.T, app *fiber.App, id int) int {
	t.Helper()

	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("/products/%d", id), nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}

	var product model.Product
	json.NewDecoder(resp.Body).Decode(&product)
	return product.Stock
}

func TestCancelOrderReleasesStock(t *testing.T) {
	app, _ := setupApp()

	// seed: product 1 has stock 10; ordering 2 reserves it down to 8
	_, created := postOrder(t, app, model.Item{ID: 1, Qty: 2})
	if got := productStock(t, app, 1); got != 8 {
		t.Fatalf("stock after order = %d, want 8", got)
	}

	req, _ := http.NewRequest(http.MethodPost, cancelURL(created), nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("cancel status = %d, want 200", resp.StatusCode)
	}

	var cancelled model.UserOrder
	json.NewDecoder(resp.Body).Decode(&cancelled)
	if cancelled.Status != model.OrderStatusCancelled {
		t.Errorf("status = %q, want %q", cancelled.Status, model.OrderStatusCancelled)
	}
	if got := productStock(t, app, 1); got != 10 {
		t.Errorf("stock after cancel = %d, want 10 (released)", got)
	}

	// cancelling twice must not release the stock again
	req2, _ := http.NewRequest(http.MethodPost, cancelURL(created), nil)
	resp2, err := app.Test(req2)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != fiber.StatusConflict {
		t.Fatalf("second cancel status = %d, want 409", resp2.StatusCode)
	}
	if got := productStock(t, app, 1); got != 10 {
		t.Errorf("stock after second cancel = %d, want 10 (not released twice)", got)
	}
}

func TestNotificationSettlementMarksPaid(t *testing.T) {
	app, gw := setupApp()

	_, created := postOrder(t, app, model.Item{ID: 1, Qty: 2})

	gw.status = &payment.StatusResult{
		TransactionStatus: "settlement",
		FraudStatus:       "accept",
		GrossAmount:       "2000.00", // Midtrans sends a decimal string
		PaymentType:       "bank_transfer",
		Bank:              "bca",
		VANumber:          "12345678",
		TransactionID:     "tx-1",
		SettlementTime:    "2026-10-12 08:00:00",
	}
	if resp := notify(t, app, created.ID); resp.StatusCode != fiber.StatusOK {
		t.Fatalf("notification status = %d, want 200", resp.StatusCode)
	}

	got := getOrder(t, app, created)
	if got.Status != model.OrderStatusPaid {
		t.Errorf("status = %q, want %q", got.Status, model.OrderStatusPaid)
	}
	if got.Payment == nil {
		t.Fatal("payment info was not recorded")
	}
	if got.Payment.Type != "bank_transfer" || got.Payment.Bank != "bca" || got.Payment.VANumber != "12345678" {
		t.Errorf("payment = %+v, want bank_transfer/bca/12345678", got.Payment)
	}
	// stock stays reserved (already decremented at order creation)
	if stock := productStock(t, app, 1); stock != 8 {
		t.Errorf("stock = %d, want 8 (no double decrement)", stock)
	}
}

func TestLatePaymentAfterCancelIsAccepted(t *testing.T) {
	app, gw := setupApp()

	_, created := postOrder(t, app, model.Item{ID: 1, Qty: 2}) // stock 10 -> 8

	// cancelling releases the stock back to 10
	cancelReq, _ := http.NewRequest(http.MethodPost, cancelURL(created), nil)
	if resp, err := app.Test(cancelReq); err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("cancel failed: %v", err)
	}
	if got := productStock(t, app, 1); got != 10 {
		t.Fatalf("stock after cancel = %d, want 10", got)
	}

	// payment arrives after the order was cancelled; stock is available again
	gw.status = &payment.StatusResult{TransactionStatus: "settlement", GrossAmount: "2000.00", PaymentType: "qris"}
	if resp := notify(t, app, created.ID); resp.StatusCode != fiber.StatusOK {
		t.Fatalf("notification status = %d, want 200", resp.StatusCode)
	}

	got := getOrder(t, app, created)
	if got.Status != model.OrderStatusPaid {
		t.Errorf("status = %q, want %q", got.Status, model.OrderStatusPaid)
	}
	if stock := productStock(t, app, 1); stock != 8 {
		t.Errorf("stock = %d, want 8 (re-reserved)", stock)
	}
}

func TestLatePaymentWithoutStockNeedsRefund(t *testing.T) {
	app, gw := setupApp()

	_, first := postOrder(t, app, model.Item{ID: 1, Qty: 10}) // stock 10 -> 0
	cancelReq, _ := http.NewRequest(http.MethodPost, cancelURL(first), nil)
	if _, err := app.Test(cancelReq); err != nil {
		t.Fatal(err)
	}

	// another order takes all the stock
	if _, second := postOrder(t, app, model.Item{ID: 1, Qty: 10}); second.ID == uuid.Nil {
		t.Fatal("second order failed")
	}

	// the cancelled order is paid late, but there is no stock left to serve it
	gw.status = &payment.StatusResult{TransactionStatus: "settlement", GrossAmount: "10000.00", PaymentType: "qris"}
	if resp := notify(t, app, first.ID); resp.StatusCode != fiber.StatusOK {
		t.Fatalf("notification status = %d, want 200", resp.StatusCode)
	}

	got := getOrder(t, app, first)
	if got.Status != model.OrderStatusNeedsRefund {
		t.Errorf("status = %q, want %q", got.Status, model.OrderStatusNeedsRefund)
	}
}

func TestDownloadInvoice(t *testing.T) {
	app, _ := setupApp()

	_, created := postOrder(t, app, model.Item{ID: 1, Qty: 1})

	// Without the guest token the receipt is not available.
	req, _ := http.NewRequest(http.MethodGet, "/orders/"+created.ID.String()+"/invoice.pdf", nil)
	anonymous, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if anonymous.StatusCode != fiber.StatusNotFound {
		t.Fatalf("anonymous invoice status = %d, want 404", anonymous.StatusCode)
	}

	path := "/orders/" + created.ID.String() + "/invoice.pdf?t=" + url.QueryEscape(created.AccessTokenIssued)
	req, _ = http.NewRequest(http.MethodGet, path, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("content type = %q", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "invoice-") {
		t.Errorf("content disposition = %q", cd)
	}
}

func TestPaidEmailIncludesInvoiceAttachment(t *testing.T) {
	app, gw, mailer := buildApp()

	_, created := postOrder(t, app, model.Item{ID: 1, Qty: 1})

	gw.status = &payment.StatusResult{
		TransactionStatus: "settlement",
		GrossAmount:       "1000.00",
		PaymentType:       "qris",
	}
	notify(t, app, created.ID)

	if len(mailer.sent) != 2 {
		t.Fatalf("emails = %d, want 2", len(mailer.sent))
	}

	attachments := mailer.sent[1].Attachments
	if len(attachments) != 1 {
		t.Fatalf("paid email attachments = %d, want 1", len(attachments))
	}
	if attachments[0].ContentType != "application/pdf" {
		t.Errorf("attachment type = %q, want application/pdf", attachments[0].ContentType)
	}
	if !bytes.HasPrefix(attachments[0].Data, []byte("%PDF")) {
		t.Error("attachment is not a PDF")
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestCreateOrderWithShipping(t *testing.T) {
	app, gw, _ := buildApp()

	order := validOrder(model.Item{ID: 1, Qty: 1})
	order.Courier = "jne"
	order.CourierService = "reg"
	// a tampered client price must be ignored
	order.ShippingCost = 1
	order.Total = 1

	resp, created := postOrderModel(t, app, order)
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}

	if created.ShippingCost != 25000 {
		t.Errorf("shipping_cost = %d, want 25000 (from the provider)", created.ShippingCost)
	}
	if created.Courier != "jne" || created.CourierService != "reg" {
		t.Errorf("courier = %q/%q", created.Courier, created.CourierService)
	}
	// product 1000 + shipping 25000
	if created.Total != 26000 {
		t.Errorf("total = %d, want 26000", created.Total)
	}
	if gw.charged == nil || gw.charged.Total != 26000 {
		t.Errorf("gateway charged %+v, want total 26000", gw.charged)
	}
}

func TestCreateOrderRejectsUnknownShipping(t *testing.T) {
	app, _, _ := buildApp()

	order := validOrder(model.Item{ID: 1, Qty: 1})
	order.Courier = "ninja"
	order.CourierService = "same-day"

	resp, _ := postOrderModel(t, app, order)
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestCreateOrderWithoutCourierHasNoShipping(t *testing.T) {
	app, _, _ := buildApp()

	resp, created := postOrderModel(t, app, validOrder(model.Item{ID: 1, Qty: 1}))
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	if created.ShippingCost != 0 || created.Total != 1000 {
		t.Errorf("shipping = %d, total = %d; want 0 and 1000", created.ShippingCost, created.Total)
	}
}

func TestCreateOrderShippingFailureReturns502(t *testing.T) {
	provider := &fakeShippingProvider{err: errors.New("provider down")}
	app, _, _ := buildAppWithShipping(provider)

	order := validOrder(model.Item{ID: 1, Qty: 1})
	order.Courier = "jne"
	order.CourierService = "reg"

	resp, _ := postOrderModel(t, app, order)
	if resp.StatusCode != fiber.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
	// stock must be released when the order could not be created
	if stock := productStock(t, app, 1); stock != 10 {
		t.Errorf("stock = %d, want 10 (released)", stock)
	}
}

func shippingNotify(t *testing.T, app *fiber.App, providerID string) *http.Response {
	t.Helper()

	body, _ := json.Marshal(map[string]string{
		"event":    "order.status",
		"order_id": providerID,
		"status":   "picked",
	})
	req, _ := http.NewRequest(http.MethodPost, "/shipping/notification", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestOrderPageAndTrack(t *testing.T) {
	app, _, _ := buildApp()

	_, created := postOrder(t, app, model.Item{ID: 1, Qty: 1})

	// the public order page renders the order + status
	req, _ := http.NewRequest(http.MethodGet, "/order/"+created.ID.String()+"?t="+url.QueryEscape(created.AccessTokenIssued), nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("order page status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	if !strings.Contains(html, created.ID.String()[:8]) {
		t.Error("order page does not show the order id")
	}
	if !strings.Contains(html, "Waiting for payment") {
		t.Error("order page does not show the pending status")
	}

	// the tracking form renders
	req, _ = http.NewRequest(http.MethodGet, "/track", nil)
	if resp, err = app.Test(req); err != nil || resp.StatusCode != fiber.StatusOK {
		t.Fatalf("track page = %v / %d, want 200", err, resp.StatusCode)
	}

	submit := func(email string) *http.Response {
		form := url.Values{"order_id": {created.ID.String()}, "email": {email}}
		req, _ := http.NewRequest(http.MethodPost, "/track", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	// matching email -> redirected to the order page
	wantLocation := "/order/" + created.ID.String() + "?t=" + url.QueryEscape(created.AccessTokenIssued)
	if resp := submit("b@mail.com"); resp.Header.Get("Location") != wantLocation {
		t.Errorf("location = %q, want the order page", resp.Header.Get("Location"))
	}

	// wrong email -> back to the form with an error
	if resp := submit("someone-else@example.com"); resp.Header.Get("Location") != "/track?error=1" {
		t.Errorf("location = %q, want the error page", resp.Header.Get("Location"))
	}
}

func TestShipmentUsesRecipientDetails(t *testing.T) {
	provider := &fakeShippingProvider{rates: testRates}
	app, gw, _ := buildAppWithShipping(provider)

	order := validOrder(model.Item{ID: 1, Qty: 1})
	order.Courier = "jne"
	order.CourierService = "reg"
	order.Username = "Kazi" // the customer placing the order
	order.Phone = "0811000000"
	order.Address.RecipientName = "Budi Santoso" // the person receiving it
	order.Address.RecipientPhone = "0822000000"
	order.Address.Line2 = "Apt 5B"

	_, created := postOrderModel(t, app, order)

	if created.Address.RecipientName != "Budi Santoso" {
		t.Errorf("recipient not stored: %+v", created.Address)
	}

	gw.status = &payment.StatusResult{TransactionStatus: "settlement", GrossAmount: "26000.00", PaymentType: "qris"}
	notify(t, app, created.ID)

	if len(provider.shipmentReqs) != 1 {
		t.Fatalf("shipment requests = %d, want 1", len(provider.shipmentReqs))
	}
	req := provider.shipmentReqs[0]
	if req.ContactName != "Budi Santoso" {
		t.Errorf("shipment contact name = %q, want the recipient", req.ContactName)
	}
	if req.ContactPhone != "0822000000" {
		t.Errorf("shipment contact phone = %q, want the recipient phone", req.ContactPhone)
	}
	if !strings.Contains(req.Address, "Apt 5B") {
		t.Errorf("shipment address = %q, want it to include the unit line", req.Address)
	}

	// receipts still go to the customer who ordered
	if len(created.Email) == 0 || created.Email != "b@mail.com" {
		t.Errorf("order email = %q", created.Email)
	}
}

func TestShipmentFallsBackToCustomerWhenNoRecipient(t *testing.T) {
	provider := &fakeShippingProvider{rates: testRates}
	app, gw, _ := buildAppWithShipping(provider)

	order := validOrder(model.Item{ID: 1, Qty: 1})
	order.Courier = "jne"
	order.CourierService = "reg"
	_, created := postOrderModel(t, app, order)

	gw.status = &payment.StatusResult{TransactionStatus: "settlement", GrossAmount: "26000.00", PaymentType: "qris"}
	notify(t, app, created.ID)

	req := provider.shipmentReqs[0]
	if req.ContactName != "budi" {
		t.Errorf("contact name = %q, want the customer fallback", req.ContactName)
	}
	if req.ContactPhone != "0812" {
		t.Errorf("contact phone = %q, want the customer fallback", req.ContactPhone)
	}
}

func TestShippingNotificationUpdatesStatus(t *testing.T) {
	provider := &fakeShippingProvider{rates: testRates}
	app, gw, mailer := buildAppWithShipping(provider)

	order := validOrder(model.Item{ID: 1, Qty: 1})
	order.Courier = "jne"
	order.CourierService = "reg"
	_, created := postOrderModel(t, app, order)

	gw.status = &payment.StatusResult{TransactionStatus: "settlement", GrossAmount: "26000.00", PaymentType: "qris"}
	notify(t, app, created.ID)

	providerID := "bite-" + created.ID.String()[:8]

	// the payload is only a trigger; the real state comes from Tracking()
	provider.tracking = &shipping.Shipment{
		ID: providerID, WaybillID: "WYB-123", TrackingID: "TRK-123", Status: "picked",
		History: []shipping.ShipmentEvent{{Status: "picked", Note: "Courier picked up the parcel"}},
	}
	if resp := shippingNotify(t, app, providerID); resp.StatusCode != fiber.StatusOK {
		t.Fatalf("notification status = %d, want 200", resp.StatusCode)
	}

	got := getOrder(t, app, created)
	if got.Shipment == nil || got.Shipment.Status != "picked" {
		t.Fatalf("shipment = %+v, want status picked", got.Shipment)
	}
	if len(got.Shipment.History) != 1 || got.Shipment.History[0].Note != "Courier picked up the parcel" {
		t.Errorf("history = %+v", got.Shipment.History)
	}

	// delivered notifies the customer exactly once
	provider.tracking = &shipping.Shipment{ID: providerID, WaybillID: "WYB-123", Status: "delivered"}
	before := len(mailer.sent)
	shippingNotify(t, app, providerID)
	shippingNotify(t, app, providerID)

	if got := len(mailer.sent); got != before+1 {
		t.Errorf("emails after delivery = %d, want %d (delivered email sent once)", got, before+1)
	}
	if subject := mailer.sent[len(mailer.sent)-1].Subject; !strings.Contains(subject, "delivered") {
		t.Errorf("last email subject = %q, want it to mention delivery", subject)
	}
}

func TestShippingNotificationUnknownProvider(t *testing.T) {
	app, _, _ := buildApp()

	if resp := shippingNotify(t, app, "unknown-provider-id"); resp.StatusCode != fiber.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestPaidOrderCreatesShipment(t *testing.T) {
	provider := &fakeShippingProvider{rates: testRates}
	app, gw, _ := buildAppWithShipping(provider)

	order := validOrder(model.Item{ID: 1, Qty: 1})
	order.Courier = "jne"
	order.CourierService = "reg"
	_, created := postOrderModel(t, app, order)

	// product 1000 + shipping 25000
	gw.status = &payment.StatusResult{TransactionStatus: "settlement", GrossAmount: "26000.00", PaymentType: "qris"}
	notify(t, app, created.ID)

	if len(provider.shipmentReqs) != 1 {
		t.Fatalf("shipment requests = %d, want 1", len(provider.shipmentReqs))
	}
	req := provider.shipmentReqs[0]
	if req.ReferenceID != created.ID.String() {
		t.Errorf("reference_id = %q, want the order id", req.ReferenceID)
	}
	if req.Courier != "jne" || req.Service != "reg" {
		t.Errorf("courier = %q/%q", req.Courier, req.Service)
	}
	if len(req.Items) != 1 || req.Items[0].WeightGram != 1500 {
		t.Errorf("items = %+v, want the snapshotted weight", req.Items)
	}

	got := getOrder(t, app, created)
	if got.Shipment == nil {
		t.Fatal("shipment was not stored on the order")
	}
	if got.Shipment.WaybillID != "WYB-123" || got.Shipment.Status != "confirmed" {
		t.Errorf("shipment = %+v", got.Shipment)
	}
}

func TestShipmentFailureKeepsOrderPaid(t *testing.T) {
	provider := &fakeShippingProvider{rates: testRates, shipmentErr: errors.New("biteship down")}
	app, gw, _ := buildAppWithShipping(provider)

	order := validOrder(model.Item{ID: 1, Qty: 1})
	order.Courier = "jne"
	order.CourierService = "reg"
	_, created := postOrderModel(t, app, order)

	gw.status = &payment.StatusResult{TransactionStatus: "settlement", GrossAmount: "26000.00", PaymentType: "qris"}
	if resp := notify(t, app, created.ID); resp.StatusCode != fiber.StatusOK {
		t.Fatalf("notification status = %d, want 200", resp.StatusCode)
	}

	got := getOrder(t, app, created)
	if got.Status != model.OrderStatusPaid {
		t.Errorf("status = %q, want paid (shipment failure must not undo payment)", got.Status)
	}
	if got.Shipment != nil {
		t.Errorf("shipment = %+v, want nil", got.Shipment)
	}
}

func TestOrderEmails(t *testing.T) {
	app, gw, mailer := buildApp()

	_, created := postOrder(t, app, model.Item{ID: 1, Qty: 1})

	if len(mailer.sent) != 1 {
		t.Fatalf("emails after create = %d, want 1", len(mailer.sent))
	}
	if mailer.sent[0].To != "b@mail.com" {
		t.Errorf("email to = %q, want b@mail.com", mailer.sent[0].To)
	}
	if !strings.Contains(mailer.sent[0].Subject, created.ID.String()[:8]) {
		t.Errorf("subject = %q, want it to contain the short order id", mailer.sent[0].Subject)
	}

	// payment confirmed -> second email
	gw.status = &payment.StatusResult{
		TransactionStatus: "settlement",
		GrossAmount:       "1000.00",
		PaymentType:       "qris",
	}
	notify(t, app, created.ID)

	if len(mailer.sent) != 2 {
		t.Fatalf("emails after payment = %d, want 2", len(mailer.sent))
	}
	if !strings.Contains(strings.ToLower(mailer.sent[1].Subject), "payment received") {
		t.Errorf("paid email subject = %q", mailer.sent[1].Subject)
	}

	// a duplicate webhook must not send the paid email again
	notify(t, app, created.ID)
	if len(mailer.sent) != 2 {
		t.Errorf("duplicate notification sent another email (total %d)", len(mailer.sent))
	}
}

func TestNotificationAmountMismatchIsRejected(t *testing.T) {
	app, gw := setupApp()

	_, created := postOrder(t, app, model.Item{ID: 1, Qty: 2}) // total 2000

	// gateway reports a smaller amount -> must not be marked as paid
	gw.status = &payment.StatusResult{TransactionStatus: "settlement", GrossAmount: "1"}
	if resp := notify(t, app, created.ID); resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("notification status = %d, want 400", resp.StatusCode)
	}

	got := getOrder(t, app, created)
	if got.Status != model.OrderStatusPending {
		t.Errorf("status = %q, want %q", got.Status, model.OrderStatusPending)
	}
}

func TestNotificationExpireReleasesStock(t *testing.T) {
	app, gw := setupApp()

	_, created := postOrder(t, app, model.Item{ID: 1, Qty: 2})

	gw.status = &payment.StatusResult{TransactionStatus: "expire"}
	if resp := notify(t, app, created.ID); resp.StatusCode != fiber.StatusOK {
		t.Fatalf("notification status = %d, want 200", resp.StatusCode)
	}

	got := getOrder(t, app, created)
	if got.Status != model.OrderStatusExpired {
		t.Errorf("status = %q, want %q", got.Status, model.OrderStatusExpired)
	}
	if stock := productStock(t, app, 1); stock != 10 {
		t.Errorf("stock = %d, want 10 (released)", stock)
	}
}
