package service

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/sha3"
)

const secp256k1HalfNHex = "7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF5D576E7357A4501DDFE92F46681B20A0"

var secp256k1HalfN = mustParseHexBigInt(secp256k1HalfNHex)

func mustParseHexBigInt(hexStr string) *big.Int {
	n, ok := new(big.Int).SetString(hexStr, 16)
	if !ok {
		panic("invalid secp256k1 half N constant")
	}
	return n
}

// IsValidWalletSignature verifies an Ethereum personal_sign signature against address and message.
func IsValidWalletSignature(address, signature, message string) bool {
	address = strings.ToLower(strings.TrimSpace(address))
	signature = strings.TrimSpace(signature)

	if len(signature) != 132 || !strings.HasPrefix(signature, "0x") {
		return false
	}

	sigHex := signature[2:]
	if !isHexString(sigHex) {
		return false
	}

	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil || len(sigBytes) != 65 {
		return false
	}

	r := new(big.Int).SetBytes(sigBytes[:32])
	s := new(big.Int).SetBytes(sigBytes[32:64])
	if s.Cmp(secp256k1HalfN) > 0 {
		return false
	}

	v := int(sigBytes[64])
	if v < 27 {
		v += 27
	}
	recoveryParam := v - 27
	if recoveryParam != 0 && recoveryParam != 1 {
		return false
	}

	msgLength := len(message)
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", msgLength, message)
	hash := keccak256([]byte(prefix))

	// decred RecoverCompact expects Bitcoin compact format: [27+recid | R | S]
	compactSig := make([]byte, 65)
	compactSig[0] = byte(27 + recoveryParam)
	copy(compactSig[1:33], sigBytes[:32])
	copy(compactSig[33:65], sigBytes[32:64])

	pubKey, _, err := ecdsa.RecoverCompact(compactSig, hash)
	if err != nil {
		return false
	}

	derivedAddress := pubkeyToAddress(pubKey)
	return derivedAddress == address && r.Sign() > 0 && s.Sign() > 0
}

func keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}

func pubkeyToAddress(pubKey *secp256k1.PublicKey) string {
	uncompressed := pubKey.SerializeUncompressed()
	hash := keccak256(uncompressed[1:])
	return "0x" + hex.EncodeToString(hash[12:])
}

func isHexString(value string) bool {
	if len(value) == 0 || len(value)%2 != 0 {
		return false
	}
	for _, c := range value {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}
