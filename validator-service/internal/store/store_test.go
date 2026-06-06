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
