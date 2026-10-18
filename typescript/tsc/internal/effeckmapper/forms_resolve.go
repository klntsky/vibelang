package effeckmapper

// The forms resolver: the per-node answers the projector needs from the
// forms table, read from one file's syntax alone. No checker is involved,
// so the answers depend only on the file's text and the table.
//
// For a lambda, a block, or an argument expression, the resolver finds the
// owner call and the path from its arguments down to the node (an argument,
// then an array element or an object property), names the callee from
// scope, picks a signature, and reads the class at the path:
//
//  1. Callee (Resolver.Callee). The root identifier of f(…), A.f(…), or
//     A.b.f(…) is resolved lexically: a local binding shadows an import,
//     and an import shadows a prelude name. An import from a package gives
//     the key spec#imported.chain, through an alias (import { Effect as E })
//     too; a name no scope binds is a prelude name, prelude#name.chain. The
//     table's aliases lead both to the canonical key. A function or const
//     lambda declared in this file gives the signature its written
//     annotations describe, and a member of a service this file declares
//     gives the member's written signature. A member of a value (xs.map), a
//     call result, a parameter, and an import from a relative module have
//     no key.
//  2. Signature (formsShapeFit). Among the candidates that take the call's
//     argument count, most required arguments first, the first one the
//     written arguments do not rule out: a lambda where no form fits, or a
//     literal where only a function fits. When the candidates left
//     disagree at the node's path, the decision says Ambiguous. When the
//     arguments rule out every candidate, the candidates answer only where
//     they all agree; where they disagree, the decision is Ambiguous and
//     takes the fallback, with a Choice where its body pauses (rule 8).
//     Overload order never decides.
//  3. Lambda form (formsFormFor). The class picks the form. Where the call
//     gives no class, the lambda's own return annotation does: an Effect or
//     a contract (A!E) makes it an Effect function, any other type plain.
//     Where both a plain function and an Effect form fit (D39), or nothing
//     is known, the body decides: a call outside tail position, a throw, an
//     Effeck statement, or a using declaration make it an Effect function.
//     A parameterless lambda takes the program or Effect function form its
//     class names, and the dual form (both at once) where its class accepts
//     both, or no class is known and its body calls anything.
//  4. Blocks (Resolver.Block). D188's order: a data-first dual takes the
//     block first, a data-last dual is applied to it, and any other callee
//     takes it appended. A moved block is a program.
//  5. By name (Resolver.Argument). An argument whose text calls something
//     outside the functions nested in it, at a position whose class accepts
//     a program, is passed by name (D147). Elsewhere a call runs.
//  6. Methods. A call of a value's member, x.name(…), takes its forms
//     from the method table when scope names the receiver's module: a
//     written type (s: Stream.Stream<A>, xs: number[]), an array literal,
//     new Map(…), Array.from(…), a call whose result the table records
//     (Stream.make(…), and inside a function body, where an Effect call
//     runs, the Effect's success type: Semaphore.make(1) is a Semaphore), a
//     method of a named receiver (xs.filter(…)), or a value export
//     (Stream.never). The receiver's own members come first, then the
//     self-first functions D152 makes methods of its module's data type. A
//     receiver scope cannot name gives no class, so a pausing callback
//     there needs a choice (rule 8). A block after a method of a named
//     receiver goes where the method puts it, as D188 orders.
//  7. Service context. A lambda or method that is a member of an object
//     literal providing a service (Layer.succeed(K, {…}), K.of({…}), the
//     object a Layer.effect program returns, Context.make, Context.add,
//     Effect.provideService) takes the member's written signature when this
//     file declares K (a service, a control, a D1 abstract class, or a
//     class K extends Context.Service<K, Shape>()), and the Effect function
//     form when K is imported from
//     another .effeck module, since every member of an Effeck service is an
//     Effect contract (D92, D128). A key from a package, such as effect's
//     Random.Random, may have plain members, so its members take the
//     defaults. This applies only where the call gives no class.
//  8. Needs a choice. A fallback decision whose position nothing names (a
//     method of an unnamed receiver, a function from another file, a lambda
//     no call takes, a parameter the table or this file leaves unclassified)
//     carries a Choice when the lambda is not invoked where it stands, has
//     no return annotation, and its body pauses: the defaults would pick an
//     Effect function that a host could silently ignore, so the user writes
//     the form. Decision.Choice; CodeFormChoice.
//
// Every decision records its provenance: the table, a written annotation in
// this file, a service context, or the fallback defaults.

