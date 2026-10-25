package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module github.com/microsoft/TypeScript/tsc\n"), 0600); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "internal", "parser")
	if err := os.MkdirAll(nested, 0700); err != nil {
		t.Fatal(err)
	}
	// A nested, unrelated Go module must not hide the compiler module.
	if err := os.WriteFile(filepath.Join(nested, "go.mod"), []byte("module example.com/unrelated\n"), 0600); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	for _, tc := range []struct {
		name, caller, cwd, override string
	}{
		{"absolute caller", filepath.Join(root, "internal/repo/paths.go"), outside, ""},
		{"trimpath caller", "github.com/microsoft/TypeScript/tsc/internal/repo/paths.go", nested, ""},
		{"versioned trimpath caller", "github.com/microsoft/TypeScript/tsc@v0.0.0/internal/repo/paths.go", nested, ""},
		{"relative caller", "internal/repo/paths.go", nested, ""},
		{"no caller", "", nested, ""},
		{"moved checkout", filepath.Join(outside, "old-checkout/internal/repo/paths.go"), nested, ""},
		{"explicit root outside checkout", "", outside, root},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := discoverRoot(tc.caller, tc.cwd, tc.override)
			if err != nil || got != root {
				t.Fatalf("root = %q, %v; want %q", got, err, root)
			}
		})
	}
	for _, tc := range []struct {
		name, cwd, override string
	}{
		{"outside checkout", outside, ""},
		{"invalid override", nested, outside},
		{"relative override", nested, "relative"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := discoverRoot("github.com/microsoft/TypeScript/tsc/internal/repo/paths.go", tc.cwd, tc.override)
			if err == nil || !strings.Contains(err.Error(), "TYPESCRIPT_REPO_ROOT") {
				t.Fatalf("missing actionable discovery error: %v", err)
			}
		})
	}
}

func TestRootPath(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(RootPath(), "go.mod"))
	if err != nil || !strings.Contains(string(data), "module github.com/microsoft/TypeScript/tsc") {
		t.Fatalf("RootPath() does not identify the compiler module: %v", err)
	}
	if TestDataPath() != filepath.Join(RootPath(), "testdata") {
		t.Fatal("fixture path must stay rooted even when the directory is absent")
	}
}
