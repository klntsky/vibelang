package effectscript

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// EffectScript diagnostics are TypeScript diagnostics whose Source is "ES",
// so tsc prints them as `error ES2001: …` and the language server publishes
// them with source "ES" and the code as a number. They are reported while the
// checker checks a file, so `esc`, `esc --noEmit`, the language server, and
// `esc --api` all see them, and @ts-ignore and @ts-expect-error do not apply
// to them.
//
// Report a diagnostic from a file check (see fileChecks) with report, add the
// lines under it with note, and hide TypeScript's own diagnostics that it
// explains with cover.

// diagnosticSource is the prefix of every ES code.
const diagnosticSource = "ES"

// A Message is the text of one EffectScript diagnostic. In Text, {0}, {1}, …
// stand for the arguments that report substitutes.
type Message struct {
	Code     int32
	Category diagnostics.Category // the default; compilerOptions.effectscript may change it
	Text     string
}

// String returns the code as the spec writes it, for example "ES2001".
func (m *Message) String() string {
	return diagnosticSource + strconv.Itoa(int(m.Code))
}

// With returns a message with m's code and category and a different text,
// for a situation that the registry's text does not fit.
func (m *Message) With(text string) *Message {
	return &Message{Code: m.Code, Category: m.Category, Text: text}
}

func message(code int32, category diagnostics.Category, text string) *Message {
	return &Message{Code: code, Category: category, Text: text}
}

const (
	errorCategory   = diagnostics.CategoryError
	warningCategory = diagnostics.CategoryWarning
)

