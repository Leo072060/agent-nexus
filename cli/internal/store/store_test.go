package store

import (
	"context"
	"math/big"
	"path/filepath"
	"testing"
)

func TestCreateOrder(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "agent-nexus.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	order, err := s.CreateOrder(context.Background(), CreateOrderInput{
		RPCURL:         "http://127.0.0.1:8545",
		MarketAddress:  "0x1111111111111111111111111111111111111111",
		ChainOrderID:   big.NewInt(12),
		BuyerAddress:   "0x2222222222222222222222222222222222222222",
		SellerURI:      "http://localhost:8083",
		ValidatorURI:   "http://localhost:8082",
		RequestContent: "hello",
		Status:         "PendingSeller",
	})
	if err != nil {
		t.Fatal(err)
	}
	if order.ID != 1 {
		t.Fatalf("order id mismatch: %d", order.ID)
	}
	if order.ChainOrderID.String() != "12" {
		t.Fatalf("chain order id mismatch: %s", order.ChainOrderID.String())
	}

	got, err := s.GetOrder(context.Background(), order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.BuyerAddress != "0x2222222222222222222222222222222222222222" {
		t.Fatalf("buyer mismatch: %s", got.BuyerAddress)
	}
	if got.ValidatorURI != "http://localhost:8082" {
		t.Fatalf("validator URI mismatch: %s", got.ValidatorURI)
	}
	if got.RequestContent != "hello" {
		t.Fatalf("request mismatch: %s", got.RequestContent)
	}

	if err := s.UpdateOrderStatus(context.Background(), order.ID, "SellerConfirmed"); err != nil {
		t.Fatal(err)
	}
	updated, err := s.GetOrder(context.Background(), order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "SellerConfirmed" {
		t.Fatalf("status mismatch: %s", updated.Status)
	}
}

func TestGetAndUpdateOrderDelivery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "agent-nexus.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	_, err = s.db.ExecContext(
		context.Background(),
		`INSERT INTO orders (rpc_url, market_address, chain_order_id, seller_uri, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"http://127.0.0.1:8545",
		"0x1111111111111111111111111111111111111111",
		"12",
		"https://seller.example.com",
		"DeliveryCommitted",
		"2026-06-06T00:00:00Z",
		"2026-06-06T00:00:00Z",
	)
	if err != nil {
		t.Fatal(err)
	}

	order, err := s.GetOrder(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if order.ChainOrderID.String() != "12" {
		t.Fatalf("chain order id mismatch: %s", order.ChainOrderID.String())
	}

	err = s.UpdateOrderDelivery(context.Background(), 1, "0xabc", "delivery body", "DeliveryReceived")
	if err != nil {
		t.Fatal(err)
	}

	updated, err := s.GetOrder(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if updated.DeliveryHash != "0xabc" {
		t.Fatalf("delivery hash mismatch: %s", updated.DeliveryHash)
	}
	if updated.Delivery != "delivery body" {
		t.Fatalf("delivery mismatch: %s", updated.Delivery)
	}
	if updated.Status != "DeliveryReceived" {
		t.Fatalf("status mismatch: %s", updated.Status)
	}
}
