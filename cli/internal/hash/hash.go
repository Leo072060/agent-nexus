package hash

import (
	"encoding/hex"

	"golang.org/x/crypto/sha3"
)

func Keccak256(data []byte) []byte {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(data)
	return hasher.Sum(nil)
}

func Keccak256Hex(data []byte) string {
	return "0x" + hex.EncodeToString(Keccak256(data))
}
