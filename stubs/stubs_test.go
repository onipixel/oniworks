package stubs

import (
	"bytes"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
	"text/template"
)

// TestAllStubsRenderToValidGo renders every embedded stub with representative
// data and parses the result, catching template or syntax errors in generated
// code before a user ever runs `oni make:*`.
func TestAllStubsRenderToValidGo(t *testing.T) {
	entries, err := fs.ReadDir(FS, ".")
	if err != nil {
		t.Fatalf("read embedded stubs: %v", err)
	}

	data := map[string]any{
		"Name":      "Post",
		"NameSnake": "post",
		"Timestamp": "20260629120000",
		"Date":      "2026-06-29",
	}

	var count int
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".stub") {
			continue
		}
		count++
		t.Run(e.Name(), func(t *testing.T) {
			raw, err := FS.ReadFile(e.Name())
			if err != nil {
				t.Fatalf("read %s: %v", e.Name(), err)
			}
			tmpl, err := template.New(e.Name()).Parse(string(raw))
			if err != nil {
				t.Fatalf("parse template %s: %v", e.Name(), err)
			}
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				t.Fatalf("execute %s: %v", e.Name(), err)
			}
			// Migration stubs are package-level Go; all stubs should parse as a
			// Go source file.
			if _, err := parser.ParseFile(token.NewFileSet(), e.Name()+".go", buf.Bytes(), parser.AllErrors); err != nil {
				t.Fatalf("generated code from %s is not valid Go: %v\n---\n%s", e.Name(), err, buf.String())
			}
		})
	}
	if count == 0 {
		t.Fatal("no stubs found embedded")
	}
}

// TestReadKnownStub verifies Read returns content for a known stub and an error
// for an unknown one.
func TestReadKnownStub(t *testing.T) {
	if _, err := Read("controller"); err != nil {
		t.Fatalf("controller stub should exist: %v", err)
	}
	if _, err := Read("does-not-exist"); err == nil {
		t.Fatal("expected error for unknown stub")
	}
}
