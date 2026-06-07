package store

import (
	"context"
	"math/big"
	"path/filepath"
	"testing"
)

func TestMarkDeliveryCommittedStoresDeliveryOnOrder(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "seller-service.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	orderID := big.NewInt(12)
	body := []byte("approved")
	evidence := []byte("request matched answer")
	_, err = s.UpsertOrderRequest(
		context.Background(),
		orderID,
		"0xbuyer",
		"0xseller",
		"0xvalidator",
		"0xrequest",
		[]byte("review this contract"),
		"seller_confirmed",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkDeliveryCommitted(context.Background(), orderID, "0xabc", body, "0xevidence", evidence, "0xcommit"); err != nil {
		t.Fatal(err)
	}

	order, err := s.GetOrder(context.Background(), orderID)
	if err != nil {
		t.Fatal(err)
	}
	if order.ChainOrderID.Cmp(orderID) != 0 {
		t.Fatalf("order id mismatch: %s", order.ChainOrderID.String())
	}
	if order.DeliveryHash != "0xabc" {
		t.Fatalf("delivery hash mismatch: %s", order.DeliveryHash)
	}
	if string(order.DeliveryBody) != string(body) {
		t.Fatalf("delivery body mismatch: %s", string(order.DeliveryBody))
	}
	if order.CommitDeliveryTxHash != "0xcommit" {
		t.Fatalf("commit tx mismatch: %s", order.CommitDeliveryTxHash)
	}
	if order.EvidenceHash != "0xevidence" {
		t.Fatalf("evidence hash mismatch: %s", order.EvidenceHash)
	}
	if string(order.EvidenceBody) != string(evidence) {
		t.Fatalf("evidence body mismatch: %s", string(order.EvidenceBody))
	}
	if err := s.MarkEvidencePosted(context.Background(), orderID, 202, "accepted"); err != nil {
		t.Fatal(err)
	}
	order, err = s.GetOrder(context.Background(), orderID)
	if err != nil {
		t.Fatal(err)
	}
	if order.EvidenceSentAt == "" {
		t.Fatalf("expected evidence sent timestamp")
	}
	if order.EvidencePostStatus != 202 {
		t.Fatalf("evidence post status mismatch: %d", order.EvidencePostStatus)
	}
	if order.EvidencePostResponse != "accepted" {
		t.Fatalf("evidence post response mismatch: %s", order.EvidencePostResponse)
	}
}

func TestMarkDeliveryGeneratedStoresDeliveryWithoutCommitTx(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "seller-service.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	orderID := big.NewInt(12)
	body := []byte("approved")
	evidence := []byte("request matched answer")
	_, err = s.UpsertOrderRequest(
		context.Background(),
		orderID,
		"0xbuyer",
		"0xseller",
		"0xvalidator",
		"0xrequest",
		[]byte("review this contract"),
		"seller_confirmed",
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkDeliveryGenerated(context.Background(), orderID, "0xabc", body, "0xevidence", evidence); err != nil {
		t.Fatal(err)
	}

	order, err := s.GetOrder(context.Background(), orderID)
	if err != nil {
		t.Fatal(err)
	}
	if order.DeliveryHash != "0xabc" {
		t.Fatalf("delivery hash mismatch: %s", order.DeliveryHash)
	}
	if string(order.DeliveryBody) != string(body) {
		t.Fatalf("delivery body mismatch: %s", string(order.DeliveryBody))
	}
	if order.EvidenceHash != "0xevidence" {
		t.Fatalf("evidence hash mismatch: %s", order.EvidenceHash)
	}
	if string(order.EvidenceBody) != string(evidence) {
		t.Fatalf("evidence body mismatch: %s", string(order.EvidenceBody))
	}
	if order.CommitDeliveryTxHash != "" {
		t.Fatalf("commit tx should be empty: %s", order.CommitDeliveryTxHash)
	}
	if order.Status != "delivery_generated" {
		t.Fatalf("status mismatch: %s", order.Status)
	}
}

func TestUpsertOrderRequestAndMarkSellerConfirmed(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "seller-service.db"))
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
		[]byte("review this contract"),
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
	if string(order.RequestBody) != "review this contract" {
		t.Fatalf("request body mismatch: %s", string(order.RequestBody))
	}
	if order.ConfirmSellerTxHash != "0xtx" {
		t.Fatalf("confirm tx mismatch: %s", order.ConfirmSellerTxHash)
	}
}
