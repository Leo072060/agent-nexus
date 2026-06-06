package httpapi

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"
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

func TestMe(t *testing.T) {
	validator := common.HexToAddress("0x2222222222222222222222222222222222222222")
	market := common.HexToAddress("0x1111111111111111111111111111111111111111")
	handler := NewHandler(
		config.Config{
			MarketAddress:    market,
			ValidatorAddress: validator,
			ValidatorBaseURL: "http://localhost:8082",
		},
		&fakeMarket{}, nil, fakeDecisionMaker{},
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/agent-nexus/me", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		ValidatorAddress string `json:"validatorAddress"`
		MarketAddress    string `json:"marketAddress"`
		BaseURL          string `json:"baseURL"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(body.ValidatorAddress, validator.Hex()) {
		t.Fatalf("validatorAddress mismatch: %s", body.ValidatorAddress)
	}
	if !strings.EqualFold(body.MarketAddress, market.Hex()) {
		t.Fatalf("marketAddress mismatch: %s", body.MarketAddress)
	}
	if body.BaseURL != "http://localhost:8082" {
		t.Fatalf("baseURL mismatch: %s", body.BaseURL)
	}
}

func TestListDisputesEndpoint(t *testing.T) {
	db := seedDisputes(t)
	handler := NewHandler(config.Config{}, &fakeMarket{}, db, fakeDecisionMaker{})

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/agent-nexus/disputes", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Disputes []struct {
			OrderID         string `json:"orderId"`
			Status          string `json:"status"`
			ReleaseToSeller bool   `json:"releaseToSeller"`
		} `json:"disputes"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Disputes) != 2 {
		t.Fatalf("expected 2 disputes, got %d", len(body.Disputes))
	}
	if body.Disputes[0].OrderID != "7" || body.Disputes[1].OrderID != "12" {
		t.Fatalf("expected ascending order [7 12], got [%s %s]", body.Disputes[0].OrderID, body.Disputes[1].OrderID)
	}
	if body.Disputes[1].Status != "resolved" || !body.Disputes[1].ReleaseToSeller {
		t.Fatalf("resolved summary mismatch: %+v", body.Disputes[1])
	}
}

func TestGetDisputeEndpoint(t *testing.T) {
	db := seedDisputes(t)
	handler := NewHandler(config.Config{}, &fakeMarket{}, db, fakeDecisionMaker{})

	// 200 + plaintext bodies (regression guard: []byte must NOT be base64-encoded).
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/agent-nexus/disputes/12", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", response.Code, response.Body.String())
	}
	raw := response.Body.String()
	if !strings.Contains(raw, "the-request-plaintext") {
		t.Fatalf("expected plaintext request in body: %s", raw)
	}
	if strings.Contains(raw, base64.StdEncoding.EncodeToString([]byte("the-request-plaintext"))) {
		t.Fatalf("request body was base64-encoded, expected plaintext: %s", raw)
	}
	var detail struct {
		OrderID  string `json:"orderId"`
		Request  string `json:"request"`
		Delivery string `json:"delivery"`
		Dispute  string `json:"dispute"`
		Decision *struct {
			Summary         string `json:"summary"`
			ReleaseToSeller bool   `json:"releaseToSeller"`
			Confidence      string `json:"confidence"`
		} `json:"decision"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.OrderID != "12" || detail.Request != "the-request-plaintext" {
		t.Fatalf("detail mismatch: %+v", detail)
	}
	if detail.Decision == nil || detail.Decision.Summary != "release to seller" || !detail.Decision.ReleaseToSeller {
		t.Fatalf("decision mismatch: %+v", detail.Decision)
	}

	// 404 for a valid-but-unknown order id.
	notFound := httptest.NewRecorder()
	handler.ServeHTTP(notFound, httptest.NewRequest(http.MethodGet, "/agent-nexus/disputes/999", nil))
	if notFound.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", notFound.Code)
	}

	// 400 for an invalid order id.
	bad := httptest.NewRecorder()
	handler.ServeHTTP(bad, httptest.NewRequest(http.MethodGet, "/agent-nexus/disputes/abc", nil))
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", bad.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	handler := NewHandler(config.Config{}, &fakeMarket{}, nil, fakeDecisionMaker{})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodOptions, "/agent-nexus/disputes", nil))

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", response.Code)
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("missing CORS origin header: %q", response.Header().Get("Access-Control-Allow-Origin"))
	}
}

// seedDisputes returns a store with order 7 (evidence_received) and order 12 (resolved).
func seedDisputes(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "validator-service.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	for _, id := range []int64{12, 7} {
		if _, err := db.UpsertDispute(context.Background(), store.Dispute{
			ChainOrderID:     big.NewInt(id),
			BuyerAddress:     "0xbuyer",
			SellerAddress:    "0xseller",
			ValidatorAddress: "0xvalidator",
			RequestHash:      "0xrequest",
			RequestBody:      []byte("the-request-plaintext"),
			DeliveryHash:     "0xdelivery",
			DeliveryBody:     []byte("the-delivery-plaintext"),
			DisputeHash:      "0xdispute",
			DisputeBody:      []byte("the-dispute-plaintext"),
			Status:           "evidence_received",
		}); err != nil {
			t.Fatal(err)
		}
	}

	resolutionBody := []byte(`{"releaseToSeller":true,"summary":"release to seller","reasoning":"delivery satisfied request","buyerClaim":"none","sellerDeliveryAssessment":"sufficient","confidence":"high"}`)
	if err := db.MarkResolved(context.Background(), big.NewInt(12), "0xresolution", resolutionBody, true, "0xtx", "resolved"); err != nil {
		t.Fatal(err)
	}

	return db
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
