package errors

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	onihttp "github.com/onipixel/oniworks/framework/http"
)

func runHandler(h func(*onihttp.Context, error), accept string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/boom", nil)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	c := onihttp.NewContext(rr, req, nil)
	h(c, errors.New("secret internal detail: dsn=postgres://user:pass@host/db"))
	return rr
}

// TestProdHandlerHidesInternals verifies a production handler never leaks the
// raw error string.
func TestProdHandlerHidesInternals(t *testing.T) {
	rr := runHandler(Handler(false), "application/json")
	if strings.Contains(rr.Body.String(), "secret internal detail") {
		t.Fatalf("production handler leaked internals: %s", rr.Body.String())
	}
}

// TestHandlerForEnvProdValues verifies non-dev env values produce a safe handler.
func TestHandlerForEnvProdValues(t *testing.T) {
	for _, env := range []string{"", "production", "prod", "staging", "PRODUCTION"} {
		rr := runHandler(HandlerForEnv(env), "application/json")
		if strings.Contains(rr.Body.String(), "secret internal detail") {
			t.Fatalf("env %q leaked internals: %s", env, rr.Body.String())
		}
	}
}

// TestHandlerForEnvDevShowsDetail verifies dev env values enable debug output.
func TestHandlerForEnvDevShowsDetail(t *testing.T) {
	for _, env := range []string{"local", "dev", "development", "DEBUG"} {
		rr := runHandler(HandlerForEnv(env), "application/json")
		if !strings.Contains(rr.Body.String(), "secret internal detail") {
			t.Fatalf("env %q should expose detail in debug mode, got: %s", env, rr.Body.String())
		}
	}
}
