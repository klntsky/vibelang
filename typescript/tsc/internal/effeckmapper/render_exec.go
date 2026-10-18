package effeckmapper

import (
	"slices"
	"strconv"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// The executable rendering: the plan rendered as the code that runs. Its
// semantics are the checking rendering's; only the way a call runs differs,
// so that the JavaScript is fast where the virtual TypeScript only types.
//
//   - A call that runs tests its value at run time instead of yielding it
//     unconditionally (L3): (($t = c), $isRun($t) ? (yield* $t) : $t).
//   - A top-level function whose body is provably pure from syntax (D10,
//     execFunction.pure) compiles to f$direct, plain JavaScript, plus f, the
//     Effect wrapper, with f[$direct] = f$direct. A same-file call that runs
//     calls f$direct; a call of a function imported from another module looks
//     the property up, so no type from another file is needed.
//   - Any other top-level Effect function compiles to its generator, f$body,
//     the wrapper f, and the fast-call entry f$fast = $fast(f$body), which a
//     same-file call that runs uses (research/typedirected.md section 6.6).
//   - A function in value position is always the wrapper f (L2): the name f
//     keeps its meaning, so xs.map(double) passes the Effect function, as the
//     virtual TypeScript types it.
//
// The rendering itself, renderExec, maps one edit to text and knows nothing
// of the file. The lowering pass before it, execLowering, makes the
// file-level decisions (which functions split, which calls are direct) and
// records them as edits of their own, so the rendering stays one function of
// one edit.

// renderExec returns the executable text of e, with helper references
// unresolved, or keep false to leave e out. It is the default for each kind;
// the lowering pass rewrites the edits whose text depends on the file.
func renderExec(e edit) (text string, keep bool) {
	t, s := helperRef("$t"), helperRef("$s")
	switch e.kind {
	case editWitness:
		return "", false
	case editSyntax, editRegion, editDispatch:
		// A region, its completion records, and their dispatch run as the
		// checking rendering writes them: $end, $jump, $returned, and
		// $jumped have run-time definitions (execPrologue), and $box and
		// $unboxed are the identity, which resolves to nothing.
		return e.text, true
	case editRecord:
		switch e.arg {
		case "return value":
			// return x: the value itself is the completion.
			return pick(e.part, "return ", "", ""), true
		case "return":
			return "", true
		}
		return e.text, true
	case editRun:
		// L3: an Effect runs, any other value is the call's value.
		return pick(e.part, "(("+t+" = ", "", "), "+helperRef("$isRun")+"("+t+") ? (yield* "+t+") : "+t+")"), true
	case editEffectBody:
		gen := generatorText(e.arg)
		switch {
		case strings.HasPrefix(e.arg, "expr"):
			// (x) => expr
			return pick(e.part, gen+"{ return ", "", " })"), true
		case strings.HasPrefix(e.arg, "block"):
			// a block param in the Effect function form
			return pick(e.part, "() => "+gen, "", ")"), true
		}
		// a function's braced body
		return pick(e.part, " return "+gen+"{", "", "})"), true
	case editContract:
		// The contract's slots stay in a return annotation, which the
		// transpiler erases: f(p): $Contract<A, $Infer, R> { return $gen(...) }.
		gen := helperRef("$gen") + "(function* () "
		if e.arg == "self" {
			gen = helperRef("$gen") + "({ self: this }, function* () "
		}
		return pick(e.part, ": $Contract<", "> { return "+gen, ")}"), true
	case editProgram:
		if strings.HasPrefix(e.arg, "expr") {
			return pick(e.part, generatorText(e.arg)+"{ return ", "", " })"), true
		}
		return pick(e.part, generatorText(e.arg), "", ")"), true
	case editDual:
		// L1's dual: the program, also a function that returns it.
		return pick(e.part, helperRef("$dual")+"(", "", ")"), true
	case editScoped:
		// A fork in a block: the runtime's fork already forks into the Scope
		// around it, which scopeBodies gives the function.
		return "", true
	case editProgramValue:
		// D7: a function value at a program position runs as its program.
		return pick(e.part, helperRef("$asProgram")+"(", "", ")"), true
	case editProducer:
		// D26: the runtime's $produce takes the body as a function that
		// returns its Effect, and runs it under its Yield handler.
		return pick(e.part, " return "+helperRef("$produce")+"(() => "+generatorText(e.arg)+"{", "", "}))"), true
	case editEmit:
		// The source's yield stays; yield x becomes yield* $yield(x), the
		// runtime's Yield operation, and yield* p yield* $yieldAll(p).
		switch e.arg {
		case "delegate":
			return pick(e.part, helperRef("$yieldAll")+"(", "", ")"), true
		case "undefined":
			return "* " + helperRef("$yield") + "(undefined)", true
		}
		return pick(e.part, "* "+helperRef("$yield")+"(", "", ")"), true
	case editEach:
		// for (const x of (yield* $each(xs))): a Stream's elements, or an
		// iterable as itself (execRendering.prologue).
		return pick(e.part, "(yield* "+helperRef("$each")+"(", "", "))"), true
	case editFail:
		if e.arg == "void" {
			return pick(e.part, "return (yield* "+helperRef("$fail")+"(", "", "))"), true
		}
		return pick(e.part, "return yield* "+helperRef("$fail")+"(", "", ")"), true
	case editServiceCall:
		// D41: a Context key's member call reads the service first.
		return pick(e.part, "(("+s+" = ", "", "), "+helperRef("$isKey")+"("+s+") ? (yield* "+s+") : "+s+")"), true
	case editValueCall:
		// D90: a function is called; an Effect value, such as a key or a
		// Config, becomes a function whose result runs it.
		return pick(e.part, helperRef("$value")+"(", "", ")"), true
	case editUsing:
		// using x = acquire(): the acquisition runs in the Scope of the code
		// around it (execPrologue's $acquire).
		return pick(e.part, "(yield* "+helperRef("$acquire")+"(", "", "))"), true
	}
	panic("exec: kind " + e.kind.String() + " not implemented")
}

// generatorText opens the generator of an Effect body or program whose
// edit has arg: one that receives the function's this when arg says self.
func generatorText(arg string) string {
	if strings.Contains(arg, "self") {
		return helperRef("$gen") + "({ self: this }, function* () "
	}
	return helperRef("$gen") + "(function* () "
}

// execRendering renders a lowered plan: renderExec's text with each helper
// reference resolved to a local name that no identifier of the file spells,
// and the prefix that imports or defines each helper it used.
type execRendering struct {
	names *names
	// used holds the helpers the rendering resolved, by name.
	used map[string]bool
}

func newExecRendering(n *names) *execRendering {
	return &execRendering{names: n, used: map[string]bool{}}
}

func (r *execRendering) text(e edit) (string, bool) {
	text, keep := renderExec(e)
	if !keep {
		return "", false
	}
	return r.resolve(text), true
}

// resolve replaces each helper reference in text with the helper's local name.
func (r *execRendering) resolve(text string) string {
	var out strings.Builder
	for {
		start := strings.IndexByte(text, '\x01')
		if start < 0 {
			out.WriteString(text)
			return out.String()
		}
		end := strings.IndexByte(text[start:], '\x02')
		if end < 0 {
			out.WriteString(text)
			return out.String()
		}
		end += start
		out.WriteString(text[:start])
		out.WriteString(r.local(text[start+1 : end]))
		text = text[end+1:]
	}
}

// local returns the local name of helper and records its use. The identity
// helpers resolve to nothing, and helpers that are only types to their own
// names, which the transpiler erases with the annotation they stand in.
func (r *execRendering) local(helper string) string {
	switch {
	case identityHelpers[helper]:
		return ""
	case typeHelpers[helper]:
		return helper
	}
	r.used[helper] = true
	return r.names.helper(helper)
}

// identityHelpers are checking helpers that are the identity at run time:
// a region's boxed return value is the value (prelude.d.ts, $box).
var identityHelpers = map[string]bool{"$box": true, "$unboxed": true}

// typeHelpers are helper names that only appear in types.
var typeHelpers = map[string]bool{"$Infer": true, "$PreludeLoaded": true, "$Contract": true}

// execSource says where the executable rendering's import of a helper comes
// from: an export of a module, or, for prologue helpers and temporaries, the
// prefix itself.
type execSource struct {
	module string
	export string
}

// execHelpers are the helpers whose run-time source differs from their name
// or from the runtime. Every other helper that starts with $ is an export of
// effeck-runtime under its own name.
var execHelpers = map[string]execSource{
	"$gen":        {"effect/Effect", "gen"},
	"$fail":       {"effect/Effect", "fail"},
	"$sync":       {"effect/Effect", "sync"},
	"$fnUntraced": {"effect/Effect", "fnUntraced"},
	"$scopedBody": {"effect/Effect", "scoped"},
	"$suspend":    {"effect/Effect", "suspend"},
	"$isStream":   {"effect/Stream", "isStream"},
	"$runCollect": {"effect/Stream", "runCollect"},
	"$isKey":      {"effect/Context", "isKey"},
	"$isEffect":   {"effect/Effect", "isEffect"},
	"$succeed":    {"effect/Effect", "succeed"},
	"$ScopeKey":   {"effect/Scope", "Scope"},
}

// prologueHelpers are defined by the prefix of each file that uses them
// (execRendering.prologue); temporaries are declared there too.
var prologueHelpers = map[string]bool{
	"$end": true, "$Jump": true, "$jump": true, "$returned": true, "$jumped": true, "$value": true, "$acquire": true,
	"$dual": true, "$asProgram": true, "$each": true,
	"$t": true, "$s": true,
}

// d151Names are the Effect functions a .effeck file calls without the
// Effect. prefix (prelude.d.ts), imported from effect/Effect.
var d151Names = setOf(`all die exit forEach log logDebug logError logInfo logWarning race repeat retry sleep timeout withSpan`)

// runtimePreludeNames are the runtime's prelude names (prelude.d.ts),
// imported from effeck-runtime.
var runtimePreludeNames = setOf(`TaggedError Control pipe wire fork handle resume resumeWith abandon sync callback Resume runPromise
	runFork runSync runCallback detach forkIn interrupt yieldNow uninterruptible restore interruptible customMatcher`)

func setOf(list string) map[string]bool {
	set := map[string]bool{}
	for _, name := range strings.Fields(list) {
		set[name] = true
	}
	return set
}

// sourceOf returns the module and export the executable rendering imports
// name from, a prelude name or a helper.
func sourceOf(name string) execSource {
	if s, ok := execHelpers[name]; ok {
		return s
	}
	switch {
	case runtimePreludeNames[name], strings.HasPrefix(name, "$"):
		return execSource{"effeck-runtime", name}
	case d151Names[name]:
		return execSource{"effect/Effect", name}
	}
	return execSource{"effect", name}
}

// prefix returns the text before the file's first line: the imports of the
// prelude names and helpers the file uses, from effect and effeck-runtime,
// then the temporaries and the prologue helpers. It runs after the edits
// are rendered, which records the helpers they use.
func (r *execRendering) prefix() string {
	prologue := r.prologue()
	byModule := map[string][]string{}
	for export, local := range r.names.imports {
		name := strings.TrimPrefix(export, "\x00")
		if typeOnlyPrelude[name] || typeHelpers[name] || prologueHelpers[name] || identityHelpers[name] {
			continue
		}
		if strings.HasPrefix(name, "$") && !r.used[name] {
			// A helper the checking rendering would use; this one does not.
			continue
		}
		source := sourceOf(name)
		specifier := source.export
		if local != source.export {
			specifier += " as " + local
		}
		byModule[source.module] = append(byModule[source.module], specifier)
	}
	modules := make([]string, 0, len(byModule))
	for module := range byModule {
		modules = append(modules, module)
	}
	slices.Sort(modules)
	var out strings.Builder
	for _, module := range modules {
		specifiers := byModule[module]
		slices.Sort(specifiers)
		out.WriteString("import { " + strings.Join(specifiers, ", ") + " } from " + strconv.Quote(module) + "; ")
	}
	out.WriteString(prologue)
	if out.Len() > 0 {
		out.WriteString("\n")
	}
	return out.String()
}

// prologue returns the declarations of the temporaries and prologue helpers
// the rendering used. A region's completion is the returned value itself,
// the unique $end for a normal end, or a $Jump for a break or continue that
// leaves it; the dispatch after the region tests it with $returned and
// $jumped, as the checking rendering writes (regions.go). The definitions
// are a file's own, because a completion never leaves the function whose
// region made it.
func (r *execRendering) prologue() string {
	var parts []string
	var temps []string
	for _, temp := range []string{"$t", "$s"} {
		if r.used[temp] {
			temps = append(temps, r.names.helper(temp))
		}
	}
	if len(temps) > 0 {
		// var, not let: a module-level let costs a TDZ check at every use.
		parts = append(parts, "var "+strings.Join(temps, ", ")+";")
	}
	needsJump := r.used["$jump"] || r.used["$returned"] || r.used["$jumped"]
	if r.used["$end"] || r.used["$returned"] {
		parts = append(parts, "const "+r.local("$end")+" = {};")
	}
	if needsJump {
		jump := r.local("$Jump")
		parts = append(parts, "class "+jump+" { constructor(k) { this.k = k } }")
		if r.used["$jump"] {
			parts = append(parts, "const "+r.local("$jump")+" = (k) => new "+jump+"(k);")
		}
		if r.used["$returned"] {
			parts = append(parts, "const "+r.local("$returned")+" = (c) => c !== "+r.local("$end")+" && !(c instanceof "+jump+");")
		}
		if r.used["$jumped"] {
			parts = append(parts, "const "+r.local("$jumped")+" = (c, k) => c instanceof "+jump+" && c.k === k;")
		}
	}
	if r.used["$value"] {
		// A Context key is a class, so a function that is a key runs too.
		parts = append(parts, "const "+r.local("$value")+" = (v) => typeof v === \"function\" && !"+r.local("$isKey")+"(v) ? v : () => "+r.local("$gen")+"(function* () { return yield* v });")
	}
	if r.used["$acquire"] {
		// The plan marks no block as a Scope region yet, so an acquisition
		// uses the Scope its caller provides; a Disposable is acquired as
		// itself and released with the Scope (the runtime's $using).
		parts = append(parts, "const "+r.local("$acquire")+" = (v) => "+r.local("$gen")+"(function* () { return yield* "+r.local("$using")+
			"(yield* "+r.local("$ScopeKey")+", "+r.local("$isEffect")+"(v) ? v : "+r.local("$succeed")+"(v)) });")
	}
	if r.used["$dual"] {
		// research/typedirected.md stock/perf/dual.mjs: the function takes the
		// program's prototype and own properties, so it is that Effect too.
		parts = append(parts, "const "+r.local("$dual")+" = (program) => { const f = () => program; Object.setPrototypeOf(f, Object.getPrototypeOf(program)); "+
			"for (const k of Reflect.ownKeys(program)) Object.defineProperty(f, k, Object.getOwnPropertyDescriptor(program, k)); return f };")
	}
	if r.used["$each"] {
		// The checking rendering iterates what $each gives, all of a
		// Stream's elements, so a Stream runs to its end before the loop's
		// first iteration; the plan has no loop region that would let a
		// producer and its loop take turns, as the fork's $forOf does.
		parts = append(parts, "const "+r.local("$each")+" = (v) => "+r.local("$isStream")+"(v) ? "+r.local("$runCollect")+"(v) : "+r.local("$succeed")+"(v);")
	}
	if r.used["$asProgram"] {
		parts = append(parts, "const "+r.local("$asProgram")+" = (v) => typeof v === \"function\" && !"+r.local("$isEffect")+"(v) && !"+r.local("$isKey")+"(v) ? "+r.local("$suspend")+"(v) : v;")
	}
	return strings.Join(parts, " ")
}

// ---------------------------------------------------------------- lowering

// span is the edits of one paired kind, by index in the plan: its open,
// middle (-1 when the kind has none), and close edit.
type span struct {
	open, middle, close int
}

// execFunction is a top-level function declaration that the executable
// rendering splits.
type execFunction struct {
	node *ast.Node
	name string
	// pure: the body is provably pure from syntax, so f compiles to
	// f$direct and a wrapper (D10); otherwise to f$body, f, and f$fast.
	pure bool
	// direct is f$direct or f$fast, the name a same-file call uses; body is
	// the generator's name, f$body.
	direct, body string
	// effectBody or contract is the function's own Effect body edits.
	effectBody, contract *span
}

// execLowering rewrites a projector's plan for the executable rendering.
type execLowering struct {
	p     *projector
	edits []edit
	// spans pairs each run, effect body, contract, program, fail, service
	// call, and value call with its other parts.
	spans map[editKind][]span
	// calls are the file's call expressions by [token start, end), and
	// receivers the identifiers that are the object of a member access.
	calls     map[[2]int]*ast.Node
	receivers map[[2]int]*ast.Node
	// valueCalls holds the callee positions of D90 value calls.
	valueCalls map[int]bool
	functions  map[string]*execFunction
	// scoped holds the open edit of each Effect body or program with a
	// using declaration or a block fork of its own (scopeBodies).
	scoped map[int]bool
	args   string
}

// execPlan returns p's plan lowered for the executable rendering.
func (p *projector) execPlan() *plan {
	l := &execLowering{
		p:          p,
		edits:      slices.Clone(p.plan.edits),
		calls:      map[[2]int]*ast.Node{},
		receivers:  map[[2]int]*ast.Node{},
		valueCalls: map[int]bool{},
		functions:  map[string]*execFunction{},
	}
	l.pair()
	l.collect(p.file.AsNode())
	for _, s := range l.spans[editValueCall] {
		l.valueCalls[l.edits[s.open].pos] = true
	}
	l.args = p.freshWord("$args")
	l.scopeBodies()
	l.splitFunctions()
	l.lowerCalls()
	l.lowerServiceCalls()
	l.lowerDeclarations()
	l.lowerImports()
	l.entry()
	return &plan{edits: l.edits}
}

// pair matches the parts of each paired kind. The walk records an open
// edit, then the edits of what it encloses, then the close, so in record
// order a close belongs to the latest open of its kind still unclosed.
func (l *execLowering) pair() {
	l.spans = map[editKind][]span{}
	stacks := map[editKind][]span{}
	for i, e := range l.edits {
		switch e.kind {
		// editEmit is left out: a bare yield is one edit with no close.
		case editRun, editEffectBody, editContract, editProgram, editFail, editServiceCall, editValueCall, editUsing, editDual, editScoped,
			editProgramValue, editProducer, editEach:
		default:
			continue
		}
		stack := stacks[e.kind]
		switch e.part {
		case partOpen:
			stacks[e.kind] = append(stack, span{open: i, middle: -1, close: -1})
		case partMiddle:
			if len(stack) > 0 {
				stack[len(stack)-1].middle = i
			}
		case partClose:
			if len(stack) == 0 {
				continue
			}
			s := stack[len(stack)-1]
			s.close = i
			stacks[e.kind] = stack[:len(stack)-1]
			l.spans[e.kind] = append(l.spans[e.kind], s)
		}
	}
}

// collect indexes the call expressions and member-access receivers under
// node by their spans, which is how the plan's edits locate them.
func (l *execLowering) collect(node *ast.Node) {
	node.ForEachChild(func(child *ast.Node) bool {
		switch child.Kind {
		case ast.KindCallExpression:
			l.calls[[2]int{l.p.tokenStart(child), child.End()}] = child
		case ast.KindPropertyAccessExpression:
			if receiver := child.Expression(); receiver.Kind == ast.KindIdentifier {
				l.receivers[[2]int{l.p.tokenStart(receiver), receiver.End()}] = receiver
			}
		}
		l.collect(child)
		return false
	})
}

// set replaces edit i with syntax text, keeping its position.
func (l *execLowering) set(i int, text string) {
	e := l.edits[i]
	l.edits[i] = edit{pos: e.pos, end: e.end, text: text}
}

// insert adds syntax text before the original character at pos. Among
// edits at one position it renders after those the walk recorded.
func (l *execLowering) insert(pos int, text string) {
	l.edits = append(l.edits, edit{pos: pos, end: pos, text: text})
}

// replace adds syntax text in place of the original text in [pos, end).
func (l *execLowering) replace(pos int, end int, text string) {
	l.edits = append(l.edits, edit{pos: pos, end: end, text: text})
}

// splitFunctions decides how each top-level Effect function compiles and
// rewrites its declaration. A function is a candidate when it has a body,
// is projected as an Effect function (no plain return annotation), and is
// the file's only declaration of its name, so no overload, parameter, or
// inner declaration shadows it at a call. The pure candidates are the
// greatest fixpoint of pureBody over the candidates (D56's rule, from
// syntax): a group of functions that call only one another and pure
// globals is pure, so pure recursion runs on the JavaScript stack.
func (l *execLowering) splitFunctions() {
	bodies := map[int]*span{}
	for i := range l.spans[editEffectBody] {
		s := &l.spans[editEffectBody][i]
		bodies[l.edits[s.open].pos] = s
	}
	contracts := map[int]*span{}
	for i := range l.spans[editContract] {
		s := &l.spans[editContract][i]
		contracts[l.edits[s.close].pos] = s
	}
	var order []*execFunction
	for _, statement := range l.p.file.Statements.Nodes {
		if !isProjectedFunction(statement) || statement.Name() == nil {
			continue
		}
		name := statement.Name().Text()
		if l.p.names.declarations[name] != 1 || ast.HasSyntacticModifier(statement, ast.ModifierFlagsAmbient) {
			continue
		}
		body := statement.Body()
		f := &execFunction{node: statement, name: name}
		if s := bodies[l.p.tokenStart(body)+1]; s != nil {
			f.effectBody = s
		} else if s := contracts[body.End()]; s != nil {
			f.contract = s
		} else {
			continue // a plain function (L2), or one the walk did not lower
		}
		if f.effectBody != nil && l.scoped[f.effectBody.open] {
			continue // its body is scoped, so it runs only as an Effect (scopeBodies)
		}
		l.functions[name] = f
		order = append(order, f)
	}
	pure := map[string]bool{}
	for _, f := range order {
		pure[f.name] = true
	}
	for changed := true; changed; {
		changed = false
		for _, f := range order {
			if pure[f.name] && !l.pureBody(f, pure) {
				pure[f.name] = false
				changed = true
			}
		}
	}
	for _, f := range order {
		f.pure = pure[f.name]
		l.splitFunction(f)
	}
}

// scopeBodies gives each Effect body or program that holds a using
// declaration or a fork in a block, its own or in a block or region inside
// it, a Scope of its own:
//
//	function f() { using r = lease(); ... }
//	function f() { return $scopedBody($gen(function* () { const r = (yield* $acquire(lease())); ... })) }
//
// The checking rendering removes Scope from the R of the using's or the
// fork's block (D14, D19), so the acquisition or the fork needs a Scope the
// caller does not provide. The plan marks no block as a Scope region yet,
// so the Scope is the function's: exact for a using or fork in the
// function's own block, and late, at the function's end, for one in a block
// inside it. Such a function runs only as an Effect, without a fast entry,
// so the Scope always closes.
func (l *execLowering) scopeBodies() {
	l.scoped = map[int]bool{}
	var bodies []span
	bodies = append(bodies, l.spans[editEffectBody]...)
	bodies = append(bodies, l.spans[editProgram]...)
	bodies = append(bodies, l.spans[editProducer]...)
	var needs []span
	needs = append(needs, l.spans[editUsing]...)
	needs = append(needs, l.spans[editScoped]...)
	for _, u := range needs {
		at := l.edits[u.open].pos
		innermost := -1
		for i, b := range bodies {
			if l.edits[b.open].pos <= at && at <= l.edits[b.close].pos &&
				(innermost < 0 || l.edits[b.open].pos > l.edits[bodies[innermost].open].pos) {
				innermost = i
			}
		}
		if innermost < 0 || l.scoped[bodies[innermost].open] {
			continue
		}
		b := bodies[innermost]
		l.scoped[b.open] = true
		open, _ := renderExec(l.edits[b.open])
		close, _ := renderExec(l.edits[b.close])
		gen := strings.Index(open, helperRef("$gen"))
		if gen < 0 {
			continue
		}
		l.set(b.open, open[:gen]+helperRef("$scopedBody")+"("+open[gen:])
		l.set(b.close, close+")")
	}
}

// pureBody reports whether f's body, read from syntax, behaves the same as
// plain JavaScript and as an Effect body, given that the functions in pure
// are pure: it calls only those functions and pure globals, and it has no
// throw, try (a region), yield, await, using, this, arguments, or Effeck
// syntax. Nested functions and classes are not part of the body. The edits
// inside the body must be calls or syntax, so a form the walk lowers some
// other way is never taken for plain code.
func (l *execLowering) pureBody(f *execFunction, pure map[string]bool) bool {
	body := f.node.Body()
	lo, hi := l.p.tokenStart(body)+1, body.End()-1
	own := map[int]bool{}
	if f.effectBody != nil {
		own[f.effectBody.open], own[f.effectBody.close] = true, true
	}
	for i, e := range l.edits {
		if e.pos < lo || e.pos > hi || own[i] {
			continue
		}
		if e.kind != editRun && e.kind != editSyntax {
			return false
		}
	}
	var visit func(node *ast.Node) bool // true: impure
	visit = func(node *ast.Node) bool {
		if node.Flags&(ast.NodeFlagsEffeck|ast.NodeFlagsReparsed) != 0 {
			return true
		}
		switch node.Kind {
		case ast.KindThisKeyword, ast.KindSuperKeyword, ast.KindThrowStatement, ast.KindTryStatement,
			ast.KindYieldExpression, ast.KindAwaitExpression, ast.KindTaggedTemplateExpression, ast.KindForOfStatement:
			return true
		case ast.KindIdentifier:
			return node.Text() == "arguments"
		case ast.KindVariableDeclarationList:
			if node.Flags&ast.NodeFlagsUsing != 0 {
				return true
			}
		case ast.KindCallExpression:
			if node.QuestionDotToken() != nil {
				return true
			}
			callee := node.Expression()
			if !(callee.Kind == ast.KindIdentifier && pure[callee.Text()]) && !l.isPureGlobalCall(node) {
				return true
			}
		}
		if ast.IsFunctionLike(node) || ast.IsClassLike(node) {
			return false
		}
		return node.ForEachChild(visit)
	}
	return !body.ForEachChild(visit)
}

// pureGlobalObjects and pureGlobalFunctions are JavaScript globals whose
// calls never return an Effect and run no Effect, so a call of one runs the
// same in plain JavaScript and in an Effect body. A file that declares the
// name shadows it.
var pureGlobalObjects = setOf(`Math Number JSON`)
var pureGlobalFunctions = setOf(`String Number Boolean parseInt parseFloat isNaN isFinite`)
var pureGlobalMembers = map[string]map[string]bool{
	"Object": setOf(`keys values entries`),
	"Array":  setOf(`isArray`),
	"String": setOf(`fromCharCode fromCodePoint`),
}

// isPureGlobalCall reports whether call calls a pure global.
func (l *execLowering) isPureGlobalCall(call *ast.Node) bool {
	if call.QuestionDotToken() != nil {
		return false
	}
	callee := call.Expression()
	switch callee.Kind {
	case ast.KindIdentifier:
		return pureGlobalFunctions[callee.Text()] && !l.p.names.declared[callee.Text()]
	case ast.KindPropertyAccessExpression:
		receiver := callee.Expression()
		if receiver.Kind != ast.KindIdentifier || callee.QuestionDotToken() != nil || l.p.names.declared[receiver.Text()] {
			return false
		}
		return pureGlobalObjects[receiver.Text()] || pureGlobalMembers[receiver.Text()][callee.Name().Text()]
	}
	return false
}

// splitFunction rewrites f's declaration. A pure function:
//
//	export function add(a, b) { return a + b }
//	export function add(...$args) { return $sync(() => add$direct(...$args)) } add[$direct] = add$direct;
//	function add$direct(a, b) { return a + b }
//
// Any other:
//
//	export function f(id) { body }
//	export function f(...$args) { return (f$effect ??= $fnUntraced(f$body)).apply(this, $args) } var f$effect; const f$fast = $fast(f$body);
//	function* f$body(id) { body }
//
// The wrapper is a function declaration, hoisted as the checking
// rendering's function is; f$fast is a const, as the fork's functions are.
// The modifiers move to the wrapper, so f$direct and f$body are module
// locals and an export default stays the wrapper.
func (l *execLowering) splitFunction(f *execFunction) {
	node := f.node
	start := l.p.tokenStart(node)
	keyword := start
	var modifiers []string
	if list := node.Modifiers(); list != nil {
		for _, modifier := range list.Nodes {
			modifiers = append(modifiers, scanner.TokenToString(modifier.Kind))
			keyword = scanner.SkipTrivia(l.p.text, modifier.End())
		}
	}
	if !strings.HasPrefix(l.p.text[keyword:], "function") {
		return
	}
	head := strings.Join(append(modifiers, "function "+f.name), " ")
	args := l.args
	var wrapper string
	if f.pure {
		f.direct = l.p.freshWord(f.name + "$direct")
		wrapper = head + "(..." + args + ") { return " + helperRef("$sync") + "(() => " + f.direct + "(..." + args + ")) } " +
			f.name + "[" + helperRef("$direct") + "] = " + f.direct + ";\n"
		l.insert(node.Name().End(), strings.TrimPrefix(f.direct, f.name))
	} else {
		f.direct = l.p.freshWord(f.name + "$fast")
		f.body = l.p.freshWord(f.name + "$body")
		effect := l.p.freshWord(f.name + "$effect")
		wrapper = head + "(..." + args + ") { return (" + effect + " ??= " + helperRef("$fnUntraced") + "(" + f.body + ")).apply(this, " + args + ") } " +
			"var " + effect + "; const " + f.direct + " = " + helperRef("$fast") + "(" + f.body + ");\n"
		l.insert(keyword+len("function"), "*")
		l.insert(node.Name().End(), strings.TrimPrefix(f.body, f.name))
	}
	l.replace(start, keyword, wrapper)
	// The body is the split function's own, not a generator's.
	if s := f.effectBody; s != nil {
		l.set(s.open, "")
		l.set(s.close, "")
	}
	if s := f.contract; s != nil {
		l.set(s.open, ": $Contract<")
		l.set(s.middle, "> ")
		l.set(s.close, "")
	}
}

// lowerCalls rewrites the calls that run by their callee:
//
//	pure global           Math.max(a, b)
//	same-file pure f      f$direct(x)
//	same-file other f     (($t = f$fast(x)), $isRun($t) ? (yield* $t) : $t)
//	imported f            (($t = (f[$direct] ?? f)(x)), f[$direct] ? $t : ($isRun($t) ? (yield* $t) : $t))
//	any other             renderExec's run check
//
// An imported callee is a name a relative import declares, read through
// its $direct property at the call, so no type of the other file is
// needed; the property is looked up again after the call, because the
// arguments may pause and run other calls in between. A value call (D90)
// keeps its own lowering.
func (l *execLowering) lowerCalls() {
	for _, s := range l.spans[editRun] {
		call := l.calls[[2]int{l.edits[s.open].pos, l.edits[s.close].pos}]
		if call == nil {
			continue
		}
		if l.isPureGlobalCall(call) {
			l.set(s.open, "")
			l.set(s.close, "")
			continue
		}
		callee := call.Expression()
		if callee.Kind != ast.KindIdentifier || call.QuestionDotToken() != nil || callee.Flags&ast.NodeFlagsReparsed != 0 {
			continue
		}
		name := callee.Text()
		if f := l.functions[name]; f != nil {
			if f.pure {
				l.set(s.open, "")
				l.set(s.close, "")
			}
			l.insert(callee.End(), strings.TrimPrefix(f.direct, name))
			continue
		}
		if from, ok := l.p.names.importedFrom[name]; ok && strings.HasPrefix(from, ".") && l.p.names.declarations[name] == 1 && !l.valueCalls[l.p.tokenStart(callee)] {
			t, direct := helperRef("$t"), helperRef("$direct")
			l.set(s.open, "(("+t+" = (")
			l.insert(callee.End(), "["+direct+"] ?? "+name+")")
			l.set(s.close, "), "+name+"["+direct+"] ? "+t+" : ("+helperRef("$isRun")+"("+t+") ? (yield* "+t+") : "+t+"))")
		}
	}
}

// keyMembers are the members a Context key owns itself (L8): a member call
// of one of these on a key calls the key's own member, unless a service the
// file declares declares it.
var keyMembers = setOf(`pipe of use key name length call apply bind toString`)

// lowerServiceCalls rewrites each member call on a possible service key
// (D41) with the member's name: a key's own members, static ones included,
// win where the service the file declares does not declare the member, as
// $use's type says, so the run-time test reads the service only when the
// key has no member of that name:
//
//	(($s = Users), $isKey($s) && !("find" in $s) ? (yield* $s) : $s).find(id)
func (l *execLowering) lowerServiceCalls() {
	for _, s := range l.spans[editServiceCall] {
		open, close := l.edits[s.open], l.edits[s.close]
		receiver := l.receivers[[2]int{open.pos, close.pos}]
		if receiver == nil {
			continue
		}
		member := receiver.Parent.Name().Text()
		declared := false
		if service := l.p.services[receiver.Text()]; service != nil {
			for _, m := range service.Members() {
				if m.Name() != nil && m.Name().Text() == member && !ast.IsStatic(m) {
					declared = true
				}
			}
		}
		if !declared && keyMembers[member] {
			l.set(s.open, "")
			l.set(s.close, "")
			continue
		}
		t := helperRef("$s")
		test := helperRef("$isKey") + "(" + t + ")"
		if !declared {
			test += " && !(" + strconv.Quote(member) + " in " + t + ")"
		}
		l.set(s.close, "), "+test+" ? (yield* "+t+") : "+t+")")
	}
}

// lowerDeclarations adds what a declaration needs at run time beyond its
// checking text: a control registers its operations with their parameter
// counts (the fork's $control), and a service with a static default passes
// its default to $reference (D40).
func (l *execLowering) lowerDeclarations() {
	reference := helperRef("$reference")
	for i, e := range l.edits {
		if e.kind != editSyntax || !strings.Contains(e.text, reference) {
			continue
		}
		at := strings.Index(e.text, reference)
		rest := e.text[at+len(reference):]
		open := strings.Index(rest, ">(")
		closing := strings.Index(rest, ")")
		if !strings.HasPrefix(rest, "<") || open < 0 || closing < open+2 {
			continue
		}
		name := rest[1:open]
		text := e.text[:at+len(reference)] + rest[:closing] + ", () => " + name + ".default" + rest[closing:]
		l.edits[i].text = text
	}
	for _, statement := range l.p.file.Statements.Nodes {
		if !isServiceClass(statement) || ast.GetClassExtendsHeritageElement(statement) == nil {
			continue
		}
		var methods, arities []string
		for _, member := range statement.Members() {
			if !ast.IsMethodDeclaration(member) || ast.IsStatic(member) || member.Name() == nil {
				continue
			}
			arity := len(member.Parameters())
			for _, parameter := range member.Parameters() {
				if parameter.AsParameterDeclaration().DotDotDotToken != nil {
					arity = -1
				}
			}
			methods = append(methods, strconv.Quote(member.Name().Text()))
			arities = append(arities, strconv.Itoa(arity))
		}
		name := statement.Name().Text()
		l.insert(statement.End(), "\n"+helperRef("$control")+"("+name+", ["+strings.Join(methods, ", ")+"], ["+strings.Join(arities, ", ")+"]);")
	}
}

// lowerImports rewrites a relative import or export of a .effeck module to
// the JavaScript the module compiles to, ./users.effeck to ./users.js, as
// the fork's emit does.
func (l *execLowering) lowerImports() {
	for _, statement := range l.p.file.Statements.Nodes {
		var specifier *ast.Node
		switch statement.Kind {
		case ast.KindImportDeclaration:
			specifier = statement.AsImportDeclaration().ModuleSpecifier
		case ast.KindExportDeclaration:
			specifier = statement.AsExportDeclaration().ModuleSpecifier
		}
		if specifier == nil || !ast.IsStringLiteral(specifier) {
			continue
		}
		module := specifier.Text()
		if !strings.HasPrefix(module, ".") || !strings.HasSuffix(module, ".effeck") {
			continue
		}
		start := l.p.tokenStart(specifier)
		quote := l.p.text[start : start+1]
		l.replace(start, specifier.End(), quote+strings.TrimSuffix(module, ".effeck")+".js"+quote)
	}
}

// entry appends the entry statement of a module that exports a function
// named main with no parameters and calls no runner at its top level
// (D135): if (import.meta.main) $runMain(main()). A main with a plain
// return annotation runs through Effect.sync. The fork runs with
// @effect/platform-node's NodeRuntime when that package resolves; a
// projection cannot resolve packages, so this one always uses the
// runtime's copy of the runner.
func (l *execLowering) entry() {
	var main *ast.Node
	for _, statement := range l.p.file.Statements.Nodes {
		switch {
		case statement.Kind == ast.KindFunctionDeclaration && statement.Name() != nil && statement.Name().Text() == "main" &&
			ast.HasSyntacticModifier(statement, ast.ModifierFlagsExport) && !ast.HasSyntacticModifier(statement, ast.ModifierFlagsDefault) &&
			len(statement.Parameters()) == 0 && statement.Body() != nil:
			main = statement
		case statement.Kind == ast.KindExpressionStatement && callsRunnerSyntax(statement.Expression()):
			return
		}
	}
	if main == nil || !isProjectedFunction(main) {
		return
	}
	program := "main()"
	if contractOf(main.Type()) == contractPlain {
		program = helperRef("$sync") + "(main)"
	}
	l.insert(len(l.p.text), "\nif (import.meta.main) "+helperRef("$runMain")+"("+program+");\n")
}

// callsRunnerSyntax reports whether expression calls a runner, such as
// NodeRuntime.runMain(…) or runPromise(…), as the fork's callsRunner does.
func callsRunnerSyntax(expression *ast.Node) bool {
	expression = ast.SkipParentheses(expression)
	if !ast.IsCallExpression(expression) {
		return false
	}
	runners := setOf(`runMain runPromise runFork runSync runCallback`)
	switch callee := expression.Expression(); {
	case ast.IsIdentifier(callee):
		return runners[callee.Text()]
	case ast.IsPropertyAccessExpression(callee):
		return runners[callee.Name().Text()]
	}
	return false
}
