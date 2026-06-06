package config

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	defaultDBPath          = "./validator-service.db"
	defaultHTTPAddr        = ":8082"
	defaultDeepSeekBaseURL = "https://api.deepseek.com"
	defaultDeepSeekModel   = "deepseek-chat"
)

type Config struct {
	RPCURL              string
	MarketAddress       common.Address
	ValidatorPrivateKey *ecdsa.PrivateKey
	ValidatorAddress    common.Address
	ValidatorBaseURL    string
	DBPath              string
	HTTPAddr            string
	DeepSeekAPIKey      string
	DeepSeekBaseURL     string
	DeepSeekModel       string
}

func Load() (Config, error) {
	rpcURL := strings.TrimSpace(os.Getenv("VALIDATOR_RPC_URL"))
	marketAddress := strings.TrimSpace(os.Getenv("VALIDATOR_MARKET_ADDRESS"))
	privateKeyHex := strings.TrimSpace(os.Getenv("VALIDATOR_PRIVATE_KEY"))
	validatorBaseURL := strings.TrimSpace(os.Getenv("VALIDATOR_BASE_URL"))
	dbPath := strings.TrimSpace(os.Getenv("VALIDATOR_DB_PATH"))
	httpAddr := strings.TrimSpace(os.Getenv("VALIDATOR_HTTP_ADDR"))
	deepSeekAPIKey := strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY"))
	deepSeekBaseURL := strings.TrimSpace(os.Getenv("DEEPSEEK_BASE_URL"))
	deepSeekModel := strings.TrimSpace(os.Getenv("DEEPSEEK_MODEL"))

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
	if deepSeekAPIKey == "" {
		missing = append(missing, "DEEPSEEK_API_KEY")
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
	if deepSeekBaseURL == "" {
		deepSeekBaseURL = defaultDeepSeekBaseURL
	}
	if deepSeekModel == "" {
		deepSeekModel = defaultDeepSeekModel
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
		DeepSeekAPIKey:      deepSeekAPIKey,
		DeepSeekBaseURL:     strings.TrimRight(deepSeekBaseURL, "/"),
		DeepSeekModel:       deepSeekModel,
	}, nil
}
