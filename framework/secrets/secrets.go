// Package secrets provides AES-256-GCM encryption for application secrets.
// Secrets are encrypted at rest and loaded from environment variables or
// an encrypted .secrets file.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"

	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/scrypt"
	"io"
	"os"
	"strings"
	"sync"
)

// Manager holds and provides access to application secrets.
// Secrets can be encrypted at rest and decrypted on demand using the app key.
type Manager struct {
	mu      sync.RWMutex
	appKey  []byte // 32-byte derived key for AES-256-GCM
	secrets map[string]string
}

// New creates a Manager. appKey is the raw application secret (e.g. APP_KEY env var).
// If appKey is empty, a random key is used (useful for testing).
func New(appKey string) (*Manager, error) {
	var key []byte
	if appKey == "" {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
	} else {
		// Honor base64:/hex/passphrase forms via ParseKey (previously this path
		// SHA-256'd the whole string, ignoring the base64: prefix).
		k, err := ParseKey(appKey)
		if err != nil {
			return nil, err
		}
		key = k
	}
	return &Manager{
		appKey:  key,
		secrets: make(map[string]string),
	}, nil
}

// Set stores a secret value under name (plain-text stored in memory only).
func (m *Manager) Set(name, value string) {
	m.mu.Lock()
	m.secrets[name] = value
	m.mu.Unlock()
}

// Get retrieves a secret by name.
func (m *Manager) Get(name string) (string, bool) {
	m.mu.RLock()
	v, ok := m.secrets[name]
	m.mu.RUnlock()
	return v, ok
}

// Must retrieves a secret or panics if not found.
func (m *Manager) Must(name string) string {
	v, ok := m.Get(name)
	if !ok {
		panic("secrets: missing secret " + name)
	}
	return v
}

// Env loads a secret from an environment variable, storing it under name.
func (m *Manager) Env(name, envVar string) {
	if v := os.Getenv(envVar); v != "" {
		m.Set(name, v)
	}
}

// ─────────────────────────── Encryption ──────────────────────────

// Encrypt encrypts plaintext using AES-256-GCM and returns a base64url-encoded ciphertext.
//
//	encrypted, err := manager.Encrypt("my-secret-password")
func (m *Manager) Encrypt(plaintext string) (string, error) {
	return Encrypt(m.appKey, []byte(plaintext))
}

// Decrypt decrypts a base64url-encoded ciphertext produced by Encrypt.
//
//	plain, err := manager.Decrypt(encrypted)
func (m *Manager) Decrypt(ciphertext string) (string, error) {
	b, err := Decrypt(m.appKey, ciphertext)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ─────────────────────────── Package-level crypto ─────────────────

// Encrypt encrypts plaintext using AES-256-GCM with the given 32-byte key.
// The returned string is URL-safe base64: nonce (12 bytes) || ciphertext || tag.
func Encrypt(key, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("secrets: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("secrets: gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("secrets: nonce: %w", err)
	}
	ct := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.URLEncoding.EncodeToString(ct), nil
}

// Decrypt decrypts a base64url-encoded ciphertext produced by Encrypt.
func Decrypt(key []byte, encoded string) ([]byte, error) {
	ct, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("secrets: base64 decode: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secrets: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secrets: gcm: %w", err)
	}
	ns := gcm.NonceSize()
	if len(ct) < ns {
		return nil, fmt.Errorf("secrets: ciphertext too short")
	}
	nonce, ct := ct[:ns], ct[ns:]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("secrets: decrypt: %w (wrong key?)", err)
	}
	return plain, nil
}

// GenerateKey generates a new cryptographically random 32-byte key
// and returns it as a hex string suitable for APP_KEY.
func GenerateKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Encode as base64 so it's easy to store in env vars
	return "base64:" + base64.StdEncoding.EncodeToString(b), nil
}

// ParseKey decodes a key produced by GenerateKey or a raw hex string.
func ParseKey(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "base64:") {
		b, err := base64.StdEncoding.DecodeString(s[7:])
		if err != nil {
			return nil, fmt.Errorf("secrets: parse key: %w", err)
		}
		if len(b) != 32 {
			return nil, fmt.Errorf("secrets: key must be 32 bytes, got %d", len(b))
		}
		return b, nil
	}
	// Treat as a passphrase and stretch it with a KDF (scrypt) rather than a
	// single SHA-256, so a weak APP_KEY is far costlier to brute-force.
	return deriveKey(s), nil
}

// scryptSalt is fixed because the derived key MUST be reproducible across
// restarts to decrypt previously-encrypted data; the work factor (not the salt)
// provides brute-force resistance. Prefer a base64: 32-byte random key in
// production (see GenerateKey) over a passphrase.
const scryptSalt = "oniworks/secrets/kdf/v1"

func deriveKey(passphrase string) []byte {
	k, err := scrypt.Key([]byte(passphrase), []byte(scryptSalt), 1<<15, 8, 1, 32)
	if err != nil {
		// Parameters are constant and valid; this path is unreachable in practice.
		h := sha256.Sum256([]byte(passphrase))
		return h[:]
	}
	return k
}
