package fixtures

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/repo"
)

// Require names the optional upstream fixture that a test needs. Missing
// fixtures skip only their consumers; unreadable or broken fixtures still fail.
func Require(tb testing.TB, relative string) string {
	tb.Helper()
	path := filepath.Join(repo.TestDataPath(), relative)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		tb.Skipf("missing upstream TypeScript fixture %s; restore typescript/tsc/testdata/%s from the matching upstream testdata checkout (not included in this VibeLang checkout)", path, filepath.ToSlash(relative))
	} else if err != nil {
		tb.Fatalf("cannot inspect fixture %s: %v", path, err)
	}
	return path
}
