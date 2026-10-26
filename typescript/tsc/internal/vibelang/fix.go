package vibelang

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
	"github.com/microsoft/TypeScript/tsc/internal/execute/tsc"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/ls/autoimport"
	"github.com/microsoft/TypeScript/tsc/internal/ls/lsconv"
	"github.com/microsoft/TypeScript/tsc/internal/ls/lsutil"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/sourcemap"
	"github.com/microsoft/TypeScript/tsc/internal/tsoptions"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"github.com/microsoft/TypeScript/tsc/internal/vfs"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/wrapvfs"
)

// vibelang --fix (D291) applies every safe quick fix to a project's .vibe
// files, then compiles as the rest of the command line says:
//
//	vibelang -p tsconfig.json --fix --noEmit
//
// A safe fix is the quick fix of a diagnostic in tsFormFixCodes that the
// project reports, so a code turned off in the vibelang setting diagnostics,
// or silenced by an @vibelang-diagnostics comment, is not fixed. Fixes apply
// in rounds, through the language server's own code actions: each round
// takes, in each file, the fixes whose edits do not overlap, and a fix that
// overlaps one taken waits for the next round, which rechecks the edited
// program. A round's fixes are kept only when the edited program reports no
// error the original did not, compared by file, code, and message; when the
// round as a whole would add one, each file's fixes are tried alone, then
// one at a time, and each fix that adds an error is refused, named with the
// first error it adds, and not tried again. After the last round, an import
// of effect or one of its modules that the fixes left unused is removed.
// Nothing is written until every round is done, and a second run finds
// nothing to fix. --fix cannot be combined with --build or --watch.

// FixFlag is the command line flag that applies the quick fixes.
const FixFlag = "--fix"

// FixArguments returns args without --fix, and whether it held one.
func FixArguments(args []string) ([]string, bool) {
	index := slices.IndexFunc(args, func(arg string) bool { return strings.EqualFold(arg, FixFlag) })
	if index < 0 {
		return args, false
	}
	return slices.Delete(slices.Clone(args), index, index+1), true
}

// Fix applies the safe quick fixes to the project that args, a command line
// without --fix, names, writes the files they change, and reports them on
// sys's writer. It returns a nonzero exit status only when it cannot run;
// diagnostics are the compilation's to report.
func Fix(ctx context.Context, sys tsc.System, args []string) int {
	for _, arg := range args {
		switch strings.ToLower(arg) {
		case "-b", "--b", "-build", "--build", "-w", "--w", "-watch", "--watch":
			fmt.Fprintf(sys.ErrorWriter(), "vibelang --fix: %s cannot be combined with --fix; run vibelang -p <tsconfig> --fix for each project.\n", arg)
			return int(tsc.ExitStatusInvalidProject_OutputsSkipped)
		}
	}
	commandLine := tsoptions.ParseCommandLine(args, sys.FS(), sys.GetCurrentDirectory())
	if len(commandLine.Errors) > 0 {
		return 0 // the compilation reports them
	}
	configFileName := fixConfigFile(sys, commandLine)
	if configFileName == "" {
		fmt.Fprintln(sys.ErrorWriter(), "vibelang --fix: no tsconfig.json: name a project with -p.")
		return int(tsc.ExitStatusInvalidProject_OutputsSkipped)
	}
	path := sys.FS().CaseSensitivity().PathKey(tspath.RootedPath(configFileName))
	config, errors := tsoptions.GetParsedCommandLineOfConfigFilePath(configFileName, path, commandLine.CompilerOptions(), nil, sys.FS(), &tsc.ExtendedConfigCache{})
	if len(errors) > 0 || config == nil {
		return 0 // the compilation reports them
	}
	result := FixProject(ctx, sys.FS(), sys.DefaultLibraryPath(), config)
	for _, name := range result.Order {
		file := result.Files[name]
		if err := os.WriteFile(name.AsString(), []byte(file.Text), 0o644); err != nil {
			fmt.Fprintf(sys.ErrorWriter(), "vibelang --fix: %s: %v\n", relativeName(sys, name), err)
			return int(tsc.ExitStatusInvalidProject_OutputsSkipped)
		}
	}
	result.Report(sys.Writer(), func(name tspath.RootedFilePath) string { return relativeName(sys, name) })
	return 0
}

