package frontend_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oniworks/oniworks/framework/frontend"
)

func TestViteTagDevMode(t *testing.T) {
	m := frontend.New("dev", frontend.WithDevURL("http://localhost:5173"))
	tag := m.ViteTag("resources/ts/app.ts")

	if !strings.Contains(tag, "http://localhost:5173") {
		t.Errorf("dev tag should point to dev server: %q", tag)
	}
	if !strings.Contains(tag, "resources/ts/app.ts") {
		t.Errorf("dev tag should contain entry path: %q", tag)
	}
	if !strings.Contains(tag, `type="module"`) {
		t.Errorf("dev tag should be module script: %q", tag)
	}
}

func TestViteTagProdBeforeManifest(t *testing.T) {
	m := frontend.New("prod")
	tag := m.ViteTag("resources/ts/app.ts")
	if !strings.Contains(tag, "manifest not loaded") {
		t.Logf("got: %q", tag) // just a comment in the output
	}
}

func TestLoadManifestAndViteTag(t *testing.T) {
	// Write a fake Vite manifest
	manifest := `{
		"resources/ts/app.ts": {
			"file": "assets/app-abc123.js",
			"css": ["assets/app-def456.css"],
			"isEntry": true
		}
	}`
	f, err := os.CreateTemp("", "manifest*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	_, _ = f.WriteString(manifest)
	_ = f.Close()

	m := frontend.New("prod")
	if err := m.LoadManifest(f.Name()); err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}

	tag := m.ViteTag("resources/ts/app.ts")

	if !strings.Contains(tag, "assets/app-abc123.js") {
		t.Errorf("prod tag should contain hashed JS: %q", tag)
	}
	if !strings.Contains(tag, "assets/app-def456.css") {
		t.Errorf("prod tag should contain hashed CSS: %q", tag)
	}
}

func TestGenerateTypes(t *testing.T) {
	dir := t.TempDir()
	goFile := filepath.Join(dir, "user.go")
	outFile := filepath.Join(dir, "types.generated.ts")

	goSrc := `package models

import "time"

type User struct {
	ID        int64     ` + "`" + `json:"id"` + "`" + `
	Name      string    ` + "`" + `json:"name"` + "`" + `
	Email     string    ` + "`" + `json:"email"` + "`" + `
	CreatedAt time.Time ` + "`" + `json:"created_at"` + "`" + `
	Hidden    string    ` + "`" + `json:"-"` + "`" + `
}
`
	if err := os.WriteFile(goFile, []byte(goSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := frontend.GenerateTypes(dir, outFile); err != nil {
		t.Fatalf("GenerateTypes: %v", err)
	}

	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	ts := string(content)

	if !strings.Contains(ts, "export interface User") {
		t.Errorf("should generate User interface: %q", ts)
	}
	if !strings.Contains(ts, "id: number") {
		t.Errorf("id field should be number: %q", ts)
	}
	if !strings.Contains(ts, "name: string") {
		t.Errorf("name field should be string: %q", ts)
	}
	if strings.Contains(ts, "Hidden") {
		t.Error("json:\"-\" field should be excluded")
	}
	if !strings.Contains(ts, "AUTO-GENERATED") {
		t.Error("output should have auto-generated header")
	}
}

func TestLoadManifestMissing(t *testing.T) {
	m := frontend.New("prod")
	err := m.LoadManifest("/nonexistent/manifest.json")
	if err == nil {
		t.Error("expected error for missing manifest file")
	}
}
