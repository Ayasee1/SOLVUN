package invoice

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/uuid"

	"golearn/model"
)

func TestGenerateReturnsPDF(t *testing.T) {
	order := model.UserOrder{
		ID:        uuid.New(),
		Username:  "Budi",
		Email:     "budi@example.com",
		Phone:     "081234567890",
		OrderItem: []model.Item{{ID: 1, NameItem: "Laptop", Price: 1000, Qty: 2, TotalPrice: 2000}},
		Address:   model.Address{Street: "Jl Mawar", City: "Bandung", State: "Jabar", ZipCode: "40123"},
		Total:     2000,
		Status:    model.OrderStatusPaid,
		CreatedAt: time.Now(),
		Payment:   &model.PaymentInfo{Type: "qris", TransactionID: "tx-1", SettledAt: "2026-10-12 09:00:00"},
	}

	pdf, err := Generate(order)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pdf) == 0 {
		t.Fatal("generated an empty PDF")
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Errorf("missing PDF magic bytes, got %q", pdf[:min(4, len(pdf))])
	}
}

func TestGenerateWithoutPayment(t *testing.T) {
	order := model.UserOrder{
		ID:        uuid.New(),
		Username:  "Budi",
		OrderItem: []model.Item{{ID: 1, NameItem: "Laptop", Price: 1000, Qty: 1, TotalPrice: 1000}},
		Total:     1000,
		Status:    model.OrderStatusPending,
		CreatedAt: time.Now(),
	}

	if _, err := Generate(order); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
