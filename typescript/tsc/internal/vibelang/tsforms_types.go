package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// EK4833 (D287, amended by D308): an Effect type written through the Effect
// module, which the prelude's Effect type writes without the module:
//
//	Effect.Effect<User, NotFound, Db>   Effect<User, NotFound, Db>
//
// The fix removes the module and keeps the type arguments as written, so the
// type is the same everywhere: the prelude's Effect<A, E = never, R = never>
// is effect's Effect, and a type argument left out means what it means in
// TypeScript. A file that imports Effect from effect still has the prelude's
// type, since the import binds only a namespace. Refused: a reference where
// the name Effect is not effect's Effect type, as under a type of that name
// the file declares, a reference with a type argument left out, and one in
// JSDoc.

func init() {
	tsFormRules[EK4833.Code] = &tsFormRule{
		message:   EK4833,
		find:      findEffectTypes,
		plan:      effectTypeRewrite,
		diagnosed: ast.IsTypeReferenceNode,
	}
}

// isEffectTypeName reports whether reference, a type reference, names
// effect's Effect interface, as Effect.Effect, the prelude's Effect type, or
// an imported Effect, in source the user wrote.
func isEffectTypeName(c *checker.Checker, reference *ast.Node) bool {
	name := reference.AsTypeReferenceNode().TypeName
	if name.Flags&ast.NodeFlagsReparsed != 0 || reference.Flags&ast.NodeFlagsJSDoc != 0 {
		return false // a retired A!E!R the parser expanded (EK1014), or JSDoc
	}
	if rightmostName(name).Text() != "Effect" {
		return false
	}
	return isEffectType(c, c.GetTypeFromTypeNode(reference))
}

// findEffectTypes reports EK4833 at each Effect type of file written through
// the Effect module that the prelude's Effect type can write.
func findEffectTypes(c *checker.Checker, file *ast.SourceFile, report func(at *ast.Node, rewrite *formRewrite, args ...any)) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ast.IsTypeReferenceNode(node) && ast.IsQualifiedName(node.AsTypeReferenceNode().TypeName) && isEffectTypeName(c, node) {
			if rewrite := effectTypeRewrite(c, file, node); rewrite.fixable() {
				report(node, rewrite, messageText(node, "This Effect type"), rewrite.description)
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// effectTypeRewrite plans EK4833's fix for reference: Effect in place of
// its qualified name.
func effectTypeRewrite(c *checker.Checker, file *ast.SourceFile, reference *ast.Node) *formRewrite {
	if !isEffectTypeName(c, reference) || !ast.IsQualifiedName(reference.AsTypeReferenceNode().TypeName) || !preludeEffectTypeAt(c, reference) {
		return nil
	}
	arguments := reference.TypeArguments()
	if len(arguments) == 0 || len(arguments) > 3 {
		return nil
	}
	for _, argument := range arguments {
		if ast.NodeIsMissing(argument) {
			return nil // a type argument left out, as in Effect.Effect<A, , R>: a syntax error the parser reported
		}
	}
	name := reference.AsTypeReferenceNode().TypeName
	edit := sourceEdit{nodeStart(file, name), name.End(), "Effect"}
	text := file.Text()
	description := "Effect" + text[name.End():reference.End()]
	return &formRewrite{edits: []sourceEdit{edit}, description: shortText(description, "Effect<…>")}
}

// preludeEffectTypeAt reports whether the bare name Effect, as a type where
// node stands, is effect's Effect type: the prelude's, unless the file
// declares or imports a type of that name that is something else.
func preludeEffectTypeAt(c *checker.Checker, node *ast.Node) bool {
	symbol := c.ResolveName("Effect", node, ast.SymbolFlagsType, false /*excludeGlobals*/)
	// The prelude's Effect is one symbol for the re-exported namespace and the
	// type alias; an imported type is an alias of an interface or a type alias.
	for symbol != nil {
		if symbol.Flags()&(ast.SymbolFlagsTypeAlias|ast.SymbolFlagsInterface) != 0 {
			return isEffectType(c, c.GetDeclaredTypeOfSymbol(symbol))
		}
		if symbol.Flags()&ast.SymbolFlagsAlias == 0 {
			return false
		}
		symbol = c.GetAliasedSymbol(symbol)
	}
	return false
}
