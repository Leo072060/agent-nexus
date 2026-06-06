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

type fakeMarket struct {
	order         chain.Order
	err           error
	confirmTxHash string
}

func (f fakeMarket) GetOrder(context.Context, *big.Int) (chain.Order, error) {
	return f.order, f.err
}

func (f fakeMarket) ConfirmAsSeller(context.Context, *big.Int) (string, error) {
	if f.confirmTxHash != "" {
		return f.confirmTxHash, nil
	}
	return "0xconfirm", nil
}

func TestHealth(t *testing.T) {
	handler := NewHandler(config.Config{}, fakeMarket{}, nil)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status mismatch: %d", response.Code)
	}
}

func TestDeliveryMarketMismatch(t *testing.T) {
	handler := NewHandler(config.Config{
		MarketAddress: common.HexToAddress("0x1111111111111111111111111111111111111111"),
	}, fakeMarket{}, nil)

	body := `{"marketAddress":"0x2222222222222222222222222222222222222222","orderId":"12","signature":"0x00"}`
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent-nexus/delivery", strings.NewReader(body))

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status mismatch: %d", response.Code)
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
	requestBody := "please review this contract"
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
		fakeMarket{
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

	payload := map[string]string{
		"marketAddress": market.Hex(),
		"orderId":       orderID.String(),
		"request":       requestBody,
		"signature":     signature,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent-nexus/orders/request", strings.NewReader(string(payloadBytes)))

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
	deliveryBody := []byte("delivery body")
	deliveryHash := agentcrypto.Keccak256Hex(deliveryBody)

	db, err := store.Open(filepath.Join(t.TempDir(), "seller-service.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.UpsertDelivery(context.Background(), orderID, deliveryHash, deliveryBody); err != nil {
		t.Fatal(err)
	}

	signature := signDeliveryRequest(t, buyerKey, market, orderID)
	handler := NewHandler(
		config.Config{
			MarketAddress: market,
			SellerAddress: seller,
		},
		fakeMarket{order: chain.Order{
			Buyer:        buyer,
			Seller:       seller,
			DeliveryHash: deliveryHash,
			Status:       chain.OrderStatusDeliveryCommitted,
		}},
		db,
	)

	payload := map[string]string{
		"marketAddress": market.Hex(),
		"orderId":       orderID.String(),
		"signature":     signature,
	}
	payloadBytes, err := json.Marshal(payload)
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

func TestDeliveryHashMismatch(t *testing.T) {
	buyerKey := mustKey(t)
	sellerKey := mustKey(t)
	buyer := gethcrypto.PubkeyToAddress(buyerKey.PublicKey)
	seller := gethcrypto.PubkeyToAddress(sellerKey.PublicKey)
	market := common.HexToAddress("0x1111111111111111111111111111111111111111")
	orderID := big.NewInt(12)

	db, err := store.Open(filepath.Join(t.TempDir(), "seller-service.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.UpsertDelivery(context.Background(), orderID, "0xabc", []byte("delivery body")); err != nil {
		t.Fatal(err)
	}

	signature := signDeliveryRequest(t, buyerKey, market, orderID)
	handler := NewHandler(
		config.Config{
			MarketAddress: market,
			SellerAddress: seller,
		},
		fakeMarket{order: chain.Order{
			Buyer:        buyer,
			Seller:       seller,
			DeliveryHash: "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			Status:       chain.OrderStatusDeliveryCommitted,
		}},
		db,
	)

	payload := `{"marketAddress":"` + market.Hex() + `","orderId":"12","signature":"` + signature + `"}`
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/agent-nexus/delivery", strings.NewReader(payload))

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status mismatch: %d body=%s", response.Code, response.Body.String())
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
