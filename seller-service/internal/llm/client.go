package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
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

type Result struct {
	Answer   []byte
	Evidence []byte
}

func NewClient(scriptPath string, apiKey string, timeout time.Duration) *Client {
	return &Client{
		scriptPath: scriptPath,
		apiKey:     apiKey,
		timeout:    timeout,
	}
}

func (c *Client) Generate(ctx context.Context, marketAddress common.Address, orderID *big.Int, requestBody []byte) (Result, error) {
	startedAt := time.Now()
	log.Printf("llm script started order_id=%s market_address=%s script=%s", orderID.String(), marketAddress.Hex(), c.scriptPath)

	runCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, c.scriptPath)
	cmd.Stdin = bytes.NewReader(requestBody)
	cmd.Env = append(os.Environ(), "SELLER_LLM_API_KEY="+c.apiKey)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if runCtx.Err() != nil {
			log.Printf("llm script failed order_id=%s error=%v stderr=%s", orderID.String(), runCtx.Err(), truncate(stderr.String()))
			return Result{}, fmt.Errorf("run llm script: %w", runCtx.Err())
		}
		log.Printf("llm script failed order_id=%s error=%v stderr=%s", orderID.String(), err, truncate(stderr.String()))
		return Result{}, fmt.Errorf("run llm script: %w", err)
	}

	result, err := parseResult(stdout.Bytes())
	if err != nil {
		log.Printf("llm script returned invalid output order_id=%s error=%v stderr=%s", orderID.String(), err, truncate(stderr.String()))
		return Result{}, err
	}

	log.Printf("llm script completed order_id=%s duration=%s answer_bytes=%d evidence_bytes=%d", orderID.String(), time.Since(startedAt), len(result.Answer), len(result.Evidence))
	return result, nil
}

func parseResult(stdout []byte) (Result, error) {
	var payload struct {
		Answer   string `json:"answer"`
		Evidence string `json:"evidence"`
	}
	if err := json.Unmarshal(stdout, &payload); err != nil {
		return Result{}, fmt.Errorf("parse llm script JSON: %w", err)
	}

	answer := []byte(strings.TrimSpace(payload.Answer))
	if len(answer) == 0 {
		return Result{}, fmt.Errorf("llm script returned empty answer")
	}
	evidence := []byte(strings.TrimSpace(payload.Evidence))
	if len(evidence) == 0 {
		return Result{}, fmt.Errorf("llm script returned empty evidence")
	}

	return Result{Answer: answer, Evidence: evidence}, nil
}

func truncate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxStderrLogBytes {
		return value
	}
	return value[:maxStderrLogBytes] + "...(truncated)"
}
