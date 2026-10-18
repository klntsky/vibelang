// Command effeck-forms generates the forms table that Effeck's text-only
// projector reads (internal/effeckmapper/forms.go, language change L1).
//
//	cd typescript/tsc && go run ./cmd/effeck-forms [-o table.jsonl]
//
// It loads effect, the @effect/* packages the corpus uses, and
// effeck-runtime's sources through the fork's checker, walks every callable
// export of the modules in forms.Modules, and classifies each signature's
// declared parameters with the fork's own classifier (effeck.FormsOfType,
// the code behind D56, D147, D188, D7, and D49). The output is sorted and
// holds no machine paths, so the same inputs always give the same bytes;
// scripts/check-forms-table.sh regenerates it and fails on any difference.
//
// Inputs, by default relative to the repository root:
//
//	-node-modules docs/tools/node_modules   effect and @effect/*, pinned by docs/tools/bun.lock
//	-runtime      packages/runtime/src      effeck-runtime's TypeScript sources
package main

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/bundled"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/effeck"
	"github.com/microsoft/TypeScript/tsc/internal/effeckmapper"
	"github.com/microsoft/TypeScript/tsc/internal/tsoptions"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"github.com/microsoft/TypeScript/tsc/internal/vfs"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/osvfs"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/wrapvfs"
)

// Command is what the table's header names as the way to regenerate it.
const Command = "scripts/check-forms-table.sh --write"

// Modules are the module specifiers whose exports the table covers: the
// ones the corpus imports (research/typedirected.md section 6.2). A module
// another one exports as a namespace, such as effect/Effect from effect, is
// walked too, under its own path.
var Modules = []string{
	"effect",
	"effect/http",
	"effect/testing",
	"effect/process",
	"effect/schema",
	"effect/observability/OtlpTracer",
	"effect/observability/OtlpSerialization",
	"effect/http/FetchHttpClient",
	"effect/process/ChildProcessSpawner",
	"effect/schema/SchemaAOTCompiler",
	"@effect/vitest",
	"@effect/platform-node",
	"@effect/platform-bun",
	"effeck-runtime",
	"effeck-runtime/kernel",
}

// methodModules are effect's modules whose data types take the module's
// self-first functions as methods (D152). It mirrors methodModules in
// internal/effeck/datamethods.go, which is not exported.
var methodModules = []string{"BigDecimal", "Cause", "Chunk", "DateTime", "Deferred", "Duration", "Effect", "Exit", "Fiber",
	"HashMap", "HashSet", "Option", "Queue", "Ref", "Result", "Schedule", "Sink", "Stream"}

// libReceivers are the lib's types whose own methods a lambda is most
// often passed to: xs.map(f), promise.then(f), set.forEach(f).
var libReceivers = []string{"Array", "ReadonlyArray", "Map", "ReadonlyMap", "Set", "ReadonlySet", "Promise"}

// distPackages are the packages whose modules live under dist/, one
// declaration file per module path (package.json maps "./*" to "./dist/*").
var distPackages = []string{"effect", "@effect/vitest", "@effect/platform-node", "@effect/platform-bun"}

