package effeckmapper

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// A service's key, and the brand its shape carries, name the module that
// declares it: "app/src/users#Users" (D45). The fork computes the module
// part from the program (internal/effeck/services.go, key): the nearest
// ancestor directory with a package.json gives the prefix, its name or else
// the directory's own, and the module path is the file's path within that
// directory without .effeck. The mapper reads the same package.json files
// from disk, so the projection stays a function of the text and this
// context, and reports those files to TypeScript as watched files, so that
// renaming a package projects every file again.

// FileContextOf returns the context of the .effeck file at fileName, an
// absolute path with forward slashes, reading package.json files from disk.
func FileContextOf(fileName string) FileContext {
	return FileContext{FileName: fileName, ModuleKey: moduleKey(fileName, os.ReadFile)}
}

// moduleKey returns the module part of the service keys declared in the file
// at fileName, using read to load package.json files. A file outside every
// package uses its base name, where the fork uses its path from the
// program's common source directory, which a per-file mapper cannot know.
func moduleKey(fileName string, read func(string) ([]byte, error)) string {
	module := strings.TrimSuffix(fileName, ".effeck")
	for dir := path.Dir(fileName); ; dir = path.Dir(dir) {
		if data, err := read(path.Join(dir, "package.json")); err == nil {
			prefix := path.Base(dir)
			if name := packageName(data); name != "" {
				prefix = name
			}
			return prefix + "/" + strings.TrimPrefix(module, strings.TrimSuffix(dir, "/")+"/")
		}
		if parent := path.Dir(dir); parent == dir {
			break
		}
	}
	return path.Base(module)
}

// packageName returns the name field of a package.json, or "".
func packageName(data []byte) string {
	var manifest struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return ""
	}
	return manifest.Name
}

// maxWatchedDirectories bounds the walk for package.json files under a
// project's directory.
const maxWatchedDirectories = 4096

// projectPackageFiles returns the package.json files whose names can enter
// the module keys of a project configured by configFileName: one in every
// directory from the configuration's up to the root, existing or not, and
// every one below it outside node_modules and hidden directories. It also
// returns an identity that changes when any of them appears, disappears, or
// is renamed.
func projectPackageFiles(configFileName string) (files []string, identity string) {
	if configFileName == "" {
		return nil, "no-config"
	}
	root := path.Dir(filepath.ToSlash(configFileName))
	for dir := root; ; dir = path.Dir(dir) {
		files = append(files, path.Join(dir, "package.json"))
		if parent := path.Dir(dir); parent == dir {
			break
		}
	}
	visited := 0
	_ = filepath.WalkDir(filepath.FromSlash(root), func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			visited++
			base := entry.Name()
			if visited > maxWatchedDirectories || filepath.ToSlash(name) != root && (base == "node_modules" || strings.HasPrefix(base, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() == "package.json" && path.Dir(filepath.ToSlash(name)) != root {
			files = append(files, filepath.ToSlash(name))
		}
		return nil
	})
	slices.Sort(files)
	files = slices.Compact(files)
	hash := sha256.New()
	for _, file := range files {
		hash.Write([]byte(file))
		hash.Write([]byte{0})
		if data, err := os.ReadFile(filepath.FromSlash(file)); err == nil {
			hash.Write([]byte("name:" + packageName(data)))
		} else {
			hash.Write([]byte("absent"))
		}
		hash.Write([]byte{0})
	}
	return files, hex.EncodeToString(hash.Sum(nil)[:16])
}
