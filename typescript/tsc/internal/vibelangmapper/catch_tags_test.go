package vibelangmapper

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// D209/D218/D238: inferred catch failures follow the same exhaustive tag
// elimination as written contracts, including singleton Schema/Data errors.
func TestCatchTagExhaustiveStock(t *testing.T) {
	source, err := os.ReadFile("../../../vibelang/cases/catch-tag-exhaustive/main.vibe")
	if err != nil {
		t.Fatal(err)
	}
	declarations, output, err := catchTagStock(t, string(source))
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	// Stock TypeScript renames the Effect namespace import when a local name
	// collides (Effect_1.Effect); parity normalizes that alias the same way.
	declarations = strings.ReplaceAll(declarations, "Effect_1.", "Effect.")
	for _, want := range []string{
		"export declare const inferredSchema: () => Effect.Effect<number, never, never>;",
		"export declare function inferredData(): Effect.Effect<number, never, never>;",
		"export declare function inferredDecl(): Effect.Effect<number, never, never>;",
		"export declare function partial(data: boolean): Effect.Effect<number, DataFailure, never>;",
	} {
		if !strings.Contains(declarations, want) {
			t.Fatalf("missing %s\n%s", want, declarations)
		}
	}
}

func TestCatchTagPartialStock(t *testing.T) {
	source, err := os.ReadFile("../../../vibelang/cases/catch-tag-partial.fail/main.vibe")
	if err != nil {
		t.Fatal(err)
	}
	_, output, err := catchTagStock(t, string(source))
	if err == nil || !strings.Contains(output, "Second") {
		t.Fatalf("partial catch must retain Second: %v\n%s", err, output)
	}
}

// A literal arm cannot eliminate a failure whose own tag is a union/string;
// a value pattern cannot eliminate every failure sharing that value's type.
func TestCatchTagNonSingletonStock(t *testing.T) {
	source := `import { Effect } from "effect"
declare const tag: string
declare function wide(): Effect.Effect<never, { readonly _tag: string }, never>
declare function union(): Effect.Effect<never, { readonly _tag: "A" | "B" }, never>
export function broad() { try { return wide() } catch (e) { return match (e) { when { _tag: "A" }: 1; default: throw e; } } }
export function several() { try { return union() } catch (e) { return match (e) { when { _tag: "A" }: 1; default: throw e; } } }
export function value() { try { return wide() } catch (e) { return match (e) { when { _tag: tag }: 1; default: throw e; } } }
`
	declarations, output, err := catchTagStock(t, source)
	if err != nil {
		t.Fatalf("%v\n%s", err, output)
	}
	if strings.Count(declarations, "readonly _tag: string;") != 2 || !strings.Contains(declarations, `readonly _tag: "A" | "B";`) {
		t.Fatalf("non-singleton failures disappeared:\n%s", declarations)
	}
}

func catchTagStock(t *testing.T, source string) (string, string, error) {
	t.Helper()
	dir, stock := stockBuiltinProject(t)
	requireStockTypeScript(t, stock)
	projection := Project(source, testContext)
	if len(projection.Diagnostics) != 0 {
		t.Fatal(projection.Diagnostics)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.ts"), []byte(projection.Text), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(stock, "--ignoreConfig", "--declaration", "--emitDeclarationOnly", "--skipLibCheck", "--strict", "--target", "esnext", "--module", "nodenext", "main.ts")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	declarations, readErr := os.ReadFile(filepath.Join(dir, "main.d.ts"))
	if err == nil && readErr != nil {
		t.Fatal(readErr)
	}
	return string(declarations), string(output), err
}
