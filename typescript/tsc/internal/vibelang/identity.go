package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/execute/tsc"
	"github.com/microsoft/TypeScript/tsc/internal/lsp"
)

// Version is the VibeLang version of this compiler. It matches the
// version in packages/compiler/package.json.
const Version = "0.0.9"

// The binary built from tsc/cmd/tsc is VibeLang's compiler, vibelang, under
// any file name, so --version and --help say so and name the TypeScript
// version it is built on:
//
//	$ vibelang --version
//	VibeLang 0.0.9 (TypeScript 7.1.0-dev)
func init() {
	base := "TypeScript " + core.Version()
	tsc.SetIdentity(tsc.Identity{
		Command: "vibelang",
		Title:   "vibelang: The VibeLang Compiler - Version " + Version + " (" + base + ")",
		Version: "VibeLang " + Version + " (" + base + ")",
		Icon:    "EK",
	})
	// The language server's initialize response names it too.
	lsp.SetServerInfo("vibelang", Version+" ("+base+")")
}
