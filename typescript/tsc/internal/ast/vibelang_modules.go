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