// The registry: every ES code in Chapter 28 of the spec
// (docs/design/src/428-part4-ch28-tooling.md) with its default category and
// the headline the spec prints for it. Every code is an error except ES2004,
// which is a warning by default; a project may change the category of ES2004
// and ES4403 (design decisions D34 and D53).
var (
	// ES1001: a module's top level runs a program. {0} is the call, such as "loadConfig()".
	ES1001 = message(1001, errorCategory, "{0} runs a program, but a module's top level is not inside one.")
	// ES1002: a generator-based API such as Effect.gen, where function* declares a producer. {0} is the API.
	ES1002 = message(1002, errorCategory, "{0} expects a JavaScript generator, but in EffectScript function* declares a producer.")
	// ES1003: async, await, for await, or await using. {0} is the keyword.
	ES1003 = message(1003, errorCategory, "{0} is not part of EffectScript: a call already waits.")
	// ES1004: using at a module's top level.
	ES1004 = message(1004, errorCategory, "using needs a block that a program runs, but a module's top level is not inside one.")
	// ES1005: a runner inside code that a fiber runs. {0} is the runner, {1} the function.
	ES1005 = message(1005, errorCategory, "{0} starts a new root fiber, but {1} already runs on one.")

	// ES2001: an expected failure escapes the written contract. {0} is the failure, {1} the function, {2} what the contract allows.
	ES2001 = message(2001, errorCategory, "{0} can escape {1}, but {2}.")
	// ES2002: a requirement escapes the written contract. {0} is the function, {1} the requirement, {2} the contract's R.
	ES2002 = message(2002, errorCategory, "{0} needs {1}, but its contract says R = {2}.")
	// ES2003: a return annotation that is not a full Effect<A, E, R>. {0} is the annotation.
	ES2003 = message(2003, errorCategory, "{0} leaves contract channels unspecified. Did you mean never or inferred?")
	// ES2004: a catch that can never run.
	ES2004 = message(2004, warningCategory, "this catch can never run: nothing in its try block fails with an expected error.")
	// ES2005: a resumeWith program that needs a service or fails outside the operation's contract. {0} is what it needs.
	ES2005 = message(2005, errorCategory, "resumeWith runs this code at the perform site, inside the handled program, where the clause's providers are not in scope. It needs {0}.")
	// ES2006: a return, break, or continue that leaves a finally block. {0} is the statement's keyword.
	ES2006 = message(2006, errorCategory, "{0} leaves a finally block, which would discard a failure in flight.")
	// ES2101: a declaration without a body that has no full contract. {0} is the declaration.
	ES2101 = message(2101, errorCategory, "{0} has no body to infer from, so its contract must be written in full.")
	// ES2102: a recursive function whose A depends on its own result. {0} is the function.
	ES2102 = message(2102, errorCategory, "{0} calls itself, and its success type depends on that call, so its contract must be written.")

	// ES3001: two services share a key. {0} is the key.
	ES3001 = message(3001, errorCategory, "two services share the key {0}.")
	// ES3002: a class extends a service.
	ES3002 = message(3002, errorCategory, "a class cannot extend a service.")
	// ES3003: new on a service. {0} is the service.
	ES3003 = message(3003, errorCategory, "{0} is a service: get its implementation with {0}(), or implement it with an object literal or a class that implements it.")
	// ES3004: instanceof on a service. {0} is the service.
	ES3004 = message(3004, errorCategory, "instanceof {0} is always false: a service is a Context key, not a base class.")
	// ES3005: a generic service class.
	ES3005 = message(3005, errorCategory, "a generic abstract class cannot be a service: one Context key cannot carry type parameters.")
	// ES3006: a static member with the same name as a service member. {0} is the name.
	ES3006 = message(3006, errorCategory, "the static member {0} has the same name as a service member.")
	// ES3101: a provider looks a service up per call where the contract allows no requirements. {0} is the member, {1} the service, {2} the contract.
	ES3101 = message(3101, errorCategory, "{0} looks up {1} each time it is called, but {2} allows no call-time requirements.")
	// ES3201: no provider for a required service. {0} is the service.
	ES3201 = message(3201, errorCategory, "no provider for {0}.")
	// ES3202: two providers for one service. {0} is the service, {1} the providers.
	ES3202 = message(3202, errorCategory, "two providers for {0}: {1}. Pass one.")
	// ES3203: providers form a cycle. {0} is the cycle.
	ES3203 = message(3203, errorCategory, "providers form a cycle: {0}")
	// ES3204: a wire argument whose type does not name its outputs. {0} is the argument.
	ES3204 = message(3204, errorCategory, "{0} does not name its outputs in its type, so wire cannot place it.")

	// ES4201: a control operation inside a combinator that runs it on another fiber. {0} is the operation, {1} the combinator, {2} the operation's control.
	ES4201 = message(4201, errorCategory, "{0} is a control operation, and {1} runs it on other fibers, where the {2} handler on this fiber cannot be reached.")
	// ES4202: a TypeScript function whose R holds a control operation. {0} is the function, {1} the control.
	ES4202 = message(4202, errorCategory, "{0} is a TypeScript function whose requirements include the control operation {1}.")
	// ES4301: a callback that can pause or fail where a plain function is expected. {0} is the callback, {1} the callee.
	ES4301 = message(4301, errorCategory, "{0} can pause, but {1} calls it synchronously.")
	// ES4401: a plain TypeScript call with no boundary or trusted contract. {0} is the function.
	ES4401 = message(4401, errorCategory, "{0} is a plain TypeScript function, and nothing says what its exceptions mean.")
	// ES4402: a Node.js or browser API. {0} is the API.
	ES4402 = message(4402, errorCategory, "{0} is a platform API, which EffectScript code does not call directly.")
	// ES4403: a clock or randomness read. {0} is the read, such as "Date.now()".
	ES4403 = message(4403, errorCategory, "{0} reads the clock or randomness; use Clock, DateTime, or Random so tests control it.")
)

// registry lists every message above by code.
var registry = map[int32]*Message{}

func init() {
	for _, m := range []*Message{
		ES1001, ES1002, ES1003, ES1004, ES1005,
		ES2001, ES2002, ES2003, ES2004, ES2005, ES2006, ES2101, ES2102,
		ES3001, ES3002, ES3003, ES3004, ES3005, ES3006, ES3101, ES3201, ES3202, ES3203, ES3204,
		ES4201, ES4202, ES4301, ES4401, ES4402, ES4403,
	} {
		registry[m.Code] = m
	}
}

// adjustableCodes are the codes whose category compilerOptions.effectscript.diagnostics
// may change (D34). tsoptions validates the same list.
var adjustableCodes = []int32{2004, 4403}

// category returns the category options give code, and false when they turn
// it off.
func category(options *core.CompilerOptions, m *Message) (diagnostics.Category, bool) {
	if options != nil && options.EffectScript != nil && slices.Contains(adjustableCodes, m.Code) {
		switch options.EffectScript.Diagnostics[m.String()] {
		case "error":
			return errorCategory, true
		case "warning":
			return warningCategory, true
		case "off":
			return 0, false
		}
	}
	return m.Category, true
}

