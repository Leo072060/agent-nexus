package config

import (
	"strings"
	"testing"
)

func TestLoadMissingRequiredEnv(t *testing.T) {
	t.Setenv("SELLER_RPC_URL", "")
	t.Setenv("SELLER_MARKET_ADDRESS", "")
	t.Setenv("SELLER_PRIVATE_KEY", "")
	t.Setenv("SELLER_BASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}

	message := err.Error()
	for _, name := range []string{
		"SELLER_RPC_URL",
		"SELLER_MARKET_ADDRESS",
		"SELLER_PRIVATE_KEY",
		"SELLER_BASE_URL",
	} {
		if !strings.Contains(message, name) {
			t.Fatalf("expected missing env %s in %q", name, message)
		}
	}
}
