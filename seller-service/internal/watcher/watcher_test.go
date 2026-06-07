package watcher

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agent-nexus-seller-service/internal/chain"
	agentcrypto "agent-nexus-seller-service/internal/crypto"
	"agent-nexus-seller-service/internal/llm"
	"agent-nexus-seller-service/internal/store"

	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

type recordingMarket struct {
	order          chain.Order
	validator      chain.Validator
	confirmedOrder *big.Int
	committedOrder *big.Int
	committedHash  [32]byte
	commitErr      error
	commitCalls    int
}

func (r *recordingMarket) GetOrderCount(context.Context) (*big.Int, error) {
	return big.NewInt(1), nil
}

func (r *recordingMarket) GetOrder(context.Context, *big.Int) (chain.Order, error) {
	return r.order, nil
}

func (r *recordingMarket) GetValidator(context.Context, common.Address) (chain.Validator, error) {
	return r.validator, nil
}

func (r *recordingMarket) ConfirmAsSeller(_ context.Context, orderID *big.Int) (string, error) {
	r.confirmedOrder = new(big.Int).Set(orderID)
	return "0xconfirm", nil
}

func (r *recordingMarket) CommitDelivery(_ context.Context, orderID *big.Int, deliveryHash [32]byte) (string, error) {
	r.commitCalls++
	r.committedOrder = new(big.Int).Set(orderID)
	r.committedHash = deliveryHash
	if r.commitErr != nil {
		return "", r.commitErr
	}
	return "0xcommit", nil
}

type memoryStore struct {
	order store.Order
}

func (m *memoryStore) GetOrder(context.Context, *big.Int) (store.Order, error) {
	return m.order, nil
}

func (m *memoryStore) MarkSellerConfirmed(_ context.Context, _ *big.Int, txHash string) error {
	m.order.ConfirmSellerTxHash = txHash
	return nil
}

func (m *memoryStore) MarkDeliveryGenerated(_ context.Context, _ *big.Int, deliveryHash string, deliveryBody []byte, evidenceHash string, evidenceBody []byte) error {
	m.order.DeliveryHash = deliveryHash
	m.order.DeliveryBody = deliveryBody
	m.order.EvidenceHash = evidenceHash
	m.order.EvidenceBody = evidenceBody
	m.order.Status = "delivery_generated"
	return nil
}

func (m *memoryStore) MarkDeliveryCommitted(_ context.Context, _ *big.Int, deliveryHash string, deliveryBody []byte, evidenceHash string, evidenceBody []byte, txHash string) error {
	m.order.DeliveryHash = deliveryHash
	m.order.DeliveryBody = deliveryBody
	m.order.EvidenceHash = evidenceHash
	m.order.EvidenceBody = evidenceBody
	m.order.CommitDeliveryTxHash = txHash
	m.order.Status = "delivery_committed"
	return nil
}

func (m *memoryStore) MarkEvidencePosted(_ context.Context, _ *big.Int, httpStatus int, responseBody string) error {
	m.order.EvidenceSentAt = time.Now().UTC().Format(time.RFC3339)
	m.order.EvidencePostStatus = httpStatus
	m.order.EvidencePostResponse = responseBody
	return nil
}

type staticGenerator struct {
	result    llm.Result
	request   []byte
	orderID   *big.Int
	wasCalled bool
	calls     int
}

func (s *staticGenerator) Generate(_ context.Context, _ common.Address, orderID *big.Int, requestBody []byte) (llm.Result, error) {
	s.wasCalled = true
	s.calls++
	s.orderID = new(big.Int).Set(orderID)
	s.request = append([]byte(nil), requestBody...)
	return s.result, nil
}

func TestProcessPendingSellerConfirmsWhenRequestExists(t *testing.T) {
	seller := common.HexToAddress("0x1111111111111111111111111111111111111111")
	requestHash := agentcrypto.Keccak256Hex([]byte("review this contract"))
	market := &recordingMarket{order: chain.Order{
		Seller:      seller,
		RequestHash: requestHash,
		Status:      chain.OrderStatusPendingSeller,
	}}
	localStore := &memoryStore{order: store.Order{RequestHash: requestHash}}
	generator := &staticGenerator{}
	w := New(market, localStore, generator, common.Address{}, seller, testKey(t), time.Second)

	if err := w.processOrder(context.Background(), big.NewInt(1)); err != nil {
		t.Fatal(err)
	}
	if market.confirmedOrder == nil || market.confirmedOrder.String() != "1" {
		t.Fatalf("expected confirmAsSeller to be called")
	}
	if localStore.order.ConfirmSellerTxHash != "0xconfirm" {
		t.Fatalf("confirm tx not stored")
	}
	if generator.wasCalled {
		t.Fatalf("generator should not be called while order is PendingSeller")
	}
}

