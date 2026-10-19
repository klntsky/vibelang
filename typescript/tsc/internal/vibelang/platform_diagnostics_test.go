package vibelang

import (
	"strings"
	"testing"
)

func TestGlobalThisPlatformDiagnostics(t *testing.T) {
	files := computedKeyFiles(false)
	files["/Users/x/src/node_modules/@types/node/package.json"] = `{"name":"@types/node","types":"index.d.ts"}`
	files["/Users/x/src/node_modules/@types/node/index.d.ts"] = `declare var process: { env: { TMPDIR: string | undefined } }`
	files["/Users/x/src/main.vibe"] = `export const direct = (): string | undefined => process.env.TMPDIR
export const viaGlobal = (): string | undefined => globalThis.process?.env?.TMPDIR
export const viaGlobalPlain = (): string | undefined => globalThis.process.env.TMPDIR
export const viaBracket = (): string | undefined => globalThis["process"].env.TMPDIR
export const held = globalThis.process
export const local = (globalThis: { process: { env: { TMPDIR: string } } }): string => globalThis.process.env.TMPDIR`
	p := computedKeyProgram(t, files)
	count := 0
	for _, d := range p.GetSemanticDiagnostics(t.Context(), nil) {
		if d.Code() != EK4402.Code {
			continue
		}
		count++
		if !strings.Contains(diagnosticWithNotes(d), "env") || !strings.Contains(diagnosticWithNotes(d), "Config") {
			t.Errorf("wrong replacement: %s", diagnosticWithNotes(d))
		}
	}
	if count != 4 {
		t.Fatalf("got %d platform diagnostics, want 4", count)
	}
}
