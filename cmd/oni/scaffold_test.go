package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"text/template"
)

// TestMainStubIsValidGo renders the scaffolded main.go and checks it parses,
// catching template/syntax errors in the generated app entrypoint.
func TestMainStubIsValidGo(t *testing.T) {
	tmpl, err := template.New("main").Parse(mainGoStub)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]any{"Name": "myapp"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", buf.Bytes(), parser.AllErrors); err != nil {
		t.Fatalf("generated main.go is not valid Go: %v\n---\n%s", err, buf.String())
	}
}

// TestScaffoldedAppBuilds scaffolds a project into a temp dir and compiles it
// against the local framework via a replace directive — the end-to-end proof
// that `oni new` produces a buildable app (and that `oni make:*` stubs embed).
func TestScaffoldedAppBuilds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping scaffold build in -short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	// scaffoldNew creates files relative to CWD; run it from tmp so the module
	// import path "myapp/database/migrations" is valid.
	orig, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)

	if err := scaffoldNew("myapp", false); err != nil {
		t.Fatalf("scaffoldNew: %v", err)
	}

	appDir := filepath.Join(tmp, "myapp")
	gomod := "module myapp\n\ngo 1.25\n\nrequire github.com/onipixel/oniworks v0.0.0\n\nreplace github.com/onipixel/oniworks => " + repoRoot + "\n"
	if err := os.WriteFile(filepath.Join(appDir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("go", args...)
		cmd.Dir = appDir
		cmd.Env = append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %v failed: %v\n%s", args, err, out)
		}
	}

	// Build with -mod=mod so missing build deps are added from the module cache
	// (avoids `go mod tidy`, which also resolves uncached test-deps of deps).
	run("build", "./...")

	// Also generate a controller via the embedded stub and rebuild — proves
	// `oni make:*` works inside a scaffolded project (no stubs/ dir present).
	if err := os.Chdir(appDir); err != nil {
		t.Fatal(err)
	}
	if err := makeStub("controller", "Post"); err != nil {
		t.Fatalf("make:controller in scaffolded app: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDir, "app", "http", "controllers", "post_controller.go")); err != nil {
		t.Fatalf("generated controller missing: %v", err)
	}
	run("build", "./...")
}
