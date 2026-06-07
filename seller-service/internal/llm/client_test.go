package llm

import (
	"context"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func TestGeneratePassesRequestTextToScriptAndReturnsAnswer(t *testing.T) {
	script := writeScript(t, `#!/bin/sh
input=$(cat)
if [ "$SELLER_LLM_API_KEY" != "secret-test-key" ]; then
  echo "unexpected api key" >&2
  exit 3
fi
if [ "$input" != "review this contract" ]; then
  echo "unexpected stdin: $input" >&2
  exit 2
fi
	printf '{"answer":"approved","evidence":"request matched delivery"}\n'
`)

	client := NewClient(script, "secret-test-key", 5*time.Second)
	result, err := client.Generate(
		context.Background(),
		common.HexToAddress("0x1111111111111111111111111111111111111111"),
		big.NewInt(12),
		[]byte("review this contract"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Answer) != "approved" {
		t.Fatalf("answer mismatch: %s", string(result.Answer))
	}
	if string(result.Evidence) != "request matched delivery" {
		t.Fatalf("evidence mismatch: %s", string(result.Evidence))
	}
}

func TestGenerateRejectsEmptyAnswer(t *testing.T) {
	script := writeScript(t, `#!/bin/sh
cat >/dev/null
printf '{"answer":"   ","evidence":"evidence"}\n'
`)

	client := NewClient(script, "secret-test-key", time.Second)
	if _, err := client.Generate(context.Background(), common.Address{}, big.NewInt(12), []byte("request")); err == nil {
		t.Fatal("expected empty answer error")
	}
}

func TestGenerateRejectsEmptyEvidence(t *testing.T) {
	script := writeScript(t, `#!/bin/sh
cat >/dev/null
printf '{"answer":"answer","evidence":"   "}\n'
`)

	client := NewClient(script, "secret-test-key", time.Second)
	if _, err := client.Generate(context.Background(), common.Address{}, big.NewInt(12), []byte("request")); err == nil {
		t.Fatal("expected empty evidence error")
	}
}

func TestGenerateRejectsInvalidJSON(t *testing.T) {
	script := writeScript(t, `#!/bin/sh
cat >/dev/null
printf 'plain answer\n'
`)

	client := NewClient(script, "secret-test-key", time.Second)
	if _, err := client.Generate(context.Background(), common.Address{}, big.NewInt(12), []byte("request")); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestGenerateRejectsNonZeroExit(t *testing.T) {
	script := writeScript(t, `#!/bin/sh
cat >/dev/null
echo "llm failed" >&2
exit 7
`)

	client := NewClient(script, "secret-test-key", time.Second)
	if _, err := client.Generate(context.Background(), common.Address{}, big.NewInt(12), []byte("request")); err == nil {
		t.Fatal("expected script failure error")
	}
}

func TestGenerateRejectsTimeout(t *testing.T) {
	script := writeScript(t, `#!/bin/sh
sleep 2
printf "late"
`)

	client := NewClient(script, "secret-test-key", 10*time.Millisecond)
	if _, err := client.Generate(context.Background(), common.Address{}, big.NewInt(12), []byte("request")); err == nil {
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
