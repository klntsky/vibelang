package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// A lambda whose body returns something context sensitive, such as an object
// literal with an untyped method, is skipped in the first pass of overload
// resolution, and TypeScript types it there as a wildcard for function types:
//
//	Layer.effect(Users, () => {
//	  const db = Database()
//	  return { find(id) { … } }     // find's id is typed by the service
//	})
//
// Where only an Effect is expected, the lambda is a program (D7), and that
// wildcard is not assignable to the Effect, so the candidate fails before the
// second pass infers its type arguments from the program, and E and R come
// out unknown. skippedProgram gives such a lambda the non-inferrable any
// type for the first pass instead, which every type accepts and inference
// ignores, so the second pass infers E and R from the program.

func init() { checker.RegisterSkippedFunctionTypeHook(skippedProgram) }

func skippedProgram(c *checker.Checker, fn *ast.Node) *checker.Type {
	if !isLambda(fn) || requiredParameterCount(fn) != 0 {
		return nil
	}
	contextual := c.GetContextualType(fn, checker.ContextFlagsNone)
	if contextual == nil {
		return nil
	}
	if e := analyzerFor(c).classify([]*checker.Type{contextual}); e.program && !e.effectFn && !e.plainFn {
		return c.GetNonInferrableAnyType()
	}
	return nil
}