import (
	"strconv"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// Form is the form a lambda or block compiles to.
type Form uint8

const (
	FormNone Form = iota
	// FormPlain is a plain JavaScript function.
	FormPlain
	// FormEffectFn is a function that returns an Effect.
	FormEffectFn
	// FormProgram is an Effect value: the lambda's body as a program.
	FormProgram
	// FormDual is a parameterless lambda that is both a program and a
	// function that returns it (research/typedirected/stock/perf/dual.mjs).
	FormDual
)

func (f Form) String() string {
	return [...]string{"none", "plain", "effectFn", "program", "dual"}[f]
}

// Provenance says where a decision's class came from.
type Provenance uint8

const (
	// FromFallback is the L1 defaults: no key, or no class at the path.
	FromFallback Provenance = iota
	// FromTable is the generated forms table.
	FromTable
	// FromAnnotation is a written annotation of a declaration in this file.
	FromAnnotation
	// FromService is a service context: the member's written signature
	// when this file declares the service, else the Effect function form.
	FromService
)

func (p Provenance) String() string {
	return [...]string{"fallback", "table", "annotation", "service"}[p]
}

// Placement is where a block param call passes its block (D188).
type Placement uint8

const (
	// BlockAppended passes it last: the callee's own signature, f(args, block).
	BlockAppended Placement = iota
	// BlockPrepended passes it first: a data-first dual, f(block, args).
	BlockPrepended
	// BlockApplied applies the call to it: a data-last dual, f(args)(block).
	BlockApplied
)

func (p Placement) String() string {
	return [...]string{"appended", "prepended", "applied"}[p]
}

// PathStep is one step below a call argument: an array element or an
// object property.
type PathStep struct {
	Element  bool
	Property string
}

// CalleeKind says how scope names a callee.
type CalleeKind uint8

const (
	// CalleeNone has no key: a member of a value, a call result, a
	// parameter, or a local without a written signature.
	CalleeNone CalleeKind = iota
	// CalleeTable is a package import or a prelude name; Key is a table key.
	CalleeTable
	// CalleeLocal is a function or const lambda this file declares;
	// Signatures come from its written annotations.
	CalleeLocal
	// CalleeService is a member of a service this file declares;
	// Signatures come from the member's written annotations.
	CalleeService
	// CalleeOtherFile is an import from a relative module, which a per-file
	// projection cannot read.
	CalleeOtherFile
)

func (k CalleeKind) String() string {
	return [...]string{"none", "table", "local", "service", "otherfile"}[k]
}

// Callee is a call's callee as scope names it.
type Callee struct {
	Kind CalleeKind
	// Key is the key as written, such as prelude#forEach or
	// effect#Effect.all, for CalleeTable; Canonical is where the table's
	// aliases lead it.
	Key, Canonical string
	// Signatures are the written signatures of a CalleeLocal or
	// CalleeService callee.
	Signatures []Signature
}

// Decision is the resolver's answer for one node.
type Decision struct {
	// Node is the lambda, method, block, or argument expression decided.
	Node *ast.Node
	// Call is Node's owner call, or nil. Argument and Steps are the path
	// from Call's arguments down to Node.
	Call     *ast.Node
	Argument int
	Steps    []PathStep
	Callee   Callee
	// Class is what the chosen signature accepts at the path.
	Class Class
	// Form is a lambda's or a block's form.
	Form Form
	// ByName reports whether an argument expression is passed by name.
	ByName bool
	// Block reports whether Node is a block param; Placement is where it goes.
	Block      bool
	Placement  Placement
	Provenance Provenance
	// Ambiguous reports that candidates the arguments did not rule out
	// disagree at the path; the first, ranked by required arguments, won.
	Ambiguous bool
	// Note names the rule or the gap behind the decision.
	Note string
	// Choice is set when the rules cannot decide the form and the user must
	// (gate 2: no position may produce a silent wrong result). Form then
	// holds the defaults' guess, for error recovery only; the projector
	// reports Choice.Message() as CodeFormChoice.
	Choice *Choice
}

// CodeFormChoice is the diagnostic code, under the source EK, for a lambda
// or block whose form the user must write.
const CodeFormChoice = 9100

// Choice describes a lambda or block whose form the rules cannot decide:
// its body pauses (a call outside tail position, a throw, a using, or an
// Effeck statement), so the defaults would make it an Effect function, but
// nothing names the position's form: a method of a receiver that syntax
// cannot name, a function from another file, a lambda no call takes, or a
// parameter the table or this file leaves unclassified.
type Choice struct {
	// Callee is the callee as written (xs.map, route), or "" for a lambda
	// that no call takes.
	Callee string
	// Path says where the lambda is: "argument 1", "argument 2, property
	// orElse", "argument 1, element", or, for a lambda no call takes,
	// "assigned to hooks.onResume", "the value of sameLabel", and so on.
	Path string
	// Forms are the candidate forms: plain and Effect function, or plain and
	// dual for a parameterless lambda.
	Forms []Form
	// Plain and Effect are the repairs, written with the lambda's own
	// parameters: "(fiber): T =>" and "(fiber): T!! =>".
	Plain, Effect string
	// Reason says why the position has no form.
	Reason string
}

// Message is the diagnostic text, stable for tests and documentation:
//
//	Cannot tell whether the lambda at argument 1 of xs.map is a plain
//	function or an Effect function: the receiver of map is a value whose
//	type this file does not name. Write `(x): T =>` for a plain function or
//	`(x): T!! =>` for an Effect function.
func (c *Choice) Message() string {
	where := "this lambda"
	switch {
	case c.Callee != "":
		where = "the lambda at " + c.Path + " of " + c.Callee
	case c.Path != "":
		where = "the lambda " + c.Path
	}
	return "Cannot tell whether " + where + " is a plain function or an Effect function: " + c.Reason +
		". Write `" + c.Plain + "` for a plain function or `" + c.Effect + "` for an Effect function."
}

// Resolver answers form decisions for one parsed .effeck file.
type Resolver struct {
	file  *ast.SourceFile
	text  string
	table *Table
}

// NewResolver returns a resolver for file, a parse of an .effeck file, over
// table.
func NewResolver(file *ast.SourceFile, table *Table) *Resolver {
	return &Resolver{file: file, text: file.Text(), table: table}
}

// formsProviderCallees are the canonical keys of calls whose object literal
// argument, or whose program's returned object literal, implements a
// service, with the index of the argument that names the service's key.
var formsProviderCallees = map[string]int{
	"effect/Layer#succeed":               0,
	"effect/Layer#sync":                  0,
	"effect/Layer#effect":                0,
	"effect/Context#make":                0,
	"effect/Context#add":                 1,
	"effect/Effect#provideService":       1, // data-first; providerKeyArgument handles the data-last form
	"effect/Effect#provideServiceEffect": 1,
	// Not effeck-runtime#$provideService, a provide block's Key = value
	// (D173): the fork types that value through
	// NoInfer<Context.Service.Shape<…>> and does not apply D92 to it
	// (research/typedirected.md section 5, defect 2), so its members take the
	// defaults, as they do in the fork.
}

// Lambda decides the form of fn, an arrow function, a function
// expression, or a method of an object literal.
func (r *Resolver) Lambda(fn *ast.Node) Decision {
	d := Decision{Node: fn}
	params := len(fn.Parameters())
	facts := formsBodyFacts(fn)
	call, arg, steps := formsOwnerCall(fn)
	d.Call, d.Argument, d.Steps = call, arg, steps
	if call != nil {
		if r.isBlockArgument(call, fn) {
			return r.Block(call)
		}
		d.Callee = r.Callee(call)
		cls, prov, ambiguous, note := r.classAt(call, d.Callee, arg, steps)
		if cls != 0 {
			d.Class, d.Provenance, d.Ambiguous, d.Note = cls, prov, ambiguous, note
			d.Form = formsFormFor(cls, params, facts)
			return d
		}
		d.Note, d.Ambiguous = note, ambiguous
	} else if cls := r.annotatedClass(fn); cls != 0 {
		d.Class, d.Provenance, d.Note = cls, FromAnnotation, "variable annotation"
		d.Form = formsFormFor(cls, params, facts)
		return d
	}
	if cls, note, ok := r.serviceMember(fn); ok {
		d.Class, d.Provenance, d.Note = cls, FromService, note
		d.Form = formsFormFor(cls, params, facts)
		return d
	}
	if cls, note, ok := r.annotatedMember(fn); ok {
		d.Class, d.Provenance, d.Note = cls, FromAnnotation, note
		d.Form = formsFormFor(cls, params, facts)
		return d
	}
	if ret := fn.Type(); ret != nil {
		// The lambda's own return annotation: an Effect or a contract makes
		// it an Effect function, any other type a plain one (as L2 reads a
		// function's annotation).
		cls := ClassPlainFn
		if r.isEffectTypeNode(ret, 0) {
			cls = ClassEffectFn
		}
		d.Class, d.Provenance, d.Note = cls, FromAnnotation, "return annotation"
		d.Form = formsFormFor(cls, params, facts)
		return d
	}
	if call == nil {
		d.Note = "no owner call"
	}
	d.Provenance = FromFallback
	d.Form = formsFormFor(0, params, facts)
	d.Choice = r.choiceFor(d, facts)
	return d
}

// choiceFor returns the Choice for a fallback decision d when the rules
// cannot decide it: no key names the position, the lambda is not invoked
// where it stands and has no return annotation, and its body pauses. A
// lambda the parser writes for Effeck syntax is decided by that syntax.
func (r *Resolver) choiceFor(d Decision, facts formsBody) *Choice {
	fn := d.Node
	if !facts.pauses || fn.Flags&ast.NodeFlagsReparsed != 0 || fn.Type() != nil || formsInvoked(fn) {
		return nil
	}
	reason, ok := r.keyless(d)
	if !ok {
		return nil
	}
	c := &Choice{Reason: reason, Forms: []Form{FormPlain, FormEffectFn}}
	if len(fn.Parameters()) == 0 {
		c.Forms = []Form{FormPlain, FormDual}
	}
	params := r.parameterText(fn)
	switch {
	case d.Block:
		c.Plain, c.Effect = "(): T => { … }", "(): T!! => { … }"
	case fn.Kind == ast.KindMethodDeclaration:
		name, _ := formsPropertyName(fn)
		c.Plain, c.Effect = name+params+": T { … }", name+params+": T!! { … }"
	default:
		c.Plain, c.Effect = params+": T =>", params+": T!! =>"
	}
	if d.Call != nil {
		c.Callee = r.snippet(d.Call.Expression())
		c.Path = formsPathText(d.Argument, d.Steps)
		if d.Block {
			c.Path = "its block"
		}
	} else {
		c.Path = r.freePosition(fn)
	}
	return c
}

// keyless reports why d's position has no form, when nothing names it.
func (r *Resolver) keyless(d Decision) (string, bool) {
	switch {
	case d.Call == nil:
		return "no call gives it a form, and a host that calls it may ignore what it returns", true
	case d.Callee.Kind == CalleeOtherFile:
		return r.snippet(d.Call.Expression()) + " comes from another file, which a per-file projection cannot read", true
	case d.Callee.Kind == CalleeNone:
		if name, ok := formsMethodName(d.Call, d.Callee); ok {
			return "the receiver of " + name + " is a value whose type this file does not name", true
		}
		return "the callee is not a name this file can resolve", true
	case d.Callee.Kind == CalleeTable:
		if r.table != nil {
			if _, _, found := r.table.Lookup(d.Callee.Key); !found {
				if name, ok := formsMethodName(d.Call, d.Callee); ok {
					return "the receiver of " + name + " is a value whose type this file does not name", true
				}
				return d.Callee.Canonical + " is not in the forms table", true
			}
		}
		if d.Ambiguous {
			return "no overload of " + d.Callee.Canonical + " fits the written arguments, and the overloads disagree here", true
		}
		return "the forms table gives no form at this position of " + d.Callee.Canonical, true
	case d.Callee.Kind == CalleeLocal || d.Callee.Kind == CalleeService:
		return "this file declares no type for this parameter of " + d.Callee.Key, true
	}
	return "", false
}

// formsInvoked reports whether fn is called where it stands, (() => …)(),
// as do, throw, and match expressions are: its call runs it in place.
func formsInvoked(fn *ast.Node) bool {
	n := fn
	for n.Parent != nil && n.Parent.Kind == ast.KindParenthesizedExpression {
		n = n.Parent
	}
	return n.Parent != nil && ast.IsCallExpression(n.Parent) && n.Parent.Expression() == n
}

// parameterText returns fn's parameter list as written, in parentheses.
func (r *Resolver) parameterText(fn *ast.Node) string {
	ps := fn.Parameters()
	if len(ps) == 0 {
		return "()"
	}
	start := scanner.SkipTrivia(r.text, ps[0].Pos())
	end := ps[len(ps)-1].End()
	return "(" + strings.Join(strings.Fields(r.text[start:end]), " ") + ")"
}

// snippet returns node's text on one line, shortened.
func (r *Resolver) snippet(node *ast.Node) string {
	text := strings.Join(strings.Fields(r.text[scanner.SkipTrivia(r.text, node.Pos()):node.End()]), " ")
	if len(text) > 60 {
		text = text[:57] + "..."
	}
	return text
}

// formsPathText writes a path below a call's arguments, 1-based.
func formsPathText(arg int, steps []PathStep) string {
	text := "argument " + strconv.Itoa(arg+1)
	for _, step := range steps {
		if step.Element {
			text += ", element"
		} else {
			text += ", property " + step.Property
		}
	}
	return text
}

// freePosition describes where a lambda that no call takes stands.
func (r *Resolver) freePosition(fn *ast.Node) string {
	n := fn
	for n.Parent != nil && n.Parent.Kind == ast.KindParenthesizedExpression {
		n = n.Parent
	}
	switch p := n.Parent; {
	case p == nil:
		return ""
	case p.Kind == ast.KindBinaryExpression && p.AsBinaryExpression().OperatorToken.Kind == ast.KindEqualsToken:
		return "assigned to " + r.snippet(p.AsBinaryExpression().Left)
	case p.Kind == ast.KindVariableDeclaration:
		return "that is the value of " + r.snippet(p.Name())
	case p.Kind == ast.KindPropertyAssignment:
		if name, ok := formsPropertyName(p); ok {
			return "that is property " + name + " of an object literal"
		}
	case p.Kind == ast.KindObjectLiteralExpression && fn.Kind == ast.KindMethodDeclaration:
		if name, ok := formsPropertyName(fn); ok {
			return "that is method " + name + " of an object literal"
		}
	case p.Kind == ast.KindReturnStatement:
		return "that a function returns"
	}
	return ""
}

// Block decides where call's block param goes and its form. call's last
// argument is the block.
func (r *Resolver) Block(call *ast.Node) Decision {
	args := call.Arguments()
	block := args[len(args)-1]
	n := len(args) - 1
	d := Decision{Node: block, Call: call, Argument: n, Block: true, Callee: r.Callee(call)}
	sigs, prov := r.signaturesOf(d.Callee)
	if len(sigs) == 0 {
		if name, ok := formsMethodName(call, d.Callee); ok {
			if placement, cls, note, ok := r.methodBlock(name, call, n); ok {
				d.Placement, d.Class, d.Provenance, d.Note = placement, cls, FromTable, note
				d.Form = formsFormFor(d.Class, 0, formsBodyFacts(block))
				return d
			}
		}
	}
	d.Placement = formsPlacement(sigs, n)
	if d.Placement == BlockPrepended && formsIsTopic(ast.SkipParentheses(args[0])) {
		// f(%, args) { block }: the topic already fills the data-first
		// dual's Effect slot, so the block goes where the rest of the
		// signature takes it, as the fork's overload resolution decides.
		d.Placement = formsPlacementAfterData(sigs, n)
	}
	switch d.Placement {
	case BlockPrepended, BlockApplied:
		d.Class, d.Provenance, d.Note = ClassProgram, prov, "D188 "+d.Placement.String()
	default:
		s, fit, ambiguous := formsShapeFit(formsCandidates(sigs, len(args)), args)
		switch {
		case fit == fitLoose && ambiguous(n, nil):
			d.Ambiguous, d.Note = true, "no overload fits the written arguments, and the overloads disagree here"
		case fit != fitNoCandidate:
			d.Class, d.Ambiguous = s.Param(n).Top, ambiguous(n, nil)
			d.Provenance = prov
		}
	}
	if d.Class == 0 {
		d.Provenance = FromFallback
	}
	facts := formsBodyFacts(block)
	d.Form = formsFormFor(d.Class, 0, facts)
	if d.Provenance == FromFallback {
		d.Choice = r.choiceFor(d, facts)
	}
	return d
}

// Argument decides whether expr, an argument expression that is not a
// lambda, is passed by name.
func (r *Resolver) Argument(expr *ast.Node) Decision {
	d := Decision{Node: expr}
	call, arg, steps := formsOwnerCall(expr)
	if call == nil {
		d.Note = "no owner call"
		return d
	}
	d.Call, d.Argument, d.Steps = call, arg, steps
	d.Callee = r.Callee(call)
	d.Class, d.Provenance, d.Ambiguous, d.Note = r.classAt(call, d.Callee, arg, steps)
	if d.Class == 0 {
		d.Provenance = FromFallback
	}
	d.ByName = d.Class&ClassProgram != 0 && (formsHoldsCall(expr) || formsIsTopic(expr))
	return d
}

// classAt reads the class at argument arg and steps of call, where the
// block of a block param call moves its other arguments.
func (r *Resolver) classAt(call *ast.Node, callee Callee, arg int, steps []PathStep) (Class, Provenance, bool, string) {
	sigs, prov := r.signaturesOf(callee)
	if len(sigs) == 0 {
		if name, ok := formsMethodName(call, callee); ok {
			if cls, ambiguous, note := r.methodClassAt(name, call, arg, steps); cls != 0 {
				return cls, FromTable, ambiguous, note
			}
		}
		return 0, FromFallback, false, "callee " + callee.Kind.String() + " " + callee.Key
	}
	args := call.Arguments()
	argc := len(args)
	if formsIsBlockCall(r.text, call) && arg < argc-1 {
		n := argc - 1
		placement := formsPlacement(sigs, n)
		if placement == BlockPrepended && formsIsTopic(ast.SkipParentheses(args[0])) {
			placement = formsPlacementAfterData(sigs, n)
		}
		switch placement {
		case BlockPrepended:
			// The data-first signature takes the block first: argument i is
			// its parameter i + 1.
			for _, s := range sigs {
				if s.DataFirst && s.Accepts(n+1) {
					return s.Param(arg + 1).At(steps), prov, false, "data-first, block moved"
				}
			}
		case BlockApplied:
			for _, s := range sigs {
				if s.DataLast && s.Accepts(n) {
					return s.Param(arg).At(steps), prov, false, "data-last, block applied"
				}
			}
		}
	}
	s, fit, ambiguous := formsShapeFit(formsCandidates(sigs, argc), args)
	switch {
	case fit == fitNoCandidate:
		return 0, FromFallback, false, "no signature takes " + strconv.Itoa(argc) + " arguments"
	case fit == fitLoose && ambiguous(arg, steps):
		return 0, FromFallback, true, "no overload fits the written arguments, and the overloads disagree here"
	}
	cls := s.Param(arg).At(steps)
	if cls == 0 {
		return 0, FromFallback, false, "no class at the path"
	}
	return cls, prov, ambiguous(arg, steps), ""
}

// formsMethodName returns the method name of a call of a value's member,
// x.name(…), whose callee has no key or a key the table lacks.
func formsMethodName(call *ast.Node, callee Callee) (string, bool) {
	if callee.Kind != CalleeNone && callee.Kind != CalleeTable {
		return "", false
	}
	e := ast.SkipParentheses(call.Expression())
	if !ast.IsPropertyAccessExpression(e) || !ast.IsIdentifier(e.Name()) {
		return "", false
	}
	if callee.Kind == CalleeTable && !strings.Contains(callee.Canonical[strings.IndexByte(callee.Canonical, '#')+1:], ".") {
		return "", false // a module function the table omits carries no form
	}
	return e.Name().Text(), true
}

// methodClassAt reads the class at argument arg and steps of a method call
// whose receiver is unknown: the forms every receiver with a method of that
// name accepts there, among the lib's collections, effect's data types'
// own members, and D152's self-first functions. Receivers whose method
// takes no such argument do not count; receivers that share no form give
// no class.
func (r *Resolver) methodClassAt(name string, call *ast.Node, arg int, steps []PathStep) (Class, bool, string) {
	if r.table == nil {
		return 0, false, ""
	}
	args := call.Arguments()
	all := ClassProgram | ClassEffectFn | ClassPlainFn
	cls, seen, differ := all, 0, false
	var first Class
	receivers, module := r.receiversOf(call, name)
	if module == "" {
		return 0, false, "" // the receiver is unnamed: a guess across receivers could be silently wrong
	}
	for _, m := range receivers {
		sigs := m.sigs
		if m.self {
			sigs = formsDropSelf(sigs)
		}
		s, fit, _ := formsShapeFit(formsCandidates(sigs, len(args)), args)
		if fit != fitOK {
			continue
		}
		c := s.Param(arg).At(steps)
		if c == 0 {
			continue
		}
		if seen > 0 && c != first {
			differ = true
		}
		if seen == 0 {
			first = c
		}
		seen++
		cls &= c
	}
	if seen == 0 || cls == 0 {
		return 0, false, ""
	}
	if module != "" {
		return cls, differ, "method " + name + " of " + module
	}
	return cls, differ, "method " + name + " on " + strconv.Itoa(seen) + " receivers"
}

// receiversOf returns the receivers whose method name a call x.name(…)
// may reach: the one module scope names for x when it has the method, else
// every receiver with a method of that name.
func (r *Resolver) receiversOf(call *ast.Node, name string) ([]methodReceiver, string) {
	all := r.table.methodReceivers(name)
	e := ast.SkipParentheses(call.Expression())
	if !ast.IsPropertyAccessExpression(e) {
		return all, ""
	}
	module := r.receiverModule(e.Expression(), 0)
	if module == "" {
		return all, ""
	}
	var mine []methodReceiver
	for _, m := range all {
		if m.receiver == module {
			mine = append(mine, m)
		}
	}
	if len(mine) == 0 {
		return all, ""
	}
	return mine, module
}

// receiverModule names, from syntax, the module whose data type the
// expression x in x.name(…) has: a call of a table function whose result
// is that module's type (Stream.make(…)), a value export (Stream.never), or
// a binding whose written type (s: Stream.Stream<A>) or initializer says so.
// It returns "" when scope cannot tell. A call that returns an Effect
// inside a function body runs there (D24), so it names no receiver.
func (r *Resolver) receiverModule(expr *ast.Node, depth int) string {
	expr = ast.SkipParentheses(expr)
	if depth > 3 || r.table == nil {
		return ""
	}
	switch expr.Kind {
	case ast.KindArrayLiteralExpression:
		return "lib/Array"
	case ast.KindNewExpression:
		if ctor := ast.SkipParentheses(expr.Expression()); ast.IsIdentifier(ctor) && r.binding(ctor, ctor.Text()) == nil && formsLibReceivers[ctor.Text()] {
			return "lib/" + ctor.Text()
		}
		return ""
	}
	if ast.IsCallExpression(expr) {
		if e := ast.SkipParentheses(expr.Expression()); ast.IsPropertyAccessExpression(e) {
			// Array.from(…), Promise.all(…): the global's own constructors.
			if root := ast.SkipParentheses(e.Expression()); ast.IsIdentifier(root) && r.binding(root, root.Text()) == nil {
				switch root.Text() + "." + e.Name().Text() {
				case "Array.from", "Array.of":
					return "lib/Array"
				case "Promise.all", "Promise.resolve", "Promise.reject", "Promise.race", "Promise.allSettled", "Promise.any":
					return "lib/Promise"
				}
			}
			// x.m(…) on a receiver scope names: the method's result.
			if receiver := r.receiverModule(e.Expression(), depth+1); receiver != "" {
				return r.methodResult(receiver, e.Name().Text(), len(expr.Arguments()))
			}
		}
		c := r.Callee(expr)
		if c.Kind != CalleeTable {
			return ""
		}
		_, sigs, ok := r.table.Lookup(c.Key)
		if !ok {
			return ""
		}
		runs := formsInFunction(expr) // an Effect call in a function body runs there (D24)
		module := ""
		for _, s := range formsCandidates(sigs, len(expr.Arguments())) {
			result := s.Result
			if runs && result == "effect/Effect" {
				result = s.Runs
			}
			if result == "" || module != "" && result != module {
				return ""
			}
			module = result
		}
		return module
	}
	root, chain, ok := formsNameChain(expr)
	if !ok {
		return ""
	}
	if len(chain) > 0 {
		if c := r.calleeOf(root, chain, 0); c.Kind == CalleeTable {
			return r.table.ValueModule(c.Key)
		}
		return ""
	}
	d := r.binding(root, root.Text())
	if d == nil {
		return r.table.ValueModule("prelude#" + root.Text())
	}
	switch d.Kind {
	case ast.KindParameter, ast.KindVariableDeclaration:
		if t := d.Type(); t != nil {
			return r.typeModuleOf(t)
		}
		if d.Kind == ast.KindVariableDeclaration && d.Initializer() != nil {
			return r.receiverModule(d.Initializer(), depth+1)
		}
	case ast.KindImportSpecifier:
		if c := r.calleeOf(root, nil, 0); c.Kind == CalleeTable {
			return r.table.ValueModule(c.Key)
		}
	}
	return ""
}

// formsLibReceivers are the lib's types whose own methods the table records.
var formsLibReceivers = map[string]bool{"Array": true, "ReadonlyArray": true, "Map": true, "ReadonlyMap": true, "Set": true, "ReadonlySet": true, "Promise": true}

// methodResult returns the module whose type the method name of receiver
// returns when called with argc arguments, when the table records it.
func (r *Resolver) methodResult(receiver, name string, argc int) string {
	for _, m := range r.table.methodReceivers(name) {
		if m.receiver != receiver {
			continue
		}
		sigs := m.sigs
		if m.self {
			sigs = formsDropSelf(sigs)
		}
		result := ""
		for _, s := range formsCandidates(sigs, argc) {
			if s.Result == "" || result != "" && s.Result != result {
				return ""
			}
			result = s.Result
		}
		return result
	}
	return ""
}

// typeModuleOf returns the module a written type names as its own data
// type: Stream.Stream<A> is effect/Stream's Stream, and number[] or
// ReadonlyArray<number> is the lib's array.
func (r *Resolver) typeModuleOf(t *ast.Node) string {
	if t != nil && t.Kind == ast.KindTypeOperator {
		if inner := formsUnwrapType(t); inner != nil && inner.Kind == ast.KindArrayType {
			return "lib/ReadonlyArray"
		}
	}
	t = formsUnwrapType(t)
	if t != nil && t.Kind == ast.KindArrayType {
		return "lib/Array"
	}
	if t == nil || t.Kind != ast.KindTypeReference {
		return ""
	}
	name := t.AsTypeReferenceNode().TypeName
	if ast.IsIdentifier(name) {
		if formsLibReceivers[name.Text()] && r.binding(name, name.Text()) == nil {
			return "lib/" + name.Text()
		}
		return ""
	}
	if name.Kind != ast.KindQualifiedName {
		return ""
	}
	q := name.AsQualifiedName()
	left := q.Left
	if !ast.IsIdentifier(left) {
		return ""
	}
	c := r.calleeOf(left, []string{q.Right.Text()}, 0)
	if c.Kind != CalleeTable {
		return ""
	}
	module, export, ok := strings.Cut(c.Canonical, "#")
	if !ok || module[strings.LastIndex(module, "/")+1:] != export {
		return ""
	}
	return module
}

// formsInFunction reports whether node is inside a function body.
func formsInFunction(node *ast.Node) bool {
	for n := node.Parent; n != nil; n = n.Parent {
		if ast.IsFunctionLike(n) {
			return true
		}
	}
	return false
}

// methodBlock places the block of a method call whose receiver is unknown,
// x.name(args) { block }, where every receiver whose method takes the
// arguments agrees: semaphore.withPermits(1) { … } is applied, because
// Semaphore's own withPermits(permits) returns a function of the program.
func (r *Resolver) methodBlock(name string, call *ast.Node, n int) (Placement, Class, string, bool) {
	if r.table == nil {
		return 0, 0, "", false
	}
	args := call.Arguments()
	var placement Placement
	var cls Class
	seen := 0
	receivers, module := r.receiversOf(call, name)
	if module == "" {
		return 0, 0, "", false
	}
	for _, m := range receivers {
		sigs := m.sigs
		if m.self {
			sigs = formsDropSelf(sigs)
		}
		if len(formsCandidates(sigs, n)) == 0 && len(formsCandidates(sigs, n+1)) == 0 {
			continue // this receiver's method takes other arguments
		}
		p := formsPlacement(sigs, n)
		c := ClassProgram
		if p == BlockAppended {
			s, fit, _ := formsShapeFit(formsCandidates(sigs, len(args)), args)
			if fit != fitOK {
				continue
			}
			c = s.Param(n).Top
		}
		if c == 0 {
			continue // no form fits a block there: this method takes no block
		}
		if seen > 0 && (p != placement || c != cls) {
			return 0, 0, "", false // receivers disagree
		}
		placement, cls = p, c
		seen++
	}
	if seen == 0 {
		return 0, 0, "", false
	}
	return placement, cls, "method " + name + " on " + strconv.Itoa(seen) + " receivers, D188 " + placement.String(), true
}

// formsDropSelf returns D152 methods: sigs without their self parameter.
func formsDropSelf(sigs []Signature) []Signature {
	out := make([]Signature, 0, len(sigs))
	for _, s := range sigs {
		if s.Max == 0 || s.Rest && s.Max == 1 {
			continue
		}
		m := Signature{Min: max(s.Min-1, 0), Max: s.Max - 1, Rest: s.Rest, DataLast: s.DataLast}
		if len(s.Params) > 1 {
			m.Params = s.Params[1:]
		}
		out = append(out, m)
	}
	return out
}

// signaturesOf returns callee's signatures and where they come from.
func (r *Resolver) signaturesOf(callee Callee) ([]Signature, Provenance) {
	switch callee.Kind {
	case CalleeTable:
		if r.table == nil {
			return nil, FromFallback
		}
		_, sigs, _ := r.table.Lookup(callee.Key)
		return sigs, FromTable
	case CalleeLocal:
		return callee.Signatures, FromAnnotation
	case CalleeService:
		return callee.Signatures, FromService
	}
	return nil, FromFallback
}

// formsCandidates returns the signatures that take argc arguments, the one that
// requires the most first, then in declaration order.
func formsCandidates(sigs []Signature, argc int) []Signature {
	var out []Signature
	for _, s := range sigs {
		if s.Accepts(argc) {
			out = append(out, s)
		}
	}
	for i := 1; i < len(out); i++ { // stable insertion sort by Min, descending
		for j := i; j > 0 && out[j].Min > out[j-1].Min; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// formsFit says how a call's written arguments fit its candidates.
type formsFit uint8

const (
	fitNoCandidate formsFit = iota // no signature takes the argument count
	fitLoose                       // the written arguments rule out every candidate; only where they all agree is there an answer
	fitOK
)

// formsShapeFit picks the first candidate the written arguments do not rule
// out. The checks read classes, which record only the callable forms a
// type accepts, so a candidate they rule out may still fit (a string at
// Duration.Input, a union with a function member). When they rule out
// every candidate, the result is fitLoose: the candidates stand, and the
// caller takes an answer only where they all agree, so overload order
// never decides. The returned function reports whether the candidates left
// disagree at a path.
func formsShapeFit(cands []Signature, args []*ast.Node) (Signature, formsFit, func(int, []PathStep) bool) {
	none := func(int, []PathStep) bool { return false }
	if len(cands) == 0 {
		return Signature{}, fitNoCandidate, none
	}
	var fits []Signature
	for _, strict := range []bool{true, false} {
		for _, s := range cands {
			if formsArgumentsFit(s, args, strict) {
				fits = append(fits, s)
			}
		}
		if len(fits) > 0 {
			break
		}
	}
	fit := fitOK
	if len(fits) == 0 {
		fits, fit = cands, fitLoose
	}
	chosen := fits[0]
	return chosen, fit, func(arg int, steps []PathStep) bool {
		want := chosen.Param(arg).At(steps)
		for _, s := range fits[1:] {
			if s.Param(arg).At(steps) != want {
				return true
			}
		}
		return false
	}
}

// formsArgumentsFit reports whether no written argument rules s out: a
// literal where s's parameter accepts only functions, or a string, number,
// or template literal where it accepts forms but not a program. With
// strict, a lambda where the parameter carries no form rules s out too: a
// preference among overloads, not a mismatch, so formsShapeFit drops it
// when no candidate passes otherwise.
func formsArgumentsFit(s Signature, args []*ast.Node, strict bool) bool {
	for i, arg := range args {
		top := s.Param(i).Top
		x := ast.SkipParentheses(arg)
		switch x.Kind {
		case ast.KindArrowFunction, ast.KindFunctionExpression:
			if strict && top == 0 {
				return false
			}
		case ast.KindStringLiteral, ast.KindNumericLiteral, ast.KindTemplateExpression, ast.KindNoSubstitutionTemplateLiteral:
			if top != 0 && top&ClassProgram == 0 {
				return false
			}
		case ast.KindObjectLiteralExpression, ast.KindArrayLiteralExpression:
			if top != 0 && top&ClassProgram == 0 {
				return false
			}
		}
	}
	return true
}

// formsPlacement applies D188's order for a block after n arguments.
func formsPlacement(sigs []Signature, n int) Placement {
	for _, s := range sigs {
		if n > 0 && s.DataFirst && s.Accepts(n+1) {
			return BlockPrepended
		}
	}
	for _, s := range sigs {
		if s.DataLast && s.Accepts(n) {
			return BlockApplied
		}
	}
	return BlockAppended
}

// formsPlacementAfterData places a block after n arguments whose first
// is the pipe topic: the call is a data-first dual's own form with its data
// written, so a data-first signature that takes the block last appends it.
// Otherwise D188's order without the data-first form applies.
func formsPlacementAfterData(sigs []Signature, n int) Placement {
	for _, s := range sigs {
		if s.DataFirst && s.Accepts(n+1) {
			return BlockAppended
		}
	}
	for _, s := range sigs {
		if s.DataLast && s.Accepts(n) {
			return BlockApplied
		}
	}
	return BlockAppended
}

// formsBody is what the text of a function's own body shows.
type formsBody struct {
	tail    bool // the body is one call, or one statement returning one
	pauses  bool // a call outside tail position, a throw, an Effeck statement, or a using declaration
	anyCall bool
}

func formsBodyFacts(fn *ast.Node) formsBody {
	var f formsBody
	body := fn.Body()
	if body == nil {
		return f
	}
	var tail *ast.Node
	if body.Kind != ast.KindBlock {
		if x := ast.SkipParentheses(body); ast.IsCallExpression(x) {
			tail = x
		}
	} else if statements := body.Statements(); len(statements) == 1 {
		s := statements[0]
		if (ast.IsReturnStatement(s) || s.Kind == ast.KindExpressionStatement) && s.Expression() != nil {
			if x := ast.SkipParentheses(s.Expression()); ast.IsCallExpression(x) {
				tail = x
			}
		}
	}
	f.tail = tail != nil
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		if formsBoundary(n) {
			return false
		}
		switch {
		case ast.IsCallExpression(n):
			f.anyCall = true
			if n != tail {
				f.pauses = true
			}
		case n.Kind == ast.KindThrowStatement:
			f.pauses = true
		case n.Flags&ast.NodeFlagsEffeck != 0 && !formsIsTopic(n): // %, a pipe's topic, is a value
			f.pauses = true
		case n.Kind == ast.KindVariableDeclarationList && n.Flags&ast.NodeFlagsUsing != 0:
			f.pauses = true
		}
		n.ForEachChild(visit)
		return false
	}
	if body.Kind == ast.KindBlock {
		body.ForEachChild(visit)
	} else {
		visit(body)
	}
	return f
}

// formsFormFor picks a lambda's form from its class, its parameter count, and
// what its body shows.
func formsFormFor(cls Class, params int, f formsBody) Form {
	p, e, plain := cls&ClassProgram != 0, cls&ClassEffectFn != 0, cls&ClassPlainFn != 0
	effectful := f.pauses || params == 0 && f.anyCall
	parameterless := func() Form {
		switch {
		case p && e:
			return FormDual
		case p:
			return FormProgram
		}
		return FormEffectFn
	}
	switch {
	case cls == 0:
		if !effectful {
			return FormPlain
		}
		if params == 0 {
			return FormDual
		}
		return FormEffectFn
	case plain && !p && !e:
		return FormPlain
	case !plain: // only Effect forms fit
		if params == 0 {
			return parameterless()
		}
		if e {
			return FormEffectFn
		}
		return FormProgram
	default: // a plain function or an Effect form fits (D39, D112): the body decides
		if !effectful {
			return FormPlain
		}
		if params == 0 {
			return parameterless()
		}
		if e {
			return FormEffectFn
		}
		return FormPlain
	}
}

// formsBoundary reports whether node starts code that runs in another
// function: a call in it does not run where its parent's calls do.
func formsBoundary(node *ast.Node) bool {
	return ast.IsFunctionLike(node) || ast.IsClassLike(node) || node.Kind == ast.KindClassStaticBlockDeclaration
}

// formsHoldsCall reports whether expr calls something outside the functions
// nested in it: the text a by-name argument shows.
func formsHoldsCall(expr *ast.Node) bool {
	if ast.IsCallExpression(expr) {
		return true
	}
	if formsBoundary(expr) {
		return false
	}
	found := false
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		if found || formsBoundary(n) {
			return false
		}
		if ast.IsCallExpression(n) {
			found = true
			return true
		}
		return n.ForEachChild(visit)
	}
	expr.ForEachChild(visit)
	return found
}

// formsIsTopic reports whether node is a pipe's topic, %: an identifier
// the parser flags as an Effeck form (parser/effeck_pipes.go). The topic
// stands for the pipe's left side, so it reads as the call it holds.
func formsIsTopic(node *ast.Node) bool {
	return ast.IsIdentifier(node) && node.Flags&ast.NodeFlagsEffeck != 0 && node.Text() == "%"
}

// formsOwnerCall returns the call whose arguments hold node, the argument's
// index, and the steps from that argument down to node through array
// elements, object properties, and object methods.
func formsOwnerCall(node *ast.Node) (*ast.Node, int, []PathStep) {
	var steps []PathStep
	for parent := node.Parent; parent != nil; node, parent = parent, parent.Parent {
		switch parent.Kind {
		case ast.KindParenthesizedExpression, ast.KindAsExpression, ast.KindSatisfiesExpression, ast.KindNonNullExpression:
			continue // [a, b] as const is still the argument
		case ast.KindObjectLiteralExpression:
			if node.Kind == ast.KindMethodDeclaration {
				name, ok := formsPropertyName(node)
				if !ok {
					return nil, 0, nil
				}
				steps = append(steps, PathStep{Property: name})
			}
			continue
		case ast.KindArrayLiteralExpression:
			steps = append(steps, PathStep{Element: true})
			continue
		case ast.KindPropertyAssignment:
			name, ok := formsPropertyName(parent)
			if parent.Initializer() != node || !ok {
				return nil, 0, nil
			}
			steps = append(steps, PathStep{Property: name})
			continue
		case ast.KindCallExpression:
			for i, arg := range parent.Arguments() {
				if arg == node {
					for a, b := 0, len(steps)-1; a < b; a, b = a+1, b-1 {
						steps[a], steps[b] = steps[b], steps[a]
					}
					return parent, i, steps
				}
			}
		}
		return nil, 0, nil
	}
	return nil, 0, nil
}

func formsPropertyName(member *ast.Node) (string, bool) {
	name := member.Name()
	if name == nil || !(ast.IsIdentifier(name) || ast.IsStringLiteral(name) || ast.IsNumericLiteral(name)) {
		return "", false
	}
	return name.Text(), true
}

// isBlockArgument reports whether fn is call's block param.
func (r *Resolver) isBlockArgument(call, fn *ast.Node) bool {
	args := call.Arguments()
	return len(args) > 0 && args[len(args)-1] == fn && formsIsBlockCall(r.text, call)
}

// formsIsBlockCall reports whether call passes a block param: its last argument
// is a parameterless arrow function with a block body that the source
// writes as a bare block, so its text starts with { rather than (.
func formsIsBlockCall(text string, call *ast.Node) bool {
	if !ast.IsCallExpression(call) {
		return false
	}
	args := call.Arguments()
	if len(args) == 0 {
		return false
	}
	arrow := args[len(args)-1]
	if arrow.Kind != ast.KindArrowFunction || len(arrow.Parameters()) != 0 || arrow.Body() == nil || arrow.Body().Kind != ast.KindBlock {
		return false
	}
	start := scanner.SkipTrivia(text, arrow.Pos())
	return start < len(text) && text[start] == '{'
}

// Callee names call's callee from scope alone.
func (r *Resolver) Callee(call *ast.Node) Callee {
	root, chain, ok := formsNameChain(call.Expression())
	if !ok {
		return Callee{}
	}
	return r.calleeOf(root, chain, 0)
}

// formsNameChain splits a.b.c into the identifier a and [b, c].
func formsNameChain(expr *ast.Node) (*ast.Node, []string, bool) {
	var chain []string
	e := ast.SkipParentheses(expr)
	for ast.IsPropertyAccessExpression(e) {
		name := e.Name()
		if !ast.IsIdentifier(name) {
			return nil, nil, false
		}
		chain = append([]string{name.Text()}, chain...)
		e = ast.SkipParentheses(e.Expression())
	}
	if !ast.IsIdentifier(e) {
		return nil, nil, false
	}
	return e, chain, true
}

func (r *Resolver) calleeOf(root *ast.Node, chain []string, depth int) Callee {
	name := root.Text()
	decl := r.binding(root, name)
	tableKey := func(prefix string) Callee {
		key := prefix
		if len(chain) > 0 {
			if !strings.HasSuffix(prefix, "#") {
				key += "."
			}
			key += strings.Join(chain, ".")
		}
		c := Callee{Kind: CalleeTable, Key: key, Canonical: key}
		if r.table != nil {
			c.Canonical = r.table.Canonical(key)
		}
		return c
	}
	if decl == nil {
		return tableKey("prelude#" + name)
	}
	switch decl.Kind {
	case ast.KindImportSpecifier:
		spec := formsModuleSpecifier(decl)
		if spec == "" || strings.HasPrefix(spec, ".") {
			return Callee{Kind: CalleeOtherFile, Key: spec}
		}
		imported := name
		if pn := decl.AsImportSpecifier().PropertyName; pn != nil {
			imported = pn.Text()
		}
		return tableKey(spec + "#" + imported)
	case ast.KindNamespaceImport:
		spec := formsModuleSpecifier(decl)
		if spec == "" || strings.HasPrefix(spec, ".") {
			return Callee{Kind: CalleeOtherFile, Key: spec}
		}
		if len(chain) == 0 {
			return Callee{}
		}
		return tableKey(spec + "#")
	case ast.KindImportClause:
		spec := formsModuleSpecifier(decl)
		if spec == "" || strings.HasPrefix(spec, ".") {
			return Callee{Kind: CalleeOtherFile, Key: spec}
		}
		return tableKey(spec + "#default")
	case ast.KindFunctionDeclaration:
		if len(chain) != 0 {
			return Callee{}
		}
		return Callee{Kind: CalleeLocal, Key: name, Signatures: r.functionSignatures(decl)}
	case ast.KindVariableDeclaration:
		init := decl.Initializer()
		if init != nil {
			init = ast.SkipParentheses(init)
		}
		if t := decl.Type(); t != nil && len(chain) == 0 && t.Kind == ast.KindFunctionType {
			return Callee{Kind: CalleeLocal, Key: name, Signatures: []Signature{r.writtenSignature(t.Parameters(), t.Type())}}
		}
		if init == nil {
			return Callee{}
		}
		if init.Kind == ast.KindArrowFunction || init.Kind == ast.KindFunctionExpression {
			if len(chain) != 0 {
				return Callee{}
			}
			return Callee{Kind: CalleeLocal, Key: name, Signatures: []Signature{r.writtenSignature(init.Parameters(), init.Type())}}
		}
		// const runner = new Runner(): a method of a class this file declares.
		if init.Kind == ast.KindNewExpression && len(chain) == 1 {
			if ctor := ast.SkipParentheses(init.Expression()); ast.IsIdentifier(ctor) {
				if class := r.binding(ctor, ctor.Text()); class != nil && class.Kind == ast.KindClassDeclaration {
					if m := formsClassMember(class, chain[0]); m != nil && m.Kind == ast.KindMethodDeclaration {
						return Callee{Kind: CalleeLocal, Key: name + "." + chain[0], Signatures: []Signature{r.writtenSignature(m.Parameters(), m.Type())}}
					}
				}
			}
		}
		// const E = Effect, const all = Effect.all: follow the alias.
		if depth < 4 && decl.Parent != nil && decl.Parent.Flags&ast.NodeFlagsConst != 0 {
			if next, more, ok := formsNameChain(init); ok {
				return r.calleeOf(next, append(more, chain...), depth+1)
			}
		}
		return Callee{}
	case ast.KindClassDeclaration:
		if len(chain) == 1 && formsIsServiceClass(decl) {
			if m := formsClassMember(decl, chain[0]); m != nil {
				switch {
				case m.Kind == ast.KindMethodDeclaration:
					return Callee{Kind: CalleeService, Key: name + "." + chain[0], Signatures: []Signature{r.writtenSignature(m.Parameters(), m.Type())}}
				case m.Kind == ast.KindPropertyDeclaration && m.Type() != nil && m.Type().Kind == ast.KindFunctionType:
					t := m.Type()
					return Callee{Kind: CalleeService, Key: name + "." + chain[0], Signatures: []Signature{r.writtenSignature(t.Parameters(), t.Type())}}
				}
			}
		}
		return Callee{}
	}
	return Callee{}
}

func formsModuleSpecifier(decl *ast.Node) string {
	for n := decl; n != nil; n = n.Parent {
		if n.Kind == ast.KindImportDeclaration {
			if spec := n.ModuleSpecifier(); spec != nil && ast.IsStringLiteral(spec) {
				return spec.Text()
			}
			return ""
		}
	}
	return ""
}

// binding returns the declaration that binds name where id stands: an
// import, a variable, a function, a class, or a parameter, or nil when no
// scope in the file binds it.
func (r *Resolver) binding(id *ast.Node, name string) *ast.Node {
	for n := id.Parent; n != nil; n = n.Parent {
		if d := formsDeclaredIn(n, name, id); d != nil {
			return d
		}
	}
	return nil
}

// formsDeclaredIn returns the declaration of name that scope introduces.
func formsDeclaredIn(scope *ast.Node, name string, from *ast.Node) *ast.Node {
	switch {
	case scope.Kind == ast.KindSourceFile:
		for _, s := range scope.AsSourceFile().Statements.Nodes {
			if d := formsStatementDeclares(s, name); d != nil {
				return d
			}
			if s.Kind == ast.KindImportDeclaration {
				if d := formsImportDeclares(s, name); d != nil {
					return d
				}
			}
		}
	case scope.Kind == ast.KindBlock || scope.Kind == ast.KindModuleBlock || scope.Kind == ast.KindCaseClause || scope.Kind == ast.KindDefaultClause:
		for _, s := range scope.Statements() {
			if d := formsStatementDeclares(s, name); d != nil {
				return d
			}
		}
	case ast.IsFunctionLike(scope):
		for _, p := range scope.Parameters() {
			if d := formsBindingDeclares(p.Name(), name, p); d != nil {
				return d
			}
		}
		if scope.Kind == ast.KindFunctionExpression && scope.Name() != nil && scope.Name().Text() == name {
			return scope
		}
		if body := scope.Body(); body != nil && body.Kind == ast.KindBlock {
			if d := formsHoistedVar(body, name); d != nil {
				return d
			}
		}
	case scope.Kind == ast.KindCatchClause:
		if v := scope.AsCatchClause().VariableDeclaration; v != nil {
			if d := formsBindingDeclares(v.Name(), name, v); d != nil {
				return d
			}
		}
	case scope.Kind == ast.KindForStatement || scope.Kind == ast.KindForInStatement || scope.Kind == ast.KindForOfStatement:
		if init := scope.Initializer(); init != nil && init.Kind == ast.KindVariableDeclarationList {
			for _, v := range init.AsVariableDeclarationList().Declarations.Nodes {
				if d := formsBindingDeclares(v.Name(), name, v); d != nil {
					return d
				}
			}
		}
	case scope.Kind == ast.KindClassExpression:
		if scope.Name() != nil && scope.Name().Text() == name {
			return scope
		}
	}
	return nil
}

// formsStatementDeclares returns the declaration of name that statement s makes
// in its own block.
func formsStatementDeclares(s *ast.Node, name string) *ast.Node {
	switch s.Kind {
	case ast.KindVariableStatement:
		for _, v := range s.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
			if d := formsBindingDeclares(v.Name(), name, v); d != nil {
				return d
			}
		}
	case ast.KindFunctionDeclaration, ast.KindClassDeclaration, ast.KindEnumDeclaration, ast.KindModuleDeclaration:
		if n := s.Name(); n != nil && ast.IsIdentifier(n) && n.Text() == name {
			return s
		}
	}
	return nil
}

