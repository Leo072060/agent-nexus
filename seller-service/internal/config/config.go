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
	defaultDBPath   = "./seller-service.db"
	defaultHTTPAddr = ":8081"
	defaultPoll     = 5 * time.Second
)

type Config struct {
	RPCURL           string
	MarketAddress    common.Address
	SellerPrivateKey *ecdsa.PrivateKey
	SellerAddress    common.Address
	SellerBaseURL    string
	DBPath           string
	HTTPAddr         string
	PollInterval     time.Duration
}

func Load() (Config, error) {
	rpcURL := strings.TrimSpace(os.Getenv("SELLER_RPC_URL"))
	marketAddress := strings.TrimSpace(os.Getenv("SELLER_MARKET_ADDRESS"))
	privateKeyHex := strings.TrimSpace(os.Getenv("SELLER_PRIVATE_KEY"))
	sellerBaseURL := strings.TrimSpace(os.Getenv("SELLER_BASE_URL"))
	dbPath := strings.TrimSpace(os.Getenv("SELLER_DB_PATH"))
	httpAddr := strings.TrimSpace(os.Getenv("SELLER_HTTP_ADDR"))
	pollIntervalText := strings.TrimSpace(os.Getenv("SELLER_POLL_INTERVAL"))

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
	if sellerBaseURL == "" {
		missing = append(missing, "SELLER_BASE_URL")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}
	if !common.IsHexAddress(marketAddress) {
		return Config{}, errors.New("SELLER_MARKET_ADDRESS must be a valid address")
	}
	if dbPath == "" {
		dbPath = defaultDBPath
	}
	if httpAddr == "" {
		httpAddr = defaultHTTPAddr
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

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(privateKeyHex, "0x"))
	if err != nil {
		return Config{}, fmt.Errorf("parse SELLER_PRIVATE_KEY: %w", err)
	}

	return Config{
		RPCURL:           rpcURL,
		MarketAddress:    common.HexToAddress(marketAddress),
		SellerPrivateKey: privateKey,
		SellerAddress:    crypto.PubkeyToAddress(privateKey.PublicKey),
		SellerBaseURL:    sellerBaseURL,
		DBPath:           dbPath,
		HTTPAddr:         httpAddr,
		PollInterval:     pollInterval,
	}, nil
}