// fixConfigFile returns the tsconfig.json that commandLine names with -p, or
// the nearest one above the current directory, as the compilation finds it.
func fixConfigFile(sys tsc.System, commandLine *tsoptions.ParsedCommandLine) tspath.RootedFilePath {
	if project := commandLine.CompilerOptions().Project; project != "" {
		directory := tspath.RootedDirectoryPathFromPath(project)
		if sys.FS().DirectoryExists(directory) {
			return directory.ResolveFile("tsconfig.json")
		}
		return tspath.RootedFilePathFromPath(project)
	}
	found, ok := sys.GetCurrentDirectory().ForEachAncestorDirectory(func(ancestor tspath.RootedDirectoryPath) (tspath.RootedFilePath, bool) {
		name := ancestor.ResolveFile("tsconfig.json")
		return name, sys.FS().FileExists(name)
	})
	if !ok {
		return ""
	}
	return found
}

// relativeName returns name relative to sys's current directory.
func relativeName(sys tsc.System, name tspath.RootedFilePath) string {
	return tspath.ConvertToRelativePath(name.AsString(), sys.GetCurrentDirectory(), sys.FS().CaseSensitivity())
}

// A FixedFile is a file that the fixes change.
type FixedFile struct {
	Original string
	Text     string
	Counts   map[int32]int // fixes applied, by code
}

// A FixRefusal is a fix that would add an error.
type FixRefusal struct {
	File  tspath.RootedFilePath
	Line  int // 1-based, in the text the fix was tried on
	Code  int32
	Error string // the first error the fix would add
}

// FixResult is what FixProject did.
type FixResult struct {
	Files   map[tspath.RootedFilePath]*FixedFile
	Order   []tspath.RootedFilePath
	Refused []FixRefusal
}

// Report writes one line per fixed file and per refusal, and a summary.
func (r *FixResult) Report(w io.Writer, name func(tspath.RootedFilePath) string) {
	total := 0
	for _, file := range r.Order {
		fixed := r.Files[file]
		var codes []int32
		count := 0
		for code, n := range fixed.Counts {
			codes = append(codes, code)
			count += n
		}
		slices.Sort(codes)
		var parts []string
		for _, code := range codes {
			parts = append(parts, diagnosticSource+strconv.Itoa(int(code))+" ×"+strconv.Itoa(fixed.Counts[code]))
		}
		total += count
		fmt.Fprintf(w, "vibelang --fix: %s: %d %s (%s)\n", name(file), count, plural(count, "fix", "fixes"), strings.Join(parts, ", "))
	}
	for _, refusal := range r.Refused {
		fmt.Fprintf(w, "vibelang --fix: %s:%d: EK%d not fixed: the fixed program would report %s\n", name(refusal.File), refusal.Line, refusal.Code, refusal.Error)
	}
	fmt.Fprintf(w, "vibelang --fix: %d %s in %d %s", total, plural(total, "fix", "fixes"), len(r.Order), plural(len(r.Order), "file", "files"))
	if len(r.Refused) > 0 {
		fmt.Fprintf(w, "; %d %s refused", len(r.Refused), plural(len(r.Refused), "fix", "fixes"))
	}
	fmt.Fprintln(w, ".")
}

func plural(n int, one, many string) string {
	return core.IfElse(n == 1, one, many)
}

// fixRounds bounds the rounds of FixProject; each round applies at least one
// fix, and a fix never draws another of the same construct.
const fixRounds = 16

// FixProject applies the safe quick fixes to config's .vibe files and
// returns the files they change, without writing them.
func FixProject(ctx context.Context, fs vfs.FS, libraryPath tspath.RootedDirectoryPath, config *tsoptions.ParsedCommandLine) *FixResult {
	result := &FixResult{Files: map[tspath.RootedFilePath]*FixedFile{}}
	texts := map[tspath.RootedFilePath]string{}
	build := func(overlay map[tspath.RootedFilePath]string) *compiler.Program {
		return fixProgram(fs, libraryPath, config, overlay)
	}
	program := build(texts)
	g := &fixGate{ctx: ctx, build: build, baseline: errorCounts(ctx, program), refused: map[string]bool{}}
	for range fixRounds {
		var proposals []proposal
		for _, file := range fixableFiles(program) {
			if p := fileFixes(ctx, program, file, g.refused); len(p.fixes) > 0 {
				proposals = append(proposals, p)
			}
		}
		if len(proposals) == 0 {
			break
		}
		accepted := g.accept(texts, proposals, func(refusal FixRefusal, key string) {
			g.refused[key] = true
			result.Refused = append(result.Refused, refusal)
		})
		if len(accepted) == 0 {
			continue // every fix refused; the next round tries the rest
		}
		for _, p := range accepted {
			name := p.file.FileName()
			fixed := result.Files[name]
			if fixed == nil {
				original, _ := fs.ReadFile(name)
				fixed = &FixedFile{Original: original, Counts: map[int32]int{}}
				result.Files[name] = fixed
				result.Order = append(result.Order, name)
			}
			fixed.Text = p.text()
			for _, f := range p.fixes {
				fixed.Counts[f.code]++
			}
			texts[name] = fixed.Text
		}
		program = build(texts)
	}
	// Remove the effect imports that the fixes left unused.
	cleaned := map[tspath.RootedFilePath]string{}
	for _, name := range result.Order {
		if text, ok := withoutUnusedEffectImports(name, result.Files[name].Original, texts[name]); ok {
			cleaned[name] = text
		}
	}
	if len(cleaned) > 0 {
		if _, bad := newError(ctx, build(maps(texts, cleaned)), g.baseline); !bad {
			for name, text := range cleaned {
				result.Files[name].Text = text
			}
		}
	}
	slices.SortFunc(result.Order, tspath.RootedFilePath.Compare)
	return result
}

