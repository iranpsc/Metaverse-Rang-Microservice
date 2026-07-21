package wallet_test

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"testing"

	secp256k1 "github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"golang.org/x/crypto/sha3"

	"metarang/auth-service/internal/service"
)

const testWalletPrivHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestIsValidWalletSignatureAcceptsPersonalSign(t *testing.T) {
	address, signature, message := generateTestWalletSignature(t)
	if !service.IsValidWalletSignature(address, signature, message) {
		t.Fatalf("expected signature to verify for derived address")
	}
}

func TestIsValidWalletSignatureRejectsWrongAddress(t *testing.T) {
	_, signature, message := generateTestWalletSignature(t)
	if service.IsValidWalletSignature("0x0000000000000000000000000000000000000001", signature, message) {
		t.Fatalf("expected signature verification to fail for wrong address")
	}
}

func TestIsValidWalletSignatureRejectsMalformedSignature(t *testing.T) {
	address, _, message := generateTestWalletSignature(t)
	if service.IsValidWalletSignature(address, "0x1234", message) {
		t.Fatalf("expected short signature to be rejected")
	}
}

func TestIsValidWalletSignatureRejectsInvalidHex(t *testing.T) {
	address, _, message := generateTestWalletSignature(t)
	invalidSig := "0x" + strings.Repeat("g", 130)
	if service.IsValidWalletSignature(address, invalidSig, message) {
		t.Fatalf("expected invalid hex signature to be rejected")
	}
}

func TestIsValidWalletSignatureRejectsTamperedMessage(t *testing.T) {
	address, signature, message := generateTestWalletSignature(t)
	if service.IsValidWalletSignature(address, signature, message+"tampered") {
		t.Fatalf("expected tampered message to fail verification")
	}
}

func TestIsValidWalletSignatureRejectsHighS(t *testing.T) {
	address, signature, message := generateTestWalletSignature(t)
	malleated := malleateSignatureToHighS(t, signature)

	if service.IsValidWalletSignature(address, malleated, message) {
		t.Fatalf("expected high-s signature to be rejected")
	}
}

func malleateSignatureToHighS(t *testing.T, signature string) string {
	t.Helper()

	sigBytes, err := hex.DecodeString(signature[2:])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}

	curveN, _ := new(big.Int).SetString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141", 16)
	halfN, _ := new(big.Int).SetString("7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF5D576E7357A4501DDFE92F46681B20A0", 16)
	s := new(big.Int).SetBytes(sigBytes[32:64])
	if s.Cmp(halfN) <= 0 {
		s.Sub(curveN, s)
	}
	if s.Cmp(halfN) <= 0 {
		t.Fatalf("expected malleated s to exceed half order")
	}
	copy(sigBytes[32:64], padTo32(s.Bytes()))

	return "0x" + hex.EncodeToString(sigBytes)
}

func padTo32(b []byte) []byte {
	if len(b) >= 32 {
		return b[len(b)-32:]
	}
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}

func testWalletPrivateKey(t *testing.T) *secp256k1.PrivateKey {
	t.Helper()

	privBytes, err := hex.DecodeString(testWalletPrivHex)
	if err != nil {
		t.Fatalf("decode priv: %v", err)
	}

	return secp256k1.PrivKeyFromBytes(privBytes)
}

func testWalletAddress(t *testing.T) string {
	t.Helper()
	return pubkeyToAddress(testWalletPrivateKey(t).PubKey())
}

func signWalletMessage(t *testing.T, message string) string {
	t.Helper()

	privKey := testWalletPrivateKey(t)
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	hash := keccak256([]byte(prefix))

	compactSig := ecdsa.SignCompact(privKey, hash, false)
	if len(compactSig) != 65 {
		t.Fatalf("expected compact signature length 65, got %d", len(compactSig))
	}

	ethSig := make([]byte, 65)
	copy(ethSig[:32], compactSig[1:33])
	copy(ethSig[32:64], compactSig[33:65])
	ethSig[64] = compactSig[0]

	return "0x" + hex.EncodeToString(ethSig)
}

func generateTestWalletSignature(t *testing.T) (string, string, string) {
	t.Helper()

	address := testWalletAddress(t)
	message := fmt.Sprintf(
		"Link wallet to your Metarang account at localhost.\n\nAccount ID: 42\nWallet: %s\nNonce: abcdefghijklmnopqrstuvwxyz123456",
		address,
	)
	signature := signWalletMessage(t, message)
	return address, signature, message
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