// newDiagnostic makes a diagnostic with message m at node, or returns nil when
// options turn m's code off. The span is TypeScript's error span for node,
// such as a declaration's name.
func newDiagnostic(options *core.CompilerOptions, node *ast.Node, m *Message, args ...any) *ast.Diagnostic {
	file := ast.GetSourceFileOfNode(node)
	return newDiagnosticAt(options, file, scanner.GetErrorRangeForNode(file, node), m, args...)
}

// newDiagnosticAt makes a diagnostic with message m at loc in file, or returns
// nil when options turn m's code off.
func newDiagnosticAt(options *core.CompilerOptions, file *ast.SourceFile, loc core.TextRange, m *Message, args ...any) *ast.Diagnostic {
	category, on := category(options, m)
	if !on {
		return nil
	}
	return ast.NewExternalDiagnostic(file, loc, diagnosticSource, category, m.Code, format(m.Text, args))
}

// report adds a diagnostic with message m at node to the diagnostics of c,
// which must be checking node's file, and returns it. It returns nil when the
// project turns m's code off.
func report(c *checker.Checker, node *ast.Node, m *Message, args ...any) *ast.Diagnostic {
	if d := newDiagnostic(c.CompilerOptions(), node, m, args...); d != nil {
		return c.AddDiagnostic(d)
	}
	return nil
}

// reportAt is report for a span that is not one node's error span, such as a
// keyword inside a node.
func reportAt(c *checker.Checker, file *ast.SourceFile, loc core.TextRange, m *Message, args ...any) *ast.Diagnostic {
	if d := newDiagnosticAt(c.CompilerOptions(), file, loc, m, args...); d != nil {
		return c.AddDiagnostic(d)
	}
	return nil
}

// note adds a line under d, such as a fix, and returns d. A nil d, which
// report returns for a code that is off, is ignored.
func note(d *ast.Diagnostic, text string, args ...any) *ast.Diagnostic {
	if d != nil {
		d.AddMessageChain(ast.NewExternalDiagnostic(nil, core.TextRange{}, diagnosticSource, diagnostics.CategoryMessage, 0, format(text, args)))
	}
	return d
}

// format replaces {0}, {1}, … in text with args.
func format(text string, args []any) string {
	for i, arg := range args {
		text = strings.ReplaceAll(text, "{"+strconv.Itoa(i)+"}", fmt.Sprint(arg))
	}
	return text
}

// fileChecks run once for each .es file that a checker checks, after
// TypeScript's own checks of that file. Each lane's checks register here from
// an init function in their own file.
var fileChecks []func(c *checker.Checker, file *ast.SourceFile)

func checkSourceFile(c *checker.Checker, file *ast.SourceFile) {
	if !isEffectScriptFile(file) {
		return
	}
	for _, check := range fileChecks {
		check(c, file)
	}
}

// diagnosticsState is what one checker remembers about diagnostics.
type diagnosticsState struct {
	covered map[*ast.SourceFile][]core.TextRange
}

type diagnosticsStateKey struct{}

func diagnosticsStateOf(c *checker.Checker) *diagnosticsState {
	return c.ExtensionState(diagnosticsStateKey{}, func() any {
		return &diagnosticsState{covered: map[*ast.SourceFile][]core.TextRange{}}
	}).(*diagnosticsState)
}

// cover hides TypeScript's own diagnostics inside node, because an ES
// diagnostic reported for node explains them. For example, ES1003 at an
// await covers TypeScript's complaint that the function is not async.
func cover(c *checker.Checker, node *ast.Node) {
	file := ast.GetSourceFileOfNode(node)
	state := diagnosticsStateOf(c)
	state.covered[file] = append(state.covered[file], core.NewTextRange(scanner.SkipTrivia(file.Text(), node.Pos()), node.End()))
}

// filterCoveredDiagnostics drops TypeScript diagnostics inside covered nodes.
func filterCoveredDiagnostics(c *checker.Checker, file *ast.SourceFile, diagnostics []*ast.Diagnostic) []*ast.Diagnostic {
	if !isEffectScriptFile(file) {
		return diagnostics
	}
	covered := diagnosticsStateOf(c).covered[file]
	if len(covered) == 0 {
		return diagnostics
	}
	isCovered := func(d *ast.Diagnostic) bool {
		return d.Source() == "" && slices.ContainsFunc(covered, func(r core.TextRange) bool {
			return r.Pos() <= d.Pos() && d.End() <= r.End()
		})
	}
	if !slices.ContainsFunc(diagnostics, isCovered) {
		return diagnostics
	}
	return slices.DeleteFunc(slices.Clone(diagnostics), isCovered)
}