// maps returns base with additions written over it.
func maps(base, additions map[tspath.RootedFilePath]string) map[tspath.RootedFilePath]string {
	merged := make(map[tspath.RootedFilePath]string, len(base)+len(additions))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range additions {
		merged[k] = v
	}
	return merged
}

// fixProgram builds config's program with the texts of overlay in place of
// the files on disk.
func fixProgram(fs vfs.FS, libraryPath tspath.RootedDirectoryPath, config *tsoptions.ParsedCommandLine, overlay map[tspath.RootedFilePath]string) *compiler.Program {
	overlaid := wrapvfs.Wrap(fs, wrapvfs.Replacements{
		ReadFile: func(name tspath.RootedFilePath) (string, bool) {
			if text, ok := overlay[name]; ok {
				return text, true
			}
			return fs.ReadFile(name)
		},
	})
	return compiler.NewProgram(compiler.ProgramOptions{Config: config, Host: compiler.NewCompilerHost(overlaid, libraryPath, nil, nil, nil)})
}

// fixableFiles returns the .vibe files of program's own project, the ones
// tsc would emit for it: not a file that a package import reached, as a
// workspace dependency's source linked into node_modules is, nor a
// referenced project's source. Their diagnostics are still the
// compilation's to report.
func fixableFiles(program *compiler.Program) []*ast.SourceFile {
	var files []*ast.SourceFile
	for _, file := range program.SourceFiles() {
		if isVibeLangFile(file) && !strings.Contains(file.FileName().AsString(), "/node_modules/") &&
			!program.IsSourceFileFromExternalLibrary(file) && program.GetProjectReferenceFromSource(file.PathKey()) == nil {
			files = append(files, file)
		}
	}
	return files
}

// errorCounts counts program's errors by file, code, and message.
func errorCounts(ctx context.Context, program *compiler.Program) map[string]int {
	counts := map[string]int{}
	for _, d := range compiler.GetDiagnosticsOfAnyProgram(ctx, program, nil, false, program.GetBindDiagnostics, program.GetSemanticDiagnostics) {
		if d.Category() == diagnostics.CategoryError {
			counts[errorKey(d)]++
		}
	}
	return counts
}

// errorKey names d by its file, code, and message, which edits elsewhere in
// the file leave as they were.
func errorKey(d *ast.Diagnostic) string {
	file := ""
	if d.File() != nil {
		file = d.File().FileName().AsString()
	}
	source := d.Source()
	if source == "" {
		source = "TS"
	}
	return file + ": " + source + strconv.Itoa(int(d.Code())) + ": " + d.String()
}

// newError returns an error program reports that baseline does not hold, as
// the line a refusal names, and true; or false.
func newError(ctx context.Context, program *compiler.Program, baseline map[string]int) (string, bool) {
	counts := errorCounts(ctx, program)
	var added []string
	for key, n := range counts {
		if n > baseline[key] {
			added = append(added, key)
		}
	}
	if len(added) == 0 {
		return "", false
	}
	slices.Sort(added)
	return added[0], true
}

// fixGate keeps the fixes whose edited program adds no error to baseline.
type fixGate struct {
	ctx      context.Context
	build    func(map[tspath.RootedFilePath]string) *compiler.Program
	baseline map[string]int
	refused  map[string]bool // fixes refused, by fixKey
}

// passes reports whether the program with overlay over texts adds no error,
// and returns the first error it adds.
func (g *fixGate) passes(texts, overlay map[tspath.RootedFilePath]string) (string, bool) {
	error, bad := newError(g.ctx, g.build(maps(texts, overlay)), g.baseline)
	return error, !bad
}

