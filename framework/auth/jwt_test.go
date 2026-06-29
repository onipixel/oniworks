package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type testUser struct {
	id    int64
	email string
}

func (u testUser) GetID() int64       { return u.id }
func (u testUser) GetEmail() string   { return u.email }
func (u testUser) GetPassword() string { return "" }

const goodSecret = "a-sufficiently-long-test-secret-key!!" // > 32 bytes

// TestJWTRejectsEmptySecret is the core forgery regression: with an empty or
// short secret, JWT operations must fail closed rather than accept/forge tokens.
func TestJWTRejectsEmptySecret(t *testing.T) {
	for _, secret := range []string{"", "short", "still-too-short-under-32-bytes"} {
		g := NewGuard(nil, nil, secret)
		if _, err := g.IssueToken(testUser{id: 1}, time.Hour); !errors.Is(err, ErrJWTNotConfigured) {
			t.Fatalf("secret %q: IssueToken err = %v, want ErrJWTNotConfigured", secret, err)
		}
		if _, err := g.ParseToken("any.token.here"); !errors.Is(err, ErrJWTNotConfigured) {
			t.Fatalf("secret %q: ParseToken err = %v, want ErrJWTNotConfigured", secret, err)
		}
	}
}

// TestJWTRoundTrip verifies a properly configured guard issues and parses tokens.
func TestJWTRoundTrip(t *testing.T) {
	g := NewGuard(nil, nil, goodSecret)
	tok, err := g.IssueToken(testUser{id: 42, email: "a@b.com"}, time.Hour)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	claims, err := g.ParseToken(tok)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != 42 || claims.Email != "a@b.com" {
		t.Fatalf("claims mismatch: %+v", claims)
	}
}

// TestJWTRejectsExpired verifies expired tokens are rejected.
func TestJWTRejectsExpired(t *testing.T) {
	g := NewGuard(nil, nil, goodSecret)
	tok, err := g.IssueToken(testUser{id: 1}, -time.Hour) // already expired
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if _, err := g.ParseToken(tok); err == nil {
		t.Fatal("expected expired token to be rejected")
	}
}

// TestJWTRequiresExpiry verifies a token with NO exp claim is rejected (it must
// not be valid forever).
func TestJWTRequiresExpiry(t *testing.T) {
	g := NewGuard(nil, nil, goodSecret)
	// Hand-craft a token with no expiry, signed with the correct secret.
	claims := Claims{UserID: 1, Email: "x@y.com"} // no ExpiresAt
	raw := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := raw.SignedString([]byte(goodSecret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := g.ParseToken(signed); err == nil {
		t.Fatal("expected token without exp to be rejected (WithExpirationRequired)")
	}
}

// TestJWTRejectsNoneAlg verifies algorithm-confusion / alg:none tokens fail.
func TestJWTRejectsNoneAlg(t *testing.T) {
	g := NewGuard(nil, nil, goodSecret)
	claims := Claims{UserID: 1, RegisteredClaims: jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}}
	raw := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	signed, err := raw.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none: %v", err)
	}
	if _, err := g.ParseToken(signed); err == nil {
		t.Fatal("expected alg:none token to be rejected")
	}
}
