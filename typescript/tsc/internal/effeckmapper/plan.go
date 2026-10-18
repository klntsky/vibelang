package effeckmapper

import (
	"cmp"
	"slices"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/spanmap"
)

// The lowering plan of one file is a list of edits to its text. The walk
// records each edit once, as what it means rather than as text; a rendering
// maps each edit to text. Two renderings share the plan:
//
//   - the checking rendering, the virtual TypeScript that stock tsc checks;
//   - the executable rendering, the code the compiler emits, which runs a
//     call by a run-time test where the checking rendering has $run's
//     types, splits a plain function into f$direct and an Effect wrapper
//     (D10), and never applies a checker-only witness, such as the second
//     call of a type guard in `((yield* $run(g(x))), g(x))`.
//
// This phase implements the checking rendering. The executable rendering
// is not implemented: it drops witnesses and otherwise writes the checking
// text, so the plan carries no checking-only text it would have to undo.

// editKind is what an edit means. Kinds come in parts: an open and a close
// edit around source text, and sometimes a middle one.
type editKind uint8

const (
	// editSyntax rewrites syntax the same way in both renderings: a
	// keyword, punctuation, or a declaration's shape. Its text names
	// helpers as helperRef writes them.
	editSyntax editKind = iota
	// editWitness is checker-only text, which the executable rendering drops.
	editWitness
	// editRun runs a call where it stands (L3).
	editRun
	// editEffectBody makes a function's body an Effect: its generator. The
	// arg "self" passes this to the generator.
	editEffectBody
	// editContract checks a body against a contract with an infer channel (D57).
	editContract
	// editProgram makes a block a program: an Effect value whose generator
	// runs the block when the Effect runs (D7).
	editProgram
	// editFail fails with a thrown value (D46).
	editFail
	// editServiceCall reads a member call's receiver as a possible service key (D41).
	editServiceCall
	// editValueCall calls a value with no arguments, which runs an Effect value (D90).
	editValueCall
	// editRegion runs a block of an Effect function as an Effect of its own:
	// arg names the region's helper chain and completion variable.
	editRegion
	// editRecord is a completion record a region's block returns: a return,
	// break, or continue that leaves the region, or its normal end.
	editRecord
	// editDispatch acts on a region's completion record after it runs.
	editDispatch
)

// part says which part of a kind an edit is.
type part uint8

const (
	partOpen part = iota
	partMiddle
	partClose
)

// edit replaces the original text in [pos, end) with what kind and part
// mean. An edit with pos == end inserts before the original character at pos.
type edit struct {
	pos  int
	end  int
	kind editKind
	part part
	// text is an editSyntax or editWitness edit's text, and the rendered
	// form of kinds whose text the walk computes (regions, records,
	// dispatches), in checking form.
	text string
	// arg is a kind-specific argument.
	arg string
}

// helperRef writes a reference to a projection helper in edit text. Each
// rendering resolves it: the checking rendering to the prelude helper's
// local name.
func helperRef(name string) string { return "\x01" + name + "\x02" }

// plan is the lowering plan of one file.
type plan struct {
	edits []edit
}

// insert records syntax text to insert before the original character at pos.
func (l *plan) insert(pos int, text string) {
	l.edits = append(l.edits, edit{pos: pos, end: pos, text: text})
}

// replace records syntax text to replace the original text in [pos, end).
func (l *plan) replace(pos int, end int, text string) {
	l.edits = append(l.edits, edit{pos: pos, end: end, text: text})
}

// witness records checker-only text to insert before the original
// character at pos.
func (l *plan) witness(pos int, text string) {
	l.edits = append(l.edits, edit{pos: pos, end: pos, kind: editWitness, text: text})
}

// mark records an edit of kind at [pos, end), with arg.
func (l *plan) mark(kind editKind, p part, pos int, end int, arg string) {
	l.edits = append(l.edits, edit{pos: pos, end: end, kind: kind, part: p, arg: arg})
}

// markText records an edit of kind at [pos, end) whose checking text the
// walk computed.
func (l *plan) markText(kind editKind, p part, pos int, end int, text string) {
	l.edits = append(l.edits, edit{pos: pos, end: end, kind: kind, part: p, text: text})
}

// rendering maps a plan's edits to text.
type rendering interface {
	// text returns the text of e, or ok false to leave e out.
	text(e edit) (text string, ok bool)
}

// checking is the checking rendering, the virtual TypeScript.
type checking struct {
	names *names
}