// accept returns proposals cut down to the fixes that, together, add no
// error. It tries the round as a whole, then each file's fixes together,
// then each of a failing file's fixes alone, and calls refuse for each fix
// that adds an error.
func (g *fixGate) accept(texts map[tspath.RootedFilePath]string, proposals []proposal, refuse func(FixRefusal, string)) []proposal {
	all := map[tspath.RootedFilePath]string{}
	for _, p := range proposals {
		all[p.file.FileName()] = p.text()
	}
	if _, ok := g.passes(texts, all); ok {
		return proposals
	}
	var accepted []proposal
	kept := map[tspath.RootedFilePath]string{}
	for _, p := range proposals {
		name := p.file.FileName()
		if _, ok := g.passes(texts, maps(kept, map[tspath.RootedFilePath]string{name: p.text()})); ok {
			kept[name] = p.text()
			accepted = append(accepted, p)
			continue
		}
		var fixes []fileFix
		for _, f := range p.fixes {
			trial := proposal{file: p.file, fixes: append(slices.Clone(fixes), f)}
			if error, ok := g.passes(texts, maps(kept, map[tspath.RootedFilePath]string{name: trial.text()})); !ok {
				line, _ := scannerLineOf(p.file, firstEdit(f.edits))
				refuse(FixRefusal{File: name, Line: line + 1, Code: f.code, Error: error}, f.key)
				continue
			}
			fixes = append(fixes, f)
		}
		if len(fixes) > 0 {
			trimmed := proposal{file: p.file, fixes: fixes}
			kept[name] = trimmed.text()
			accepted = append(accepted, trimmed)
		}
	}
	return accepted
}

// scannerLineOf returns the 0-based line and column of pos in file.
func scannerLineOf(file *ast.SourceFile, pos int) (int, int) {
	text := file.Text()[:pos]
	return strings.Count(text, "\n"), len(text) - strings.LastIndex(text, "\n") - 1
}

// A fileFix is one diagnostic's quick fix: its code, its edits, and the key
// that names it across rounds, its code and the text it replaces.
type fileFix struct {
	code  int32
	edits []sourceEdit
	key   string
}

// A proposal is one round's fixes of a file, whose edits do not overlap.
type proposal struct {
	file  *ast.SourceFile
	fixes []fileFix
}

// text returns the proposal's file with its fixes applied.
func (p proposal) text() string {
	var edits []sourceEdit
	for _, f := range p.fixes {
		edits = append(edits, f.edits...)
	}
	return applyEdits(p.file.Text(), edits)
}

// fileFixes asks the language server for the quick fix of each safe
// diagnostic of file and applies the fixes whose edits do not overlap.
func fileFixes(ctx context.Context, program *compiler.Program, file *ast.SourceFile, refused map[string]bool) proposal {
	var fixable []*ast.Diagnostic
	for _, d := range program.GetSemanticDiagnostics(ctx, file) {
		if d.Source() == diagnosticSource && slices.Contains(tsFormFixCodes, d.Code()) && d.File() == file {
			fixable = append(fixable, d)
		}
	}
	if len(fixable) == 0 {
		return proposal{file: file}
	}
	converters := lsconv.NewConverters(lsproto.PositionEncodingKindUTF16, func(name tspath.RootedFilePath) *lsconv.LSPLineMap {
		if f := program.GetSourceFile(name); f != nil {
			return lsconv.ComputeLSPLineStarts(f.Text())
		}
		return lsconv.ComputeLSPLineStarts("")
	})
	service := ls.NewLanguageService(nil, program, fixHost{program: program, converters: converters}, file.FileName().AsString())
	uri := lsconv.FileNameToDocumentURI(file.FileName())
	only := []lsproto.CodeActionKind{lsproto.CodeActionKindQuickFix}
	var candidates []fileFix
	for _, d := range fixable {
		key := diagnosticSource + strconv.Itoa(int(d.Code())) + ":" + file.Text()[d.Pos():d.End()]
		if refused[key] {
			continue
		}
		diagnostic := lsconv.DiagnosticToLSPPush(ctx, converters, d)
		response, err := service.ProvideCodeActions(ctx, &lsproto.CodeActionParams{
			TextDocument: lsproto.TextDocumentIdentifier{Uri: uri},
			Range:        diagnostic.Range,
			Context:      &lsproto.CodeActionContext{Diagnostics: []*lsproto.Diagnostic{diagnostic}, Only: &only},
		})
		if err != nil || response.CommandOrCodeActionArray == nil {
			continue
		}
		for _, action := range *response.CommandOrCodeActionArray {
			if action.CodeAction == nil || action.CodeAction.Edit == nil || action.CodeAction.Edit.Changes == nil {
				continue
			}
			changes := *action.CodeAction.Edit.Changes
			if len(changes) != 1 || changes[uri] == nil {
				continue // a fix that edits another file is not one --fix applies
			}
			var edits []sourceEdit
			for _, edit := range changes[uri] {
				span := converters.FromLSPRangeToOriginal(file, edit.Range)
				edits = append(edits, sourceEdit{span.Pos(), span.End(), edit.NewText})
			}
			candidates = append(candidates, fileFix{d.Code(), edits, key})
			break
		}
	}
	// Take the fixes in source order whose edits overlap no fix taken.
	slices.SortStableFunc(candidates, func(a, b fileFix) int { return firstEdit(a.edits) - firstEdit(b.edits) })
	var taken []sourceEdit
	p := proposal{file: file}
	for _, candidate := range candidates {
		if slices.ContainsFunc(candidate.edits, func(e sourceEdit) bool { return overlapsAny(e, taken) }) {
			continue
		}
		taken = append(taken, candidate.edits...)
		p.fixes = append(p.fixes, candidate)
	}
	return p
}

