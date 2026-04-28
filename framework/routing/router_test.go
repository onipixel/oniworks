package routing_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	onihttp "github.com/oniworks/oniworks/framework/http"
	"github.com/oniworks/oniworks/framework/routing"
)


func TestStaticRoute(t *testing.T) {
	r := routing.New()
	r.Get("/", func(c *onihttp.Context) error {
		return c.JSON(200, map[string]any{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ok") {
		t.Errorf("body should contain 'ok': %s", w.Body.String())
	}
}

func TestParamRoute(t *testing.T) {
	r := routing.New()
	r.Get("/users/:id", func(c *onihttp.Context) error {
		return c.JSON(200, map[string]any{"id": c.Param("id")})
	})

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "42") {
		t.Errorf("body should contain id 42: %s", w.Body.String())
	}
}

func TestNotFound(t *testing.T) {
	r := routing.New()
	r.Get("/exists", func(c *onihttp.Context) error { return c.JSON(200, nil) })

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestCustomNotFound(t *testing.T) {
	r := routing.New()
	r.NotFound(func(c *onihttp.Context) error {
		return c.JSON(404, map[string]any{"error": "custom not found"})
	})

	req := httptest.NewRequest(http.MethodGet, "/nowhere", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "custom not found") {
		t.Error("expected custom not found message")
	}
}

func TestGroupPrefix(t *testing.T) {
	r := routing.New()
	r.Group("/api/v1", func(g *routing.Group) {
		g.Get("/ping", func(c *onihttp.Context) error {
			return c.JSON(200, map[string]any{"pong": true})
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestMiddlewareChain(t *testing.T) {
	r := routing.New()
	order := []string{}

	mw1 := func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
		return func(c *onihttp.Context) error {
			order = append(order, "mw1-before")
			err := next(c)
			order = append(order, "mw1-after")
			return err
		}
	}
	mw2 := func(next onihttp.HandlerFunc) onihttp.HandlerFunc {
		return func(c *onihttp.Context) error {
			order = append(order, "mw2-before")
			err := next(c)
			order = append(order, "mw2-after")
			return err
		}
	}

	r.Use(mw1, mw2)
	r.Get("/chain", func(c *onihttp.Context) error {
		order = append(order, "handler")
		return c.JSON(200, nil)
	})

	req := httptest.NewRequest(http.MethodGet, "/chain", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("middleware order: got %v, want %v", order, expected)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d]: got %q, want %q", i, order[i], v)
		}
	}
}

func TestHTTPMethods(t *testing.T) {
	r := routing.New()
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		m := method
		r.Any("/resource", func(c *onihttp.Context) error {
			return c.JSON(200, map[string]any{"method": m})
		})
		break // Any registers all methods, test one
	}

	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut} {
		req := httptest.NewRequest(method, "/resource", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Errorf("%s /resource: expected 200, got %d", method, w.Code)
		}
	}
}

func TestMultipleParams(t *testing.T) {
	r := routing.New()
	r.Get("/users/:userID/posts/:postID", func(c *onihttp.Context) error {
		return c.JSON(200, map[string]any{
			"user": c.Param("userID"),
			"post": c.Param("postID"),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/users/10/posts/99", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "10") || !strings.Contains(body, "99") {
		t.Errorf("expected both IDs in body: %s", body)
	}
}
