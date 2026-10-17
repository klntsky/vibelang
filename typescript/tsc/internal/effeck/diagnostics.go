package effeck

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

// Effeck diagnostics are TypeScript diagnostics whose Source is "EK",
// so tsc prints them as `error EK2001: …` and the language server publishes
// them with source "EK" and the code as a number. They are reported while the
// checker checks a file, so `effeck`, `effeck --noEmit`, the language server, and
// `effeck --api` all see them, and @ts-ignore and @ts-expect-error do not apply
// to them.
//
// Report a diagnostic from a file check (see fileChecks) with report, add the
// lines under it with note, and hide TypeScript's own diagnostics that it
// explains with cover.

// diagnosticSource is the prefix of every EK code.
const diagnosticSource = "EK"

// A Message is the text of one Effeck diagnostic. In Text, {0}, {1}, …
// stand for the arguments that report substitutes.
type Message struct {
	Code     int32
	Category diagnostics.Category // the default; compilerOptions.effeck may change it
	Text     string
}

// String returns the code as the spec writes it, for example "EK2001".
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

// The registry: every EK code in Chapter 28 of the spec
// (docs/design/src/428-part4-ch28-tooling.md) with its default category and
// the headline the spec prints for it. Every code is an error except EK2004,
// which is a warning by default; a project may change the category of EK2004
// and EK4403 (design decisions D34 and D53).
var (
	// EK1001: a module's top level runs a program. {0} is the call, such as "loadConfig()".
	EK1001 = message(1001, errorCategory, "{0} runs a program, but a module's top level is not inside one.")
	// EK1002: a generator-based API such as Effect.gen, where function* declares a producer. {0} is the API.
	EK1002 = message(1002, errorCategory, "{0} expects a JavaScript generator, but in Effeck function* declares a producer.")
	// EK1003: async, await, for await, or await using. {0} is the keyword.
	EK1003 = message(1003, errorCategory, "{0} is not part of Effeck: a call already waits.")
	// EK1004: using at a module's top level.
	EK1004 = message(1004, errorCategory, "using needs a block that a program runs, but a module's top level is not inside one.")
	// EK1005: a runner inside code that a fiber runs. {0} is the runner, {1} the function.
	EK1005 = message(1005, errorCategory, "{0} starts a new root fiber, but {1} already runs on one.")
	// EK1006: a pipe whose body never uses its topic.
	EK1006 = message(1006, errorCategory, "This pipe's body does not use its topic, %, so the value before |> would go nowhere.")
	// EK1007: % outside every pipe's body.
	EK1007 = message(1007, errorCategory, "% is the topic of a pipe and stands only in a pipe's body, after |>.")
	// EK1008: a pipe that passes its left side by name and also uses it as a value. {0} is the left side.
	EK1008 = message(1008, errorCategory, "% stands for {0}, which this pipe passes by name to a program elsewhere, so it cannot also be a value here.")

	// EK2001: an expected failure escapes the written contract. {0} is the failure, {1} the function, {2} what the contract allows.
	EK2001 = message(2001, errorCategory, "{0} can escape {1}, but {2}.")
	// EK2002: a requirement escapes the written contract. {0} is the function, {1} the requirement, {2} the contract's R.
	EK2002 = message(2002, errorCategory, "{0} needs {1}, but its contract says R = {2}.")
	// EK2003: a return annotation that is not a full Effect<A, E, R>. {0} is the annotation.
	EK2003 = message(2003, errorCategory, "{0} leaves contract channels unspecified. Did you mean never or inferred?")
	// EK2004: a catch that can never run.
	EK2004 = message(2004, warningCategory, "this catch can never run: nothing in its try block fails with an expected error.")
	// EK2005: a resumeWith program that needs a service or fails outside the operation's contract. {0} is what it needs.
	EK2005 = message(2005, errorCategory, "resumeWith runs this code at the perform site, inside the handled program, where the clause's providers are not in scope. It needs {0}.")
	// EK2006: a return, break, or continue that leaves a finally block. {0} is the statement's keyword.
	EK2006 = message(2006, errorCategory, "{0} leaves a finally block, which would discard a failure in flight.")
	// EK2101: a declaration without a body that has no full contract. {0} is the declaration.
	EK2101 = message(2101, errorCategory, "{0} has no body to infer from, so its contract must be written in full.")
	// EK2103: a member of a service or control declaration with a body or an initializer. {0} is the member, {1} the word.
	EK2103 = message(2103, errorCategory, "{0} has an implementation, but a {1} declares only contracts.")
	// EK2104: a setup statement of a layer declaration that follows one of its members.
	EK2104 = message(2104, errorCategory, "This statement follows the layer's members, but a layer's setup statements come first.")
	// EK2105: a typed catch clause whose type has a member without a literal _tag. {0} is the clause's type, {1} the member.
	EK2105 = message(2105, errorCategory, "catch (…: {0}) matches failures by _tag, but {1} has no literal _tag.")
	// EK2106: a catch clause after an untyped catch clause, which catches every failure.
	EK2106 = message(2106, errorCategory, "This catch clause can never run: the untyped catch clause before it catches every failure.")
	// EK2102: a recursive function whose A depends on its own result. {0} is the function.
	EK2102 = message(2102, errorCategory, "{0} calls itself, and its success type depends on that call, so its contract must be written.")

	// EK3001: two services share a key. {0} is the key.
	EK3001 = message(3001, errorCategory, "two services share the key {0}.")
	// EK3002: a class extends a service.
	EK3002 = message(3002, errorCategory, "a class cannot extend a service.")
	// EK3003: new on a service. {0} is the service.
	EK3003 = message(3003, errorCategory, "{0} is a service: get its implementation with {0}(), or implement it with an object literal or a class that implements it.")
	// EK3004: instanceof on a service. {0} is the service.
	EK3004 = message(3004, errorCategory, "instanceof {0} is always false: a service is a Context key, not a base class.")
	// EK3005: a generic service class.
	EK3005 = message(3005, errorCategory, "a generic abstract class cannot be a service: one Context key cannot carry type parameters.")
	// EK3006: a static member with the same name as a service member. {0} is the name.
	EK3006 = message(3006, errorCategory, "the static member {0} has the same name as a service member.")
	// EK3101: a provider looks a service up per call where the contract allows no requirements. {0} is the member, {1} the service, {2} the contract.
	EK3101 = message(3101, errorCategory, "{0} looks up {1} each time it is called, but {2} allows no call-time requirements.")
	// EK3201: no provider for a required service. {0} is the service.
	EK3201 = message(3201, errorCategory, "no provider for {0}.")
	// EK3202: two providers for one service. {0} is the service, {1} the providers.
	EK3202 = message(3202, errorCategory, "two providers for {0}: {1}. Pass one.")
	// EK3203: providers form a cycle. {0} is the cycle.
	EK3203 = message(3203, errorCategory, "providers form a cycle: {0}")
	// EK3204: a wire argument whose type does not name its outputs. {0} is the argument.
	EK3204 = message(3204, errorCategory, "{0} does not name its outputs in its type, so wire cannot place it.")

	// EK4201: a control operation inside a combinator that runs it on another fiber. {0} is the operation, {1} the combinator, {2} the operation's control.
	EK4201 = message(4201, errorCategory, "{0} is a control operation, and {1} runs it on other fibers, where the {2} handler on this fiber cannot be reached.")
	// EK4202: a TypeScript function whose R holds a control operation. {0} is the function, {1} the control.
	EK4202 = message(4202, errorCategory, "{0} is a TypeScript function whose requirements include the control operation {1}.")
	// EK4301: a callback that can pause or fail where a plain function is expected. {0} is the callback, {1} the callee.
	EK4301 = message(4301, errorCategory, "{0} can pause, but {1} calls it synchronously.")
	// EK4401: a plain TypeScript call with no boundary or trusted contract. {0} is the function.
	EK4401 = message(4401, errorCategory, "{0} is a plain TypeScript function, and nothing says what its exceptions mean.")
	// EK4402: a Node.js or browser API. {0} is the API.
	EK4402 = message(4402, errorCategory, "{0} is a platform API, which Effeck code does not call directly.")
	// EK4403: a clock or randomness read. {0} is the read, such as "Date.now()".
	EK4403 = message(4403, errorCategory, "{0} reads the clock or randomness; use Clock, DateTime, or Random so tests control it.")
	// EK4501: method syntax without a call. {0} is the method, {1} the module.
	EK4501 = message(4501, errorCategory, "{0} is a method of {1}'s values only in a call: x.{0}(…) calls {1}.{0}(x, …).")
	// EK4404: a hatch given Effect's options object. {0} is the hatch.
	EK4404 = message(4404, errorCategory, "{0} takes the code and the error mapping as two arguments, not Effect's options object.")
)

