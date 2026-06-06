package watcher

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"agent-nexus-seller-service/internal/chain"
	agentcrypto "agent-nexus-seller-service/internal/crypto"
	"agent-nexus-seller-service/internal/delivery"
	"agent-nexus-seller-service/internal/store"

	"github.com/ethereum/go-ethereum/common"
)

type Market interface {
	GetOrderCount(ctx context.Context) (*big.Int, error)
	GetOrder(ctx context.Context, orderID *big.Int) (chain.Order, error)
	ConfirmAsSeller(ctx context.Context, orderID *big.Int) (string, error)
	CommitDelivery(ctx context.Context, orderID *big.Int, deliveryHash [32]byte) (string, error)
}

type Store interface {
	GetOrder(ctx context.Context, chainOrderID *big.Int) (store.Order, error)
	MarkSellerConfirmed(ctx context.Context, chainOrderID *big.Int, txHash string) error
	MarkDeliveryCommitted(ctx context.Context, chainOrderID *big.Int, deliveryHash string, deliveryBody []byte, txHash string) error
}

type Watcher struct {
	market        Market
	store         Store
	marketAddress common.Address
	sellerAddress common.Address
	pollInterval  time.Duration
}

func New(
	market Market,
	store Store,
	marketAddress common.Address,
	sellerAddress common.Address,
	pollInterval time.Duration,
) *Watcher {
	return &Watcher{
		market:        market,
		store:         store,
		marketAddress: marketAddress,
		sellerAddress: sellerAddress,
		pollInterval:  pollInterval,
	}
}

func (w *Watcher) Run(ctx context.Context) {
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	w.scan(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.scan(ctx)
		}
	}
}

func (w *Watcher) scan(ctx context.Context) {
	count, err := w.market.GetOrderCount(ctx)
	if err != nil {
		log.Printf("watcher get order count: %v", err)
		return
	}

	for i := int64(1); i <= count.Int64(); i++ {
		orderID := big.NewInt(i)
		if err := w.processOrder(ctx, orderID); err != nil {
			log.Printf("watcher process order %s: %v", orderID.String(), err)
		}
	}
}

func (w *Watcher) processOrder(ctx context.Context, orderID *big.Int) error {
	chainOrder, err := w.market.GetOrder(ctx, orderID)
	if err != nil {
		return err
	}
	if chainOrder.Seller != w.sellerAddress {
		return nil
	}

	switch chainOrder.Status {
	case chain.OrderStatusPendingSeller:
		return w.confirmIfRequestExists(ctx, orderID, chainOrder)
	case chain.OrderStatusCreated:
		return w.commitIfReady(ctx, orderID, chainOrder)
	default:
		return nil
	}
}

func (w *Watcher) confirmIfRequestExists(ctx context.Context, orderID *big.Int, chainOrder chain.Order) error {
	localOrder, err := w.store.GetOrder(ctx, orderID)
	if err != nil {
		if strings.Contains(err.Error(), "order not found") {
			return nil
		}
		return err
	}
	if !strings.EqualFold(localOrder.RequestHash, chainOrder.RequestHash) {
		return fmt.Errorf("local request hash does not match chain requestHash")
	}
	if localOrder.ConfirmSellerTxHash != "" {
		return nil
	}

	txHash, err := w.market.ConfirmAsSeller(ctx, orderID)
	if err != nil {
		return err
	}

	return w.store.MarkSellerConfirmed(ctx, orderID, txHash)
}

func (w *Watcher) commitIfReady(ctx context.Context, orderID *big.Int, chainOrder chain.Order) error {
	localOrder, err := w.store.GetOrder(ctx, orderID)
	if err != nil {
		if strings.Contains(err.Error(), "order not found") {
			return nil
		}
		return err
	}
	if !strings.EqualFold(localOrder.RequestHash, chainOrder.RequestHash) {
		return fmt.Errorf("local request hash does not match chain requestHash")
	}
	if localOrder.CommitDeliveryTxHash != "" {
		return nil
	}

	body := localOrder.DeliveryBody
	if len(body) == 0 {
		body = delivery.GenerateEchoBody(w.marketAddress, orderID, localOrder.RequestBody)
	}
	deliveryHash := agentcrypto.Keccak256(body)
	txHash, err := w.market.CommitDelivery(ctx, orderID, deliveryHash)
	if err != nil {
		return err
	}

	return w.store.MarkDeliveryCommitted(ctx, orderID, agentcrypto.Keccak256Hex(body), body, txHash)
}
