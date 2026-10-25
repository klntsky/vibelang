package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/execute/tsc"
)

// Version is the VibeLang version of this compiler. It matches the
// version in packages/compiler/package.json.
const Version = "0.0.6"

// The binary built from tsc/cmd/tsc is VibeLang's compiler, vibelang, under
// any file name, so --version and --help say so and name the TypeScript
// version it is built on:
//
//	$ vibelang --version
//	VibeLang 0.0.6 (TypeScript 7.1.0-dev)
func init() {
	base := "TypeScript " + core.Version()
	tsc.SetIdentity(tsc.Identity{
		Command: "vibelang",
		Title:   "vibelang: The VibeLang Compiler - Version " + Version + " (" + base + ")",
		Version: "VibeLang " + Version + " (" + base + ")",
		Icon:    "EK",
	})
}