// formsImportDeclares returns the import specifier, namespace import, or default
// import clause that binds name.
func formsImportDeclares(s *ast.Node, name string) *ast.Node {
	clause := s.AsImportDeclaration().ImportClause
	if clause == nil {
		return nil
	}
	if n := clause.Name(); n != nil && n.Text() == name {
		return clause
	}
	bindings := clause.AsImportClause().NamedBindings
	switch {
	case bindings == nil:
	case bindings.Kind == ast.KindNamespaceImport:
		if bindings.Name().Text() == name {
			return bindings
		}
	case bindings.Kind == ast.KindNamedImports:
		for _, e := range bindings.AsNamedImports().Elements.Nodes {
			if e.Name().Text() == name {
				return e
			}
		}
	}
	return nil
}

// formsBindingDeclares returns decl when the binding name, an identifier or a
// destructuring pattern, binds name.
func formsBindingDeclares(binding *ast.Node, name string, decl *ast.Node) *ast.Node {
	if binding == nil {
		return nil
	}
	if ast.IsIdentifier(binding) {
		if binding.Text() == name {
			return decl
		}
		return nil
	}
	var found *ast.Node
	binding.ForEachChild(func(child *ast.Node) bool {
		if child.Kind == ast.KindBindingElement {
			if d := formsBindingDeclares(child.Name(), name, child); d != nil {
				found = d
				return true
			}
		}
		return false
	})
	return found
}

