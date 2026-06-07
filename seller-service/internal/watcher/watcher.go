package watcher

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strings"
	"time"

	"agent-nexus-seller-service/internal/chain"
	agentcrypto "agent-nexus-seller-service/internal/crypto"
	"agent-nexus-seller-service/internal/llm"
	"agent-nexus-seller-service/internal/store"

	"github.com/ethereum/go-ethereum/common"
)

type Market interface {
	GetOrderCount(ctx context.Context) (*big.Int, error)
	GetOrder(ctx context.Context, orderID *big.Int) (chain.Order, error)
	GetValidator(ctx context.Context, validator common.Address) (chain.Validator, error)
	ConfirmAsSeller(ctx context.Context, orderID *big.Int) (string, error)
	CommitDelivery(ctx context.Context, orderID *big.Int, deliveryHash [32]byte) (string, error)
}

type Store interface {
	GetOrder(ctx context.Context, chainOrderID *big.Int) (store.Order, error)
	MarkSellerConfirmed(ctx context.Context, chainOrderID *big.Int, txHash string) error
	MarkDeliveryGenerated(ctx context.Context, chainOrderID *big.Int, deliveryHash string, deliveryBody []byte, evidenceHash string, evidenceBody []byte) error
	MarkDeliveryCommitted(ctx context.Context, chainOrderID *big.Int, deliveryHash string, deliveryBody []byte, evidenceHash string, evidenceBody []byte, txHash string) error
	MarkEvidencePosted(ctx context.Context, chainOrderID *big.Int, httpStatus int, responseBody string) error
}

type Generator interface {
	Generate(ctx context.Context, marketAddress common.Address, orderID *big.Int, requestBody []byte) (llm.Result, error)
}

type Watcher struct {
	market        Market
	store         Store
	generator     Generator
	marketAddress common.Address
	sellerAddress common.Address
	sellerKey     *ecdsa.PrivateKey
	httpClient    *http.Client
	pollInterval  time.Duration
}

