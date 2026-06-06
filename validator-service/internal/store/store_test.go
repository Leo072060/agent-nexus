package store

import (
	"context"
	"math/big"
	"path/filepath"
	"testing"
)

func TestUpsertAndGetDispute(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "validator-service.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	orderID := big.NewInt(12)
	_, err = s.UpsertDispute(context.Background(), Dispute{
		ChainOrderID:     orderID,
		BuyerAddress:     "0xbuyer",
		SellerAddress:    "0xseller",
		ValidatorAddress: "0xvalidator",
		RequestHash:      "0xrequest",
		RequestBody:      []byte("request"),
		DeliveryHash:     "0xdelivery",
		DeliveryBody:     []byte("delivery"),
		DisputeHash:      "0xdispute",
		DisputeBody:      []byte("dispute"),
		Status:           "evidence_received",
	})
	if err != nil {
		t.Fatal(err)
	}

	dispute, err := s.GetDispute(context.Background(), orderID)
	if err != nil {
		t.Fatal(err)
	}
	if dispute.ChainOrderID.Cmp(orderID) != 0 {
		t.Fatalf("order id mismatch: %s", dispute.ChainOrderID.String())
	}
	if string(dispute.RequestBody) != "request" {
		t.Fatalf("request body mismatch: %s", string(dispute.RequestBody))
	}
	if dispute.Status != "evidence_received" {
		t.Fatalf("status mismatch: %s", dispute.Status)
	}
}

func TestListDisputes(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "validator-service.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	empty, err := s.ListDisputes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected empty list, got %d", len(empty))
	}

	// Insert out of order to verify numeric ordering (10 before 2 lexically).
	for _, id := range []int64{10, 2} {
		if _, err := s.UpsertDispute(context.Background(), Dispute{
			ChainOrderID:     big.NewInt(id),
			BuyerAddress:     "0xbuyer",
			SellerAddress:    "0xseller",
			ValidatorAddress: "0xvalidator",
			RequestHash:      "0xrequest",
			RequestBody:      []byte("request"),
			DeliveryHash:     "0xdelivery",
			DeliveryBody:     []byte("delivery"),
			DisputeHash:      "0xdispute",
			DisputeBody:      []byte("dispute"),
			Status:           "evidence_received",
		}); err != nil {
			t.Fatal(err)
		}
	}

	disputes, err := s.ListDisputes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(disputes) != 2 {
		t.Fatalf("expected 2 disputes, got %d", len(disputes))
	}
	if disputes[0].ChainOrderID.Int64() != 2 || disputes[1].ChainOrderID.Int64() != 10 {
		t.Fatalf("expected numeric ascending order [2 10], got [%s %s]", disputes[0].ChainOrderID, disputes[1].ChainOrderID)
	}
	if string(disputes[0].RequestBody) != "request" {
		t.Fatalf("request body mismatch: %s", string(disputes[0].RequestBody))
	}
}
