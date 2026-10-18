package repository

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"golearn/model"
)

func TestTransitionStatus(t *testing.T) {
	r := NewMemoryOrderRepository()
	order := model.UserOrder{ID: uuid.New(), Status: model.OrderStatusPending}
	if err := r.Save(order); err != nil {
		t.Fatal(err)
	}

	got, ok, err := r.TransitionStatus(order.ID, model.OrderStatusPending, model.OrderStatusCancelled)
	if err != nil || !ok {
		t.Fatalf("ok = %v, err = %v; want true, nil", ok, err)
	}
	if got.Status != model.OrderStatusCancelled {
		t.Errorf("status = %q, want %q", got.Status, model.OrderStatusCancelled)
	}

	// repeating the same transition must not happen (idempotency guard)
	if _, ok, err := r.TransitionStatus(order.ID, model.OrderStatusPending, model.OrderStatusCancelled); err != nil || ok {
		t.Errorf("second transition: ok = %v, err = %v; want false, nil", ok, err)
	}

	// unknown order
	if _, _, err := r.TransitionStatus(uuid.New(), model.OrderStatusPending, model.OrderStatusCancelled); !errors.Is(err, ErrOrderNotFound) {
		t.Errorf("err = %v, want ErrOrderNotFound", err)
	}
}

func TestTransitionStatusWithPayment(t *testing.T) {
	r := NewMemoryOrderRepository()
	order := model.UserOrder{ID: uuid.New(), Status: model.OrderStatusPending, Total: 2000}
	if err := r.Save(order); err != nil {
		t.Fatal(err)
	}

	info := model.PaymentInfo{Type: "bank_transfer", Bank: "bca", VANumber: "123", TransactionID: "tx-1"}
	got, ok, err := r.TransitionStatusWithPayment(order.ID, model.OrderStatusPending, model.OrderStatusPaid, info)
	if err != nil || !ok {
		t.Fatalf("ok = %v, err = %v; want true, nil", ok, err)
	}
	if got.Status != model.OrderStatusPaid {
		t.Errorf("status = %q, want %q", got.Status, model.OrderStatusPaid)
	}
	if got.Payment == nil || got.Payment.Bank != "bca" {
		t.Errorf("payment = %+v, want bank bca", got.Payment)
	}

	// confirming twice must not overwrite or re-apply (idempotency)
	if _, ok, err := r.TransitionStatusWithPayment(order.ID, model.OrderStatusPending, model.OrderStatusPaid, model.PaymentInfo{Type: "qris"}); err != nil || ok {
		t.Errorf("second confirm: ok = %v, err = %v; want false, nil", ok, err)
	}
	if r.orders[0].Payment.Type != "bank_transfer" {
		t.Errorf("payment type = %q, want bank_transfer (not overwritten)", r.orders[0].Payment.Type)
	}
}