// formsHoistedVar returns a var declaration of name anywhere in body outside
// nested functions.
func formsHoistedVar(body *ast.Node, name string) *ast.Node {
	var found *ast.Node
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		if found != nil || formsBoundary(n) {
			return false
		}
		if n.Kind == ast.KindVariableDeclarationList && n.Flags&(ast.NodeFlagsLet|ast.NodeFlagsConst|ast.NodeFlagsUsing) == 0 {
			for _, v := range n.AsVariableDeclarationList().Declarations.Nodes {
				if d := formsBindingDeclares(v.Name(), name, v); d != nil {
					found = d
					return true
				}
			}
		}
		return n.ForEachChild(visit)
	}
	body.ForEachChild(visit)
	return found
}

// functionSignatures returns the written signatures of a function this
// file declares: its overloads, else the implementation.
func (r *Resolver) functionSignatures(decl *ast.Node) []Signature {
	name := decl.Name().Text()
	var overloads, impls []Signature
	scope := decl.Parent
	var statements []*ast.Node
	switch {
	case scope != nil && scope.Kind == ast.KindSourceFile:
		statements = scope.AsSourceFile().Statements.Nodes
	case scope != nil && scope.Kind == ast.KindBlock:
		statements = scope.Statements()
	default:
		statements = []*ast.Node{decl}
	}
	for _, s := range statements {
		if s.Kind != ast.KindFunctionDeclaration || s.Name() == nil || s.Name().Text() != name {
			continue
		}
		sig := r.writtenSignature(s.Parameters(), s.Type())
		if s.Body() == nil {
			overloads = append(overloads, sig)
		} else {
			impls = append(impls, sig)
		}
	}
	if len(overloads) > 0 {
		return overloads
	}
	return impls
}

