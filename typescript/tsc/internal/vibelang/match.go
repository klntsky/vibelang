package vibelang

import (
	"strconv"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// Pattern matching (D206-D225). The parser writes `match (x) { when P: v; }`
// and `x is P` as TypeScript the checker narrows (vibelang_match.go in package
// parser). A match without a default arm ends in a call of $noMatch with the
// subject, so the subject's type there is what no arm matched: when the
// subject's type is a finite union and that is not never, the match misses a
// case (EK4701), as Effect's Match.exhaustive does. A pattern form VibeLang
// does not support is a call of $patternError, reported here.

var (
	// EK4701: a match over a finite union with no default arm misses a case. {0} is the subject's type, {1} the cases left.
	EK4701 = message(4701, errorCategory, "This match does not handle every case of {0}: {1} is left.")
	// EK4702: a binding inside an or or not pattern.
	EK4702 = message(4702, errorCategory, "A binding cannot appear inside an or or not pattern yet.")
	// EK4703: an is whose pattern binds names outside an if or while condition.
	EK4703 = message(4703, errorCategory, "Bindings in an is pattern work only in the condition of an if or while statement, joined by &&.")
	// EK4704: a pattern form VibeLang does not support yet. {0} names it.
	EK4704 = message(4704, errorCategory, "{0} patterns are not supported yet.")
	// EK4705: and, or, and not combined at one level without parentheses.
	EK4705 = message(4705, errorCategory, "Patterns joined by and, or, or not cannot share a level; add parentheses.")
)

func init() {
	for _, m := range []*Message{EK4701, EK4702, EK4703, EK4704, EK4705} {
		registry[m.Code] = m
	}
	fileChecks = append(fileChecks, checkPatterns)
}

// patternHelper returns the runtime helper that call, a call the parser wrote
// for a match or is (its callee is synthesized), calls: "$noMatch" or
// "$patternError". It returns "" for any other node.
func patternHelper(call *ast.Node) string {
	if call == nil || !ast.IsCallExpression(call) {
		return ""
	}
	callee := call.Expression()
	if callee.Kind != ast.KindIdentifier || callee.Flags&ast.NodeFlagsReparsed == 0 {
		return ""
	}
	switch name := callee.Text(); name {
	case "$noMatch", "$patternError":
		return name
	}
	return ""
}

func checkPatterns(c *checker.Checker, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		switch patternHelper(node) {
		case "$noMatch":
			checkExhaustive(c, file, node)
		case "$patternError":
			reportPatternError(c, file, node)
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// checkExhaustive reports a match whose subject, a finite union, still has
// cases at the $noMatch call that ends its arms.
func checkExhaustive(c *checker.Checker, file *ast.SourceFile, call *ast.Node) {
	declared, left := matchRemainder(c, call)
	if declared == nil || left.Flags()&checker.TypeFlagsNever != 0 {
		return
	}
	declaredText, leftText := c.TypeToString(declared), c.TypeToString(left)
	if symbol := c.GetSymbolAtLocation(declaredSubject(c, call.Arguments()[0])); symbol != nil {
		// A data type's variants by name, Delivery.Sent, not their object types (D307).
		if names := dataNamesFor(c, symbol, call); names.hasVariants(c, declared) {
			declaredText, leftText = names.typeText(c, declared), names.unionText(c, left)
		}
	}
	if nested := nestedMatchRemainder(c, call, left); nested != "" {
		leftText = nested
	}
	d := reportAt(c, file, call.Expression().Loc, EK4701, declaredText, leftText)
	note(d, "Add an arm for each case that is left, or a default arm.")
}

// provenExhaustive reports whether D209 proves that call, the $noMatch call
// that ends a match without a default arm, never runs: the subject's
// declared type is a finite union and no case of it is left there (D294).
func provenExhaustive(c *checker.Checker, call *ast.Node) bool {
	declared, left := matchRemainder(c, call)
	return declared != nil && left.Flags()&checker.TypeFlagsNever != 0
}

// matchRemainder returns the declared type of the subject of call, the
// $noMatch call that ends a match, and its narrowed type there, what no arm
// matched. It returns nils unless the declared type is a finite union. A
// subject that is an identifier expression has the type its declaration
// gives it, not the type the subject's capture has (match_capture.go).
func matchRemainder(c *checker.Checker, call *ast.Node) (declared, left *checker.Type) {
	args := call.Arguments()
	if len(args) != 1 {
		return nil, nil
	}
	subject := args[0]
	left = c.GetTypeAtLocation(subject)
	if left == nil {
		return nil, nil
	}
	symbol := c.GetSymbolAtLocation(declaredSubject(c, subject))
	if symbol == nil {
		return nil, nil
	}
	declared = c.GetTypeOfSymbol(symbol)
	if declared == nil || !isFiniteMatchType(c, declared) {
		return nil, nil
	}
	return declared, left
}

// isFiniteMatchType reports whether t is a finite union a match can cover:
// each member a literal, an enum member, null, undefined, or void, or an
// object type whose _tag is a string literal (tagged errors, Option, Exit,
// Result).
func isFiniteMatchType(c *checker.Checker, t *checker.Type) bool {
	members := []*checker.Type{t}
	if t.Flags()&checker.TypeFlagsUnion != 0 {
		members = t.Types()
	}
	for _, m := range members {
		flags := m.Flags()
		if flags&(checker.TypeFlagsUnit|checker.TypeFlagsVoid|checker.TypeFlagsNever) != 0 {
			continue
		}
		if flags&checker.TypeFlagsObject != 0 {
			if tag := c.GetPropertyOfType(m, "_tag"); tag != nil && c.GetTypeOfSymbol(tag).Flags()&checker.TypeFlagsStringLiteral != 0 {
				continue
			}
		}
		return false
	}
	return true
}

// reportPatternError reports the pattern a $patternError call stands for, at
// the pattern's first token (or the is keyword).
func reportPatternError(c *checker.Checker, file *ast.SourceFile, call *ast.Node) {
	args := call.Arguments()
	if len(args) == 0 || args[0].Kind != ast.KindNumericLiteral {
		return
	}
	code, err := strconv.Atoi(args[0].Text())
	if err != nil {
		return
	}
	at := call.Expression().Loc
	switch code {
	case 4702:
		reportAt(c, file, at, EK4702)
	case 4703:
		note(reportAt(c, file, at, EK4703), "Use the pattern in an if or while condition, or match the value with match.")
	case 4704:
		detail := "These"
		if len(args) > 1 && args[1].Kind == ast.KindStringLiteral {
			detail = args[1].Text()
		}
		reportAt(c, file, at, EK4704, detail)
	case 4705:
		note(reportAt(c, file, at, EK4705), "Write (a and b) or c, or a and (b or c).")
	}
	cover(c, call)
}
