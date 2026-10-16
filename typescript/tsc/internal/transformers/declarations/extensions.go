package declarations

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// TypeAnnotationHook reports whether the declaration file must declare the
// type of node, a declaration in a source file, from the checker instead of
// copying node's type annotation: a language extension gives the annotation a
// meaning that TypeScript code reading the declaration file would not see.
type TypeAnnotationHook func(node *ast.Node) bool

var typeAnnotationHooks []TypeAnnotationHook

// RegisterTypeAnnotationHook adds hook to every declaration emit. Call it only
// from the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterTypeAnnotationHook(hook TypeAnnotationHook) {
	typeAnnotationHooks = append(typeAnnotationHooks, hook)
}

func declaresTypeFromChecker(node *ast.Node) bool {
	for _, hook := range typeAnnotationHooks {
		if hook(node) {
			return true
		}
	}
	return false
}