// writtenSignature reads a signature from written parameters and a written
// return type.
func (r *Resolver) writtenSignature(params []*ast.Node, returnType *ast.Node) Signature {
	s := Signature{Max: len(params)}
	required := true
	for _, p := range params {
		pd := p.AsParameterDeclaration()
		if pd.DotDotDotToken != nil {
			s.Rest = true
			required = false
		}
		if pd.QuestionToken != nil || pd.Initializer != nil {
			required = false
		}
		if required {
			s.Min++
		}
		s.Params = append(s.Params, r.paramOfType(p.Type(), s.Rest))
	}
	if len(params) > 0 {
		s.DataFirst = r.isEffectTypeNode(params[0].Type(), 0)
	}
	if returnType != nil && returnType.Kind == ast.KindFunctionType {
		if ps := returnType.Parameters(); len(ps) > 0 && r.isEffectTypeNode(ps[0].Type(), 0) {
			s.DataLast = true
		}
	}
	return s
}

// paramOfType reads what a parameter whose written type is t accepts. A
// rest parameter's type is an array of the argument type.
func (r *Resolver) paramOfType(t *ast.Node, rest bool) Param {
	if t == nil {
		return Param{}
	}
	if rest {
		if elem := formsElementType(t); elem != nil {
			t = elem
		}
	}
	p := Param{Top: r.classOfType(t, 0)}
	if elem := formsElementType(formsUnwrapType(t)); elem != nil {
		p.Elem = r.classOfType(elem, 0)
	}
	if lit := r.typeLiteral(t, 0); lit != nil {
		for _, m := range lit.Members() {
			switch m.Kind {
			case ast.KindPropertySignature:
				if name, ok := formsPropertyName(m); ok {
					if c := r.classOfType(m.Type(), 0); c != 0 {
						if p.Props == nil {
							p.Props = map[string]Class{}
						}
						p.Props[name] = c
					}
				}
			case ast.KindMethodSignature:
				if name, ok := formsPropertyName(m); ok {
					if p.Props == nil {
						p.Props = map[string]Class{}
					}
					p.Props[name] = r.callbackClass(m.Type())
				}
			case ast.KindIndexSignature:
				p.AnyProp = r.classOfType(m.Type(), 0)
			}
		}
	}
	return p
}

