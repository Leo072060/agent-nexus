package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

const maxStderrLogBytes = 2048

type Client struct {
	scriptPath string
	apiKey     string
	timeout    time.Duration
}

type Evidence struct {
	MarketAddress    common.Address `json:"marketAddress"`
	OrderID          string         `json:"orderId"`
	BuyerAddress     string         `json:"buyerAddress"`
	SellerAddress    string         `json:"sellerAddress"`
	ValidatorAddress string         `json:"validatorAddress"`
	RequestHash      string         `json:"requestHash"`
	DeliveryHash     string         `json:"deliveryHash"`
	Request          string         `json:"request"`
	Delivery         string         `json:"delivery"`
	Dispute          string         `json:"dispute"`
}

type Decision struct {
	ReleaseToSeller          bool   `json:"releaseToSeller"`
	Summary                  string `json:"summary"`
	Reasoning                string `json:"reasoning"`
	BuyerClaim               string `json:"buyerClaim"`
	SellerDeliveryAssessment string `json:"sellerDeliveryAssessment"`
	Confidence               string `json:"confidence"`
}

func NewClient(scriptPath string, apiKey string, timeout time.Duration) (*Client, error) {
	if strings.TrimSpace(scriptPath) == "" {
		return nil, errors.New("VALIDATOR_LLM_SCRIPT is required")
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("VALIDATOR_LLM_API_KEY is required")
	}
	if timeout <= 0 {
		return nil, errors.New("VALIDATOR_LLM_TIMEOUT must be positive")
	}

	return &Client{
		scriptPath: scriptPath,
		apiKey:     apiKey,
		timeout:    timeout,
	}, nil
}

func (c *Client) Decide(ctx context.Context, evidence Evidence) (Decision, []byte, error) {
	startedAt := time.Now()
	log.Printf("validator llm script started order_id=%s market_address=%s script=%s", evidence.OrderID, evidence.MarketAddress.Hex(), c.scriptPath)

	stdin, err := json.Marshal(evidence)
	if err != nil {
		return Decision{}, nil, fmt.Errorf("marshal llm evidence: %w", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, c.scriptPath)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Env = append(os.Environ(), "VALIDATOR_LLM_API_KEY="+c.apiKey)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if runCtx.Err() != nil {
			log.Printf("validator llm script failed order_id=%s error=%v stderr=%s", evidence.OrderID, runCtx.Err(), truncate(stderr.String()))
			return Decision{}, nil, fmt.Errorf("run validator llm script: %w", runCtx.Err())
		}
		log.Printf("validator llm script failed order_id=%s error=%v stderr=%s", evidence.OrderID, err, truncate(stderr.String()))
		return Decision{}, nil, fmt.Errorf("run validator llm script: %w", err)
	}

	decision, reportBody, err := parseDecision(stdout.Bytes())
	if err != nil {
		log.Printf("validator llm script returned invalid output order_id=%s error=%v stderr=%s", evidence.OrderID, err, truncate(stderr.String()))
		return Decision{}, nil, err
	}

	log.Printf("validator llm script completed order_id=%s duration=%s report_bytes=%d", evidence.OrderID, time.Since(startedAt), len(reportBody))
	return decision, reportBody, nil
}

func parseDecision(stdout []byte) (Decision, []byte, error) {
	var decision Decision
	if err := json.Unmarshal(stdout, &decision); err != nil {
		return Decision{}, nil, fmt.Errorf("parse validator llm script JSON: %w", err)
	}
	decision.Summary = strings.TrimSpace(decision.Summary)
	decision.Reasoning = strings.TrimSpace(decision.Reasoning)
	decision.BuyerClaim = strings.TrimSpace(decision.BuyerClaim)
	decision.SellerDeliveryAssessment = strings.TrimSpace(decision.SellerDeliveryAssessment)
	decision.Confidence = strings.TrimSpace(decision.Confidence)

	if decision.Summary == "" || decision.Reasoning == "" {
		return Decision{}, nil, errors.New("validator llm decision missing summary or reasoning")
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

func truncate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxStderrLogBytes {
		return value
	}
	return value[:maxStderrLogBytes] + "...(truncated)"
}
