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

	"agent-nexus-seller-service/internal/chain"
	"agent-nexus-seller-service/internal/config"
	agentcrypto "agent-nexus-seller-service/internal/crypto"
	"agent-nexus-seller-service/internal/store"

	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

type testMarket struct {
	order         chain.Order
	err           error
	confirmTxHash string
}

func (t testMarket) GetOrder(context.Context, *big.Int) (chain.Order, error) {
	return t.order, t.err
}

func (t testMarket) ConfirmAsSeller(context.Context, *big.Int) (string, error) {
	if t.confirmTxHash != "" {
		return t.confirmTxHash, nil
	}
	return "0xconfirm", nil
}

func TestHealth(t *testing.T) {
	handler := NewHandler(config.Config{}, testMarket{}, nil)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", response.Code)
	}
}

func TestServices(t *testing.T) {
	market := common.HexToAddress("0x1111111111111111111111111111111111111111")
	seller := common.HexToAddress("0x2222222222222222222222222222222222222222")
	handler := NewHandler(config.Config{
		ServiceID:          "contract-review",
		ServiceName:        "Contract Review",
		ServiceDescription: "Review contract text and return risk notes.",
		MarketAddress:      market,
		SellerAddress:      seller,
	}, testMarket{}, nil)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/agent-nexus/services", nil)

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", response.Code)
	}

	var payload map[string]string
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload["id"] != "contract-review" {
		t.Fatalf("id mismatch: %s", payload["id"])
	}
	if payload["name"] != "Contract Review" {
		t.Fatalf("name mismatch: %s", payload["name"])
	}
	if payload["description"] != "Review contract text and return risk notes." {
		t.Fatalf("description mismatch: %s", payload["description"])
	}
	if payload["marketAddress"] != market.Hex() {
		t.Fatalf("market mismatch: %s", payload["marketAddress"])
	}
	if payload["sellerAddress"] != seller.Hex() {
		t.Fatalf("seller mismatch: %s", payload["sellerAddress"])
	}
}

func TestOrderRequestSuccess(t *testing.T) {
	buyerKey := mustKey(t)
	sellerKey := mustKey(t)
	buyer := gethcrypto.PubkeyToAddress(buyerKey.PublicKey)
	seller := gethcrypto.PubkeyToAddress(sellerKey.PublicKey)
	validator := common.HexToAddress("0x3333333333333333333333333333333333333333")
	market := common.HexToAddress("0x1111111111111111111111111111111111111111")
	orderID := big.NewInt(12)
	requestBody := "review this contract"
	requestHash := agentcrypto.Keccak256Hex([]byte(requestBody))

	db, err := store.Open(filepath.Join(t.TempDir(), "seller-service.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	signature := signOrderRequest(t, buyerKey, market, orderID, requestHash)
	handler := NewHandler(
		config.Config{
			MarketAddress: market,
			SellerAddress: seller,
		},
		testMarket{
			order: chain.Order{
				Buyer:       buyer,
				Seller:      seller,
				Validator:   validator,
				RequestHash: requestHash,
				Status:      chain.OrderStatusPendingSeller,
			},
			confirmTxHash: "0xabc",
		},
		db,
	)

	payloadBytes, err := json.Marshal(map[string]string{
		"marketAddress": market.Hex(),
		"orderId":       orderID.String(),
		"request":       requestBody,
		"signature":     signature,
	})
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent-nexus/request", strings.NewReader(string(payloadBytes)))

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", response.Code, response.Body.String())
	}

	order, err := db.GetOrder(context.Background(), orderID)
	if err != nil {
		t.Fatal(err)
	}
	if order.RequestHash != requestHash {
		t.Fatalf("request hash mismatch: %s", order.RequestHash)
	}
	if order.ConfirmSellerTxHash != "0xabc" {
		t.Fatalf("confirm tx mismatch: %s", order.ConfirmSellerTxHash)
	}
}

func TestDeliverySuccess(t *testing.T) {
	buyerKey := mustKey(t)
	sellerKey := mustKey(t)
	buyer := gethcrypto.PubkeyToAddress(buyerKey.PublicKey)
	seller := gethcrypto.PubkeyToAddress(sellerKey.PublicKey)
	market := common.HexToAddress("0x1111111111111111111111111111111111111111")
	orderID := big.NewInt(12)
	deliveryBody := []byte("approved")
	deliveryHash := agentcrypto.Keccak256Hex(deliveryBody)

	db, err := store.Open(filepath.Join(t.TempDir(), "seller-service.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.UpsertOrderRequest(
		context.Background(),
		orderID,
		buyer.Hex(),
		seller.Hex(),
		common.HexToAddress("0x3333333333333333333333333333333333333333").Hex(),
		agentcrypto.Keccak256Hex([]byte("review this contract")),
		[]byte("review this contract"),
		"seller_confirmed",
	); err != nil {
		t.Fatal(err)
	}
	if err := db.MarkDeliveryCommitted(context.Background(), orderID, deliveryHash, deliveryBody, "0xevidence", []byte("evidence"), "0xcommit"); err != nil {
		t.Fatal(err)
	}

	signature := signDeliveryRequest(t, buyerKey, market, orderID)
	handler := NewHandler(
		config.Config{
			MarketAddress: market,
			SellerAddress: seller,
		},
		testMarket{order: chain.Order{
			Buyer:        buyer,
			Seller:       seller,
			DeliveryHash: deliveryHash,
			Status:       chain.OrderStatusDeliveryCommitted,
		}},
		db,
	)

	payloadBytes, err := json.Marshal(map[string]string{
		"marketAddress": market.Hex(),
		"orderId":       orderID.String(),
		"signature":     signature,
	})
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent-nexus/delivery", strings.NewReader(string(payloadBytes)))

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d body=%s", response.Code, response.Body.String())
	}
	if response.Body.String() != string(deliveryBody) {
		t.Fatalf("body mismatch: %s", response.Body.String())
	}
}

func mustKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func signDeliveryRequest(t *testing.T, key *ecdsa.PrivateKey, market common.Address, orderID *big.Int) string {
	t.Helper()
	message := agentcrypto.DeliveryRequestMessage(market, orderID)
	prefix := "\x19Ethereum Signed Message:\n" + strconv.Itoa(len(message))
	hash := gethcrypto.Keccak256Hash([]byte(prefix + message))
	signature, err := gethcrypto.Sign(hash.Bytes(), key)
	if err != nil {
		t.Fatal(err)
	}
	signature[64] += 27
	return "0x" + common.Bytes2Hex(signature)
}

func signOrderRequest(t *testing.T, key *ecdsa.PrivateKey, market common.Address, orderID *big.Int, requestHash string) string {
	t.Helper()
	message := agentcrypto.OrderRequestMessage(market, orderID, requestHash)
	prefix := "\x19Ethereum Signed Message:\n" + strconv.Itoa(len(message))
	hash := gethcrypto.Keccak256Hash([]byte(prefix + message))
	signature, err := gethcrypto.Sign(hash.Bytes(), key)
	if err != nil {
		t.Fatal(err)
	}
	signature[64] += 27
	return "0x" + common.Bytes2Hex(signature)
}
