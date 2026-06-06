package commands

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"agent-nexus-cli/internal/chain"
	agentcrypto "agent-nexus-cli/internal/crypto"
	"agent-nexus-cli/internal/store"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
)

func NewOrdersCommand(cfg *AppConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "orders",
		Short: "Manage buyer orders",
	}

	cmd.AddCommand(newOrdersWatchCommand(cfg))
	cmd.AddCommand(newOrdersDisputeCommand(cfg))

	return cmd
}

func newOrdersDisputeCommand(cfg *AppConfig) *cobra.Command {
	var id int64
	var requestText string
	var requestFile string
	var deliveryText string
	var deliveryFile string
	var reasonText string
	var reasonFile string

	cmd := &cobra.Command{
		Use:   "dispute",
		Short: "Open an order dispute and submit evidence to validator",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id <= 0 {
				return fmt.Errorf("--id is required")
			}

			requestBody, err := readTextOrFile(requestText, requestFile, "request")
			if err != nil {
				return err
			}
			deliveryBody, err := readOptionalTextOrFile(deliveryText, deliveryFile, "delivery")
			if err != nil {
				return err
			}
			disputeBody, err := readTextOrFile(reasonText, reasonFile, "reason")
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			db, err := store.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			order, err := db.GetOrder(ctx, id)
			if err != nil {
				return err
			}
			if strings.TrimSpace(order.ValidatorURI) == "" {
				return fmt.Errorf("order validatorURI is required to submit evidence")
			}

			privateKey, err := loadBuyerPrivateKey()
			if err != nil {
				return err
			}
			buyerAddress := crypto.PubkeyToAddress(privateKey.PublicKey)

			market, err := chain.NewMarketClientWithPrivateKey(ctx, order.RPCURL, order.MarketAddress, privateKey)
			if err != nil {
				return err
			}
			defer market.Close()

			chainOrder, err := market.GetOrder(ctx, order.ChainOrderID)
			if err != nil {
				return err
			}
			if chainOrder.Buyer != buyerAddress {
				return fmt.Errorf("buyer private key does not match chain order buyer")
			}

			requestHash := agentcrypto.Keccak256Hex([]byte(requestBody))
			if !strings.EqualFold(requestHash, chainOrder.RequestHash) {
				return fmt.Errorf("request hash mismatch: got %s want %s", requestHash, chainOrder.RequestHash)
			}

			deliveryHash := agentcrypto.Keccak256Hex([]byte(deliveryBody))
			deliveryHashVerified := false
			if deliveryBody != "" {
				if !strings.EqualFold(deliveryHash, chainOrder.DeliveryHash) {
					return fmt.Errorf("delivery hash mismatch: got %s want %s", deliveryHash, chainOrder.DeliveryHash)
				}
				deliveryHashVerified = true
			}

			disputeHash := agentcrypto.Keccak256Hex([]byte(disputeBody))
			openDisputeTxHash, err := market.OpenDispute(ctx, order.ChainOrderID)
			if err != nil {
				return err
			}
			if err := waitOrderStatus(ctx, market, order.ChainOrderID, chain.OrderStatusDisputed, 30*time.Second); err != nil {
				return err
			}

			if err := db.UpdateOrderDispute(ctx, order.ID, requestBody, deliveryBody, disputeBody, openDisputeTxHash, "Disputed"); err != nil {
				return err
			}

			message := agentcrypto.DisputeEvidenceMessage(market.Address(), order.ChainOrderID, requestHash, deliveryHash, disputeHash)
			signature, err := agentcrypto.SignPersonalMessage(privateKey, message)
			if err != nil {
				return err
			}

			endpoint, err := disputeEndpoint(order.ValidatorURI)
			if err != nil {
				return err
			}

			evidenceSubmitted := true
			evidenceError := ""
			err = submitDisputeEvidence(ctx, endpoint, disputeEvidenceRequest{
				MarketAddress: order.MarketAddress,
				OrderID:       order.ChainOrderID.String(),
				BuyerAddress:  buyerAddress.Hex(),
				Request:       requestBody,
				Delivery:      deliveryBody,
				Dispute:       disputeBody,
				Signature:     signature,
			})
			if err != nil {
				evidenceSubmitted = false
				evidenceError = err.Error()
			}

			output := map[string]any{
				"status":               "dispute_opened",
				"orderId":              order.ID,
				"chainOrderId":         order.ChainOrderID.String(),
				"requestHashVerified":  true,
				"deliveryHashVerified": deliveryHashVerified,
				"openDisputeTxHash":    openDisputeTxHash,
				"evidenceSubmitted":    evidenceSubmitted,
			}
			if evidenceError != "" {
				output["evidenceError"] = evidenceError
			}
			return writeJSON(cmd, output)
		},
	}

	cmd.Flags().Int64Var(&id, "id", 0, "local order id")
	cmd.Flags().StringVar(&requestText, "request", "", "buyer request text")
	cmd.Flags().StringVar(&requestFile, "request-file", "", "buyer request file")
	cmd.Flags().StringVar(&deliveryText, "delivery", "", "seller delivery text")
	cmd.Flags().StringVar(&deliveryFile, "delivery-file", "", "seller delivery file")
	cmd.Flags().StringVar(&reasonText, "reason", "", "dispute reason text")
	cmd.Flags().StringVar(&reasonFile, "reason-file", "", "dispute reason file")
	return cmd
}

