package checker

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// GlobalSymbol returns the symbol that the global scope declares for name
// with meaning, or nil, so a name resolution hook can let a global win over a
// name it would otherwise supply.
func (c *Checker) GlobalSymbol(name string, meaning ast.SymbolFlags) *ast.Symbol {
	return c.getSymbol(c.globals, name, meaning)
}
