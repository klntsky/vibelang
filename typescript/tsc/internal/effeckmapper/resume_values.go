package effeckmapper

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
)

// lowerResumeValues separates a continuation's pending completion from the
// value a resume expression observes. Executable regions return values
// directly, with identity-based sentinels for normal ends and jumps; only
// those sentinels become undefined. User objects resembling records survive.
func (l *execLowering) lowerResumeValues() {
	for _, span := range l.spans[editRun] {
		open, close := l.edits[span.open], l.edits[span.close]
		call := l.calls[[2]int{open.pos, close.pos}]
		if call == nil || !ast.IsIdentifier(call.Expression()) ||
			(call.Expression().Text() != "resume" && call.Expression().Text() != "resumeWith") ||
			len(call.Arguments()) == 0 || !isContinuationArgument(call.Arguments()[0]) {
			continue
		}
		pending, _, found := strings.Cut(open.text, " = ")
		if !found || !strings.HasPrefix(pending, "(") || !strings.HasSuffix(close.text, ")") {
			continue
		}
		pending = strings.TrimPrefix(pending, "(")
		l.set(span.close, strings.TrimSuffix(close.text, ")")+", "+helperRef("$returned")+"("+pending+") ? "+pending+" : undefined)")
	}
}
