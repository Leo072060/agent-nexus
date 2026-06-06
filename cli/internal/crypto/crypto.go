package crypto

import (
	"crypto/ecdsa"
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

func DisputeEvidenceMessage(marketAddress common.Address, orderID *big.Int, requestHash string, deliveryHash string, disputeHash string) string {
	return fmt.Sprintf(
		"Agent Nexus dispute evidence\nmarketAddress: %s\norderId: %s\nrequestHash: %s\ndeliveryHash: %s\ndisputeHash: %s",
		marketAddress.Hex(),
		orderID.String(),
		strings.ToLower(requestHash),
		strings.ToLower(deliveryHash),
		strings.ToLower(disputeHash),
	)
}

func SignPersonalMessage(privateKey *ecdsa.PrivateKey, message string) (string, error) {
	hash := gethcrypto.Keccak256Hash([]byte("\x19Ethereum Signed Message:\n" + strconv.Itoa(len(message)) + message))
	signature, err := gethcrypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		return "", fmt.Errorf("sign delivery request: %w", err)
	}

	signature[64] += 27
	return "0x" + common.Bytes2Hex(signature), nil
}

func Keccak256Hex(data []byte) string {
	hasher := sha3.NewLegacyKeccak256()
	_, _ = hasher.Write(data)
	sum := hasher.Sum(nil)
	return "0x" + common.Bytes2Hex(sum)
}
