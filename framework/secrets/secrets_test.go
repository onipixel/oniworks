package secrets_test

import (
	"strings"
	"testing"

	"github.com/onipixel/oniworks/framework/secrets"
)

func TestGenerateKey(t *testing.T) {
	key, err := secrets.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if !strings.HasPrefix(key, "base64:") {
		t.Errorf("key should start with 'base64:', got %q", key)
	}
	// ParseKey should decode it without error
	b, err := secrets.ParseKey(key)
	if err != nil {
		t.Fatalf("ParseKey: %v", err)
	}
	if len(b) != 32 {
		t.Errorf("key should be 32 bytes, got %d", len(b))
	}
}

func TestParseKeyRawString(t *testing.T) {
	b, err := secrets.ParseKey("my-secret-app-key")
	if err != nil {
		t.Fatalf("ParseKey raw: %v", err)
	}
	if len(b) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(b))
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key, _ := secrets.ParseKey("test-key-for-oniworks")

	plaintext := "super secret database password 🔐"
	ct, err := secrets.Encrypt(key, []byte(plaintext))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if ct == "" {
		t.Fatal("ciphertext is empty")
	}
	if ct == plaintext {
		t.Fatal("ciphertext equals plaintext")
	}

	pt, err := secrets.Decrypt(key, ct)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(pt) != plaintext {
		t.Errorf("decrypted: got %q, want %q", string(pt), plaintext)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1, _ := secrets.ParseKey("key-one")
	key2, _ := secrets.ParseKey("key-two")

	ct, _ := secrets.Encrypt(key1, []byte("secret"))
	_, err := secrets.Decrypt(key2, ct)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

func TestManagerSetGet(t *testing.T) {
	m, err := secrets.New("test-app-key")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	m.Set("db.password", "s3cr3t")
	got, ok := m.Get("db.password")
	if !ok {
		t.Fatal("Get: key not found")
	}
	if got != "s3cr3t" {
		t.Errorf("Get: got %q, want %q", got, "s3cr3t")
	}
}

func TestManagerMust(t *testing.T) {
	m, _ := secrets.New("test-app-key")
	m.Set("api.key", "abc123")

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic: %v", r)
		}
	}()
	val := m.Must("api.key")
	if val != "abc123" {
		t.Errorf("Must: got %q", val)
	}
}

func TestManagerMustPanicsOnMissing(t *testing.T) {
	m, _ := secrets.New("test-app-key")
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing secret")
		}
	}()
	_ = m.Must("nonexistent.secret")
}

func TestManagerEncryptDecrypt(t *testing.T) {
	m, _ := secrets.New("my-app-key-12345")
	ct, err := m.Encrypt("production password")
	if err != nil {
		t.Fatalf("Manager.Encrypt: %v", err)
	}
	pt, err := m.Decrypt(ct)
	if err != nil {
		t.Fatalf("Manager.Decrypt: %v", err)
	}
	if pt != "production password" {
		t.Errorf("got %q", pt)
	}
}