// formsUnwrapType drops parentheses and readonly from a written type.
func formsUnwrapType(t *ast.Node) *ast.Node {
	for t != nil {
		switch {
		case t.Kind == ast.KindParenthesizedType:
			t = t.Type()
		case t.Kind == ast.KindTypeOperator:
			t = t.AsTypeOperatorNode().Type
		default:
			return t
		}
	}
	return t
}

// formsElementType returns the element type of a written array, readonly
// array, or iterable type.
func formsElementType(t *ast.Node) *ast.Node {
	t = formsUnwrapType(t)
	if t == nil {
		return nil
	}
	switch t.Kind {
	case ast.KindArrayType:
		return t.AsArrayTypeNode().ElementType
	case ast.KindTypeReference:
		name := t.AsTypeReferenceNode().TypeName
		if ast.IsIdentifier(name) && len(t.TypeArguments()) == 1 {
			switch name.Text() {
			case "Array", "ReadonlyArray", "Iterable", "NonEmptyArray", "NonEmptyReadonlyArray":
				return t.TypeArguments()[0]
			}
		}
	}
	return nil
}

// typeLiteral returns the written object type t names: a type literal, or
// a type alias or interface of this file that is one.
func (r *Resolver) typeLiteral(t *ast.Node, depth int) *ast.Node {
	t = formsUnwrapType(t)
	if t == nil || depth > 4 {
		return nil
	}
	switch t.Kind {
	case ast.KindTypeLiteral:
		return t
	case ast.KindTypeReference:
		if d := r.localType(t); d != nil {
			if d.Kind == ast.KindInterfaceDeclaration {
				return d
			}
			return r.typeLiteral(d.Type(), depth+1)
		}
	}
	return nil
}

