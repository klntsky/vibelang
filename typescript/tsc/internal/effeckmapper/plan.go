package effeckmapper

import (
	"cmp"
	"slices"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/spanmap"
)

// The lowering plan of one file is a list of edits to its text. The walk
// records each edit once; a rendering applies them. Two renderings share the
// plan:
//
//   - the checking rendering, the virtual TypeScript that stock tsc checks,
//     applies every edit;
//   - the executable rendering, the JavaScript a bundler would run, must
//     never apply a checker-only witness, text that exists only to give the
//     checker a fact, such as the second call of a type guard in
//     `((yield* $run(g(x))), g(x))`, which keeps the guard's narrowing.
//
// This phase implements the checking rendering. An edit's text is its
// checking text; a witness edit is an insertion that the executable
// rendering drops.

// edit replaces the original text in [pos, end) with text. An edit with
// pos == end inserts text before the original character at pos.
type edit struct {
	pos  int
	end  int
	text string
	// witness marks checker-only text, which only the checking rendering applies.
	witness bool
}

// rendering selects which edits of a plan apply.
type rendering uint8

const (
	// renderChecking applies every edit: the virtual TypeScript.
	renderChecking rendering = iota
	// renderExecutable leaves out checker-only witnesses.
	renderExecutable
)

// plan is the lowering plan of one file.
type plan struct {
	edits []edit
}

// insert records text to insert before the original character at pos.
func (l *plan) insert(pos int, text string) {
	l.edits = append(l.edits, edit{pos: pos, end: pos, text: text})
}

// replace records text to replace the original text in [pos, end).
func (l *plan) replace(pos int, end int, text string) {
	l.edits = append(l.edits, edit{pos: pos, end: end, text: text})
}

// witness records checker-only text to insert before the original
// character at pos.
func (l *plan) witness(pos int, text string) {
	l.edits = append(l.edits, edit{pos: pos, end: pos, text: text, witness: true})
}

// render applies the plan's edits to original, after prefix, synthesized
// text that comes before every edit. Edits are sorted by position,
// stably, so edits at one position apply in the order the walk recorded
// them: an outer run opens before an inner one and closes after it. Each
// stretch of original text between edits is copied verbatim and gets a
// Verbatim segment with every language feature; inserted text is
// synthesized and gets none. ok is false when two edits overlap, a projector
// bug; the caller then passes the file through unchanged.
func (l *plan) render(prefix string, original string, mode rendering) (text string, segments []spanmap.Segment, overlap int, ok bool) {
	edits := slices.Clone(l.edits)
	if mode == renderExecutable {
		edits = slices.DeleteFunc(edits, func(e edit) bool { return e.witness })
	}
	slices.SortStableFunc(edits, func(a, b edit) int { return cmp.Compare(a.pos, b.pos) })
	var out strings.Builder
	out.WriteString(prefix)
	cursor := 0
	for _, e := range edits {
		if e.pos < cursor {
			return "", nil, e.pos, false
		}
		segments = appendVerbatim(segments, &out, original, cursor, e.pos)
		out.WriteString(e.text)
		cursor = e.end
	}
	segments = appendVerbatim(segments, &out, original, cursor, len(original))
	return out.String(), segments, 0, true
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
