package llm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func TestDecidePassesEvidenceJSONToScriptAndReturnsDecision(t *testing.T) {
	capturePath := filepath.Join(t.TempDir(), "stdin.json")
	script := writeScript(t, `#!/bin/sh
input=$(cat)
printf '%s' "$input" > "$CAPTURE_PATH"
if [ "$VALIDATOR_LLM_API_KEY" != "secret-test-key" ]; then
  echo "unexpected api key" >&2
  exit 3
fi
printf '{"releaseToSeller":true,"summary":"seller wins","reasoning":"delivery matches request","buyerClaim":"bad delivery","sellerDeliveryAssessment":"sufficient","confidence":"high"}\n'
`)

	t.Setenv("CAPTURE_PATH", capturePath)
	client, err := NewClient(script, "secret-test-key", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	decision, reportBody, err := client.Decide(context.Background(), Evidence{
		MarketAddress:    common.HexToAddress("0x1111111111111111111111111111111111111111"),
		OrderID:          "12",
		BuyerAddress:     "0x2222222222222222222222222222222222222222",
		SellerAddress:    "0x3333333333333333333333333333333333333333",
		ValidatorAddress: "0x4444444444444444444444444444444444444444",
		RequestHash:      "0xrequest",
		DeliveryHash:     "0xdelivery",
		Request:          "review contract",
		Delivery:         "approved",
		Dispute:          "not good enough",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !decision.ReleaseToSeller {
		t.Fatal("expected releaseToSeller=true")
	}
	if decision.Confidence != "high" {
		t.Fatalf("confidence mismatch: %s", decision.Confidence)
	}
	if !strings.Contains(string(reportBody), "\n  \"summary\": \"seller wins\"") {
		t.Fatalf("expected pretty report body, got %s", string(reportBody))
	}

	captured, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatal(err)
	}
	var evidence Evidence
	if err := json.Unmarshal(captured, &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.OrderID != "12" || evidence.Request != "review contract" || evidence.Dispute != "not good enough" {
		t.Fatalf("evidence mismatch: %+v", evidence)
	}
	if evidence.MarketAddress != common.HexToAddress("0x1111111111111111111111111111111111111111") {
		t.Fatalf("market address mismatch: %s", evidence.MarketAddress.Hex())
	}
}

func TestDecideDefaultsEmptyConfidence(t *testing.T) {
	script := writeScript(t, `#!/bin/sh
cat >/dev/null
printf '{"releaseToSeller":false,"summary":"buyer wins","reasoning":"delivery incomplete","buyerClaim":"missing work","sellerDeliveryAssessment":"insufficient","confidence":"   "}\n'
`)
	client, err := NewClient(script, "secret-test-key", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	decision, _, err := client.Decide(context.Background(), Evidence{OrderID: "12"})
	if err != nil {
		t.Fatal(err)
	}
	if decision.Confidence != "medium" {
		t.Fatalf("confidence mismatch: %s", decision.Confidence)
	}
}

func TestDecideRejectsMissingSummaryOrReasoning(t *testing.T) {
	script := writeScript(t, `#!/bin/sh
cat >/dev/null
printf '{"releaseToSeller":true,"summary":"   ","reasoning":"delivery matches request"}\n'
`)
	client, err := NewClient(script, "secret-test-key", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.Decide(context.Background(), Evidence{OrderID: "12"}); err == nil {
		t.Fatal("expected missing summary error")
	}
}

func TestDecideRejectsInvalidJSON(t *testing.T) {
	script := writeScript(t, `#!/bin/sh
cat >/dev/null
printf 'not json\n'
`)
	client, err := NewClient(script, "secret-test-key", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.Decide(context.Background(), Evidence{OrderID: "12"}); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestDecideRejectsNonZeroExit(t *testing.T) {
	script := writeScript(t, `#!/bin/sh
cat >/dev/null
echo "llm failed" >&2
exit 7
`)
	client, err := NewClient(script, "secret-test-key", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.Decide(context.Background(), Evidence{OrderID: "12"}); err == nil {
		t.Fatal("expected script failure error")
	}
}

func TestDecideRejectsTimeout(t *testing.T) {
	script := writeScript(t, `#!/bin/sh
sleep 2
printf '{"releaseToSeller":true,"summary":"late","reasoning":"late"}\n'
`)
	client, err := NewClient(script, "secret-test-key", 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.Decide(context.Background(), Evidence{OrderID: "12"}); err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestNewClientRequiresScriptAndAPIKey(t *testing.T) {
	if _, err := NewClient("", "secret-test-key", time.Second); err == nil {
		t.Fatal("expected missing script error")
	}
	if _, err := NewClient("/tmp/script.sh", "", time.Second); err == nil {
		t.Fatal("expected missing api key error")
	}
	if _, err := NewClient("/tmp/script.sh", "secret-test-key", 0); err == nil {
		t.Fatal("expected timeout error")
	}
}

func writeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "run.sh")
	if err := os.WriteFile(path, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}
