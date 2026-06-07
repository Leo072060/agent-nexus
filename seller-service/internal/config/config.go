package config

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	defaultDBPath     = "./seller-service.db"
	defaultHTTPAddr   = ":8081"
	defaultLogPath    = "./seller-service.log"
	defaultPoll       = 5 * time.Second
	defaultLLMTimeout = 60 * time.Second
)

type Config struct {
	RPCURL                string
	MarketAddress         common.Address
	SellerPrivateKey      *ecdsa.PrivateKey
	SellerAddress         common.Address
	SellerURI             string
	SellerPriceWei        *big.Int
	SellerContentURI      string
	SellerContentHash     [32]byte
	SellerDeliveryTimeout *big.Int
	SupportedValidators   []common.Address
	ServiceID             string
	ServiceName           string
	ServiceDescription    string
	LLMScript             string
	LLMAPIKey             string
	LLMTimeout            time.Duration
	DBPath                string
	HTTPAddr              string
	LogPath               string
	PollInterval          time.Duration
}

func Load() (Config, error) {
	rpcURL := strings.TrimSpace(os.Getenv("SELLER_RPC_URL"))
	marketAddress := strings.TrimSpace(os.Getenv("SELLER_MARKET_ADDRESS"))
	privateKeyHex := strings.TrimSpace(os.Getenv("SELLER_PRIVATE_KEY"))
	sellerURI := strings.TrimSpace(os.Getenv("SELLER_URI"))
	sellerPriceText := strings.TrimSpace(os.Getenv("SELLER_PRICE_WEI"))
	sellerContentURI := strings.TrimSpace(os.Getenv("SELLER_CONTENT_URI"))
	sellerContentHashText := strings.TrimSpace(os.Getenv("SELLER_CONTENT_HASH"))
	sellerDeliveryTimeoutText := strings.TrimSpace(os.Getenv("SELLER_DELIVERY_TIMEOUT"))
	supportedValidatorsText := strings.TrimSpace(os.Getenv("SELLER_SUPPORTED_VALIDATORS"))
	serviceID := strings.TrimSpace(os.Getenv("SELLER_SERVICE_ID"))
	serviceName := strings.TrimSpace(os.Getenv("SELLER_SERVICE_NAME"))
	serviceDescription := strings.TrimSpace(os.Getenv("SELLER_SERVICE_DESCRIPTION"))
	llmScript := strings.TrimSpace(os.Getenv("SELLER_LLM_SCRIPT"))
	llmAPIKey := strings.TrimSpace(os.Getenv("SELLER_LLM_API_KEY"))
	dbPath := strings.TrimSpace(os.Getenv("SELLER_DB_PATH"))
	httpAddr := strings.TrimSpace(os.Getenv("SELLER_HTTP_ADDR"))
	logPath := strings.TrimSpace(os.Getenv("SELLER_LOG_PATH"))
	pollIntervalText := strings.TrimSpace(os.Getenv("SELLER_POLL_INTERVAL"))
	llmTimeoutText := strings.TrimSpace(os.Getenv("SELLER_LLM_TIMEOUT"))

	var missing []string
	if rpcURL == "" {
		missing = append(missing, "SELLER_RPC_URL")
	}
	if marketAddress == "" {
		missing = append(missing, "SELLER_MARKET_ADDRESS")
	}
	if privateKeyHex == "" {
		missing = append(missing, "SELLER_PRIVATE_KEY")
	}
	if sellerURI == "" {
		missing = append(missing, "SELLER_URI")
	}
	if sellerPriceText == "" {
		missing = append(missing, "SELLER_PRICE_WEI")
	}
	if sellerContentURI == "" {
		missing = append(missing, "SELLER_CONTENT_URI")
	}
	if sellerContentHashText == "" {
		missing = append(missing, "SELLER_CONTENT_HASH")
	}
	if sellerDeliveryTimeoutText == "" {
		missing = append(missing, "SELLER_DELIVERY_TIMEOUT")
	}
	if supportedValidatorsText == "" {
		missing = append(missing, "SELLER_SUPPORTED_VALIDATORS")
	}
	if serviceID == "" {
		missing = append(missing, "SELLER_SERVICE_ID")
	}
	if serviceName == "" {
		missing = append(missing, "SELLER_SERVICE_NAME")
	}
	if serviceDescription == "" {
		missing = append(missing, "SELLER_SERVICE_DESCRIPTION")
	}
	if llmScript == "" {
		missing = append(missing, "SELLER_LLM_SCRIPT")
	}
	if llmAPIKey == "" {
		missing = append(missing, "SELLER_LLM_API_KEY")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}
	if !common.IsHexAddress(marketAddress) {
		return Config{}, errors.New("SELLER_MARKET_ADDRESS must be a valid address")
	}
	sellerPriceWei, ok := new(big.Int).SetString(sellerPriceText, 10)
	if !ok || sellerPriceWei.Sign() < 0 {
		return Config{}, errors.New("SELLER_PRICE_WEI must be a non-negative decimal integer")
	}
	contentHashBytes := common.FromHex(sellerContentHashText)
	if len(contentHashBytes) != 32 {
		return Config{}, errors.New("SELLER_CONTENT_HASH must be 32 bytes")
	}
	var sellerContentHash [32]byte
	copy(sellerContentHash[:], contentHashBytes)
	if sellerContentHash == ([32]byte{}) {
		return Config{}, errors.New("SELLER_CONTENT_HASH must be non-zero")
	}
	sellerDeliveryTimeout, ok := new(big.Int).SetString(sellerDeliveryTimeoutText, 10)
	if !ok || sellerDeliveryTimeout.Sign() <= 0 {
		return Config{}, errors.New("SELLER_DELIVERY_TIMEOUT must be a positive decimal integer")
	}
	supportedValidators, err := parseAddresses(supportedValidatorsText)
	if err != nil {
		return Config{}, err
	}
	if dbPath == "" {
		dbPath = defaultDBPath
	}
	if httpAddr == "" {
		httpAddr = defaultHTTPAddr
	}
	if logPath == "" {
		logPath = defaultLogPath
	}
	pollInterval := defaultPoll
	if pollIntervalText != "" {
		parsed, err := time.ParseDuration(pollIntervalText)
		if err != nil {
			return Config{}, fmt.Errorf("parse SELLER_POLL_INTERVAL: %w", err)
		}
		if parsed <= 0 {
			return Config{}, errors.New("SELLER_POLL_INTERVAL must be positive")
		}
		pollInterval = parsed
	}
	llmTimeout := defaultLLMTimeout
	if llmTimeoutText != "" {
		parsed, err := time.ParseDuration(llmTimeoutText)
		if err != nil {
			return Config{}, fmt.Errorf("parse SELLER_LLM_TIMEOUT: %w", err)
		}
		if parsed <= 0 {
			return Config{}, errors.New("SELLER_LLM_TIMEOUT must be positive")
		}
		llmTimeout = parsed
	}

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return Config{}, fmt.Errorf("parse SELLER_PRIVATE_KEY: %w", err)
	}

	return Config{
		RPCURL:                rpcURL,
		MarketAddress:         common.HexToAddress(marketAddress),
		SellerPrivateKey:      privateKey,
		SellerAddress:         crypto.PubkeyToAddress(privateKey.PublicKey),
		SellerURI:             sellerURI,
		SellerPriceWei:        sellerPriceWei,
		SellerContentURI:      sellerContentURI,
		SellerContentHash:     sellerContentHash,
		SellerDeliveryTimeout: sellerDeliveryTimeout,
		SupportedValidators:   supportedValidators,
		ServiceID:             serviceID,
		ServiceName:           serviceName,
		ServiceDescription:    serviceDescription,
		LLMScript:             llmScript,
		LLMAPIKey:             llmAPIKey,
		LLMTimeout:            llmTimeout,
		DBPath:                dbPath,
		HTTPAddr:              httpAddr,
		LogPath:               logPath,
		PollInterval:          pollInterval,
	}, nil
}

func parseAddresses(value string) ([]common.Address, error) {
	parts := strings.Split(value, ",")
	addresses := make([]common.Address, 0, len(parts))
	seen := make(map[common.Address]bool, len(parts))
	for _, part := range parts {
		text := strings.TrimSpace(part)
		if text == "" {
			continue
		}
		if !common.IsHexAddress(text) {
			return nil, fmt.Errorf("SELLER_SUPPORTED_VALIDATORS contains invalid address: %s", text)
		}
		address := common.HexToAddress(text)
		if seen[address] {
			continue
		}
		seen[address] = true
		addresses = append(addresses, address)
	}
	if len(addresses) == 0 {
		return nil, errors.New("SELLER_SUPPORTED_VALIDATORS must include at least one address")
	}
	return addresses, nil
}