func TestProcessCreatedGeneratesAndCommitsDelivery(t *testing.T) {
	seller := common.HexToAddress("0x1111111111111111111111111111111111111111")
	marketAddress := common.HexToAddress("0x2222222222222222222222222222222222222222")
	requestBody := []byte("review this contract")
	requestHash := agentcrypto.Keccak256Hex(requestBody)
	deliveryBody := []byte("approved")
	evidenceBody := []byte("request matched answer")
	market := &recordingMarket{order: chain.Order{
		Seller:      seller,
		RequestHash: requestHash,
		Status:      chain.OrderStatusCreated,
	}}
	localStore := &memoryStore{order: store.Order{
		RequestHash: requestHash,
		RequestBody: requestBody,
	}}
	generator := &staticGenerator{result: llm.Result{Answer: deliveryBody, Evidence: evidenceBody}}
	w := New(market, localStore, generator, marketAddress, seller, testKey(t), time.Second)

	if err := w.processOrder(context.Background(), big.NewInt(1)); err != nil {
		t.Fatal(err)
	}
	if !generator.wasCalled {
		t.Fatalf("expected generator to be called")
	}
	if string(generator.request) != string(requestBody) {
		t.Fatalf("generator request mismatch: %s", string(generator.request))
	}
	if market.committedOrder == nil || market.committedOrder.String() != "1" {
		t.Fatalf("expected commitDelivery to be called")
	}
	if localStore.order.CommitDeliveryTxHash != "0xcommit" {
		t.Fatalf("commit tx not stored")
	}
	if string(localStore.order.DeliveryBody) != string(deliveryBody) {
		t.Fatalf("delivery body mismatch: %s", string(localStore.order.DeliveryBody))
	}
	if string(localStore.order.EvidenceBody) != string(evidenceBody) {
		t.Fatalf("evidence body mismatch: %s", string(localStore.order.EvidenceBody))
	}
	if agentcrypto.Keccak256Hex(localStore.order.DeliveryBody) != localStore.order.DeliveryHash {
		t.Fatalf("delivery hash mismatch")
	}
	if agentcrypto.Keccak256Hex(localStore.order.EvidenceBody) != localStore.order.EvidenceHash {
		t.Fatalf("evidence hash mismatch")
	}
	if localStore.order.Status != "delivery_committed" {
		t.Fatalf("status mismatch: %s", localStore.order.Status)
	}
}

func TestProcessCreatedReusesGeneratedDeliveryWhenCommitMissing(t *testing.T) {
	seller := common.HexToAddress("0x1111111111111111111111111111111111111111")
	marketAddress := common.HexToAddress("0x2222222222222222222222222222222222222222")
	requestBody := []byte("review this contract")
	requestHash := agentcrypto.Keccak256Hex(requestBody)
	deliveryBody := []byte("approved")
	evidenceBody := []byte("request matched answer")
	market := &recordingMarket{order: chain.Order{
		Seller:      seller,
		RequestHash: requestHash,
		Status:      chain.OrderStatusCreated,
	}}
	localStore := &memoryStore{order: store.Order{
		RequestHash:  requestHash,
		RequestBody:  requestBody,
		DeliveryHash: agentcrypto.Keccak256Hex(deliveryBody),
		DeliveryBody: deliveryBody,
		EvidenceHash: agentcrypto.Keccak256Hex(evidenceBody),
		EvidenceBody: evidenceBody,
		Status:       "delivery_generated",
	}}
	generator := &staticGenerator{result: llm.Result{Answer: []byte("new answer"), Evidence: []byte("new evidence")}}
	w := New(market, localStore, generator, marketAddress, seller, testKey(t), time.Second)

	if err := w.processOrder(context.Background(), big.NewInt(1)); err != nil {
		t.Fatal(err)
	}
	if generator.wasCalled {
		t.Fatalf("generator should not be called when delivery body already exists")
	}
	if market.commitCalls != 1 {
		t.Fatalf("commit calls mismatch: %d", market.commitCalls)
	}
	if market.committedHash != agentcrypto.Keccak256(deliveryBody) {
		t.Fatalf("committed hash mismatch")
	}
	if localStore.order.CommitDeliveryTxHash != "0xcommit" {
		t.Fatalf("commit tx not stored")
	}
	if string(localStore.order.DeliveryBody) != string(deliveryBody) {
		t.Fatalf("delivery body changed: %s", string(localStore.order.DeliveryBody))
	}
}