func newOrdersWatchCommand(cfg *AppConfig) *cobra.Command {
	var id int64

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Fetch seller delivery for a committed order",
		RunE: func(cmd *cobra.Command, args []string) error {
			if id <= 0 {
				return fmt.Errorf("--id is required")
			}

			ctx := cmd.Context()
			db, err := store.Open(cfg.DBPath)
			if err != nil {
				return err
			}
			defer db.Close()

			order, err := db.GetOrder(ctx, id)
			if err != nil {
				return err
			}

			privateKey, err := loadBuyerPrivateKey()
			if err != nil {
				return err
			}

			market, err := chain.NewMarketClient(ctx, order.RPCURL, order.MarketAddress)
			if err != nil {
				return err
			}
			defer market.Close()

			chainOrder, err := market.GetOrder(ctx, order.ChainOrderID)
			if err != nil {
				return err
			}
			if chainOrder.Status != chain.OrderStatusDeliveryCommitted {
				return fmt.Errorf("order is not DeliveryCommitted")
			}

			message := agentcrypto.DeliveryRequestMessage(market.Address(), order.ChainOrderID)
			signature, err := agentcrypto.SignPersonalMessage(privateKey, message)
			if err != nil {
				return err
			}

			deliveryURL, err := deliveryEndpoint(order.SellerURI)
			if err != nil {
				return err
			}

			responseBody, err := requestDelivery(ctx, deliveryURL, order.MarketAddress, order.ChainOrderID.String(), signature)
			if err != nil {
				return err
			}

			deliveryHash := agentcrypto.Keccak256Hex(responseBody)
			if !strings.EqualFold(deliveryHash, chainOrder.DeliveryHash) {
				return fmt.Errorf("delivery hash mismatch: got %s want %s", deliveryHash, chainOrder.DeliveryHash)
			}

			if err := db.UpdateOrderDelivery(ctx, order.ID, deliveryHash, string(responseBody), "DeliveryReceived"); err != nil {
				return err
			}

			return writeJSON(cmd, map[string]any{
				"status":               "delivery_received",
				"orderId":              order.ID,
				"chainOrderId":         order.ChainOrderID.String(),
				"deliveryHash":         deliveryHash,
				"deliveryHashVerified": true,
				"deliverySaved":        true,
			})
		},
	}

	cmd.Flags().Int64Var(&id, "id", 0, "local order id")
	return cmd
}

func deliveryEndpoint(sellerURI string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(sellerURI))
	if err != nil {
		return "", fmt.Errorf("parse sellerURI: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("sellerURI must be an absolute HTTP URL")
	}

	path := strings.TrimRight(base.Path, "/") + "/agent-nexus/delivery"
	base.Path = path
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}

func disputeEndpoint(validatorURI string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(validatorURI))
	if err != nil {
		return "", fmt.Errorf("parse validatorURI: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("validatorURI must be an absolute HTTP URL")
	}

	path := strings.TrimRight(base.Path, "/") + "/agent-nexus/disputes"
	base.Path = path
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}

type disputeEvidenceRequest struct {
	MarketAddress string `json:"marketAddress"`
	OrderID       string `json:"orderId"`
	BuyerAddress  string `json:"buyerAddress"`
	Request       string `json:"request"`
	Delivery      string `json:"delivery"`
	Dispute       string `json:"dispute"`
	Signature     string `json:"signature"`
}

func submitDisputeEvidence(ctx context.Context, endpoint string, payload disputeEvidenceRequest) error {
	body := []byte(fmt.Sprintf(
		`{"marketAddress":%q,"orderId":%q,"buyerAddress":%q,"request":%q,"delivery":%q,"dispute":%q,"signature":%q}`,
		payload.MarketAddress,
		payload.OrderID,
		payload.BuyerAddress,
		payload.Request,
		payload.Delivery,
		payload.Dispute,
		payload.Signature,
	))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("submit dispute evidence: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read dispute evidence response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("dispute evidence request failed: status=%d body=%s", resp.StatusCode, string(responseBody))
	}

	return nil
}

func readTextOrFile(text string, path string, name string) (string, error) {
	value, err := readOptionalTextOrFile(text, path, name)
	if err != nil {
		return "", err
	}
	if value == "" {
		return "", fmt.Errorf("--%s or --%s-file is required", name, name)
	}
	return value, nil
}

func readOptionalTextOrFile(text string, path string, name string) (string, error) {
	if strings.TrimSpace(text) != "" && strings.TrimSpace(path) != "" {
		return "", fmt.Errorf("--%s and --%s-file cannot be used together", name, name)
	}
	if strings.TrimSpace(path) == "" {
		return text, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s file: %w", name, err)
	}
	return string(data), nil
}

func loadBuyerPrivateKey() (*ecdsa.PrivateKey, error) {
	privateKeyHex := strings.TrimSpace(os.Getenv("AGENT_NEXUS_PRIVATE_KEY"))
	if privateKeyHex == "" {
		privateKeyHex = strings.TrimSpace(os.Getenv("BUYER_PRIVATE_KEY"))
	}
	if privateKeyHex == "" {
		return nil, fmt.Errorf("missing buyer private key env: AGENT_NEXUS_PRIVATE_KEY or BUYER_PRIVATE_KEY")
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("parse buyer private key: %w", err)
	}
	return privateKey, nil
}

type orderReader interface {
	GetOrder(ctx context.Context, orderID *big.Int) (chain.Order, error)
}

func waitOrderStatus(ctx context.Context, market orderReader, orderID *big.Int, want uint8, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		order, err := market.GetOrder(ctx, orderID)
		if err != nil {
			return err
		}
		if order.Status == want {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for order status %d", want)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func requestDelivery(ctx context.Context, endpoint string, marketAddress string, orderID string, signature string) ([]byte, error) {
	body := []byte(fmt.Sprintf(
		`{"marketAddress":%q,"orderId":%q,"signature":%q}`,
		marketAddress,
		orderID,
		signature,
	))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request delivery: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read delivery response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("delivery request failed: status=%d body=%s", resp.StatusCode, string(responseBody))
	}

	return responseBody, nil
}