func New(
	market Market,
	store Store,
	generator Generator,
	marketAddress common.Address,
	sellerAddress common.Address,
	sellerKey *ecdsa.PrivateKey,
	pollInterval time.Duration,
) *Watcher {
	return &Watcher{
		market:        market,
		store:         store,
		generator:     generator,
		marketAddress: marketAddress,
		sellerAddress: sellerAddress,
		sellerKey:     sellerKey,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
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
	case chain.OrderStatusDisputed:
		return w.sendEvidenceIfReady(ctx, orderID, chainOrder)
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
	log.Printf("watcher confirmed seller order_id=%s tx_hash=%s", orderID.String(), txHash)

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
	evidence := localOrder.EvidenceBody
	if len(body) == 0 {
		var err error
		result, err := w.generator.Generate(ctx, w.marketAddress, orderID, localOrder.RequestBody)
		if err != nil {
			return err
		}
		body = result.Answer
		evidence = result.Evidence
		if len(body) == 0 {
			return fmt.Errorf("local delivery body is empty")
		}
		if len(evidence) == 0 {
			return fmt.Errorf("local evidence body is empty")
		}
		if err := w.store.MarkDeliveryGenerated(ctx, orderID, agentcrypto.Keccak256Hex(body), body, agentcrypto.Keccak256Hex(evidence), evidence); err != nil {
			return err
		}
	}
	if len(evidence) == 0 {
		return fmt.Errorf("local evidence body is empty")
	}
	deliveryHash := agentcrypto.Keccak256(body)
	txHash, err := w.market.CommitDelivery(ctx, orderID, deliveryHash)
	if err != nil {
		return err
	}
	log.Printf("watcher committed delivery order_id=%s tx_hash=%s delivery_hash=%s", orderID.String(), txHash, agentcrypto.Keccak256Hex(body))

	return w.store.MarkDeliveryCommitted(ctx, orderID, agentcrypto.Keccak256Hex(body), body, agentcrypto.Keccak256Hex(evidence), evidence, txHash)
}

func (w *Watcher) sendEvidenceIfReady(ctx context.Context, orderID *big.Int, chainOrder chain.Order) error {
	localOrder, err := w.store.GetOrder(ctx, orderID)
	if err != nil {
		if strings.Contains(err.Error(), "order not found") {
			return nil
		}
		return err
	}
	if localOrder.EvidenceSentAt != "" {
		return nil
	}
	if len(localOrder.RequestBody) == 0 || len(localOrder.DeliveryBody) == 0 || len(localOrder.EvidenceBody) == 0 {
		return fmt.Errorf("local dispute evidence materials are incomplete")
	}
	if !strings.EqualFold(localOrder.RequestHash, chainOrder.RequestHash) {
		return fmt.Errorf("local request hash does not match chain requestHash")
	}
	if !agentcrypto.EqualHash(agentcrypto.Keccak256(localOrder.DeliveryBody), chainOrder.DeliveryHash) {
		return fmt.Errorf("local delivery body hash does not match chain deliveryHash")
	}

	validator, err := w.market.GetValidator(ctx, chainOrder.Validator)
	if err != nil {
		return err
	}
	if !validator.Registered || !validator.Active {
		return fmt.Errorf("validator is not active")
	}
	if strings.TrimSpace(validator.ValidatorURI) == "" {
		return fmt.Errorf("validatorURI is empty")
	}

	evidenceHash := agentcrypto.Keccak256Hex(localOrder.EvidenceBody)
	deliveryHash := agentcrypto.Keccak256Hex(localOrder.DeliveryBody)
	message := agentcrypto.EvidenceMessage(w.marketAddress, orderID, localOrder.RequestHash, deliveryHash, evidenceHash)
	signature, err := agentcrypto.SignMessage(w.sellerKey, message)
	if err != nil {
		return err
	}

	payload := evidencePayload{
		MarketAddress:    w.marketAddress.Hex(),
		OrderID:          orderID.String(),
		BuyerAddress:     chainOrder.Buyer.Hex(),
		SellerAddress:    chainOrder.Seller.Hex(),
		ValidatorAddress: chainOrder.Validator.Hex(),
		RequestHash:      localOrder.RequestHash,
		DeliveryHash:     deliveryHash,
		EvidenceHash:     evidenceHash,
		Request:          string(localOrder.RequestBody),
		Answer:           string(localOrder.DeliveryBody),
		Evidence:         string(localOrder.EvidenceBody),
		SellerSignature:  signature,
	}
	status, responseBody, err := w.postEvidence(ctx, validator.ValidatorURI, payload)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("validator evidence POST returned status %d: %s", status, truncate(responseBody))
	}

	log.Printf("watcher sent dispute evidence order_id=%s validator_uri=%s status=%d evidence_hash=%s", orderID.String(), validator.ValidatorURI, status, evidenceHash)
	return w.store.MarkEvidencePosted(ctx, orderID, status, truncate(responseBody))
}

type evidencePayload struct {
	MarketAddress    string `json:"marketAddress"`
	OrderID          string `json:"orderId"`
	BuyerAddress     string `json:"buyerAddress"`
	SellerAddress    string `json:"sellerAddress"`
	ValidatorAddress string `json:"validatorAddress"`
	RequestHash      string `json:"requestHash"`
	DeliveryHash     string `json:"deliveryHash"`
	EvidenceHash     string `json:"evidenceHash"`
	Request          string `json:"request"`
	Answer           string `json:"answer"`
	Evidence         string `json:"evidence"`
	SellerSignature  string `json:"sellerSignature"`
}

func (w *Watcher) postEvidence(ctx context.Context, uri string, payload evidencePayload) (int, string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, "", fmt.Errorf("marshal evidence payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uri, bytes.NewReader(body))
	if err != nil {
		return 0, "", fmt.Errorf("create evidence request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("post evidence: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return resp.StatusCode, "", fmt.Errorf("read evidence response: %w", err)
	}
	return resp.StatusCode, string(responseBody), nil
}

func truncate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 512 {
		return value
	}
	return value[:512] + "...(truncated)"
}
