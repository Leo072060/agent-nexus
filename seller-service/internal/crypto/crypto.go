package crypto

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/sha3"
)

func DeliveryRequestMessage(marketAddress common.Address, orderID *big.Int) string {
	return fmt.Sprintf(
		"Agent Nexus delivery request\nmarketAddress: %s\norderId: %s",
		marketAddress.Hex(),
		orderID.String(),
	)
}

func RecoverSigner(message string, signatureHex string) (common.Address, error) {
	signature := common.FromHex(signatureHex)
	if len(signature) != 65 {
		return common.Address{}, errors.New("signature must be 65 bytes")
	}

	if signature[64] >= 27 {
		signature[64] -= 27
	}
	if signature[64] > 1 {
		return common.Address{}, errors.New("invalid signature recovery id")
	}

	hash := gethcrypto.Keccak256Hash([]byte("\x19Ethereum Signed Message:\n" + strconv.Itoa(len(message)) + message))
	pubkey, err := gethcrypto.SigToPub(hash.Bytes(), signature)
	if err != nil {
		return common.Address{}, fmt.Errorf("recover signer: %w", err)
	}

	return gethcrypto.PubkeyToAddress(*pubkey), nil
}

func Keccak256Hex(data []byte) string {
	hash := Keccak256(data)
	return "0x" + common.Bytes2Hex(hash[:])
}

func Keccak256(data []byte) [32]byte {
	hasher := sha3.NewLegacyKeccak256()
	_, _ = hasher.Write(data)
	sum := hasher.Sum(nil)

	var out [32]byte
	copy(out[:], sum)
	return out
}

func EqualHash(hash [32]byte, hexHash string) bool {
	return strings.EqualFold("0x"+common.Bytes2Hex(hash[:]), hexHash)
}
