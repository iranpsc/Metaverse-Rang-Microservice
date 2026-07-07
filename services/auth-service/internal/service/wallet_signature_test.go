package service

import (
	"encoding/hex"
	"fmt"
	"testing"

	secp256k1 "github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

func TestIsValidWalletSignatureAcceptsPersonalSign(t *testing.T) {
	address, signature, message := generateTestWalletSignature(t)
	if !IsValidWalletSignature(address, signature, message) {
		t.Fatalf("expected signature to verify for derived address")
	}
}

func TestIsValidWalletSignatureRejectsWrongAddress(t *testing.T) {
	_, signature, message := generateTestWalletSignature(t)
	if IsValidWalletSignature("0x0000000000000000000000000000000000000001", signature, message) {
		t.Fatalf("expected signature verification to fail for wrong address")
	}
}

func generateTestWalletSignature(t *testing.T) (string, string, string) {
	t.Helper()

	privHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	privBytes, err := hex.DecodeString(privHex)
	if err != nil {
		t.Fatalf("decode priv: %v", err)
	}

	privKey := secp256k1.PrivKeyFromBytes(privBytes)
	address := pubkeyToAddress(privKey.PubKey())
	message := fmt.Sprintf(
		"Link wallet to your Metarang account at localhost.\n\nAccount ID: 42\nWallet: %s\nNonce: abcdefghijklmnopqrstuvwxyz123456",
		address,
	)

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

	signature := "0x" + hex.EncodeToString(ethSig)
	return address, signature, message
}
