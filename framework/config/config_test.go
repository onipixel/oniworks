package config_test

import (
	"os"
	"testing"

	"github.com/oniworks/oniworks/framework/config"
)

func TestConfigSetGet(t *testing.T) {
	c := config.New()
	c.Set("app.name", "OniWorks")
	c.Set("app.debug", true)
	c.Set("server.port", 8080)

	if got := c.String("app.name", ""); got != "OniWorks" {
		t.Errorf("String: got %q, want %q", got, "OniWorks")
	}
	if got := c.Bool("app.debug", false); !got {
		t.Error("Bool: expected true")
	}
	if got := c.Int("server.port", 0); got != 8080 {
		t.Errorf("Int: got %d, want 8080", got)
	}
}

func TestConfigDefaults(t *testing.T) {
	c := config.New()
	if got := c.String("missing.key", "fallback"); got != "fallback" {
		t.Errorf("expected fallback, got %q", got)
	}
	if got := c.Int("missing.key", 42); got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
	if got := c.Bool("missing.key", true); !got {
		t.Error("expected true fallback")
	}
}

func TestConfigMerge(t *testing.T) {
	c := config.New()
	c.Merge(map[string]any{
		"database": map[string]any{
			"host": "localhost",
			"port": 5432,
		},
	})
	if got := c.String("database.host", ""); got != "localhost" {
		t.Errorf("merge host: got %q", got)
	}
	if got := c.Int("database.port", 0); got != 5432 {
		t.Errorf("merge port: got %d", got)
	}
}

func TestLoadEnv(t *testing.T) {
	f, err := os.CreateTemp("", "test*.env")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	_, _ = f.WriteString("APP_ENV=testing\nAPP_KEY=secret123\n# comment\n")
	_ = f.Close()

	if err := config.LoadEnv(f.Name()); err != nil {
		t.Fatalf("LoadEnv: %v", err)
	}
	if got := os.Getenv("APP_ENV"); got != "testing" {
		t.Errorf("APP_ENV: got %q, want %q", got, "testing")
	}
	if got := os.Getenv("APP_KEY"); got != "secret123" {
		t.Errorf("APP_KEY: got %q", got)
	}
}

func TestEnvHelper(t *testing.T) {
	t.Setenv("TEST_ONI_KEY", "hello")
	if got := config.Env("TEST_ONI_KEY", ""); got != "hello" {
		t.Errorf("Env: got %q", got)
	}
	if got := config.Env("MISSING_ONI_KEY", "default"); got != "default" {
		t.Errorf("Env default: got %q", got)
	}
}

func TestLoadYAML(t *testing.T) {
	yaml := `
app:
  name: "TestApp"
  debug: true
server:
  port: 3000
`
	f, err := os.CreateTemp("", "test*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	_, _ = f.WriteString(yaml)
	_ = f.Close()

	c := config.New()
	if err := config.Load(c, f.Name()); err != nil {
		t.Fatalf("Load YAML: %v", err)
	}

	if got := c.String("app.name", ""); got != "TestApp" {
		t.Errorf("yaml app.name: got %q", got)
	}
	if got := c.Bool("app.debug", false); !got {
		t.Error("yaml app.debug: expected true")
	}
	if got := c.Int("server.port", 0); got != 3000 {
		t.Errorf("yaml server.port: got %d", got)
	}
}

func TestConfigAll(t *testing.T) {
	c := config.New()
	c.Set("x", 1)
	c.Set("y", 2)
	all := c.All()
	if len(all) == 0 {
		t.Error("All() should return non-empty map")
	}
}
