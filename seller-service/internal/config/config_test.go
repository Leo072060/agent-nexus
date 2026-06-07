package config

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func TestLoadMissingRequiredEnv(t *testing.T) {
	t.Setenv("SELLER_RPC_URL", "")
	t.Setenv("SELLER_MARKET_ADDRESS", "")
	t.Setenv("SELLER_PRIVATE_KEY", "")
	t.Setenv("SELLER_URI", "")
	t.Setenv("SELLER_PRICE_WEI", "")
	t.Setenv("SELLER_CONTENT_URI", "")
	t.Setenv("SELLER_CONTENT_HASH", "")
	t.Setenv("SELLER_DELIVERY_TIMEOUT", "")
	t.Setenv("SELLER_SUPPORTED_VALIDATORS", "")
	t.Setenv("SELLER_SERVICE_ID", "")
	t.Setenv("SELLER_SERVICE_NAME", "")
	t.Setenv("SELLER_SERVICE_DESCRIPTION", "")
	t.Setenv("SELLER_LLM_SCRIPT", "")
	t.Setenv("SELLER_LLM_API_KEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}

	message := err.Error()
	for _, name := range []string{
		"SELLER_RPC_URL",
		"SELLER_MARKET_ADDRESS",
		"SELLER_PRIVATE_KEY",
		"SELLER_URI",
		"SELLER_PRICE_WEI",
		"SELLER_CONTENT_URI",
		"SELLER_CONTENT_HASH",
		"SELLER_DELIVERY_TIMEOUT",
		"SELLER_SUPPORTED_VALIDATORS",
		"SELLER_SERVICE_ID",
		"SELLER_SERVICE_NAME",
		"SELLER_SERVICE_DESCRIPTION",
		"SELLER_LLM_SCRIPT",
		"SELLER_LLM_API_KEY",
	} {
		if !strings.Contains(message, name) {
			t.Fatalf("expected missing env %s in %q", name, message)
		}
	}
}

func TestLoadValidConfig(t *testing.T) {
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("SELLER_RPC_URL", "http://127.0.0.1:8545")
	t.Setenv("SELLER_MARKET_ADDRESS", "0x1111111111111111111111111111111111111111")
	t.Setenv("SELLER_PRIVATE_KEY", "0x"+hex.EncodeToString(gethcrypto.FromECDSA(key)))
	t.Setenv("SELLER_URI", "https://seller.example/agent-nexus")
	t.Setenv("SELLER_PRICE_WEI", "100")
	t.Setenv("SELLER_CONTENT_URI", "ipfs://seller/product")
	t.Setenv("SELLER_CONTENT_HASH", "0x0100000000000000000000000000000000000000000000000000000000000000")
	t.Setenv("SELLER_DELIVERY_TIMEOUT", "3600")
	t.Setenv("SELLER_SUPPORTED_VALIDATORS", "0x3333333333333333333333333333333333333333,0x4444444444444444444444444444444444444444")
	t.Setenv("SELLER_SERVICE_ID", "contract-review")
	t.Setenv("SELLER_SERVICE_NAME", "Contract Review")
	t.Setenv("SELLER_SERVICE_DESCRIPTION", "Review contract text and return risk notes.")
	t.Setenv("SELLER_LLM_SCRIPT", "/tmp/seller-llm.sh")
	t.Setenv("SELLER_LLM_API_KEY", "test-api-key")
	t.Setenv("SELLER_LLM_TIMEOUT", "2s")
	t.Setenv("SELLER_POLL_INTERVAL", "100ms")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLMScript != "/tmp/seller-llm.sh" {
		t.Fatalf("llm script mismatch: %s", cfg.LLMScript)
	}
	if cfg.LLMAPIKey != "test-api-key" {
		t.Fatalf("llm api key mismatch: %s", cfg.LLMAPIKey)
	}
	if cfg.SellerURI != "https://seller.example/agent-nexus" {
		t.Fatalf("seller uri mismatch: %s", cfg.SellerURI)
	}
	if cfg.SellerPriceWei.String() != "100" {
		t.Fatalf("seller price mismatch: %s", cfg.SellerPriceWei.String())
	}
	if cfg.SellerContentURI != "ipfs://seller/product" {
		t.Fatalf("seller content uri mismatch: %s", cfg.SellerContentURI)
	}
	if cfg.SellerContentHash[0] != 1 {
		t.Fatalf("seller content hash mismatch")
	}
	if cfg.SellerDeliveryTimeout.String() != "3600" {
		t.Fatalf("seller delivery timeout mismatch: %s", cfg.SellerDeliveryTimeout.String())
	}
	if len(cfg.SupportedValidators) != 2 {
		t.Fatalf("supported validators mismatch: %#v", cfg.SupportedValidators)
	}
	if cfg.ServiceID != "contract-review" {
		t.Fatalf("service id mismatch: %s", cfg.ServiceID)
	}
	if cfg.ServiceName != "Contract Review" {
		t.Fatalf("service name mismatch: %s", cfg.ServiceName)
	}
	if cfg.ServiceDescription != "Review contract text and return risk notes." {
		t.Fatalf("service description mismatch: %s", cfg.ServiceDescription)
	}
	if cfg.LLMTimeout != 2*time.Second {
		t.Fatalf("llm timeout mismatch: %s", cfg.LLMTimeout)
	}
	if cfg.PollInterval != 100*time.Millisecond {
		t.Fatalf("poll interval mismatch: %s", cfg.PollInterval)
	}
	if cfg.LogPath != "./seller-service.log" {
		t.Fatalf("log path mismatch: %s", cfg.LogPath)
	}
}

func TestLoadCustomLogPath(t *testing.T) {
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("SELLER_RPC_URL", "http://127.0.0.1:8545")
	t.Setenv("SELLER_MARKET_ADDRESS", "0x1111111111111111111111111111111111111111")
	t.Setenv("SELLER_PRIVATE_KEY", "0x"+hex.EncodeToString(gethcrypto.FromECDSA(key)))
	t.Setenv("SELLER_URI", "https://seller.example/agent-nexus")
	t.Setenv("SELLER_PRICE_WEI", "100")
	t.Setenv("SELLER_CONTENT_URI", "ipfs://seller/product")
	t.Setenv("SELLER_CONTENT_HASH", "0x0100000000000000000000000000000000000000000000000000000000000000")
	t.Setenv("SELLER_DELIVERY_TIMEOUT", "3600")
	t.Setenv("SELLER_SUPPORTED_VALIDATORS", "0x3333333333333333333333333333333333333333")
	t.Setenv("SELLER_SERVICE_ID", "contract-review")
	t.Setenv("SELLER_SERVICE_NAME", "Contract Review")
	t.Setenv("SELLER_SERVICE_DESCRIPTION", "Review contract text and return risk notes.")
	t.Setenv("SELLER_LLM_SCRIPT", "/tmp/seller-llm.sh")
	t.Setenv("SELLER_LLM_API_KEY", "test-api-key")
	t.Setenv("SELLER_LOG_PATH", "/tmp/custom-seller-service.log")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LogPath != "/tmp/custom-seller-service.log" {
		t.Fatalf("log path mismatch: %s", cfg.LogPath)
	}
}

func TestLoadRejectsInvalidSellerChainConfig(t *testing.T) {
	key, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("SELLER_RPC_URL", "http://127.0.0.1:8545")
	t.Setenv("SELLER_MARKET_ADDRESS", "0x1111111111111111111111111111111111111111")
	t.Setenv("SELLER_PRIVATE_KEY", "0x"+hex.EncodeToString(gethcrypto.FromECDSA(key)))
	t.Setenv("SELLER_URI", "https://seller.example/agent-nexus")
	t.Setenv("SELLER_PRICE_WEI", "100")
	t.Setenv("SELLER_CONTENT_URI", "ipfs://seller/product")
	t.Setenv("SELLER_CONTENT_HASH", "0x0000000000000000000000000000000000000000000000000000000000000000")
	t.Setenv("SELLER_DELIVERY_TIMEOUT", "0")
	t.Setenv("SELLER_SUPPORTED_VALIDATORS", "not-an-address")
	t.Setenv("SELLER_SERVICE_ID", "contract-review")
	t.Setenv("SELLER_SERVICE_NAME", "Contract Review")
	t.Setenv("SELLER_SERVICE_DESCRIPTION", "Review contract text and return risk notes.")
	t.Setenv("SELLER_LLM_SCRIPT", "/tmp/seller-llm.sh")
	t.Setenv("SELLER_LLM_API_KEY", "test-api-key")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid seller chain config error")
	}
}
