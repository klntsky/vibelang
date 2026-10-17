package effeck

import (
	"path"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// Where a declaration comes from decides how Effeck code may call it
// (design D5, D22, D23, D38): its file is an .effeck file, one of TypeScript's lib
// files, or a file of a package, and the package may be one of Effect's. The
// helpers here answer those questions for the trust levels (trust.go), the
// platform checks (platform.go), and the runners (runners.go).

// originsState is what one checker remembers about the files it has asked
// about.
type originsState struct {
	packages map[*ast.SourceFile]string // file -> the name of the package that holds it
}

type originsStateKey struct{}

func originsOf(c *checker.Checker) *originsState {
	return c.ExtensionState(originsStateKey{}, func() any {
		return &originsState{packages: map[*ast.SourceFile]string{}}
	}).(*originsState)
}

// packageOf returns the name in the nearest package.json above file, or ""
// when there is none or it has no name. A package installed through a
// symbolic link, as a file: dependency is, keeps its name, although the file
// lies outside node_modules.
func packageOf(c *checker.Checker, file *ast.SourceFile) string {
	s := originsOf(c)
	if name, ok := s.packages[file]; ok {
		return name
	}
	name := ""
	program := c.Program()
	if directory := program.GetNearestAncestorDirectoryWithPackageJson(file.FileName().Directory()); directory != "" {
		if info := program.GetPackageJsonInfo(directory.ResolveFile("package.json")); info.Exists() {
			name, _ = info.Contents.Name.GetValue()
		}
	}
	s.packages[file] = name
	return name
}

// isEffectPackage reports whether name is effect, an @effect/* package, or
// effeck-runtime, the packages whose exports Effeck trusts
// (D22).
func isEffectPackage(name string) bool {
	return name == "effect" || strings.HasPrefix(name, "@effect/") || name == "effeck-runtime"
}

// isLibFile reports whether file is one of TypeScript's lib files, which
// declare JavaScript's built-ins and the browser's globals.
func isLibFile(c *checker.Checker, file *ast.SourceFile) bool {
	return file != nil && c.Program().IsSourceFileDefaultLibrary(file.PathKey())
}

// moduleOf returns the module name of a package file: its base name without
// extensions, such as "Effect" for effect/dist/Effect.d.ts and "NodeRuntime"
// for @effect/platform-node/dist/NodeRuntime.d.ts.
func moduleOf(file *ast.SourceFile) string {
	name := path.Base(string(file.FileName()))
	for _, extension := range []string{".d.ts", ".d.mts", ".d.cts", ".ts", ".mts", ".cts", ".js", ".mjs", ".cjs"} {
		if trimmed, ok := strings.CutSuffix(name, extension); ok {
			return trimmed
		}
	}
	return name
}

// ownerOf returns the named declaration that declares decl, the declaration
// of a signature: decl itself when it is a function or method, else the
// variable, property, or parameter whose type or value decl is, or the
// interface or class that declares a call or construct signature.
func ownerOf(decl *ast.Node) *ast.Node {
	for n := decl; n != nil; n = n.Parent {
		switch n.Kind {
		case ast.KindFunctionDeclaration, ast.KindMethodDeclaration, ast.KindMethodSignature,
			ast.KindVariableDeclaration, ast.KindPropertyDeclaration, ast.KindPropertySignature,
			ast.KindPropertyAssignment, ast.KindParameter,
			ast.KindInterfaceDeclaration, ast.KindClassDeclaration, ast.KindTypeAliasDeclaration:
			return n
		case ast.KindSourceFile, ast.KindBlock, ast.KindModuleBlock:
			return nil
		}
	}
	return nil
}

// declarationName returns the name of decl when it is an identifier or a
// string literal, else "".
func declarationName(decl *ast.Node) string {
	if decl == nil {
		return ""
	}
	if name := decl.Name(); name != nil && (ast.IsIdentifier(name) || ast.IsStringLiteral(name)) {
		return name.Text()
	}
	return ""
}

// containerName returns the name of the interface, class, or namespace that
// declares member, or "" for a top-level declaration.
func containerName(member *ast.Node) string {
	for n := member.Parent; n != nil; n = n.Parent {
		switch n.Kind {
		case ast.KindInterfaceDeclaration, ast.KindClassDeclaration, ast.KindModuleDeclaration, ast.KindTypeAliasDeclaration:
			return declarationName(n)
		case ast.KindSourceFile:
			return ""
		}
	}
	return ""
}

// signatureDeclaration returns the declaration of the signature that call or
// new expression node resolves to, or nil when the checker has none, as for a
// callee of type any or a signature an extension made.
func signatureDeclaration(c *checker.Checker, node *ast.Node) *ast.Node {
	signature := c.GetResolvedSignature(node)
	if signature == nil {
		return nil
	}
	return signature.Declaration()
}

// calleeSymbol returns the symbol that the callee of call names, with
// aliases resolved, or nil: f for f(x), and m for o.m(x).
func calleeSymbol(c *checker.Checker, call *ast.Node) *ast.Symbol {
	callee := ast.SkipParentheses(call.Expression())
	var name *ast.Node
	switch {
	case ast.IsIdentifier(callee):
		name = callee
	case ast.IsPropertyAccessExpression(callee):
		name = callee.Name()
	default:
		return nil
	}
	symbol := c.GetSymbolAtLocation(name)
	if symbol != nil && symbol.Flags()&ast.SymbolFlagsAlias != 0 {
		symbol = c.GetAliasedSymbol(symbol)
	}
	return symbol
}

// isGlobalOf reports whether symbol is a global that only declaration files
// declare, at least one of them a file that matches.
func isGlobalOf(symbol *ast.Symbol, matches func(file *ast.SourceFile) bool) bool {
	if symbol == nil || len(symbol.Declarations()) == 0 {
		return false
	}
	found := false
	for _, decl := range symbol.Declarations() {
		file := ast.GetSourceFileOfNode(decl)
		if file == nil || !file.IsDeclarationFile {
			return false
		}
		if ast.IsExternalModule(file) && !isGlobalAugmentation(decl) {
			return false
		}
		found = found || matches(file)
	}
	return found
}

// isGlobalAugmentation reports whether decl is inside `declare global { … }`.
func isGlobalAugmentation(decl *ast.Node) bool {
	for n := decl.Parent; n != nil; n = n.Parent {
		if n.Kind == ast.KindModuleDeclaration && ast.IsGlobalScopeAugmentation(n) {
			return true
		}
	}
	return false
}
