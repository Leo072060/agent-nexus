package crypto

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func TestDeliveryRequestMessage(t *testing.T) {
	message := DeliveryRequestMessage(common.HexToAddress("0x1234567890123456789012345678901234567890"), big.NewInt(12))
	expected := "Agent Nexus delivery request\nmarketAddress: 0x1234567890123456789012345678901234567890\norderId: 12"
	if message != expected {
		t.Fatalf("message mismatch\nexpected: %q\nactual:   %q", expected, message)
	}
}

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

func TestSignPersonalMessage(t *testing.T) {
	privateKey, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	signature, err := SignPersonalMessage(privateKey, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(common.FromHex(signature)) != 65 {
		t.Fatalf("signature length mismatch: %d", len(common.FromHex(signature)))
	}
}

func TestKeccak256Hex(t *testing.T) {
	got := Keccak256Hex([]byte("hello"))
	want := "0x1c8aff950685c2ed4bc3174f3472287b56d9517b9c948127319a09a7a36deac8"
	if got != want {
		t.Fatalf("hash mismatch: want %s got %s", want, got)
	}
}