// registry lists every message above by code.
var registry = map[int32]*Message{}

func init() {
	for _, m := range []*Message{
		EK1001, EK1002, EK1003, EK1004, EK1005, EK1006, EK1007, EK1008,
		EK2001, EK2002, EK2003, EK2004, EK2005, EK2006, EK2101, EK2102, EK2103, EK2104, EK2105, EK2106,
		EK3001, EK3002, EK3003, EK3004, EK3005, EK3006, EK3101, EK3201, EK3202, EK3203, EK3204,
		EK4201, EK4202, EK4301, EK4401, EK4402, EK4403, EK4404, EK4501,
	} {
		registry[m.Code] = m
	}
}

// adjustableCodes are the codes whose category compilerOptions.effeck.diagnostics
// may change (D34). tsoptions validates the same list.
var adjustableCodes = []int32{2004, 4403}

// category returns the category options give code, and false when they turn
// it off.
func category(options *core.CompilerOptions, m *Message) (diagnostics.Category, bool) {
	if options != nil && options.Effeck != nil && slices.Contains(adjustableCodes, m.Code) {
		switch options.Effeck.Diagnostics[m.String()] {
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

// fileChecks run once for each .effeck file that a checker checks, after
// TypeScript's own checks of that file. Each lane's checks register here from
// an init function in their own file.
var fileChecks []func(c *checker.Checker, file *ast.SourceFile)

func checkSourceFile(c *checker.Checker, file *ast.SourceFile) {
	if !isEffeckFile(file) {
		return
	}
	for _, check := range fileChecks {
		check(c, file)
	}
}

// diagnosticsState is what one checker remembers about diagnostics.
type diagnosticsState struct {
	covered   map[*ast.SourceFile][]core.TextRange
	arguments map[*ast.SourceFile][]int // the starts of arguments whose type error an EK diagnostic explains
}

type diagnosticsStateKey struct{}

func diagnosticsStateOf(c *checker.Checker) *diagnosticsState {
	return c.ExtensionState(diagnosticsStateKey{}, func() any {
		return &diagnosticsState{covered: map[*ast.SourceFile][]core.TextRange{}, arguments: map[*ast.SourceFile][]int{}}
	}).(*diagnosticsState)
}

// cover hides TypeScript's own diagnostics inside node, because an EK
// diagnostic reported for node explains them. For example, EK1003 at an
// await covers TypeScript's complaint that the function is not async.
func cover(c *checker.Checker, node *ast.Node) {
	file := ast.GetSourceFileOfNode(node)
	state := diagnosticsStateOf(c)
	state.covered[file] = append(state.covered[file], core.NewTextRange(scanner.SkipTrivia(file.Text(), node.Pos()), node.End()))
}

// coverArgument hides TypeScript's error that argument's type does not fit
// its parameter (TS2345, or TS2769 for an overloaded callee), because an EK
// diagnostic reported for argument explains it. Unlike cover, it keeps the
// errors inside argument, such as those in a test's body.
func coverArgument(c *checker.Checker, argument *ast.Node) {
	file := ast.GetSourceFileOfNode(argument)
	state := diagnosticsStateOf(c)
	state.arguments[file] = append(state.arguments[file], scanner.SkipTrivia(file.Text(), argument.Pos()))
}

// filterCoveredDiagnostics drops TypeScript diagnostics inside covered nodes,
// and the type errors of covered arguments.
func filterCoveredDiagnostics(c *checker.Checker, file *ast.SourceFile, diagnostics []*ast.Diagnostic) []*ast.Diagnostic {
	if !isEffeckFile(file) {
		return diagnostics
	}
	state := diagnosticsStateOf(c)
	covered, arguments := state.covered[file], state.arguments[file]
	if len(covered) == 0 && len(arguments) == 0 {
		return diagnostics
	}
	isCovered := func(d *ast.Diagnostic) bool {
		if d.Source() != "" {
			return false
		}
		if (d.Code() == 2345 || d.Code() == 2769) && slices.Contains(arguments, d.Pos()) {
			return true
		}
		return slices.ContainsFunc(covered, func(r core.TextRange) bool {
			return r.Pos() <= d.Pos() && d.End() <= r.End()
		})
	}
	if !slices.ContainsFunc(diagnostics, isCovered) {
		return diagnostics
	}
	return slices.DeleteFunc(slices.Clone(diagnostics), isCovered)
}
