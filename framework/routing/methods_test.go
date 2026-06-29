package routing

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	onihttp "github.com/onipixel/oniworks/framework/http"
)

func okHandler(body string) onihttp.HandlerFunc {
	return func(c *onihttp.Context) error { return c.JSON(200, onihttp.Map{"body": body}) }
}

func newRouterWith(setup func(*Router)) *Router {
	r := New()
	setup(r)
	return r
}

// TestMethodNotAllowed verifies a path registered under a different method
// returns 405 with an Allow header (not 404).
func TestMethodNotAllowed(t *testing.T) {
	r := newRouterWith(func(r *Router) {
		r.Get("/users", okHandler("list"))
		r.Post("/users", okHandler("create"))
	})

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/users", nil))

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("got %d, want 405", rr.Code)
	}
	allow := rr.Header().Get("Allow")
	if !strings.Contains(allow, "GET") || !strings.Contains(allow, "POST") {
		t.Fatalf("Allow header = %q, want GET and POST", allow)
	}
}

// TestNotFoundStill404 verifies a truly unknown path is still 404.
func TestNotFoundStill404(t *testing.T) {
	r := newRouterWith(func(r *Router) { r.Get("/users", okHandler("list")) })
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rr.Code)
	}
}

// TestAutoOptions verifies OPTIONS on a known path returns 204 + Allow.
func TestAutoOptions(t *testing.T) {
	r := newRouterWith(func(r *Router) {
		r.Get("/items", okHandler("list"))
		r.Post("/items", okHandler("create"))
	})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodOptions, "/items", nil))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("got %d, want 204", rr.Code)
	}
	allow := rr.Header().Get("Allow")
	for _, m := range []string{"GET", "POST", "HEAD"} {
		if !strings.Contains(allow, m) {
			t.Errorf("Allow %q missing %s", allow, m)
		}
	}
}

// TestImplicitHead verifies HEAD falls back to the GET handler with no body.
func TestImplicitHead(t *testing.T) {
	r := newRouterWith(func(r *Router) { r.Get("/page", okHandler("hello")) })
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodHead, "/page", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 for HEAD via GET", rr.Code)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("HEAD response must have an empty body, got %q", rr.Body.String())
	}
}

// TestExplicitHeadStillWorks verifies an explicit HEAD route takes precedence.
func TestExplicitHeadStillWorks(t *testing.T) {
	r := newRouterWith(func(r *Router) {
		r.Get("/x", okHandler("get"))
		r.Head("/x", func(c *onihttp.Context) error {
			c.Response.Header().Set("X-Head", "1")
			return c.NoContent()
		})
	})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodHead, "/x", nil))
	if rr.Header().Get("X-Head") != "1" {
		t.Fatal("explicit HEAD handler should run")
	}
}

// TestConcurrentServeAndRegister exercises serving while routes are being
// registered, guarding against the lock-free map-iteration race in the
// 405/allowed-methods path. Run with -race in CI to enforce.
func TestConcurrentServeAndRegister(t *testing.T) {
	r := New()
	r.Get("/seed", okHandler("seed"))

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Serving goroutines (some hit unknown methods → allowedMethods scan).
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					rr := httptest.NewRecorder()
					r.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/seed", nil))
					rr2 := httptest.NewRecorder()
					r.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/seed", nil))
				}
			}
		}()
	}

	// Registrar goroutine mutating the routes map concurrently.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			r.Get("/r"+strconv.Itoa(i), okHandler("x"))
		}
	}()

	time.Sleep(30 * time.Millisecond)
	close(stop)
	wg.Wait()
}
