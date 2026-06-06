package watcher

import (
	"context"
	"math/big"
	"testing"
	"time"

	"agent-nexus-seller-service/internal/chain"
	agentcrypto "agent-nexus-seller-service/internal/crypto"
	"agent-nexus-seller-service/internal/store"

	"github.com/ethereum/go-ethereum/common"
)

type fakeMarket struct {
	order          chain.Order
	confirmedOrder *big.Int
	committedOrder *big.Int
	committedHash  [32]byte
}

func (f *fakeMarket) GetOrderCount(context.Context) (*big.Int, error) {
	return big.NewInt(1), nil
}

func (f *fakeMarket) GetOrder(context.Context, *big.Int) (chain.Order, error) {
	return f.order, nil
}

func (f *fakeMarket) ConfirmAsSeller(_ context.Context, orderID *big.Int) (string, error) {
	f.confirmedOrder = new(big.Int).Set(orderID)
	return "0xconfirm", nil
}

func (f *fakeMarket) CommitDelivery(_ context.Context, orderID *big.Int, deliveryHash [32]byte) (string, error) {
	f.committedOrder = new(big.Int).Set(orderID)
	f.committedHash = deliveryHash
	return "0xcommit", nil
}

type fakeStore struct {
	order store.Order
}

func (f *fakeStore) GetOrder(context.Context, *big.Int) (store.Order, error) {
	return f.order, nil
}

func (f *fakeStore) MarkSellerConfirmed(_ context.Context, _ *big.Int, txHash string) error {
	f.order.ConfirmSellerTxHash = txHash
	return nil
}

func (f *fakeStore) MarkDeliveryCommitted(_ context.Context, _ *big.Int, deliveryHash string, deliveryBody []byte, txHash string) error {
	f.order.DeliveryHash = deliveryHash
	f.order.DeliveryBody = deliveryBody
	f.order.CommitDeliveryTxHash = txHash
	return nil
}

func TestProcessPendingSellerConfirmsWhenRequestExists(t *testing.T) {
	seller := common.HexToAddress("0x1111111111111111111111111111111111111111")
	requestHash := agentcrypto.Keccak256Hex([]byte("request"))
	market := &fakeMarket{order: chain.Order{
		Seller:      seller,
		RequestHash: requestHash,
		Status:      chain.OrderStatusPendingSeller,
	}}
	localStore := &fakeStore{order: store.Order{RequestHash: requestHash}}
	w := New(market, localStore, common.Address{}, seller, time.Second)

	if err := w.processOrder(context.Background(), big.NewInt(1)); err != nil {
		t.Fatal(err)
	}
	if market.confirmedOrder == nil || market.confirmedOrder.String() != "1" {
		t.Fatalf("expected confirmAsSeller to be called")
	}
	if localStore.order.ConfirmSellerTxHash != "0xconfirm" {
		t.Fatalf("confirm tx not stored")
	}
}

func TestProcessCreatedGeneratesAndCommitsDelivery(t *testing.T) {
	seller := common.HexToAddress("0x1111111111111111111111111111111111111111")
	marketAddress := common.HexToAddress("0x2222222222222222222222222222222222222222")
	requestBody := []byte("request")
	requestHash := agentcrypto.Keccak256Hex(requestBody)
	market := &fakeMarket{order: chain.Order{
		Seller:      seller,
		RequestHash: requestHash,
		Status:      chain.OrderStatusCreated,
	}}
	localStore := &fakeStore{order: store.Order{
		RequestHash: requestHash,
		RequestBody: requestBody,
	}}
	w := New(market, localStore, marketAddress, seller, time.Second)

	if err := w.processOrder(context.Background(), big.NewInt(1)); err != nil {
		t.Fatal(err)
	}
	if market.committedOrder == nil || market.committedOrder.String() != "1" {
		t.Fatalf("expected commitDelivery to be called")
	}
	if localStore.order.CommitDeliveryTxHash != "0xcommit" {
		t.Fatalf("commit tx not stored")
	}
	if agentcrypto.Keccak256Hex(localStore.order.DeliveryBody) != localStore.order.DeliveryHash {
		t.Fatalf("delivery hash mismatch")
	}
}
