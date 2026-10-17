package effeck

import (
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/execute/tsc"
)

// Version is the Effeck version of this compiler. It matches the
// version in packages/compiler/package.json.
const Version = "0.0.1"

// The binary built from tsc/cmd/tsc is Effeck's compiler, effeck, under
// any file name, so --version and --help say so and name the TypeScript
// version it is built on:
//
//	$ effeck --version
//	Effeck 0.0.1 (TypeScript 7.1.0-dev)
func init() {
	base := "TypeScript " + core.Version()
	tsc.SetIdentity(tsc.Identity{
		Command: "effeck",
		Title:   "effeck: The Effeck Compiler - Version " + Version + " (" + base + ")",
		Version: "Effeck " + Version + " (" + base + ")",
		Icon:    "EK",
	})
}
