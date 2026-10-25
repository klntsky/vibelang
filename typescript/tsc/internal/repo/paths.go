package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var rootPath = sync.OnceValue(func() string {
	_, filename, _, _ := runtime.Caller(0)
	cwd, _ := os.Getwd()
	root, err := discoverRoot(filepath.FromSlash(filename), cwd, os.Getenv("TYPESCRIPT_REPO_ROOT"))
	if err != nil {
		panic(err)
	}
	return root
})

// discoverRoot also works when -trimpath makes runtime.Caller return a module
// import path, or a test binary has moved since it was built. Fixtures are
// optional and must not be used to identify the checkout itself.
func discoverRoot(filename, cwd, override string) (string, error) {
	isRoot := func(dir string) bool {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err != nil {
			return false
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "module" {
				return strings.Trim(fields[1], "\"") == "github.com/microsoft/TypeScript/tsc"
			}
		}
		return false
	}
	if override != "" {
		if filepath.IsAbs(override) && isRoot(override) {
			return filepath.Clean(override), nil
		}
		return "", fmt.Errorf("TYPESCRIPT_REPO_ROOT=%q must be an absolute path to the TypeScript tsc module (containing its go.mod)", override)
	}
	starts := []string{cwd}
	if filepath.IsAbs(filename) {
		starts = append([]string{filepath.Dir(filename)}, starts...)
	}
	for _, start := range starts {
		if !filepath.IsAbs(start) {
			continue
		}
		for dir := filepath.Clean(start); ; dir = filepath.Dir(dir) {
			if isRoot(dir) {
				return dir, nil
			}
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	return "", fmt.Errorf("cannot find the TypeScript tsc module from caller %q or working directory %q; run inside typescript/tsc or set TYPESCRIPT_REPO_ROOT to its absolute path", filename, cwd)
}

func RootPath() string {
	return rootPath()
}

var testDataPath = sync.OnceValue(func() string {
	return filepath.Join(rootPath(), "testdata")
})

func TestDataPath() string {
	return testDataPath()
}
