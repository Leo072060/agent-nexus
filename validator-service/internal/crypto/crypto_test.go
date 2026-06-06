package crypto

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestDisputeEvidenceMessage(t *testing.T) {
	message := DisputeEvidenceMessage(
		common.HexToAddress("0x1234567890123456789012345678901234567890"),
		big.NewInt(12),
		"0xAAA",
		"0xBBB",
		"0xCCC",
	)
	expected := "Agent Nexus dispute evidence\nmarketAddress: 0x1234567890123456789012345678901234567890\norderId: 12\nrequestHash: 0xaaa\ndeliveryHash: 0xbbb\ndisputeHash: 0xccc"
	if message != expected {
		t.Fatalf("message mismatch\nexpected: %q\nactual:   %q", expected, message)
	}
}
