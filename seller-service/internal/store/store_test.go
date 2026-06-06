package store

import (
	"context"
	"math/big"
	"path/filepath"
	"testing"
)

func TestUpsertAndGetDelivery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "seller-service.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	orderID := big.NewInt(12)
	body := []byte("delivery body")
	_, err = s.UpsertDelivery(context.Background(), orderID, "0xabc", body)
	if err != nil {
		t.Fatal(err)
	}

	delivery, err := s.GetDelivery(context.Background(), orderID)
	if err != nil {
		t.Fatal(err)
	}
	if delivery.ChainOrderID.Cmp(orderID) != 0 {
		t.Fatalf("order id mismatch: %s", delivery.ChainOrderID.String())
	}
	if delivery.DeliveryHash != "0xabc" {
		t.Fatalf("delivery hash mismatch: %s", delivery.DeliveryHash)
	}
	if string(delivery.DeliveryBody) != string(body) {
		t.Fatalf("delivery body mismatch: %s", string(delivery.DeliveryBody))
	}
}

func TestUpsertOrderRequestAndMarkSellerConfirmed(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "seller-service.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	orderID := big.NewInt(12)
	_, err = s.UpsertOrderRequest(
		context.Background(),
		orderID,
		"0xbuyer",
		"0xseller",
		"0xvalidator",
		"0xrequest",
		[]byte("request body"),
		"request_received",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkSellerConfirmed(context.Background(), orderID, "0xtx"); err != nil {
		t.Fatal(err)
	}

	order, err := s.GetOrder(context.Background(), orderID)
	if err != nil {
		t.Fatal(err)
	}
	if string(order.RequestBody) != "request body" {
		t.Fatalf("request body mismatch: %s", string(order.RequestBody))
	}
	if order.ConfirmSellerTxHash != "0xtx" {
		t.Fatalf("confirm tx mismatch: %s", order.ConfirmSellerTxHash)
	}
}
