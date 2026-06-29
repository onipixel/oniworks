// Package auth provides session-based and JWT authentication for OniWorks.
// Session auth is the default. JWT is opt-in for API-only applications.
package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/onipixel/oniworks/framework/session"
	"golang.org/x/crypto/bcrypt"
)

// User is the interface your application's User model must implement for auth to work.
type User interface {
	GetID() int64
	GetEmail() string
	GetPassword() string // bcrypt hash
}

// UserProvider retrieves users from a data source.
type UserProvider interface {
	FindByID(ctx context.Context, id int64) (User, error)
	FindByEmail(ctx context.Context, email string) (User, error)
}

// Guard manages authentication state for the current request.
type Guard struct {
	provider  UserProvider
	sessions  *session.Manager
	jwtSecret []byte
}

// NewGuard creates a Guard.
func NewGuard(provider UserProvider, sessions *session.Manager, jwtSecret string) *Guard {
	return &Guard{
		provider:  provider,
		sessions:  sessions,
		jwtSecret: []byte(jwtSecret),
	}
}

// ─────────────────────────── Session Auth ──────────────────────────

// Attempt verifies credentials and creates an authenticated session on success.
//
// On success the session ID is rotated (Regenerate) to defeat session fixation.
// When the email is unknown a dummy bcrypt comparison is still performed so the
// not-found path takes the same time as a wrong-password path, preventing
// account enumeration via response timing.
func (g *Guard) Attempt(ctx context.Context, email, password string, sess *session.Session) (User, error) {
	user, err := g.provider.FindByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	if user == nil {
		_ = bcrypt.CompareHashAndPassword(dummyBcryptHash, []byte(password))
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.GetPassword()), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	if sess != nil {
		// Rotate the session ID now that the principal is changing (login).
		if err := sess.Regenerate(ctx); err != nil {
			return nil, fmt.Errorf("auth: %w", err)
		}
		sess.Set("_auth_user_id", user.GetID())
	}
	return user, nil
}

// UserFromSession retrieves the authenticated user from the session.
// Returns nil (and no error) if the session has no authenticated user.
func (g *Guard) UserFromSession(ctx context.Context, sess *session.Session) (User, error) {
	idVal, ok := sess.Get("_auth_user_id")
	if !ok {
		return nil, nil
	}
	var id int64
	switch v := idVal.(type) {
	case int64:
		id = v
	case int:
		id = int64(v)
	case float64:
		id = int64(v)
	default:
		return nil, nil
	}
	return g.provider.FindByID(ctx, id)
}

// Check reports whether the session has an authenticated user.
func (g *Guard) Check(ctx context.Context, sess *session.Session) bool {
	user, _ := g.UserFromSession(ctx, sess)
	return user != nil
}

// Logout removes the user from the session.
func (g *Guard) Logout(sess *session.Session) {
	sess.Delete("_auth_user_id")
}

// ─────────────────────────── JWT Auth ──────────────────────────────

// Claims is the JWT payload.
type Claims struct {
	UserID int64  `json:"uid"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// IssueToken creates a signed JWT for the given user.
func (g *Guard) IssueToken(user User, ttl time.Duration) (string, error) {
	if len(g.jwtSecret) < minJWTSecretLen {
		return "", ErrJWTNotConfigured
	}
	claims := Claims{
		UserID: user.GetID(),
		Email:  user.GetEmail(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(g.jwtSecret)
}

// ParseToken validates a JWT and returns the embedded Claims.
//
// It fails closed if the signing secret is unconfigured/too short, restricts
// the accepted algorithm to HS256 (blocking algorithm-confusion and "alg:none"
// attacks), and requires the token to carry an expiry so non-expiring tokens
// are never accepted.
func (g *Guard) ParseToken(tokenStr string) (*Claims, error) {
	if len(g.jwtSecret) < minJWTSecretLen {
		return nil, ErrJWTNotConfigured
	}
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("auth: unexpected signing method %v", t.Header["alg"])
		}
		return g.jwtSecret, nil
	},
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// UserFromToken resolves the user from a parsed JWT token string.
func (g *Guard) UserFromToken(ctx context.Context, tokenStr string) (User, error) {
	claims, err := g.ParseToken(tokenStr)
	if err != nil {
		return nil, err
	}
	return g.provider.FindByID(ctx, claims.UserID)
}

// ─────────────────────────── Password helpers ──────────────────────

// HashPassword bcrypt-hashes a plaintext password.
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

// CheckPassword verifies a plaintext password against a bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// ─────────────────────────── Errors ────────────────────────────────

var (
	ErrInvalidCredentials = errors.New("auth: invalid email or password")
	ErrInvalidToken       = errors.New("auth: invalid or expired token")
	ErrUnauthenticated    = errors.New("auth: unauthenticated")
	// ErrJWTNotConfigured is returned when a JWT operation is attempted but no
	// (or too short a) signing secret was provided to NewGuard. An empty secret
	// would let anyone forge tokens, so JWT operations fail closed.
	ErrJWTNotConfigured = errors.New("auth: JWT secret not configured (must be at least 32 bytes)")
)

// minJWTSecretLen is the minimum acceptable HS256 signing-key length in bytes.
const minJWTSecretLen = 32

// dummyBcryptHash is compared against the supplied password when the account is
// not found, so that the not-found path costs the same as a real bcrypt verify
// (constant-time account-enumeration defense).
var dummyBcryptHash, _ = bcrypt.GenerateFromPassword([]byte("oniworks-timing-equalizer"), bcrypt.DefaultCost)
