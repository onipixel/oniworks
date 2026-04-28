package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	onihttp "github.com/onipixel/oniworks/framework/http"
)

func makeContext(method, path, body string) (*onihttp.Context, *httptest.ResponseRecorder) {
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	} else {
		bodyReader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	return onihttp.NewContext(w, req, nil), w
}

func TestContextJSON(t *testing.T) {
	c, w := makeContext(http.MethodGet, "/", "")
	if err := c.JSON(200, map[string]any{"hello": "world"}); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if w.Code != 200 {
		t.Errorf("status: got %d, want 200", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type: got %q", ct)
	}
	var result map[string]any
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["hello"] != "world" {
		t.Errorf("body: got %v", result)
	}
}

func TestContextBind(t *testing.T) {
	type Input struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	body := `{"name":"Alice","email":"alice@example.com"}`
	c, _ := makeContext(http.MethodPost, "/", body)

	var in Input
	if err := c.Bind(&in); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if in.Name != "Alice" {
		t.Errorf("Name: got %q", in.Name)
	}
	if in.Email != "alice@example.com" {
		t.Errorf("Email: got %q", in.Email)
	}
}

func TestContextParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()
	c := onihttp.NewContext(w, req, map[string]string{"id": "123"})
	if got := c.Param("id"); got != "123" {
		t.Errorf("Param: got %q, want %q", got, "123")
	}
}

func TestContextQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/search?q=oniworks&page=2", nil)
	w := httptest.NewRecorder()
	c := onihttp.NewContext(w, req, nil)

	if got := c.Query("q"); got != "oniworks" {
		t.Errorf("Query q: got %q", got)
	}
	if got := c.Query("page"); got != "2" {
		t.Errorf("Query page: got %q", got)
	}
	if got := c.QueryD("missing", "default"); got != "default" {
		t.Errorf("QueryD: got %q", got)
	}
}

func TestContextString(t *testing.T) {
	c, w := makeContext(http.MethodGet, "/", "")
	if err := c.String(200, "Hello, %s!", "OniWorks"); err != nil {
		t.Fatalf("String: %v", err)
	}
	if !strings.Contains(w.Body.String(), "Hello, OniWorks!") {
		t.Errorf("body: %q", w.Body.String())
	}
}

func TestContextHTML(t *testing.T) {
	c, w := makeContext(http.MethodGet, "/", "")
	if err := c.HTML(200, "<h1>Welcome</h1>"); err != nil {
		t.Fatalf("HTML: %v", err)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type: got %q", ct)
	}
}

func TestContextNoContent(t *testing.T) {
	c, w := makeContext(http.MethodDelete, "/item/1", "")
	if err := c.NoContent(); err != nil {
		t.Fatalf("NoContent: %v", err)
	}
	if w.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want 204", w.Code)
	}
}

func TestContextRedirect(t *testing.T) {
	c, w := makeContext(http.MethodGet, "/old", "")
	if err := c.Redirect(302, "/new"); err != nil {
		t.Fatalf("Redirect: %v", err)
	}
	if w.Code != 302 {
		t.Errorf("redirect status: got %d", w.Code)
	}
	if got := w.Header().Get("Location"); got != "/new" {
		t.Errorf("Location: got %q", got)
	}
}

func TestContextSetGet(t *testing.T) {
	c, _ := makeContext(http.MethodGet, "/", "")
	c.Set("user_id", int64(42))
	val, ok := c.Get("user_id")
	if !ok {
		t.Fatal("Get: key not found")
	}
	if val.(int64) != 42 {
		t.Errorf("Get: got %v", val)
	}
}

func TestContextIsJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	c := onihttp.NewContext(httptest.NewRecorder(), req, nil)
	if !c.IsJSON() {
		t.Error("IsJSON should be true for JSON content type")
	}
}
