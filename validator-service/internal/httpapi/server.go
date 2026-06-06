package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	"agent-nexus-validator-service/internal/chain"
	"agent-nexus-validator-service/internal/config"
	agentcrypto "agent-nexus-validator-service/internal/crypto"
	"agent-nexus-validator-service/internal/llm"
	"agent-nexus-validator-service/internal/store"

	"github.com/ethereum/go-ethereum/common"
)

type MarketReader interface {
	GetOrder(ctx context.Context, orderID *big.Int) (chain.Order, error)
}

type MarketWriter interface {
	ResolveDispute(ctx context.Context, orderID *big.Int, releaseToSeller bool, resolutionHash [32]byte) (string, error)
}

type MarketClient interface {
	MarketReader
	MarketWriter
}

type DecisionMaker interface {
	Decide(ctx context.Context, evidence llm.Evidence) (llm.Decision, []byte, error)
}

type DisputeStore interface {
	UpsertDispute(ctx context.Context, dispute store.Dispute) (store.Dispute, error)
	MarkResolved(ctx context.Context, chainOrderID *big.Int, resolutionHash string, resolutionBody []byte, releaseToSeller bool, txHash string, status string) error
	ListDisputes(ctx context.Context) ([]store.Dispute, error)
	GetDispute(ctx context.Context, chainOrderID *big.Int) (store.Dispute, error)
}

type Handler struct {
	cfg      config.Config
	market   MarketClient
	store    DisputeStore
	decision DecisionMaker
	mux      *http.ServeMux
}