// localType returns the top-level type alias or interface a written type
// reference names, when this file declares it without type parameters.
func (r *Resolver) localType(ref *ast.Node) *ast.Node {
	name := ref.AsTypeReferenceNode().TypeName
	if !ast.IsIdentifier(name) {
		return nil
	}
	for _, s := range r.file.Statements.Nodes {
		if (s.Kind == ast.KindTypeAliasDeclaration || s.Kind == ast.KindInterfaceDeclaration) && s.Name().Text() == name.Text() && s.TypeParameters() == nil {
			return s
		}
	}
	return nil
}

// classOfType reads the forms a written parameter type accepts: an Effect
// is a program; a function type is an Effect function (callbackClass); a
// union accepts what its members accept. A type parameter, an imported
// type, or anything else carries no class.
func (r *Resolver) classOfType(t *ast.Node, depth int) Class {
	t = formsUnwrapType(t)
	if t == nil || depth > 4 {
		return 0
	}
	switch t.Kind {
	case ast.KindUnionType:
		var c Class
		for _, m := range t.AsUnionTypeNode().Types.Nodes {
			c |= r.classOfType(m, depth+1)
		}
		return c
	case ast.KindFunctionType:
		return r.callbackClass(t.Type())
	case ast.KindTypeReference:
		if r.isEffectTypeNode(t, depth) {
			return ClassProgram
		}
		if d := r.localType(t); d != nil && d.Kind == ast.KindTypeAliasDeclaration {
			return r.classOfType(d.Type(), depth+1)
		}
	}
	return 0
}

// callbackClass is the class of a function type written in a parameter of
// a declaration in this file, whose written return type is ret. A callback
// type without an Effect contract accepts any contract (D12, D68): the fork
// reads (a: A) => B as (a: A) => Effect<B, E, R>, so a lambda passed there
// is an Effect function. A union with an Effect accepts either form.
func (r *Resolver) callbackClass(ret *ast.Node) Class {
	ret = formsUnwrapType(ret)
	if ret != nil && ret.Kind == ast.KindUnionType {
		for _, m := range ret.AsUnionTypeNode().Types.Nodes {
			if r.isEffectTypeNode(m, 0) {
				return ClassEffectFn | ClassPlainFn
			}
		}
	}
	return ClassEffectFn
}

// memberClass is the class of a service member whose written result is
// ret: an Effect function when ret is an Effect or a contract, as every
// member of a service or control is (D128), and plain otherwise.
func (r *Resolver) memberClass(ret *ast.Node) Class {
	ret = formsUnwrapType(ret)
	switch {
	case ret == nil:
		return ClassEffectFn
	case r.isEffectTypeNode(ret, 0):
		return ClassEffectFn
	case ret.Kind == ast.KindUnionType:
		for _, m := range ret.AsUnionTypeNode().Types.Nodes {
			if r.isEffectTypeNode(m, 0) {
				return ClassEffectFn | ClassPlainFn
			}
		}
	}
	return ClassPlainFn
}

// isEffectTypeNode reports whether written type t is effect's Effect: an
// Effeck contract (A!E!R), Effect.Effect<…> through an import of effect or
// effect/Effect or through the prelude, or the prelude's Effect<…> (D4).
func (r *Resolver) isEffectTypeNode(t *ast.Node, depth int) bool {
	t = formsUnwrapType(t)
	if t == nil || t.Kind != ast.KindTypeReference {
		return false
	}
	name := t.AsTypeReferenceNode().TypeName
	if name.Kind == ast.KindQualifiedName && name.Flags&ast.NodeFlagsReparsed != 0 && len(t.TypeArguments()) == 3 {
		return true // A!E!R
	}
	var parts []string
	for name.Kind == ast.KindQualifiedName {
		q := name.AsQualifiedName()
		parts = append([]string{q.Right.Text()}, parts...)
		name = q.Left
	}
	if !ast.IsIdentifier(name) {
		return false
	}
	root := name.Text()
	decl := r.binding(name, root)
	switch {
	case decl == nil:
		if len(parts) == 0 {
			return root == "Effect" // the prelude's type Effect<A, E, R> (D4)
		}
		return r.canonicalIsEffect("prelude#" + root + "." + strings.Join(parts, "."))
	case decl.Kind == ast.KindImportSpecifier:
		spec := formsModuleSpecifier(decl)
		imported := root
		if pn := decl.AsImportSpecifier().PropertyName; pn != nil {
			imported = pn.Text()
		}
		key := spec + "#" + imported
		if len(parts) > 0 {
			key += "." + strings.Join(parts, ".")
		}
		return r.canonicalIsEffect(key)
	case decl.Kind == ast.KindNamespaceImport && len(parts) > 0:
		return r.canonicalIsEffect(formsModuleSpecifier(decl) + "#" + strings.Join(parts, "."))
	case decl.Kind == ast.KindTypeAliasDeclaration && depth < 4:
		return r.isEffectTypeNode(decl.Type(), depth+1)
	}
	return false
}

func (r *Resolver) canonicalIsEffect(key string) bool {
	if key == "effect/Effect#Effect" || key == "effect#Effect.Effect" {
		return true
	}
	return r.table != nil && r.table.Canonical(key) == "effect/Effect#Effect"
}

// annotatedClass returns the class a free lambda's variable annotation
// gives it: const f: (x: number) => Effect<…> = (x) => ….
func (r *Resolver) annotatedClass(fn *ast.Node) Class {
	n := fn
	for n.Parent != nil && n.Parent.Kind == ast.KindParenthesizedExpression {
		n = n.Parent
	}
	if p := n.Parent; p != nil && p.Kind == ast.KindVariableDeclaration && p.Initializer() == n && p.Type() != nil {
		return r.classOfType(p.Type(), 0)
	}
	return 0
}

// annotatedMember decides a lambda or method that is a member of an object
// literal whose type is written: the initializer of an annotated variable
// (const file: LineFile = { readLine: () => … }) or the result of a function
// with a written return type (make(): Banner!!Settings { return { … } }).
// The member's written type in this file's interface, type literal, or
// service gives the class: an Effect function when it returns an Effect.
func (r *Resolver) annotatedMember(fn *ast.Node) (Class, string, bool) {
	member, obj := fn, fn.Parent
	if obj != nil && obj.Kind == ast.KindPropertyAssignment && obj.Initializer() == fn {
		member, obj = obj, obj.Parent
	}
	if obj == nil || obj.Kind != ast.KindObjectLiteralExpression {
		return 0, "", false
	}
	name, ok := formsPropertyName(member)
	if !ok {
		return 0, "", false
	}
	written := r.writtenTypeOf(obj)
	if written == nil {
		return 0, "", false
	}
	if t := formsUnwrapType(written); t != nil && t.Kind == ast.KindTypeReference && t.AsTypeReferenceNode().TypeName.Kind == ast.KindQualifiedName && t.AsTypeReferenceNode().TypeName.Flags&ast.NodeFlagsReparsed != 0 && len(t.TypeArguments()) == 3 {
		written = t.TypeArguments()[0] // a contract A!E!R types its value A
	}
	t := formsUnwrapType(written)
	if t == nil {
		return 0, "", false
	}
	var m *ast.Node
	if t.Kind == ast.KindTypeReference && ast.IsIdentifier(t.AsTypeReferenceNode().TypeName) {
		if d := r.binding(t.AsTypeReferenceNode().TypeName, t.AsTypeReferenceNode().TypeName.Text()); d != nil && formsIsServiceClass(d) {
			m = formsClassMember(d, name)
		}
	}
	if m == nil {
		if lit := r.typeLiteral(t, 0); lit != nil {
			m = formsShapeMember(lit, name)
		}
	}
	if m == nil {
		return 0, "", false
	}
	note := "member " + name + " of a written type"
	switch m.Kind {
	case ast.KindMethodDeclaration, ast.KindMethodSignature:
		if m.Type() == nil {
			return 0, "", false
		}
		return r.memberClass(m.Type()), note, true
	case ast.KindPropertyDeclaration, ast.KindPropertySignature:
		if mt := formsUnwrapType(m.Type()); mt != nil && mt.Kind == ast.KindFunctionType {
			return r.memberClass(mt.Type()), note, true
		}
	}
	return 0, "", false
}

