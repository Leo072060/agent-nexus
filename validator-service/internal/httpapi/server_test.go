package httpapi

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"agent-nexus-validator-service/internal/chain"
	"agent-nexus-validator-service/internal/config"
	agentcrypto "agent-nexus-validator-service/internal/crypto"
	"agent-nexus-validator-service/internal/llm"
	"agent-nexus-validator-service/internal/store"

	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

type fakeMarket struct {
	order          chain.Order
	err            error
	resolved       bool
	releaseSeller  bool
	resolutionHash [32]byte
}

func (f *fakeMarket) GetOrder(context.Context, *big.Int) (chain.Order, error) {
	return f.order, f.err
}

func (f *fakeMarket) ResolveDispute(_ context.Context, _ *big.Int, releaseToSeller bool, resolutionHash [32]byte) (string, error) {
	f.resolved = true
	f.releaseSeller = releaseToSeller
	f.resolutionHash = resolutionHash
	return "0xresolve", nil
}

type fakeDecisionMaker struct{}

func (fakeDecisionMaker) Decide(context.Context, llm.Evidence) (llm.Decision, []byte, error) {
	return llm.Decision{
		ReleaseToSeller:          false,
		Summary:                  "refund buyer",
		Reasoning:                "delivery did not satisfy request",
		BuyerClaim:               "bad delivery",
		SellerDeliveryAssessment: "insufficient",
		Confidence:               "high",
	}, []byte(`{"releaseToSeller":false,"summary":"refund buyer","reasoning":"delivery did not satisfy request","buyerClaim":"bad delivery","sellerDeliveryAssessment":"insufficient","confidence":"high"}`), nil
}

func TestHealth(t *testing.T) {
	handler := NewHandler(config.Config{}, &fakeMarket{}, nil, fakeDecisionMaker{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", response.Code)
	}
}

func TestDisputeSuccessResolves(t *testing.T) {
	buyerKey := mustKey(t)
	validatorKey := mustKey(t)
	buyer := gethcrypto.PubkeyToAddress(buyerKey.PublicKey)
	validator := gethcrypto.PubkeyToAddress(validatorKey.PublicKey)
	seller := common.HexToAddress("0x3333333333333333333333333333333333333333")
	market := common.HexToAddress("0x1111111111111111111111111111111111111111")
	orderID := big.NewInt(12)
	requestText := "request"
	deliveryText := "delivery"
	disputeText := "dispute"
	requestHash := agentcrypto.Keccak256Hex([]byte(requestText))
	deliveryHash := agentcrypto.Keccak256Hex([]byte(deliveryText))
	disputeHash := agentcrypto.Keccak256Hex([]byte(disputeText))
	signature := signDisputeEvidence(t, buyerKey, market, orderID, requestHash, deliveryHash, disputeHash)

	db, err := store.Open(filepath.Join(t.TempDir(), "validator-service.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	marketClient := &fakeMarket{order: chain.Order{
		Buyer:        buyer,
		Seller:       seller,
		Validator:    validator,
		RequestHash:  requestHash,
		DeliveryHash: deliveryHash,
		Status:       chain.OrderStatusDisputed,
	}}
	handler := NewHandler(
		config.Config{
			MarketAddress:    market,
			ValidatorAddress: validator,
		},
		marketClient,
		db,
		fakeDecisionMaker{},
	)

	payload := map[string]string{
		"marketAddress": market.Hex(),
		"orderId":       orderID.String(),
		"buyerAddress":  buyer.Hex(),
		"request":       requestText,
		"delivery":      deliveryText,
		"dispute":       disputeText,
		"signature":     signature,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent-nexus/disputes", strings.NewReader(string(payloadBytes)))

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", response.Code, response.Body.String())
	}

	dispute, err := db.GetDispute(context.Background(), orderID)
	if err != nil {
		t.Fatal(err)
	}
	if dispute.RequestHash != requestHash {
		t.Fatalf("request hash mismatch: %s", dispute.RequestHash)
	}
	if dispute.Status != "resolved" {
		t.Fatalf("status mismatch: %s", dispute.Status)
	}
	if dispute.ResolveTxHash != "0xresolve" {
		t.Fatalf("resolve tx mismatch: %s", dispute.ResolveTxHash)
	}
	if marketClient.resolved {
		return
	}
	t.Fatalf("expected resolveDispute to be called")
}

func mustKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func signDisputeEvidence(t *testing.T, key *ecdsa.PrivateKey, market common.Address, orderID *big.Int, requestHash string, deliveryHash string, disputeHash string) string {
	t.Helper()
	message := agentcrypto.DisputeEvidenceMessage(market, orderID, requestHash, deliveryHash, disputeHash)
	prefix := "\x19Ethereum Signed Message:\n" + strconv.Itoa(len(message))
	hash := gethcrypto.Keccak256Hash([]byte(prefix + message))
	signature, err := gethcrypto.Sign(hash.Bytes(), key)
	if err != nil {
		t.Fatal(err)
	}
	signature[64] += 27
	return "0x" + common.Bytes2Hex(signature)
}
