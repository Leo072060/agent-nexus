package config

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	defaultDBPath     = "./validator-service.db"
	defaultHTTPAddr   = ":8082"
	defaultLLMTimeout = 60 * time.Second
)

type Config struct {
	RPCURL              string
	MarketAddress       common.Address
	ValidatorPrivateKey *ecdsa.PrivateKey
	ValidatorAddress    common.Address
	ValidatorBaseURL    string
	DBPath              string
	HTTPAddr            string
	LLMScript           string
	LLMAPIKey           string
	LLMTimeout          time.Duration
}

func Load() (Config, error) {
	rpcURL := strings.TrimSpace(os.Getenv("VALIDATOR_RPC_URL"))
	marketAddress := strings.TrimSpace(os.Getenv("VALIDATOR_MARKET_ADDRESS"))
	privateKeyHex := strings.TrimSpace(os.Getenv("VALIDATOR_PRIVATE_KEY"))
	validatorBaseURL := strings.TrimSpace(os.Getenv("VALIDATOR_BASE_URL"))
	dbPath := strings.TrimSpace(os.Getenv("VALIDATOR_DB_PATH"))
	httpAddr := strings.TrimSpace(os.Getenv("VALIDATOR_HTTP_ADDR"))
	llmScript := strings.TrimSpace(os.Getenv("VALIDATOR_LLM_SCRIPT"))
	llmAPIKey := strings.TrimSpace(os.Getenv("VALIDATOR_LLM_API_KEY"))
	llmTimeoutText := strings.TrimSpace(os.Getenv("VALIDATOR_LLM_TIMEOUT"))

	var missing []string
	if rpcURL == "" {
		missing = append(missing, "VALIDATOR_RPC_URL")
	}
	if marketAddress == "" {
		missing = append(missing, "VALIDATOR_MARKET_ADDRESS")
	}
	if privateKeyHex == "" {
		missing = append(missing, "VALIDATOR_PRIVATE_KEY")
	}
	if validatorBaseURL == "" {
		missing = append(missing, "VALIDATOR_BASE_URL")
	}
	if llmScript == "" {
		missing = append(missing, "VALIDATOR_LLM_SCRIPT")
	}
	if llmAPIKey == "" {
		missing = append(missing, "VALIDATOR_LLM_API_KEY")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}
	if !common.IsHexAddress(marketAddress) {
		return Config{}, errors.New("VALIDATOR_MARKET_ADDRESS must be a valid address")
	}
	if dbPath == "" {
		dbPath = defaultDBPath
	}
	if httpAddr == "" {
		httpAddr = defaultHTTPAddr
	}
	llmTimeout := defaultLLMTimeout
	if llmTimeoutText != "" {
		parsed, err := time.ParseDuration(llmTimeoutText)
		if err != nil {
			return Config{}, fmt.Errorf("parse VALIDATOR_LLM_TIMEOUT: %w", err)
		}
		if parsed <= 0 {
			return Config{}, errors.New("VALIDATOR_LLM_TIMEOUT must be positive")
		}
		llmTimeout = parsed
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return Config{}, fmt.Errorf("parse VALIDATOR_PRIVATE_KEY: %w", err)
	}

	return Config{
		RPCURL:              rpcURL,
		MarketAddress:       common.HexToAddress(marketAddress),
		ValidatorPrivateKey: privateKey,
		ValidatorAddress:    crypto.PubkeyToAddress(privateKey.PublicKey),
		ValidatorBaseURL:    validatorBaseURL,
		DBPath:              dbPath,
		HTTPAddr:            httpAddr,
		LLMScript:           llmScript,
		LLMAPIKey:           llmAPIKey,
		LLMTimeout:          llmTimeout,
	}, nil
}
