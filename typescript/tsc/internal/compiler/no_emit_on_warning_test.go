package compiler_test

import (
	"context"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/bundled"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
	"github.com/microsoft/TypeScript/tsc/internal/tsoptions"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/vfstest"
	"gotest.tools/v3/assert"
)

type emitBaseProgram = compiler.Program

type emitDiagnosticProgram struct {
	*emitBaseProgram
	semantic, declaration []*ast.Diagnostic
}

func (p *emitDiagnosticProgram) GetSemanticDiagnostics(context.Context, *ast.SourceFile) []*ast.Diagnostic {
	return p.semantic
}

func (p *emitDiagnosticProgram) GetDeclarationDiagnostics(context.Context, *ast.SourceFile) []*ast.Diagnostic {
	return p.declaration
}

func TestNoEmitOnErrorDiagnosticSeverity(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name             string
		semanticCategory diagnostics.Category
		declarationError bool
		wantSkipped      bool
	}{
		{"warning permits emit", diagnostics.CategoryWarning, false, false},
		{"error blocks emit", diagnostics.CategoryError, false, true},
		{"warning does not hide declaration error", diagnostics.CategoryWarning, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if !bundled.Embedded {
				t.Skip("bundled files are not embedded")
			}
			fs := bundled.WrapFS(vfstest.FromMap(map[string]string{
				"/src/index.ts": "export const answer = 42;",
			}, tspath.CaseSensitive))
			opts := &core.CompilerOptions{
				NoEmitOnError: core.TSTrue,
				Declaration:   core.TSTrue,
				SkipLibCheck:  core.TSTrue,
			}
			program := compiler.NewProgram(compiler.ProgramOptions{
				Config: tsoptions.NewParsedCommandLine(opts, testFileNames("/src/index.ts"), nil, "/src", fs.CaseSensitivity()),
				Host:   compiler.NewCompilerHost(fs, bundled.LibPath(), nil, nil, nil),
			})
			file := program.GetSourceFile("/src/index.ts")
			p := &emitDiagnosticProgram{
				emitBaseProgram: program,
				semantic: []*ast.Diagnostic{
					ast.NewExternalDiagnostic(file, core.NewTextRange(0, 1), "vibelang", test.semanticCategory, 4804, "Use a tagged failure."),
				},
			}
			if test.declarationError {
				p.declaration = []*ast.Diagnostic{
					ast.NewExternalDiagnostic(file, core.NewTextRange(0, 1), "test", diagnostics.CategoryError, 9001, "Declaration cannot be emitted."),
				}
			}
			result := compiler.HandleNoEmitOptions(t.Context(), p, nil, nil)
			if !test.wantSkipped {
				assert.Assert(t, result == nil, "warnings must not suppress JavaScript output")
			} else {
				assert.Assert(t, result != nil && result.EmitSkipped)
				if test.declarationError {
					assert.Equal(t, len(result.Diagnostics), 2, "declaration errors must still be checked after a warning")
				}
			}
		})
	}
}
