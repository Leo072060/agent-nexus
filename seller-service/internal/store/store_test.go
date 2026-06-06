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
