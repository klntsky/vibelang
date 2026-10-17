package effeck

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// preludeCompletions offers the prelude names (prelude.go) in every scope of
// an .effeck file, so completions list Effect, Layer, TaggedError, pipe, and the
// rest without an import. The file's own declarations and imports shadow
// them, as they do in resolvePreludeName. A file that uses no prelude name
// yet does not import the runtime, so its list comes from the runtime that
// another .effeck file of the program imports; once the file uses a name, it
// imports the runtime itself.
func preludeCompletions(c *checker.Checker, location *ast.Node, meaning ast.SymbolFlags) []*ast.Symbol {
	file := ast.GetSourceFileOfNode(location)
	if !isEffeckFile(file) {
		return nil
	}
	module := runtimeModuleSymbol(c, preludeStateOf(c), file)
	if module == nil {
		module = programRuntimeModuleSymbol(c)
	}
	if module == nil {
		return nil
	}
	var symbols []*ast.Symbol
	for name := range preludeModules {
		if symbol := c.TryGetMemberInModuleExports(name, module); symbol != nil && hasMeaning(c, symbol, meaning) {
			symbols = append(symbols, symbol)
		}
	}
	return symbols
}

type programRuntimeKey struct{}

type programRuntime struct{ module *ast.Symbol }

// programRuntimeModuleSymbol returns the runtime module that some file of the
// program imports, or nil when none does.
func programRuntimeModuleSymbol(c *checker.Checker) *ast.Symbol {
	return c.ExtensionState(programRuntimeKey{}, func() any {
		program := c.Program()
		for _, resolutions := range program.GetResolvedModules() {
			for key, resolved := range resolutions {
				if key.Name != runtimeModule || !resolved.IsResolved() {
					continue
				}
				if moduleFile := program.GetSourceFileForResolvedModule(resolved); moduleFile != nil {
					return &programRuntime{module: c.GetSymbolOfDeclaration(moduleFile.AsNode())}
				}
			}
		}
		return &programRuntime{}
	}).(*programRuntime).module
}

// serviceMemberCompletions lists a service's members after `Users.` in an .effeck
// function body. A call there reads the member from the service, not from
// the Context key that the class compiles to (design D41), so the key's own
// members, such as of, pipe, and use, would be the wrong list.
func serviceMemberCompletions(c *checker.Checker, left *ast.Node, t *checker.Type) *checker.Type {
	if !inLowerableBody(left) {
		return t
	}
	if _, shape := serviceOfKey(c, c.GetApparentType(t)); shape != nil {
		return shape
	}
	return t
}
