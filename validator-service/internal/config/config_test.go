package config

import (
	"strings"
	"testing"
	"time"
)

const testValidatorKey = "0x7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6"

func TestLoadRequiresScriptConfig(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("VALIDATOR_LLM_SCRIPT", "")
	t.Setenv("VALIDATOR_LLM_API_KEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected missing env error")
	}
	if !strings.Contains(err.Error(), "VALIDATOR_LLM_SCRIPT") {
		t.Fatalf("expected missing script error, got %v", err)
	}
	if !strings.Contains(err.Error(), "VALIDATOR_LLM_API_KEY") {
		t.Fatalf("expected missing api key error, got %v", err)
	}
}

func TestLoadDefaultsAndParsesValidatorLLMConfig(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLMScript != "/tmp/validator-llm.sh" {
		t.Fatalf("llm script mismatch: %s", cfg.LLMScript)
	}
	if cfg.LLMAPIKey != "test-validator-key" {
		t.Fatalf("llm api key mismatch: %s", cfg.LLMAPIKey)
	}
	if cfg.LLMTimeout != 60*time.Second {
		t.Fatalf("llm timeout mismatch: %s", cfg.LLMTimeout)
	}
}

func TestLoadParsesValidatorLLMTimeout(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("VALIDATOR_LLM_TIMEOUT", "2s")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLMTimeout != 2*time.Second {
		t.Fatalf("llm timeout mismatch: %s", cfg.LLMTimeout)
	}
}

func TestLoadRejectsInvalidValidatorLLMTimeout(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("VALIDATOR_LLM_TIMEOUT", "not-a-duration")

	if _, err := Load(); err == nil {
		t.Fatal("expected invalid timeout error")
	}
}

func TestLoadRejectsNonPositiveValidatorLLMTimeout(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("VALIDATOR_LLM_TIMEOUT", "0s")

	if _, err := Load(); err == nil {
		t.Fatal("expected non-positive timeout error")
	}
}

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("VALIDATOR_RPC_URL", "http://localhost:8545")
	t.Setenv("VALIDATOR_MARKET_ADDRESS", "0x1111111111111111111111111111111111111111")
	t.Setenv("VALIDATOR_PRIVATE_KEY", testValidatorKey)
	t.Setenv("VALIDATOR_BASE_URL", "http://localhost:8082")
	t.Setenv("VALIDATOR_LLM_SCRIPT", "/tmp/validator-llm.sh")
	t.Setenv("VALIDATOR_LLM_API_KEY", "test-validator-key")
	t.Setenv("VALIDATOR_DB_PATH", "")
	t.Setenv("VALIDATOR_HTTP_ADDR", "")
	t.Setenv("VALIDATOR_LLM_TIMEOUT", "")
}
