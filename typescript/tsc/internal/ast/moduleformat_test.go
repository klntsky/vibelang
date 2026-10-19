package ast_test

import (
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

func TestVibeImpliedNodeFormat(t *testing.T) {
	for _, packageType := range []string{"module", "commonjs", ""} {
		t.Run(packageType, func(t *testing.T) {
			file := tspath.RootedFilePath("/pkg/main.vibe")
			want := ast.GetImpliedNodeFormatForFile(tspath.RootedFilePath("/pkg/main.ts"), packageType)
			got := ast.GetImpliedNodeFormatForFile(file, packageType)
			if got != want {
				t.Fatalf("format = %v, want %v", got, want)
			}
			meta := ast.SourceFileMetaData{ImpliedNodeFormat: got, PackageJsonType: packageType}
			for _, moduleKind := range []core.ModuleKind{core.ModuleKindNode16, core.ModuleKindNode18, core.ModuleKindNodeNext} {
				if emit := ast.GetImpliedNodeFormatForEmitWorker(file, moduleKind, meta); emit != want {
					t.Errorf("emit format for %v = %v, want %v", moduleKind, emit, want)
				}
			}
		})
	}
}