func main() {
	repo := flag.String("repo", "", "repository root (default: found from the working directory)")
	nodeModules := flag.String("node-modules", "", "directory holding effect and @effect/* (default docs/tools/node_modules)")
	runtime := flag.String("runtime", "", "effeck-runtime's source directory (default packages/runtime/src)")
	out := flag.String("o", "", "output file (default stdout)")
	stats := flag.Bool("stats", false, "print counts and timing to stderr")
	flag.Parse()
	root := *repo
	if root == "" {
		var err error
		if root, err = findRepo(); err != nil {
			fail(err)
		}
	}
	if *nodeModules == "" {
		*nodeModules = filepath.Join(root, "docs", "tools", "node_modules")
	}
	if *runtime == "" {
		*runtime = filepath.Join(root, "packages", "runtime", "src")
	}
	start := time.Now()
	g, err := generate(must(filepath.Abs(*nodeModules)), must(filepath.Abs(*runtime)))
	if err != nil {
		fail(err)
	}
	var w io.Writer = os.Stdout
	var f *os.File
	if *out != "" {
		if f, err = os.Create(*out); err != nil {
			fail(err)
		}
		w = f
	}
	if err := effeckmapper.WriteTable(w, g.header, g.entries, g.aliases); err != nil {
		fail(err)
	}
	if f != nil {
		if err := f.Close(); err != nil {
			fail(err)
		}
	}
	if *stats {
		sigs, formKeys := 0, 0
		for _, s := range g.entries {
			sigs += len(s)
			formKeys++
		}
		fmt.Fprintf(os.Stderr, "effeck-forms: %d keys (%d signatures), %d aliases, %d callable exports walked, %s\n",
			formKeys, sigs, len(g.aliases), g.walked, time.Since(start).Round(time.Millisecond))
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		fail(err)
	}
	return v
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "effeck-forms:", err)
	os.Exit(1)
}

// findRepo returns the directory above the working directory that holds
// typescript/tsc/go.mod.
func findRepo() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "typescript", "tsc", "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no repository root above the working directory; pass -repo")
		}
		dir = parent
	}
}

type generated struct {
	header  effeckmapper.Header
	entries map[string][]effeckmapper.Signature
	aliases map[string]string
	walked  int
}

// generate builds a program whose one root file, an .effeck file kept in
// memory, imports every module in Modules as a namespace, and walks them.
func generate(nodeModules, runtime string) (*generated, error) {
	for _, pkg := range distPackages {
		if _, err := os.Stat(filepath.Join(nodeModules, pkg, "package.json")); err != nil {
			return nil, fmt.Errorf("%s is not installed in %s (run bun install in docs/tools): %w", pkg, nodeModules, err)
		}
	}
	if _, err := os.Stat(filepath.Join(runtime, "index.ts")); err != nil {
		return nil, fmt.Errorf("no effeck-runtime sources: %w", err)
	}
	// The program's own directory exists only in memory. Every package is
	// reached through paths, so one copy of effect serves the runtime's
	// sources and the @effect packages alike.
	dir := filepath.ToSlash(filepath.Join(filepath.Dir(runtime), ".effeck-forms"))
	paths := map[string][]string{
		"effeck-runtime":        {filepath.ToSlash(filepath.Join(runtime, "index.ts"))},
		"effeck-runtime/kernel": {filepath.ToSlash(filepath.Join(runtime, "kernel", "kernel.ts"))},
	}
	for _, pkg := range distPackages {
		if err := exportPaths(paths, nodeModules, pkg); err != nil {
			return nil, err
		}
	}
	config, _ := json.Marshal(map[string]any{
		"compilerOptions": map[string]any{
			"target": "es2022", "lib": []string{"esnext", "dom"}, "module": "nodenext", "moduleResolution": "nodenext",
			"strict": true, "types": []string{}, "skipLibCheck": true, "noEmit": true, "resolveJsonModule": true, "paths": paths,
		},
		"files": []string{"forms.effeck"},
	})
	var source strings.Builder
	for i, m := range Modules {
		fmt.Fprintf(&source, "import * as M%d from %q\n", i, m)
	}
	source.WriteString("export const keep = [")
	for i := range Modules {
		if i > 0 {
			source.WriteString(", ")
		}
		fmt.Fprintf(&source, "M%d", i)
	}
	source.WriteString("]\n")
	virtual := map[string]string{dir + "/tsconfig.json": string(config), dir + "/forms.effeck": source.String()}
	fs := memoryOverlay(bundled.WrapFS(osvfs.FS()), dir, virtual)
	parsed, diags := tsoptions.GetParsedCommandLineOfConfigFile(tspath.RootedFilePathFromAbsolute(dir+"/tsconfig.json"), &core.CompilerOptions{}, nil, fs, nil)
	if len(diags) != 0 {
		return nil, fmt.Errorf("tsconfig: %v", diags[0].String())
	}
	p := compiler.NewProgram(compiler.ProgramOptions{Config: parsed, Host: compiler.NewCompilerHost(fs, bundled.LibPath(), nil, nil, nil)})
	p.BindSourceFiles()
	c, done := p.GetTypeChecker(context.Background())
	defer done()
	file := p.GetSourceFile(tspath.RootedFilePathFromAbsolute(dir + "/forms.effeck"))
	if file == nil {
		return nil, fmt.Errorf("the root file did not load")
	}
	w := &walker{c: c, runtime: runtime, nodeModules: nodeModules, reach: map[*ast.Symbol][]reached{}, modulePaths: map[*ast.Symbol]string{}, sigs: map[string][]effeckmapper.Signature{}, aliases: map[string]string{}}
	for i, statement := range file.Statements.Nodes {
		if statement.Kind != ast.KindImportDeclaration {
			continue
		}
		module := c.ResolveExternalModuleName(statement.ModuleSpecifier(), nil)
		if module == nil {
			return nil, fmt.Errorf("cannot resolve %s", Modules[i])
		}
		w.enqueue(module, Modules[i])
	}
	w.run()
	w.prelude(file)
	entries := w.finish()
	w.methods(entries)
	g := &generated{entries: entries, aliases: w.aliases, walked: w.walked}
	inputs, err := pinInputs(p, runtime)
	if err != nil {
		return nil, err
	}
	var methodPaths []string
	for _, m := range methodModules {
		methodPaths = append(methodPaths, "effect/"+m)
	}
	g.header = effeckmapper.Header{Notice: "generated; do not edit", Command: Command, Inputs: inputs, Modules: Modules, MethodModules: methodPaths}
	return g, nil
}

