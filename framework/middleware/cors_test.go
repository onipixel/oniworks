package middleware

import (
	"net/http/httptest"
	"testing"

	onihttp "github.com/onipixel/oniworks/framework/http"
)

func corsResult(t *testing.T, cfg CORSConfig, origin string) (*httptest.ResponseRecorder) {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Origin", origin)
	c := onihttp.NewContext(rr, req, nil)
	h := CORS(cfg)(func(c *onihttp.Context) error { return c.JSON(200, onihttp.Map{}) })
	if err := h(c); err != nil {
		t.Fatalf("handler: %v", err)
	}
	return rr
}

// TestCORSWildcardWithCredentialsNeverEmitsCredentials is the core regression:
// "*" + AllowCredentials must NOT send Access-Control-Allow-Credentials, since
// that would let any site read credentialed responses.
func TestCORSWildcardWithCredentialsNeverEmitsCredentials(t *testing.T) {
	rr := corsResult(t, CORSConfig{AllowOrigins: []string{"*"}, AllowCredentials: true}, "https://evil.example")
	if rr.Header().Get("Access-Control-Allow-Credentials") == "true" {
		t.Fatal("wildcard origin must never grant credentials")
	}
	// It may still reflect the origin (for non-credentialed use) but must Vary.
	if rr.Header().Get("Access-Control-Allow-Origin") == "https://evil.example" &&
		rr.Header().Get("Vary") == "" {
		t.Fatal("reflected origin must set Vary: Origin")
	}
}

// TestCORSExplicitOriginGetsCredentials verifies an explicitly allowed origin
// does receive credentials and a Vary header.
func TestCORSExplicitOriginGetsCredentials(t *testing.T) {
	rr := corsResult(t, CORSConfig{
		AllowOrigins:     []string{"https://app.example"},
		AllowCredentials: true,
	}, "https://app.example")
	if rr.Header().Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Fatalf("expected origin reflected, got %q", rr.Header().Get("Access-Control-Allow-Origin"))
	}
	if rr.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("explicit origin should receive credentials")
	}
	if rr.Header().Get("Vary") == "" {
		t.Fatal("explicit origin reflection must set Vary: Origin")
	}
}

// TestCORSWildcardNoCredentialsUsesLiteralStar verifies non-credentialed
// wildcard uses a cacheable literal "*".
func TestCORSWildcardNoCredentialsUsesLiteralStar(t *testing.T) {
	rr := corsResult(t, CORSConfig{AllowOrigins: []string{"*"}}, "https://anything.example")
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("got %q, want literal *", got)
	}
}

// TestCORSDisallowedOrigin verifies an origin not in the list gets no CORS
// allow header.
func TestCORSDisallowedOrigin(t *testing.T) {
	rr := corsResult(t, CORSConfig{AllowOrigins: []string{"https://app.example"}}, "https://other.example")
	if rr.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("disallowed origin must not receive an allow-origin header")
	}
}
