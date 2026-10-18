package vibelang

import (
	"context"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// Three rules of Effect's language service also apply to the .ts files that
// VibeLang checks (D242), because each marks Effect code that runs differently
// from how it reads and the language server that serves those files replaces
// @effect/tsgo: a discarded Effect (EK4801), a yield without * in an
// Effect generator (EK4819), and a yield* of an Effect that never succeeds
// without return (EK4820). The other rules are about VibeLang's own forms, or
// are style, and @effect/tsgo reports them in projects whose .ts files it
// serves.

var (
	// EK4819 (missingStarInYieldEffectGen): yield of an Effect in an Effect generator.
	EK4819 = message(4819, errorCategory, "yield hands this Effect to the generator's caller without running it; yield* runs it.")
	// EK4820 (missingReturnYieldStar): yield* of an Effect that never succeeds, without return.
	EK4820 = message(4820, errorCategory, "This Effect never succeeds, but TypeScript does not know the generator stops here.")
)

func init() {
	registry[EK4819.Code] = EK4819
	registry[EK4820.Code] = EK4820
	typeScriptFileChecks = append(typeScriptFileChecks, checkTypeScriptRules)
	ls.RegisterCodeFixProvider(yieldStarFix)
}

// typeScriptFileChecks run once for each .ts or .tsx file outside
// node_modules that a checker checks, after TypeScript's own checks of that
// file.
var typeScriptFileChecks []func(c *checker.Checker, file *ast.SourceFile)

// isCheckedTypeScriptFile reports whether file is a .ts, .tsx, .mts, or .cts
// source file outside node_modules.
func isCheckedTypeScriptFile(file *ast.SourceFile) bool {
	if file == nil || file.IsDeclarationFile || strings.Contains(string(file.FileName()), "/node_modules/") {
		return false
	}
	return tspath.FileExtensionIsOneOf(string(file.FileName()), []string{tspath.ExtensionTs, tspath.ExtensionTsx, tspath.ExtensionMts, tspath.ExtensionCts})
}

// checkTypeScriptRules reports EK4801, EK4819, and EK4820 in file.
func checkTypeScriptRules(c *checker.Checker, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		switch {
		case ast.IsYieldExpression(node):
			checkYield(c, node)
		case ast.IsExpressionStatement(node):
			checkTypeScriptStatement(c, node)
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// checkYield reports EK4819 at a yield without * of an Effect in an Effect
// generator.
func checkYield(c *checker.Checker, yield *ast.Node) {
	operand := yield.Expression()
	if yield.AsYieldExpression().AsteriskToken != nil || operand == nil || effectGeneratorOf(c, yield) == nil {
		return
	}
	if isEffectType(c, c.GetTypeAtLocation(operand)) {
		report(c, yield, EK4819)
	}
}

// checkTypeScriptStatement reports EK4820 at a yield* statement of an Effect
// that never succeeds in an Effect generator, and EK4801 at a statement that
// discards an Effect.
func checkTypeScriptStatement(c *checker.Checker, statement *ast.Node) {
	expression := ast.SkipParentheses(statement.Expression())
	generator := effectGeneratorOf(c, statement)
	if ast.IsYieldExpression(expression) {
		operand := expression.Expression()
		if generator == nil || operand == nil || expression.AsYieldExpression().AsteriskToken == nil {
			return
		}
		t := c.GetTypeAtLocation(operand)
		if success := channelType(c, t, "_A"); isEffectType(c, t) && success != nil && success.Flags()&checker.TypeFlagsNever != 0 {
			note(report(c, expression, EK4820), "Write return yield*, so that TypeScript narrows what follows.")
		}
		return
	}
	if isDiscardingForm(expression) || !isEffectType(c, c.GetTypeAtLocation(expression)) {
		return
	}
	d := report(c, statement.Expression(), EK4801, messageText(statement.Expression(), "This expression"))
	if generator != nil {
		note(d, "Run it with yield*.")
	} else {
		note(d, "Run it with yield* inside Effect.gen, or keep it in a variable.")
	}
}

// effectGeneratorOf returns the generator function whose body evaluates
// node, when that generator is the body of an Effect: an argument of an
// effect function such as Effect.gen, Effect.fn("name"), or Layer.effect's
// Effect.gen. It returns nil otherwise.
func effectGeneratorOf(c *checker.Checker, node *ast.Node) *ast.Node {
	fn := ast.GetContainingFunction(node)
	if fn == nil || !ast.IsFunctionExpression(fn) || fn.AsFunctionExpression().AsteriskToken == nil || ast.GetFunctionFlags(fn)&ast.FunctionFlagsAsync != 0 {
		return nil
	}
	call := fn.Parent
	if call == nil || !ast.IsCallExpression(call) {
		return nil
	}
	callee := call.Expression()
	if ast.IsCallExpression(callee) {
		callee = callee.Expression() // Effect.fn("name")(function* () { … })
	}
	if module, _ := effectFunctionOf(c, callee); module != "Effect" {
		return nil
	}
	return fn
}

// yieldStarFix writes the * that EK4819 reports missing, the return that
// EK4820 does, and yield* before an Effect that EK4801 reports inside an
// Effect generator in a .ts file.
var yieldStarFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK4801.Code, EK4819.Code, EK4820.Code},
	GetCodeActions: func(ctx context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		if !isCheckedTypeScriptFile(fix.SourceFile) {
			return nil, nil
		}
		node := reportedNode(fix.SourceFile, fix.Span, func(node *ast.Node) bool {
			if fix.ErrorCode == EK4819.Code {
				return ast.IsYieldExpression(node)
			}
			return node.Parent != nil && ast.IsExpressionStatement(node.Parent) && node.Parent.Expression() == node
		})
		if node == nil {
			return nil, nil
		}
		start := nodeStart(fix.SourceFile, node)
		switch fix.ErrorCode {
		case EK4819.Code:
			return []*ls.CodeAction{{Description: "Write yield*", Changes: replaceRange(fix, start+len("yield"), start+len("yield"), "*")}}, nil
		case EK4820.Code:
			return []*ls.CodeAction{{Description: "Write return yield*", Changes: replaceRange(fix, start, start, "return ")}}, nil
		}
		c, done := fix.Program.GetTypeCheckerForFile(ctx, fix.SourceFile)
		defer done()
		if effectGeneratorOf(c, node) == nil {
			return nil, nil
		}
		changes := replaceRange(fix, start, start, "yield* ")
		if needsParentheses(node) {
			changes = append(replaceRange(fix, start, start, "yield* ("), replaceRange(fix, node.End(), node.End(), ")")...)
		}
		return []*ls.CodeAction{{Description: "Run it with yield*", Changes: changes}}, nil
	},
}