// exportPaths adds a paths entry for each subpath pkg's package.json
// exports, pointing at its declaration file: "./schema" at
// dist/schema/index.d.ts and "./*" at dist/*.d.ts. As in module resolution,
// an exact subpath wins over the wildcard, which matters on a
// case-insensitive file system where effect/schema could match Schema.d.ts.
func exportPaths(paths map[string][]string, nodeModules, pkg string) error {
	root := filepath.ToSlash(filepath.Join(nodeModules, pkg))
	data, err := os.ReadFile(filepath.Join(nodeModules, pkg, "package.json"))
	if err != nil {
		return err
	}
	var manifest struct {
		Exports map[string]json.RawMessage `json:"exports"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("%s/package.json: %w", root, err)
	}
	for sub, raw := range manifest.Exports {
		var target string
		if json.Unmarshal(raw, &target) != nil {
			var conditions map[string]string
			if json.Unmarshal(raw, &conditions) != nil {
				continue // null: not exported
			}
			target = cmp.Or(conditions["types"], conditions["import"], conditions["default"])
		}
		if target == "" || strings.HasSuffix(sub, "/index") {
			continue
		}
		name := pkg + strings.TrimPrefix(sub, ".")
		if !strings.HasSuffix(target, ".json") {
			target = strings.TrimSuffix(target, ".js") + ".d.ts"
		}
		paths[name] = []string{root + "/" + strings.TrimPrefix(target, "./")}
	}
	return nil
}

// memoryOverlay serves files, held in memory under dir, over fs.
func memoryOverlay(fs vfs.FS, dir string, files map[string]string) vfs.FS {
	inDir := func(p string) bool { return p == dir || strings.HasPrefix(p, dir+"/") }
	return wrapvfs.Wrap(fs, wrapvfs.Replacements{
		FileExists: func(p tspath.RootedFilePath) bool {
			if _, ok := files[string(p)]; ok {
				return true
			}
			return !inDir(string(p)) && fs.FileExists(p)
		},
		ReadFile: func(p tspath.RootedFilePath) (string, bool) {
			if text, ok := files[string(p)]; ok {
				return text, true
			}
			if inDir(string(p)) {
				return "", false
			}
			return fs.ReadFile(p)
		},
		DirectoryExists: func(p tspath.RootedDirectoryPath) bool {
			if string(p) == dir {
				return true
			}
			return !inDir(string(p)) && fs.DirectoryExists(p)
		},
		Realpath: func(p tspath.RootedPath) tspath.RootedPath {
			if inDir(string(p)) {
				return p
			}
			return fs.Realpath(p)
		},
	})
}

// reached is one key under which the walk reached a value.
type reached struct {
	module string // the module path the walk was in
	key    string
	order  int
}

type walker struct {
	c           *checker.Checker
	runtime     string
	nodeModules string
	queue       []*ast.Symbol
	modulePaths map[*ast.Symbol]string // module symbol -> module path
	reach       map[*ast.Symbol][]reached
	values      []*ast.Symbol // in the order first reached
	sigs        map[string][]effeckmapper.Signature
	aliases     map[string]string
	order       int
	walked      int
}

// enqueue schedules module for walking under path, once.
func (w *walker) enqueue(module *ast.Symbol, path string) {
	if _, ok := w.modulePaths[module]; ok {
		return
	}
	w.modulePaths[module] = path
	w.queue = append(w.queue, module)
}

func (w *walker) run() {
	for i := 0; i < len(w.queue); i++ {
		module := w.queue[i]
		path := w.modulePaths[module]
		w.walkExports(module, path, path+"#", 0)
	}
}

// walkExports walks the exports of module, a source file or a namespace,
// under keys that start with prefix.
func (w *walker) walkExports(module *ast.Symbol, path, prefix string, depth int) {
	exports := w.c.GetExportsOfModule(module)
	names := make([]string, 0, len(exports))
	byName := map[string]*ast.Symbol{}
	for _, e := range exports {
		names = append(names, e.Name())
		byName[e.Name()] = e
	}
	sort.Strings(names)
	for _, name := range names {
		if strings.HasPrefix(name, "\xfe") || strings.HasPrefix(name, "__") || name == "default" && depth > 0 {
			continue
		}
		key := prefix + name
		target := byName[name]
		if target.Flags()&ast.SymbolFlagsAlias != 0 {
			target = w.c.GetAliasedSymbol(target)
		}
		if target == nil || target.Flags()&ast.SymbolFlagsValue == 0 {
			continue
		}
		if isNamespace(target) {
			if modulePath := w.modulePathOf(target); modulePath != "" {
				// Another module, exported as a namespace: walk it under its
				// own path, and alias this name to it.
				if modulePath != path {
					w.aliases[key] = modulePath
				}
				w.enqueue(target, modulePath)
				continue
			}
			if depth < 2 {
				w.walkExports(target, path, key+".", depth+1)
			}
			continue
		}
		w.reachValue(target, path, key)
	}
}

// reachValue records that the walk reached value under key.
func (w *walker) reachValue(value *ast.Symbol, module, key string) {
	if _, seen := w.reach[value]; !seen {
		w.values = append(w.values, value)
	}
	w.order++
	w.reach[value] = append(w.reach[value], reached{module: module, key: key, order: w.order})
}

// isNamespace reports whether symbol is a module or a namespace that is not
// also a function, class, or variable.
func isNamespace(symbol *ast.Symbol) bool {
	return symbol.Flags()&(ast.SymbolFlagsValueModule|ast.SymbolFlagsNamespaceModule) != 0 &&
		symbol.Flags()&(ast.SymbolFlagsFunction|ast.SymbolFlagsVariable|ast.SymbolFlagsClass) == 0
}

// modulePathOf returns the module path that a module symbol's source file
// is published under, or "" when it is not a public module of a covered
// package: effect/Effect for .../effect/dist/Effect.d.ts, effect/http for
// .../effect/dist/http/index.d.ts, effeck-runtime for the runtime's index.ts.
func (w *walker) modulePathOf(symbol *ast.Symbol) string {
	for _, d := range symbol.Declarations() {
		if d.Kind == ast.KindSourceFile {
			return w.publicPath(string(d.AsSourceFile().FileName()))
		}
	}
	return ""
}

func (w *walker) publicPath(file string) string {
	runtime := filepath.ToSlash(w.runtime)
	switch file {
	case runtime + "/index.ts":
		return "effeck-runtime"
	case runtime + "/kernel/kernel.ts":
		return "effeck-runtime/kernel"
	}
	for _, pkg := range distPackages {
		dist := filepath.ToSlash(filepath.Join(w.nodeModules, pkg, "dist")) + "/"
		rel, ok := strings.CutPrefix(file, dist)
		if !ok || !strings.HasSuffix(rel, ".d.ts") {
			continue
		}
		rel = strings.TrimSuffix(rel, ".d.ts")
		rel = strings.TrimSuffix(strings.TrimSuffix(rel, "index"), "/")
		if strings.Contains(rel, "internal") {
			return ""
		}
		if rel == "" {
			return pkg
		}
		return pkg + "/" + rel
	}
	return ""
}

// finish picks each value's canonical key, records its signatures and its
// callable members under that key, and aliases the other keys it was
// reached under.
func (w *walker) finish() map[string][]effeckmapper.Signature {
	entries := map[string][]effeckmapper.Signature{}
	for _, value := range w.values {
		reach := w.reach[value]
		home := w.homeOf(value)
		canonical := reach[0]
		for _, r := range reach {
			if r.module == home {
				canonical = r
				break
			}
		}
		for _, r := range reach {
			if r.key != canonical.key && !w.coveredByAlias(r.key, canonical.key) {
				w.aliases[r.key] = canonical.key
			}
		}
		w.record(entries, canonical.key, w.c.GetTypeOfSymbol(value), 0)
	}
	return entries
}

// methods records the own methods of the lib's collection types and of
// each effect module's main type (the interface or class named like the
// module, as Semaphore in effect/Semaphore), keyed method:<receiver>#<name>.
// A method module's self-first functions need no entries: the resolver
// reads them from the module's own keys (Signature.Self).
func (w *walker) methods(entries map[string][]effeckmapper.Signature) {
	for _, name := range libReceivers {
		if symbol := w.c.GlobalSymbol(name, ast.SymbolFlagsType); symbol != nil {
			w.ownMethods(entries, "method:lib/"+name, w.c.GetDeclaredTypeOfSymbol(symbol))
		}
	}
	paths := make([]string, 0, len(w.modulePaths))
	byPath := map[string]*ast.Symbol{}
	for module, path := range w.modulePaths {
		paths = append(paths, path)
		byPath[path] = module
	}
	sort.Strings(paths)
	for _, path := range paths {
		if !strings.HasPrefix(path, "effect/") {
			continue
		}
		base := path[strings.LastIndex(path, "/")+1:]
		for _, e := range w.c.GetExportsOfModule(byPath[path]) {
			if e.Name() != base {
				continue
			}
			target := e
			if target.Flags()&ast.SymbolFlagsAlias != 0 {
				target = w.c.GetAliasedSymbol(target)
			}
			if target != nil && target.Flags()&(ast.SymbolFlagsInterface|ast.SymbolFlagsClass) != 0 {
				w.ownMethods(entries, "method:"+path, w.c.GetDeclaredTypeOfSymbol(target))
			}
		}
	}
}

// ownMethods records the callable members of t that carry a form.
func (w *walker) ownMethods(entries map[string][]effeckmapper.Signature, prefix string, t *checker.Type) {
	if t == nil {
		return
	}
	for _, prop := range w.c.GetPropertiesOfType(t) {
		name := prop.Name()
		if skipMembers[name] || strings.HasPrefix(name, "\xfe") || strings.HasPrefix(name, "__@") || strings.HasPrefix(name, "~") {
			continue
		}
		pt := w.c.GetTypeOfSymbol(prop)
		if pt == nil {
			continue
		}
		if sigs := w.signatures(pt); len(sigs) > 0 && carriesForm(sigs) {
			entries[prefix+"#"+name] = sigs
		}
	}
}

// homeOf returns the module path of the file that declares value.
func (w *walker) homeOf(value *ast.Symbol) string {
	for _, d := range value.Declarations() {
		if file := ast.GetSourceFileOfNode(d); file != nil {
			return w.publicPath(string(file.FileName()))
		}
	}
	return ""
}

// coveredByAlias reports whether a module alias already leads key to
// canonical, as effect#Effect leads effect#Effect.forEach to
// effect/Effect#forEach.
func (w *walker) coveredByAlias(key, canonical string) bool {
	t := effeckmapper.NewTable(effeckmapper.Header{}, nil, w.aliases)
	return t.Canonical(key) == canonical
}

var skipMembers = map[string]bool{"prototype": true, "call": true, "apply": true, "bind": true, "toString": true, "length": true, "name": true, "caller": true, "arguments": true}

// record adds t's call signatures under key when one carries a form, then
// does the same for t's callable members, two levels deep (it.effect.each).
func (w *walker) record(entries map[string][]effeckmapper.Signature, key string, t *checker.Type, depth int) {
	if t == nil {
		return
	}
	if sigs := w.signatures(t); len(sigs) > 0 {
		w.walked++
		if carriesForm(sigs) {
			entries[key] = sigs
		}
	}
	if depth >= 2 || t.Flags()&checker.TypeFlagsObject == 0 {
		return
	}
	props := w.c.GetPropertiesOfType(t)
	if len(props) > 200 {
		return
	}
	for _, prop := range props {
		name := prop.Name()
		if skipMembers[name] || strings.HasPrefix(name, "\xfe") || strings.HasPrefix(name, "__@") || strings.HasPrefix(name, "~") {
			continue
		}
		if _, exists := entries[key+"."+name]; exists {
			continue
		}
		pt := w.c.GetTypeOfSymbol(prop)
		if pt == nil || len(w.c.GetSignaturesOfType(pt, checker.SignatureKindCall)) == 0 {
			continue
		}
		w.record(entries, key+"."+name, pt, depth+1)
	}
}

func carriesForm(sigs []effeckmapper.Signature) bool {
	for _, s := range sigs {
		if s.DataFirst || s.DataLast {
			return true
		}
		for _, p := range s.Params {
			if !p.IsZero() {
				return true
			}
		}
	}
	return false
}

// noiseProps are members that every object or array type has; their
// classes say nothing about an object literal written at the position.
var noiseProps = map[string]bool{"pipe": true, "toJSON": true, "toString": true, "valueOf": true, "toLocaleString": true, "hasOwnProperty": true, "isPrototypeOf": true, "propertyIsEnumerable": true,
	"at": true, "concat": true, "copyWithin": true, "entries": true, "every": true, "fill": true, "filter": true, "find": true, "findIndex": true, "findLast": true, "findLastIndex": true, "flat": true, "flatMap": true,
	"forEach": true, "includes": true, "indexOf": true, "join": true, "keys": true, "lastIndexOf": true, "map": true, "pop": true, "push": true, "reduce": true, "reduceRight": true, "reverse": true, "shift": true,
	"slice": true, "some": true, "sort": true, "splice": true, "unshift": true, "values": true, "toReversed": true, "toSorted": true, "toSpliced": true, "with": true}

// anyProperty is a property name no type declares, so classifying it reads
// a type's string index signature, as the checker does for an object
// literal property the type does not name.
const anyProperty = "\x00any"

// signatures reads t's call signatures as the table stores them.
func (w *walker) signatures(t *checker.Type) []effeckmapper.Signature {
	c := w.c
	var out []effeckmapper.Signature
	for _, sig := range c.GetSignaturesOfType(t, checker.SignatureKindCall) {
		declared := declaredSignature(c, sig)
		n := len(declared.Parameters())
		s := effeckmapper.Signature{Min: declared.MinArgumentCount(), Max: n, Rest: declared.HasRestParameter()}
		for i := range n {
			s.Params = append(s.Params, w.param(c.GetTypeAtPosition(declared, i)))
		}
		if n > 0 {
			_, s.DataFirst = effeck.FormsOfType(c, c.GetTypeOfSymbol(declared.Parameters()[0]))
			s.Self = takesSelfFirst(declared)
		}
		for _, inner := range c.GetSignaturesOfType(c.GetReturnTypeOfSignature(declared), checker.SignatureKindCall) {
			if ps := inner.Parameters(); len(ps) > 0 {
				if _, effect := effeck.FormsOfType(c, c.GetTypeOfSymbol(ps[0])); effect {
					s.DataLast = true
				}
			}
		}
		out = append(out, s)
	}
	return out
}

// takesSelfFirst reports whether sig's first parameter is named self or
// ends in OrOptions: effect's data-first form, which D152 offers as a
// method. It mirrors takesSelfFirst in internal/effeck/datamethods.go.
func takesSelfFirst(sig *checker.Signature) bool {
	d := sig.Declaration()
	if d == nil {
		return false
	}
	params := d.Parameters()
	if len(params) == 0 || !ast.IsIdentifier(params[0].Name()) {
		return false
	}
	name := params[0].Name().Text()
	return name == "self" || strings.HasSuffix(name, "OrOptions")
}

// declaredSignature returns the uninstantiated signature behind sig, whose
// declared parameter types the fork classifies (D44).
func declaredSignature(c *checker.Checker, sig *checker.Signature) *checker.Signature {
	if d := sig.Declaration(); d != nil && !ast.IsJSDocSignature(d) && ast.IsFunctionLike(d) {
		return c.GetSignatureFromDeclaration(d)
	}
	for sig.Target() != nil {
		sig = sig.Target()
	}
	return sig
}

// param classifies one declared parameter type: the argument itself, an
// array element, and, when a program does not fit, each property and any
// other property of an object argument.
func (w *walker) param(pt *checker.Type) effeckmapper.Param {
	var p effeckmapper.Param
	if pt == nil {
		return p
	}
	c := w.c
	p.Top = class(c, pt)
	p.Elem = class(c, pt, "[]")
	if p.Top&effeckmapper.ClassProgram != 0 {
		return p
	}
	base := pt
	for base.Flags()&checker.TypeFlagsSubstitution != 0 {
		base = base.AsSubstitutionType().BaseType()
	}
	if base.Flags()&checker.TypeFlagsTypeParameter != 0 {
		if k := c.GetBaseConstraintOfType(base); k != nil {
			base = k
		}
	}
	if base.Flags()&(checker.TypeFlagsObject|checker.TypeFlagsUnion|checker.TypeFlagsIntersection) == 0 || len(c.GetSignaturesOfType(base, checker.SignatureKindCall)) != 0 {
		return p
	}
	if props := c.GetPropertiesOfType(base); len(props) <= 200 {
		for _, prop := range props {
			name := prop.Name()
			if noiseProps[name] || strings.HasPrefix(name, "\xfe") || strings.HasPrefix(name, "__@") || strings.HasPrefix(name, "~") {
				continue
			}
			if cls := class(c, pt, name); cls != 0 {
				if p.Props == nil {
					p.Props = map[string]effeckmapper.Class{}
				}
				p.Props[name] = cls
			}
		}
	}
	p.AnyProp = class(c, pt, anyProperty)
	return p
}

func class(c *checker.Checker, t *checker.Type, steps ...string) effeckmapper.Class {
	forms, _ := effeck.FormsOfType(c, t, steps...)
	cls, err := effeckmapper.ParseClass(forms)
	if err != nil {
		fail(err)
	}
	return cls
}

// prelude aliases each prelude name (D4, D48), effect root export (D146),
// and unprefixed Effect function (D151) to its canonical key or module,
// asking the fork's own name resolution in an .effeck file which names are
// in scope and what they mean.
func (w *walker) prelude(file *ast.SourceFile) {
	location := file.Statements.Nodes[len(file.Statements.Nodes)-1]
	candidates := map[string]bool{}
	for module, path := range w.modulePaths {
		if path == "effect" || path == "effect/Effect" || path == "effeck-runtime" {
			for _, e := range w.c.GetExportsOfModule(module) {
				candidates[e.Name()] = true
			}
		}
	}
	names := make([]string, 0, len(candidates))
	for name := range candidates {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		symbol := w.c.ResolveName(name, location, ast.SymbolFlagsValue, false)
		if symbol == nil {
			continue
		}
		target := symbol
		if target.Flags()&ast.SymbolFlagsAlias != 0 {
			target = w.c.GetAliasedSymbol(target)
		}
		if target == nil {
			continue
		}
		if path, ok := w.modulePaths[target]; ok {
			w.aliases["prelude#"+name] = path
			continue
		}
		reach, ok := w.reach[target]
		if !ok {
			continue // a JavaScript global, such as Array (D146)
		}
		key := reach[0].key
		home := w.homeOf(target)
		for _, r := range reach {
			if r.module == home {
				key = r.key
				break
			}
		}
		w.aliases["prelude#"+name] = key
	}
}

// pinInputs hashes every file the program loaded from a package, grouped by
// the package that holds it: the declarations the table was generated from.
func pinInputs(p *compiler.Program, runtime string) ([]effeckmapper.Input, error) {
	type pkg struct {
		name, version string
		files         map[string]string
	}
	packages := map[string]*pkg{}
	dirs := map[string]string{} // directory -> package root holding it
	var rootOf func(dir string) string
	rootOf = func(dir string) string {
		if r, ok := dirs[dir]; ok {
			return r
		}
		r := ""
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			r = dir
		} else if parent := filepath.Dir(dir); parent != dir {
			r = rootOf(parent)
		}
		dirs[dir] = r
		return r
	}
	for _, f := range p.SourceFiles() {
		name := filepath.FromSlash(string(f.FileName()))
		if strings.HasPrefix(string(f.FileName()), "bundled:") || !filepath.IsAbs(name) {
			continue
		}
		if !strings.Contains(name, string(filepath.Separator)+"node_modules"+string(filepath.Separator)) && !strings.HasPrefix(name, runtime+string(filepath.Separator)) {
			continue
		}
		root := rootOf(filepath.Dir(name))
		if root == "" {
			continue
		}
		pk := packages[root]
		if pk == nil {
			data, err := os.ReadFile(filepath.Join(root, "package.json"))
			if err != nil {
				return nil, err
			}
			var manifest struct{ Name, Version string }
			if err := json.Unmarshal(data, &manifest); err != nil {
				return nil, fmt.Errorf("%s: %w", filepath.Join(root, "package.json"), err)
			}
			pk = &pkg{name: manifest.Name, version: manifest.Version, files: map[string]string{}}
			packages[root] = pk
		}
		rel, _ := filepath.Rel(root, name)
		pk.files[filepath.ToSlash(rel)] = f.Text()
	}
	var inputs []effeckmapper.Input
	for _, pk := range packages {
		rels := make([]string, 0, len(pk.files))
		for rel := range pk.files {
			rels = append(rels, rel)
		}
		sort.Strings(rels)
		h := sha256.New()
		for _, rel := range rels {
			io.WriteString(h, rel)
			h.Write([]byte{0})
			io.WriteString(h, pk.files[rel])
			h.Write([]byte{0})
		}
		inputs = append(inputs, effeckmapper.Input{Package: pk.name, Version: pk.version, Files: len(rels), SHA256: hex.EncodeToString(h.Sum(nil))})
	}
	slices.SortFunc(inputs, func(a, b effeckmapper.Input) int {
		return cmp.Or(strings.Compare(a.Package, b.Package), strings.Compare(a.Version, b.Version), strings.Compare(a.SHA256, b.SHA256))
	})
	return inputs, nil
}