// firstEdit returns the earliest start of edits.
func firstEdit(edits []sourceEdit) int {
	first := -1
	for _, e := range edits {
		if first < 0 || e.start < first {
			first = e.start
		}
	}
	return first
}

// overlapsAny reports whether e overlaps an edit of taken. Two insertions at
// one place overlap too, since their order would be ambiguous.
func overlapsAny(e sourceEdit, taken []sourceEdit) bool {
	for _, t := range taken {
		if e.start < t.end && t.start < e.end || e.start == t.start || e.start == e.end && t.start <= e.start && e.start <= t.end || t.start == t.end && e.start <= t.start && t.start <= e.end {
			return true
		}
	}
	return false
}

// applyEdits applies edits, which do not overlap, to text.
func applyEdits(text string, edits []sourceEdit) string {
	edits = slices.Clone(edits)
	slices.SortFunc(edits, func(a, b sourceEdit) int { return b.start - a.start })
	for _, e := range edits {
		text = text[:e.start] + e.text + text[e.end:]
	}
	return text
}

// withoutUnusedEffectImports removes from text, a file the fixes changed
// from original, each import binding of effect or one of its modules that
// original's code used and text's does not.
func withoutUnusedEffectImports(name tspath.RootedFilePath, original, text string) (string, bool) {
	parse := func(source string) *ast.SourceFile {
		return parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: name, PathKey: tspath.CaseSensitive.PathKey(tspath.RootedPath(name))}, source, core.ScriptKindTS)
	}
	before, after := parse(original), parse(text)
	names := map[string]bool{}
	for _, b := range importBindingsOf(after) {
		if module := b.declaration.ModuleSpecifier(); module != nil && (module.Text() == "effect" || strings.HasPrefix(module.Text(), "effect/")) {
			names[b.name] = true
		}
	}
	if len(names) == 0 {
		return "", false
	}
	_, used := identifierUses(before, names, nil)
	_, still := identifierUses(after, names, nil)
	gone := map[string]int{}
	for name := range names {
		if used[name] > 0 && still[name] == 0 {
			gone[name] = used[name]
		}
	}
	if len(gone) == 0 {
		return "", false
	}
	edits := unusedImportEdits(after, nil, gone)
	if len(edits) == 0 {
		return "", false
	}
	return applyEdits(text, edits), true
}

// fixHost is the language service host of a fix: a program on disk, with no
// editor.
type fixHost struct {
	program    *compiler.Program
	converters *lsconv.Converters
}

func (h fixHost) CaseSensitivity() tspath.CaseSensitivity { return tspath.CaseSensitive }
func (h fixHost) ReadFile(path tspath.RootedFilePath) (string, bool) {
	if file := h.program.GetSourceFile(path); file != nil {
		return file.Text(), true
	}
	return "", false
}
func (h fixHost) Converters() *lsconv.Converters                                { return h.converters }
func (h fixHost) GetPreferences(string) lsutil.UserPreferences                  { return lsutil.UserPreferences{} }
func (h fixHost) GetECMALineInfo(tspath.RootedFilePath) *sourcemap.ECMALineInfo { return nil }
func (h fixHost) AutoImportRegistry() *autoimport.Registry                      { return nil }
func (h fixHost) ReadDirectory(tspath.RootedDirectoryPath, []string, []string, []string, int) []tspath.RootedFilePath {
	return nil
}
func (h fixHost) GetDirectories(tspath.RootedDirectoryPath) []string { return nil }
func (h fixHost) DirectoryExists(tspath.RootedDirectoryPath) bool    { return false }
func (h fixHost) FileExists(tspath.RootedFilePath) bool              { return false }
