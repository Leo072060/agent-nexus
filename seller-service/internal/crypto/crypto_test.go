package crypto

import (
	"math/big"
	"strconv"
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

func TestRecoverSigner(t *testing.T) {
	privateKey, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	want := gethcrypto.PubkeyToAddress(privateKey.PublicKey)
	message := DeliveryRequestMessage(common.HexToAddress("0x1234567890123456789012345678901234567890"), big.NewInt(12))
	hash := gethcrypto.Keccak256Hash([]byte("\x19Ethereum Signed Message:\n" + strconv.Itoa(len(message)) + message))

	signature, err := gethcrypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		t.Fatal(err)
	}
	signature[64] += 27

	got, err := RecoverSigner(message, "0x"+common.Bytes2Hex(signature))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("signer mismatch: want %s got %s", want.Hex(), got.Hex())
	}
}

func TestKeccak256Hex(t *testing.T) {
	got := Keccak256Hex([]byte("hello"))
	want := "0x1c8aff950685c2ed4bc3174f3472287b56d9517b9c948127319a09a7a36deac8"
	if got != want {
		t.Fatalf("hash mismatch: want %s got %s", want, got)
	}
}