// writtenTypeOf returns the type written for the value of expr: the
// annotation of the variable it initializes, or the return type of the
// function whose return statement or expression body it is.
func (r *Resolver) writtenTypeOf(expr *ast.Node) *ast.Node {
	up := expr.Parent
	for up != nil && (up.Kind == ast.KindParenthesizedExpression || up.Kind == ast.KindSatisfiesExpression) {
		up = up.Parent
	}
	switch {
	case up == nil:
		return nil
	case up.Kind == ast.KindVariableDeclaration && up.Type() != nil:
		return up.Type()
	case up.Kind == ast.KindReturnStatement:
		for fn := up.Parent; fn != nil; fn = fn.Parent {
			if formsBoundary(fn) {
				if ast.IsFunctionLike(fn) {
					return fn.Type()
				}
				return nil
			}
		}
	case ast.IsArrowFunction(up) && up.Body() != nil && ast.SkipParentheses(up.Body()) == expr:
		return up.Type()
	}
	return nil
}

// serviceMember decides a lambda or method in a service context: a member
// of an object literal that a provider call takes, or that a program
// passed to Layer.effect returns.
func (r *Resolver) serviceMember(fn *ast.Node) (Class, string, bool) {
	member, obj := fn, fn.Parent
	if obj != nil && obj.Kind == ast.KindPropertyAssignment && obj.Initializer() == fn {
		member, obj = obj, obj.Parent
	}
	for obj != nil && obj.Kind == ast.KindParenthesizedExpression {
		obj = obj.Parent
	}
	if obj == nil || obj.Kind != ast.KindObjectLiteralExpression {
		return 0, "", false
	}
	name, ok := formsPropertyName(member)
	if !ok {
		return 0, "", false
	}
	key := r.providedKey(obj)
	if key == nil {
		return 0, "", false
	}
	root, chain, ok := formsNameChain(key)
	if ok && len(chain) == 0 {
		if class := r.serviceClassOf(root); class != nil {
			if m := formsClassMember(class, name); m != nil {
				switch m.Kind {
				case ast.KindMethodDeclaration:
					return r.memberClass(m.Type()), "member " + class.Name().Text() + "." + name, true
				case ast.KindPropertyDeclaration:
					if t := formsUnwrapType(m.Type()); t != nil && t.Kind == ast.KindFunctionType {
						return r.memberClass(t.Type()), "member " + class.Name().Text() + "." + name, true
					}
					return r.classOfType(m.Type(), 0), "member " + class.Name().Text() + "." + name, true
				}
			}
		}
	}
	if ok && len(chain) == 0 {
		if d := r.binding(root, root.Text()); d != nil && d.Kind == ast.KindClassDeclaration {
			if shape := r.typeLiteral(formsServiceShape(d), 0); shape != nil {
				if m := formsShapeMember(shape, name); m != nil {
					note := "member " + d.Name().Text() + "." + name
					switch m.Kind {
					case ast.KindMethodSignature:
						return r.memberClass(m.Type()), note, true
					case ast.KindPropertySignature:
						if t := formsUnwrapType(m.Type()); t != nil && t.Kind == ast.KindFunctionType {
							return r.memberClass(t.Type()), note, true
						}
						return r.classOfType(m.Type(), 0), note, true
					}
				}
			}
		}
	}
	if ok {
		if d := r.binding(root, root.Text()); d != nil && (d.Kind == ast.KindImportSpecifier || d.Kind == ast.KindNamespaceImport || d.Kind == ast.KindImportClause) {
			if spec := formsModuleSpecifier(d); strings.HasPrefix(spec, ".") && strings.HasSuffix(spec, ".effeck") {
				return ClassEffectFn, "provider of a service from " + spec, true
			}
		}
	}
	return 0, "", false
}

// providedKey returns the expression naming the service that obj, an
// object literal, implements, or nil when obj is not a provider.
func (r *Resolver) providedKey(obj *ast.Node) *ast.Node {
	up := obj.Parent
	for up != nil && up.Kind == ast.KindParenthesizedExpression {
		up = up.Parent
	}
	if up == nil {
		return nil
	}
	if ast.IsCallExpression(up) {
		return r.providerKeyArgument(up, obj)
	}
	if up.Kind == ast.KindReturnStatement || formsBoundary(up) && up.Body() == obj {
		// The object a program returns: find the enclosing lambda and its call.
		fn := up
		for fn != nil && !formsBoundary(fn) {
			fn = fn.Parent
		}
		if fn == nil || !(fn.Kind == ast.KindArrowFunction || fn.Kind == ast.KindFunctionExpression) {
			return nil
		}
		if call, _, steps := formsOwnerCall(fn); call != nil && len(steps) == 0 {
			return r.providerKeyArgument(call, fn)
		}
	}
	return nil
}

// providerKeyArgument returns the key argument of call when call is a
// provider call and arg is the argument that implements the service.
func (r *Resolver) providerKeyArgument(call, arg *ast.Node) *ast.Node {
	args := call.Arguments()
	index := -1
	for i, a := range args {
		if ast.SkipParentheses(a) == arg || a == arg {
			index = i
		}
	}
	if index < 0 {
		return nil
	}
	if e := ast.SkipParentheses(call.Expression()); ast.IsPropertyAccessExpression(e) && e.Name().Text() == "of" && index == 0 {
		return e.Expression() // K.of({ … })
	}
	c := r.Callee(call)
	if c.Kind != CalleeTable {
		return nil
	}
	at, ok := formsProviderCallees[c.Canonical]
	if !ok {
		return nil
	}
	if at >= len(args) || at >= index {
		at = index - 1 // a data-last form: the key comes just before the value
	}
	if at < 0 {
		return nil
	}
	return ast.SkipParentheses(args[at])
}

// serviceClassOf returns the service class this file declares that id
// names where it stands.
func (r *Resolver) serviceClassOf(id *ast.Node) *ast.Node {
	d := r.binding(id, id.Text())
	if d != nil && formsIsServiceClass(d) {
		return d
	}
	return nil
}

// formsIsServiceClass reports whether node declares a service or control: an
// abstract class with no type parameters, no implements clause, no extends
// clause other than Control, and only abstract instance members (D1, D128).
// The parser writes service and control declarations in this shape.
func formsIsServiceClass(node *ast.Node) bool {
	if node == nil || node.Kind != ast.KindClassDeclaration || node.Name() == nil || node.TypeParameters() != nil {
		return false
	}
	if !ast.HasSyntacticModifier(node, ast.ModifierFlagsAbstract) || len(ast.GetImplementsHeritageClauseElements(node)) != 0 {
		return false
	}
	if extends := ast.GetClassExtendsHeritageElement(node); extends != nil {
		if e := extends.Expression(); !ast.IsIdentifier(e) || e.Text() != "Control" {
			return false
		}
	}
	members := 0
	for _, m := range node.Members() {
		if ast.IsSemicolonClassElement(m) || ast.IsStatic(m) {
			continue
		}
		if !ast.IsMethodDeclaration(m) && !ast.IsPropertyDeclaration(m) || !ast.HasSyntacticModifier(m, ast.ModifierFlagsAbstract) {
			return false
		}
		members++
	}
	return members > 0
}

// formsServiceShape returns the written shape of a service class declared
// TypeScript's way, class K extends Context.Service<K, Shape>()("K") {}:
// the second type argument of the call its extends clause calls. It
// returns nil for any other class.
func formsServiceShape(class *ast.Node) *ast.Node {
	extends := ast.GetClassExtendsHeritageElement(class)
	if extends == nil {
		return nil
	}
	outer := ast.SkipParentheses(extends.Expression())
	if !ast.IsCallExpression(outer) {
		return nil
	}
	inner := ast.SkipParentheses(outer.Expression())
	if !ast.IsCallExpression(inner) || len(inner.TypeArguments()) != 2 {
		return nil
	}
	_, chain, ok := formsNameChain(inner.Expression())
	if !ok || len(chain) == 0 || chain[len(chain)-1] != "Service" && chain[len(chain)-1] != "Tag" {
		return nil
	}
	return inner.TypeArguments()[1]
}

// formsShapeMember returns the member named name of a written object type.
func formsShapeMember(shape *ast.Node, name string) *ast.Node {
	for _, m := range shape.Members() {
		if n, ok := formsPropertyName(m); ok && n == name {
			return m
		}
	}
	return nil
}

// formsClassMember returns class's instance member named name.
func formsClassMember(class *ast.Node, name string) *ast.Node {
	for _, m := range class.Members() {
		if n, ok := formsPropertyName(m); ok && n == name && !ast.IsStatic(m) {
			return m
		}
	}
	return nil
}
