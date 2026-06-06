package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

type Client struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

type Evidence struct {
	MarketAddress    common.Address
	OrderID          string
	BuyerAddress     string
	SellerAddress    string
	ValidatorAddress string
	RequestHash      string
	DeliveryHash     string
	Request          string
	Delivery         string
	Dispute          string
}

type Decision struct {
	ReleaseToSeller          bool   `json:"releaseToSeller"`
	Summary                  string `json:"summary"`
	Reasoning                string `json:"reasoning"`
	BuyerClaim               string `json:"buyerClaim"`
	SellerDeliveryAssessment string `json:"sellerDeliveryAssessment"`
	Confidence               string `json:"confidence"`
}

func NewClient(apiKey string, baseURL string, model string) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("DEEPSEEK_API_KEY is required")
	}
	if strings.TrimSpace(baseURL) == "" {
		return nil, errors.New("DeepSeek base URL is required")
	}
	if strings.TrimSpace(model) == "" {
		return nil, errors.New("DeepSeek model is required")
	}

	return &Client{
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		model:      model,
		httpClient: http.DefaultClient,
	}, nil
}

func (c *Client) Decide(ctx context.Context, evidence Evidence) (Decision, []byte, error) {
	payload := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: systemPrompt(),
			},
			{
				Role:    "user",
				Content: userPrompt(evidence),
			},
		},
		Temperature: 0,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Decision{}, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Decision{}, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Decision{}, nil, fmt.Errorf("call DeepSeek: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Decision{}, nil, fmt.Errorf("read DeepSeek response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Decision{}, nil, fmt.Errorf("DeepSeek request failed: status=%d body=%s", resp.StatusCode, string(responseBody))
	}

	var response chatResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return Decision{}, nil, fmt.Errorf("decode DeepSeek response: %w", err)
	}
	if len(response.Choices) == 0 {
		return Decision{}, nil, errors.New("DeepSeek response has no choices")
	}

	content := strings.TrimSpace(response.Choices[0].Message.Content)
	content = trimCodeFence(content)
	var decision Decision
	if err := json.Unmarshal([]byte(content), &decision); err != nil {
		return Decision{}, nil, fmt.Errorf("decode DeepSeek decision JSON: %w", err)
	}
	if decision.Summary == "" || decision.Reasoning == "" {
		return Decision{}, nil, errors.New("DeepSeek decision missing summary or reasoning")
	}
	if decision.Confidence == "" {
		decision.Confidence = "medium"
	}

	reportBody, err := json.MarshalIndent(decision, "", "  ")
	if err != nil {
		return Decision{}, nil, err
	}

	return decision, reportBody, nil
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature int           `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func systemPrompt() string {
	return `You are an Agent Nexus validator. Decide a digital delivery escrow dispute.
Return ONLY valid JSON with exactly these fields:
{
  "releaseToSeller": true,
  "summary": "...",
  "reasoning": "...",
  "buyerClaim": "...",
  "sellerDeliveryAssessment": "...",
  "confidence": "low|medium|high"
}
Set releaseToSeller=true if the seller substantially satisfied the buyer request and delivery standard. Set it false if the buyer should receive the escrow amount.`
}

func userPrompt(e Evidence) string {
	return fmt.Sprintf(`Order:
marketAddress: %s
orderId: %s
buyerAddress: %s
sellerAddress: %s
validatorAddress: %s
requestHash: %s
deliveryHash: %s

Buyer request:
%s

Seller delivery:
%s

Buyer dispute reason:
%s
`, e.MarketAddress.Hex(), e.OrderID, e.BuyerAddress, e.SellerAddress, e.ValidatorAddress, e.RequestHash, e.DeliveryHash, e.Request, e.Delivery, e.Dispute)
}

func trimCodeFence(content string) string {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "```") {
		return content
	}

	lines := strings.Split(content, "\n")
	if len(lines) >= 3 {
		lines = lines[1 : len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