func TestProcessCreatedStoresGeneratedDeliveryBeforeCommitFailure(t *testing.T) {
	seller := common.HexToAddress("0x1111111111111111111111111111111111111111")
	marketAddress := common.HexToAddress("0x2222222222222222222222222222222222222222")
	requestBody := []byte("review this contract")
	requestHash := agentcrypto.Keccak256Hex(requestBody)
	deliveryBody := []byte("approved")
	evidenceBody := []byte("request matched answer")
	commitErr := errors.New("commit failed")
	market := &recordingMarket{
		order: chain.Order{
			Seller:      seller,
			RequestHash: requestHash,
			Status:      chain.OrderStatusCreated,
		},
		commitErr: commitErr,
	}
	localStore := &memoryStore{order: store.Order{
		RequestHash: requestHash,
		RequestBody: requestBody,
	}}
	generator := &staticGenerator{result: llm.Result{Answer: deliveryBody, Evidence: evidenceBody}}
	w := New(market, localStore, generator, marketAddress, seller, testKey(t), time.Second)

	if err := w.processOrder(context.Background(), big.NewInt(1)); !errors.Is(err, commitErr) {
		t.Fatalf("expected commit error, got %v", err)
	}
	if generator.calls != 1 {
		t.Fatalf("generator calls mismatch after first pass: %d", generator.calls)
	}
	if string(localStore.order.DeliveryBody) != string(deliveryBody) {
		t.Fatalf("delivery body not stored: %s", string(localStore.order.DeliveryBody))
	}
	if string(localStore.order.EvidenceBody) != string(evidenceBody) {
		t.Fatalf("evidence body not stored: %s", string(localStore.order.EvidenceBody))
	}
	if localStore.order.CommitDeliveryTxHash != "" {
		t.Fatalf("commit tx should not be stored after commit failure")
	}
	if localStore.order.Status != "delivery_generated" {
		t.Fatalf("status mismatch after commit failure: %s", localStore.order.Status)
	}

	market.commitErr = nil
	if err := w.processOrder(context.Background(), big.NewInt(1)); err != nil {
		t.Fatal(err)
	}
	if generator.calls != 1 {
		t.Fatalf("generator should not be called again, calls=%d", generator.calls)
	}
	if market.commitCalls != 2 {
		t.Fatalf("commit calls mismatch: %d", market.commitCalls)
	}
	if localStore.order.CommitDeliveryTxHash != "0xcommit" {
		t.Fatalf("commit tx not stored after retry")
	}
}

func TestProcessDisputedPostsEvidenceToValidatorURI(t *testing.T) {
	sellerKey := testKey(t)
	seller := gethcrypto.PubkeyToAddress(sellerKey.PublicKey)
	buyer := common.HexToAddress("0x2222222222222222222222222222222222222222")
	validator := common.HexToAddress("0x3333333333333333333333333333333333333333")
	marketAddress := common.HexToAddress("0x4444444444444444444444444444444444444444")
	orderID := big.NewInt(1)
	requestBody := []byte("review this contract")
	answerBody := []byte("approved")
	evidenceBody := []byte("request matched answer")
	requestHash := agentcrypto.Keccak256Hex(requestBody)
	deliveryHash := agentcrypto.Keccak256Hex(answerBody)
	evidenceHash := agentcrypto.Keccak256Hex(evidenceBody)

	var payload map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method mismatch: %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("accepted"))
	}))
	defer server.Close()

	market := &recordingMarket{
		order: chain.Order{
			Buyer:        buyer,
			Seller:       seller,
			Validator:    validator,
			RequestHash:  requestHash,
			DeliveryHash: deliveryHash,
			Status:       chain.OrderStatusDisputed,
		},
		validator: chain.Validator{Registered: true, Active: true, ValidatorURI: server.URL},
	}
	localStore := &memoryStore{order: store.Order{
		RequestHash:  requestHash,
		RequestBody:  requestBody,
		DeliveryHash: deliveryHash,
		DeliveryBody: answerBody,
		EvidenceHash: evidenceHash,
		EvidenceBody: evidenceBody,
	}}
	w := New(market, localStore, &staticGenerator{}, marketAddress, seller, sellerKey, time.Second)

	if err := w.processOrder(context.Background(), orderID); err != nil {
		t.Fatal(err)
	}
	if payload["request"] != string(requestBody) {
		t.Fatalf("request mismatch: %s", payload["request"])
	}
	if payload["answer"] != string(answerBody) {
		t.Fatalf("answer mismatch: %s", payload["answer"])
	}
	if payload["evidence"] != string(evidenceBody) {
		t.Fatalf("evidence mismatch: %s", payload["evidence"])
	}
	if payload["evidenceHash"] != evidenceHash {
		t.Fatalf("evidence hash mismatch: %s", payload["evidenceHash"])
	}
	message := agentcrypto.EvidenceMessage(marketAddress, orderID, requestHash, deliveryHash, evidenceHash)
	signer, err := agentcrypto.RecoverSigner(message, payload["sellerSignature"])
	if err != nil {
		t.Fatal(err)
	}
	if signer != seller {
		t.Fatalf("signature signer mismatch: %s", signer.Hex())
	}
	if localStore.order.EvidenceSentAt == "" {
		t.Fatalf("expected evidence sent timestamp")
	}
	if localStore.order.EvidencePostStatus != http.StatusAccepted {
		t.Fatalf("post status mismatch: %d", localStore.order.EvidencePostStatus)
	}
}

func testKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}