func (r checking) text(e edit) (string, bool) {
	var text string
	switch e.kind {
	case editSyntax, editWitness, editRegion, editDispatch:
		text = e.text
	case editRecord:
		// A return in a region stays as written (regions.go); other
		// records carry their checking text.
		text = e.text
	case editRun:
		text = pick(e.part, "(yield* "+helperRef("$run")+"(", "", "))")
	case editEffectBody:
		open := " return " + helperRef("$gen") + "(function* () {"
		if e.arg == "self" {
			open = " return " + helperRef("$gen") + "({ self: this }, function* () {"
		}
		text = pick(e.part, open, "", "})")
	case editContract:
		gen := helperRef("$gen") + "(function* () "
		if e.arg == "self" {
			gen = helperRef("$gen") + "({ self: this }, function* () "
		}
		text = pick(e.part, " { return "+helperRef("$contract")+"<", ">()("+gen, "))}")
	case editProgram:
		text = pick(e.part, helperRef("$gen")+"(function* () ", "", ")")
	case editFail:
		if e.arg == "void" {
			text = pick(e.part, "return (yield* "+helperRef("$fail")+"(", "", ")) as void")
		} else {
			text = pick(e.part, "return yield* "+helperRef("$fail")+"(", "", ")")
		}
	case editServiceCall:
		text = pick(e.part, helperRef("$use")+"(", "", ")")
	case editValueCall:
		text = pick(e.part, helperRef("$value")+"(", "", ")")
	}
	return r.resolve(text), true
}

// resolve replaces each helper reference in text with the helper's local name.
func (r checking) resolve(text string) string {
	for {
		start := strings.IndexByte(text, '\x01')
		if start < 0 {
			return text
		}
		end := strings.IndexByte(text[start:], '\x02')
		if end < 0 {
			return text
		}
		end += start
		text = text[:start] + r.names.helper(text[start+1:end]) + text[end+1:]
	}
}

// executable is the executable rendering. It is not implemented yet: it
// leaves out checker-only witnesses and writes the checking text for every
// other edit, which is not yet the code the compiler will emit.
type executable struct {
	checking checking
}

func (r executable) text(e edit) (string, bool) {
	if e.kind == editWitness {
		return "", false
	}
	return r.checking.text(e)
}

// pick returns the text of part p among open, middle, and close.
func pick(p part, open, middle, close string) string {
	switch p {
	case partOpen:
		return open
	case partMiddle:
		return middle
	}
	return close
}

// render applies the plan's edits to original, after prefix, text that
// comes before every edit. Edits are sorted by position, stably, so edits
// at one position apply in the order the walk recorded them: an outer run
// opens before an inner one and closes after it. Each stretch of original
// text between edits is copied verbatim and gets a Verbatim segment with
// every language feature; rendered text is synthesized and gets none. ok
// is false when two edits overlap, a projector bug; the caller then passes
// the file through unchanged.
func (l *plan) render(r rendering, original string, prefix func() string) (text string, segments []spanmap.Segment, overlap int, ok bool) {
	edits := slices.Clone(l.edits)
	slices.SortStableFunc(edits, func(a, b edit) int { return cmp.Compare(a.pos, b.pos) })
	var body strings.Builder
	cursor := 0
	for _, e := range edits {
		if e.pos < cursor {
			return "", nil, e.pos, false
		}
		segments = appendVerbatim(segments, &body, original, cursor, e.pos)
		if t, keep := r.text(e); keep {
			body.WriteString(t)
		}
		cursor = e.end
	}
	segments = appendVerbatim(segments, &body, original, cursor, len(original))
	// The prefix is computed after the edits, because rendering them
	// records which helpers the prefix imports.
	head := prefix()
	for i := range segments {
		segments[i].VirtualStart += core.TextPos(len(head))
		segments[i].VirtualEnd += core.TextPos(len(head))
	}
	return head + body.String(), segments, 0, true
}

// appendVerbatim copies original[from:to] to out and appends its Verbatim
// segment, merged into the previous segment when the two are contiguous in
// both texts.
func appendVerbatim(segments []spanmap.Segment, out *strings.Builder, original string, from int, to int) []spanmap.Segment {
	if to <= from {
		return segments
	}
	virtualStart := core.TextPos(out.Len())
	out.WriteString(original[from:to])
	if n := len(segments); n > 0 && segments[n-1].VirtualEnd == virtualStart && segments[n-1].OriginalEnd == core.TextPos(from) {
		segments[n-1].VirtualEnd = core.TextPos(out.Len())
		segments[n-1].OriginalEnd = core.TextPos(to)
		return segments
	}
	return append(segments, spanmap.Segment{
		VirtualStart:  virtualStart,
		VirtualEnd:    core.TextPos(out.Len()),
		OriginalStart: core.TextPos(from),
		OriginalEnd:   core.TextPos(to),
		Kind:          spanmap.KindVerbatim,
		Features:      spanmap.FeatureAll,
	})
}
