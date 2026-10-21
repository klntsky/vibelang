package ast

// EmbeddedModule describes a module declaration of a VibeLang file (D283),
// module text { ... }, whose body is TypeScript. The body is a source file of
// its own, a supplemental file of the VibeLang file whose text is the
// VibeLang file's with every character outside the body blanked, so its
// positions are the VibeLang file's.
type EmbeddedModule struct {
	Name        string // the declared name, text
	Specifier   string // the specifier that names the file, ./main.vibe.text.js
	Declaration *Node  // the ModuleDeclaration in the VibeLang file
}

// EmbeddedModuleContentMapper is the content mapper identity of the files
// that hold module declarations' bodies: no external mapper makes them.
const EmbeddedModuleContentMapper = "vibelang:module-declaration"

// EmbeddedModule returns the module declaration whose body file is, or nil
// when file holds no module declaration's body.
func (node *SourceFile) EmbeddedModule() *EmbeddedModule {
	if node.contentMapperInfo == nil {
		return nil
	}
	return node.contentMapperInfo.EmbeddedModule
}

// EmbeddedModules returns the files that hold the bodies of node's module
// declarations, in the order they are declared.
func (node *SourceFile) EmbeddedModules() []*SourceFile {
	var modules []*SourceFile
	for _, file := range node.SupplementalSourceFiles() {
		if file.EmbeddedModule() != nil {
			modules = append(modules, file)
		}
	}
	return modules
}

// EmbeddedModuleBySpecifier returns the file of node's module declaration that
// specifier names, or nil.
func (node *SourceFile) EmbeddedModuleBySpecifier(specifier string) *SourceFile {
	for _, file := range node.SupplementalSourceFiles() {
		if module := file.EmbeddedModule(); module != nil && module.Specifier == specifier {
			return file
		}
	}
	return nil
}

// embeddedModuleNames returns args with each argument that names a module
// declaration of file, or of the file whose module file is, as TypeScript
// quotes a module ("./main.vibe.text.js"), replaced by the declared name
// (text), or nil when no argument names one.
func embeddedModuleNames(file *SourceFile, args []string) []string {
	if canonical := file.CanonicalSourceFile(); canonical != nil {
		file = canonical
	}
	var result []string
	for i, arg := range args {
		if len(arg) < 2 || arg[0] != '"' || arg[len(arg)-1] != '"' {
			continue
		}
		module := file.EmbeddedModuleBySpecifier(arg[1 : len(arg)-1])
		if module == nil {
			continue
		}
		if result == nil {
			result = append([]string(nil), args...)
		}
		result[i] = module.EmbeddedModule().Name
	}
	return result
}
