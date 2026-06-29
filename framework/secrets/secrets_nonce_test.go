package secrets_test

import (
	"strings"
	"testing"

	"github.com/onipixel/oniworks/framework/secrets"
)

// TestEncryptUsesFreshNonce is the IV-reuse regression: encrypting the same
// plaintext twice must yield different ciphertexts (a random GCM nonce per
// call). Identical ciphertexts would mean a fixed/reused nonce — a critical
// AES-GCM weakness.
func TestEncryptUsesFreshNonce(t *testing.T) {
	key, err := secrets.ParseKey(mustGenKey(t))
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		ct, err := secrets.Encrypt(key, []byte("same plaintext"))
		if err != nil {
			t.Fatal(err)
		}
		if seen[ct] {
			t.Fatalf("duplicate ciphertext — nonce was reused: %s", ct)
		}
		seen[ct] = true
	}
}

// TestDecryptRejectsTamperedCiphertext verifies GCM authentication: flipping a
// byte of the ciphertext makes decryption fail rather than returning garbage.
func TestDecryptRejectsTamperedCiphertext(t *testing.T) {
	key, _ := secrets.ParseKey(mustGenKey(t))
	ct, err := secrets.Encrypt(key, []byte("authentic data"))
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt a character in the middle of the encoded ciphertext.
	idx := len(ct) / 2
	tampered := ct[:idx] + flip(ct[idx]) + ct[idx+1:]
	if _, err := secrets.Decrypt(key, tampered); err == nil {
		t.Fatal("tampered ciphertext must fail authentication, not decrypt")
	}
}

func mustGenKey(t *testing.T) string {
	t.Helper()
	k, err := secrets.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func flip(b byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	// Return a different character so the ciphertext actually changes.
	if strings.IndexByte(alphabet, b) == 0 {
		return "B"
	}
	return "A"
}
