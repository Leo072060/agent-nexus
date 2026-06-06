package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	"agent-nexus-seller-service/internal/chain"
	"agent-nexus-seller-service/internal/config"
	agentcrypto "agent-nexus-seller-service/internal/crypto"
	"agent-nexus-seller-service/internal/store"

	"github.com/ethereum/go-ethereum/common"
)

type MarketReader interface {
	GetOrder(ctx context.Context, orderID *big.Int) (chain.Order, error)
}

type DeliveryStore interface {
	GetDelivery(ctx context.Context, chainOrderID *big.Int) (store.Delivery, error)
}

type Handler struct {
	cfg    config.Config
	market MarketReader
	store  DeliveryStore
	mux    *http.ServeMux
}

type deliveryRequest struct {
	MarketAddress string `json:"marketAddress"`
	OrderID       string `json:"orderId"`
	Signature     string `json:"signature"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewHandler(cfg config.Config, market MarketReader, store DeliveryStore) http.Handler {
	handler := &Handler{
		cfg:    cfg,
		market: market,
		store:  store,
		mux:    http.NewServeMux(),
	}

	handler.mux.HandleFunc("GET /health", handler.handleHealth)
	handler.mux.HandleFunc("POST /agent-nexus/delivery", handler.handleDelivery)

	return handler
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) handleDelivery(w http.ResponseWriter, r *http.Request) {
	var req deliveryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	orderID, err := parseOrderID(req.OrderID)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !common.IsHexAddress(req.MarketAddress) {
		writeJSONError(w, http.StatusBadRequest, "invalid marketAddress")
		return
	}

	requestMarket := common.HexToAddress(req.MarketAddress)
	if requestMarket != h.cfg.MarketAddress {
		writeJSONError(w, http.StatusBadRequest, "marketAddress does not match seller service market")
		return
	}

	order, err := h.market.GetOrder(r.Context(), orderID)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("read order: %v", err))
		return
	}
	if order.Seller != h.cfg.SellerAddress {
		writeJSONError(w, http.StatusForbidden, "order seller does not match seller service wallet")
		return
	}
	if order.Status != chain.OrderStatusDeliveryCommitted {
		writeJSONError(w, http.StatusConflict, "order is not DeliveryCommitted")
		return
	}

	message := agentcrypto.DeliveryRequestMessage(h.cfg.MarketAddress, orderID)
	signer, err := agentcrypto.RecoverSigner(message, req.Signature)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, fmt.Sprintf("invalid signature: %v", err))
		return
	}
	if signer != order.Buyer {
		writeJSONError(w, http.StatusForbidden, "signature signer is not order buyer")
		return
	}

	delivery, err := h.store.GetDelivery(r.Context(), orderID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "delivery not found") {
			status = http.StatusNotFound
		}
		writeJSONError(w, status, err.Error())
		return
	}

	bodyHash := agentcrypto.Keccak256(delivery.DeliveryBody)
	if !agentcrypto.EqualHash(bodyHash, order.DeliveryHash) {
		writeJSONError(w, http.StatusConflict, "local delivery body hash does not match chain deliveryHash")
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(delivery.DeliveryBody)
}

func parseOrderID(value string) (*big.Int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("orderId is required")
	}

	orderID, ok := new(big.Int).SetString(value, 10)
	if !ok || orderID.Sign() <= 0 {
		return nil, errors.New("orderId must be a positive decimal integer")
	}

	return orderID, nil
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: message})
}
