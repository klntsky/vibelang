package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/vibelangmapper"
)

func writeFixture(t *testing.T, root, name, text string) {
	t.Helper()
	file := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

// Conditional exports are ordinary Node package manifests. The generator
// must load the selected declarations exactly, including a types condition
// that already names a .d.ts, rather than silently reading an empty module.
func TestExportPathsConditionalDeclarations(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "sample/package.json", `{"exports":{
		".":{"types":"./dist/index.d.ts","import":"./dist/index.js"},
		"./direct":"./dist/direct.js",
		"./fallback":{"import":"./dist/fallback.js"},
		"./default":{"default":"./dist/default.js"},
		"./package.json":"./package.json",
		"./private":null,
		"./internal/index":"./dist/internal/index.js"
	}}`)
	paths := map[string][]string{}
	if err := exportPaths(paths, root, "sample"); err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"sample":              {filepath.ToSlash(filepath.Join(root, "sample/dist/index.d.ts"))},
		"sample/direct":       {filepath.ToSlash(filepath.Join(root, "sample/dist/direct.d.ts"))},
		"sample/fallback":     {filepath.ToSlash(filepath.Join(root, "sample/dist/fallback.d.ts"))},
		"sample/default":      {filepath.ToSlash(filepath.Join(root, "sample/dist/default.d.ts"))},
		"sample/package.json": {filepath.ToSlash(filepath.Join(root, "sample/package.json"))},
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("export paths:\ngot %#v\nwant %#v", paths, want)
	}
}

func TestPublishedModulesExcludePrivateSubpaths(t *testing.T) {
	root := t.TempDir()
	for _, pkg := range distPackages {
		writeFixture(t, root, pkg+"/package.json", `{"exports":{".":"./dist/index.js","./*":"./dist/*.js","./internal/*":null,"./*/index":null,"./private":null,"./package.json":"./package.json","./named":{"types":"./dist/named.d.ts"}}}`)
		for _, name := range []string{"index.d.ts", "named.d.ts", "Public.d.ts", "nested/Visible.d.ts", "internal/secret.d.ts", "nested/index.d.ts", "private.d.ts", "Public.js"} {
			writeFixture(t, root, pkg+"/dist/"+name, "export {}")
		}
	}
	got, err := publishedModules(root)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, pkg := range distPackages {
		want = append(want, pkg, pkg+"/Public", pkg+"/index", pkg+"/named", pkg+"/nested/Visible")
	}
	slices.Sort(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("published modules:\ngot %v\nwant %v", got, want)
	}
}

// Exercise the complete checker-backed generator using the same pinned
// inputs as scripts/check-forms-table.sh. These classifications determine
// whether users' callbacks run as programs, Effect functions or plain JS.
func TestGenerateCallbackForms(t *testing.T) {
	root, err := findRepo()
	if err != nil {
		t.Fatal(err)
	}
	modules := filepath.Join(root, "docs/tools/node_modules")
	runtime := filepath.Join(root, "packages/runtime/src")
	generated, err := generate(modules, runtime)
	if err != nil {
		t.Fatal(err)
	}
	table := vibelangmapper.NewTable(generated.header, generated.entries, generated.aliases)
	for _, tc := range []struct {
		key         string
		argc, param int
		want        vibelangmapper.Class
	}{
		{"effect/Effect#map", 2, 1, vibelangmapper.ClassPlainFn},
		{"effect/Effect#flatMap", 2, 1, vibelangmapper.ClassEffectFn},
		{"effect/Effect#retry", 2, 0, vibelangmapper.ClassProgram},
		{"effect/Array#map", 2, 1, vibelangmapper.ClassPlainFn},
	} {
		forms, ok := table.FormsFor(tc.key, tc.argc)
		if !ok || len(forms.Params) <= tc.param || forms.Params[tc.param].Top != tc.want {
			t.Errorf("%s arg %d: %+v; want %s", tc.key, tc.param, forms, tc.want)
		}
	}
	all, ok := table.FormsFor("effect/Effect#all", 1)
	if !ok || len(all.Params) == 0 || all.Params[0].Elem != vibelangmapper.ClassProgram || all.Params[0].AnyProp != vibelangmapper.ClassProgram {
		t.Errorf("collections must accept program elements: %+v", all)
	}
	// The generated JSONL is the real interchange boundary, not the in-memory
	// walker. Loading it must preserve the same program/callback decisions.
	var out bytes.Buffer
	if err := vibelangmapper.WriteTable(&out, generated.header, generated.modules, generated.entries, generated.aliases, generated.values); err != nil {
		t.Fatal(err)
	}
	for _, line := range bytes.Split(bytes.TrimSpace(out.Bytes()), []byte{'\n'}) {
		if !json.Valid(line) {
			t.Fatalf("invalid generated JSONL: %s", line)
		}
	}
	parsed, err := vibelangmapper.ReadTable(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	before, _ := table.FormsFor("effect/Effect#flatMap", 2)
	after, _ := parsed.FormsFor("effect/Effect#flatMap", 2)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("callback contract changed across table serialization: %+v -> %+v", before, after)
	}
}