type disputeRequest struct {
	MarketAddress string `json:"marketAddress"`
	OrderID       string `json:"orderId"`
	BuyerAddress  string `json:"buyerAddress"`
	Request       string `json:"request"`
	Delivery      string `json:"delivery"`
	Dispute       string `json:"dispute"`
	Signature     string `json:"signature"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type meResponse struct {
	ValidatorAddress string `json:"validatorAddress"`
	MarketAddress    string `json:"marketAddress"`
	BaseURL          string `json:"baseURL"`
}

// disputeSummary is the public-shaped view of a stored dispute used in list responses.
// Evidence bodies are intentionally omitted; they are only returned in the detail view.
type disputeSummary struct {
	OrderID          string `json:"orderId"`
	BuyerAddress     string `json:"buyerAddress"`
	SellerAddress    string `json:"sellerAddress"`
	ValidatorAddress string `json:"validatorAddress"`
	Status           string `json:"status"`
	ReleaseToSeller  bool   `json:"releaseToSeller"`
	ResolutionHash   string `json:"resolutionHash"`
	ResolveTxHash    string `json:"resolveTxHash"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

// disputeDetail extends the summary with the plaintext evidence bodies and the parsed
// LLM decision. The store keeps bodies as []byte (base64 under encoding/json), so they
// are converted to strings here and ResolutionBody is decoded into a nested object.
type disputeDetail struct {
	disputeSummary
	RequestHash  string        `json:"requestHash"`
	Request      string        `json:"request"`
	DeliveryHash string        `json:"deliveryHash"`
	Delivery     string        `json:"delivery"`
	DisputeHash  string        `json:"disputeHash"`
	Dispute      string        `json:"dispute"`
	Decision     *llm.Decision `json:"decision"`
}

type listDisputesResponse struct {
	Disputes []disputeSummary `json:"disputes"`
}

func NewHandler(cfg config.Config, market MarketClient, store DisputeStore, decision DecisionMaker) http.Handler {
	handler := &Handler{
		cfg:      cfg,
		market:   market,
		store:    store,
		decision: decision,
		mux:      http.NewServeMux(),
	}

	handler.mux.HandleFunc("GET /health", handler.handleHealth)
	handler.mux.HandleFunc("GET /agent-nexus/me", handler.handleMe)
	handler.mux.HandleFunc("GET /agent-nexus/disputes", handler.handleListDisputes)
	handler.mux.HandleFunc("GET /agent-nexus/disputes/{orderId}", handler.handleGetDispute)
	handler.mux.HandleFunc("POST /agent-nexus/disputes", handler.handleDispute)

	return withCORS(handler)
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleMe reports which validator this service speaks for, so a dashboard can tell
// whether a given order's validator is "me".
func (h *Handler) handleMe(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, meResponse{
		ValidatorAddress: h.cfg.ValidatorAddress.Hex(),
		MarketAddress:    h.cfg.MarketAddress.Hex(),
		BaseURL:          h.cfg.ValidatorBaseURL,
	})
}

// handleListDisputes returns the summaries of every dispute this validator processed.
func (h *Handler) handleListDisputes(w http.ResponseWriter, r *http.Request) {
	disputes, err := h.store.ListDisputes(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	summaries := make([]disputeSummary, 0, len(disputes))
	for _, dispute := range disputes {
		summaries = append(summaries, toDisputeSummary(dispute))
	}

	writeJSON(w, http.StatusOK, listDisputesResponse{Disputes: summaries})
}

// handleGetDispute returns the full decision process (evidence + LLM ruling) for one order.
func (h *Handler) handleGetDispute(w http.ResponseWriter, r *http.Request) {
	orderID, err := parseOrderID(r.PathValue("orderId"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	dispute, err := h.store.GetDispute(r.Context(), orderID)
	if errors.Is(err, store.ErrDisputeNotFound) {
		writeJSONError(w, http.StatusNotFound, "dispute not found")
		return
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toDisputeDetail(dispute))
}

func (h *Handler) handleDispute(w http.ResponseWriter, r *http.Request) {
	var req disputeRequest
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
	if !common.IsHexAddress(req.BuyerAddress) {
		writeJSONError(w, http.StatusBadRequest, "invalid buyerAddress")
		return
	}

	requestMarket := common.HexToAddress(req.MarketAddress)
	if requestMarket != h.cfg.MarketAddress {
		writeJSONError(w, http.StatusBadRequest, "marketAddress does not match validator service market")
		return
	}

	order, err := h.market.GetOrder(r.Context(), orderID)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("read order: %v", err))
		return
	}
	if order.Validator != h.cfg.ValidatorAddress {
		writeJSONError(w, http.StatusForbidden, "order validator does not match validator service wallet")
		return
	}
	if order.Status != chain.OrderStatusDisputed {
		writeJSONError(w, http.StatusConflict, "order is not Disputed")
		return
	}
	if common.HexToAddress(req.BuyerAddress) != order.Buyer {
		writeJSONError(w, http.StatusForbidden, "buyerAddress does not match chain order buyer")
		return
	}

	requestHash := agentcrypto.Keccak256Hex([]byte(req.Request))
	if !strings.EqualFold(requestHash, order.RequestHash) {
		writeJSONError(w, http.StatusConflict, "request hash does not match chain requestHash")
		return
	}

	deliveryHash := agentcrypto.Keccak256Hex([]byte(req.Delivery))
	deliveryHashVerified := false
	if req.Delivery != "" {
		if !strings.EqualFold(deliveryHash, order.DeliveryHash) {
			writeJSONError(w, http.StatusConflict, "delivery hash does not match chain deliveryHash")
			return
		}
		deliveryHashVerified = true
	}

	disputeHash := agentcrypto.Keccak256Hex([]byte(req.Dispute))
	message := agentcrypto.DisputeEvidenceMessage(h.cfg.MarketAddress, orderID, requestHash, deliveryHash, disputeHash)
	signer, err := agentcrypto.RecoverSigner(message, req.Signature)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, fmt.Sprintf("invalid signature: %v", err))
		return
	}
	if signer != order.Buyer {
		writeJSONError(w, http.StatusForbidden, "signature signer is not order buyer")
		return
	}

	if _, err := h.store.UpsertDispute(r.Context(), store.Dispute{
		ChainOrderID:     orderID,
		BuyerAddress:     order.Buyer.Hex(),
		SellerAddress:    order.Seller.Hex(),
		ValidatorAddress: order.Validator.Hex(),
		RequestHash:      requestHash,
		RequestBody:      []byte(req.Request),
		DeliveryHash:     deliveryHash,
		DeliveryBody:     []byte(req.Delivery),
		DisputeHash:      disputeHash,
		DisputeBody:      []byte(req.Dispute),
		Status:           "evidence_received",
	}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	decision, resolutionBody, err := h.decision.Decide(r.Context(), llm.Evidence{
		MarketAddress:    h.cfg.MarketAddress,
		OrderID:          orderID.String(),
		BuyerAddress:     order.Buyer.Hex(),
		SellerAddress:    order.Seller.Hex(),
		ValidatorAddress: order.Validator.Hex(),
		RequestHash:      requestHash,
		DeliveryHash:     deliveryHash,
		Request:          req.Request,
		Delivery:         req.Delivery,
		Dispute:          req.Dispute,
	})
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("LLM decision: %v", err))
		return
	}

	resolutionHashBytes := agentcrypto.Keccak256([]byte(resolutionBody))
	resolutionHash := agentcrypto.Keccak256Hex(resolutionBody)
	resolveTxHash, err := h.market.ResolveDispute(r.Context(), orderID, decision.ReleaseToSeller, resolutionHashBytes)
	if err != nil {
		_ = h.store.MarkResolved(r.Context(), orderID, resolutionHash, resolutionBody, decision.ReleaseToSeller, "", "resolution_failed")
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("resolve dispute: %v", err))
		return
	}
	if err := h.store.MarkResolved(r.Context(), orderID, resolutionHash, resolutionBody, decision.ReleaseToSeller, resolveTxHash, "resolved"); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":               "resolved",
		"orderId":              orderID.String(),
		"requestHashVerified":  true,
		"deliveryHashVerified": deliveryHashVerified,
		"releaseToSeller":      decision.ReleaseToSeller,
		"resolutionHash":       resolutionHash,
		"resolveTxHash":        resolveTxHash,
	})
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
	writeJSON(w, status, errorResponse{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func toDisputeSummary(dispute store.Dispute) disputeSummary {
	orderID := ""
	if dispute.ChainOrderID != nil {
		orderID = dispute.ChainOrderID.String()
	}

	return disputeSummary{
		OrderID:          orderID,
		BuyerAddress:     dispute.BuyerAddress,
		SellerAddress:    dispute.SellerAddress,
		ValidatorAddress: dispute.ValidatorAddress,
		Status:           dispute.Status,
		ReleaseToSeller:  dispute.ReleaseToSeller,
		ResolutionHash:   dispute.ResolutionHash,
		ResolveTxHash:    dispute.ResolveTxHash,
		CreatedAt:        dispute.CreatedAt,
		UpdatedAt:        dispute.UpdatedAt,
	}
}

func toDisputeDetail(dispute store.Dispute) disputeDetail {
	detail := disputeDetail{
		disputeSummary: toDisputeSummary(dispute),
		RequestHash:    dispute.RequestHash,
		Request:        string(dispute.RequestBody),
		DeliveryHash:   dispute.DeliveryHash,
		Delivery:       string(dispute.DeliveryBody),
		DisputeHash:    dispute.DisputeHash,
		Dispute:        string(dispute.DisputeBody),
	}

	if len(dispute.ResolutionBody) > 0 {
		var decision llm.Decision
		if err := json.Unmarshal(dispute.ResolutionBody, &decision); err == nil {
			detail.Decision = &decision
		}
	}

	return detail
}

// withCORS allows the read-only dashboard (a separate origin, e.g. the Vite dev server)
// to call these endpoints from a browser. Permissive origin is acceptable because every
// route is read-only and unauthenticated; OPTIONS preflight is short-circuited here.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
