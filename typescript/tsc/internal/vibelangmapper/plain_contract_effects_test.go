package vibelangmapper

import (
	"strings"
	"testing"
)

// D263 retains EK2003 for actual Effect calls under a plain return contract.
func TestPlainContractEffectGreet(t *testing.T) {
	source := "import { Console } from \"effect\"; export function greet(name: string): number { Console.log(`hello, ${name}`); return name.length }"
	assertPlainContractDiagnostic(t, source, true)
}

// D263 makes a throw under a plain annotation an ordinary JavaScript throw.
func TestPlainContractThrowHalf(t *testing.T) {
	source := `export const half = (n: number): number => { if (n < 0) throw "negative"; return n / 2 }`
	assertPlainContractDiagnostic(t, source, false)
	for _, projection := range []Projection{Project(source, testContext), Executable(source, testContext)} {
		if !strings.Contains(projection.Text, `throw "negative"`) {
			t.Fatal(projection.Text)
		}
	}
}

func TestPlainContractEffectArrow(t *testing.T) {
	assertPlainContractDiagnostic(t, `export const invalid = (): number => { Effect.succeed(1); return 1 }`, true)
}

func TestPlainContractEffectInferred(t *testing.T) {
	for _, source := range []string{
		`export const invalid: () => number = () => { Effect.succeed(1); return 1 }`,
		`export const invalid: { value(): number } = { value() { Effect.succeed(1); return 1 } }`,
	} {
		assertPlainContractDiagnostic(t, source, true)
	}
}

func TestPlainContractEffectOperations(t *testing.T) {
	for _, source := range []string{
		`export function invalid(): number { log("hello"); return 1 }`,
		`abstract class Names { abstract lookup(): Effect<string, never, never> }; export function invalid(): number { Names.lookup(); return 1 }`,
		`export function invalid(): number { if (true) throw "negative"; Effect.succeed(1); return 1 }`,
		`export class Example { invalid(): number { Effect.succeed(1); return 1 } }`,
	} {
		assertPlainContractDiagnostic(t, source, true)
	}
}

// Uncalled nested functions and synchronous calls do not run an Effect here.
func TestPlainContractEffectBoundaries(t *testing.T) {
	for _, source := range []string{
		`export function valid(): number { function later() { Effect.log("later"); return 1 }; return 1 }`,
		`export const valid: () => number = () => { throw "negative" }`,
		`export function valid(n: number): number { return Math.max(n, 1) }`,
		`export function valid(): boolean { return Effect.isEffect(1) }`,
		`export function valid(): number { const log = (s: string): number => s.length; return log("hello") }`,
	} {
		assertPlainContractDiagnostic(t, source, false)
	}
}

// D265's contextual fields store programs even inside a plain factory.
// Only the initializer call is cold; calls in its operands still run.
func TestPlainContractContextualProgramFields(t *testing.T) {
	for _, source := range []string{
		`interface Shape { read: Effect.Effect<number, never, never> }; const make = (): Shape => ({ read: Effect.sync(() => 1) })`,
		`interface Shape { read: Effect.Effect<number, never, never> }; function make(): Shape { return { read: (Effect.succeed(1)) } }`,
		`interface Shape { read: Effect.Effect<number, never, never> }; function make(): Shape { const stored: Shape = { read: Effect.succeed(1) }; return stored }`,
	} {
		assertPlainContractDiagnostic(t, source, false)
	}
	for _, source := range []string{
		`interface Shape { read: Effect.Effect<number, never, never> }; function make(): Shape { return { read: Effect.succeed(Effect.succeed(1)) } }`,
		`interface Shape { read: number }; function make(): Shape { return { read: Effect.succeed(1) } }`,
	} {
		assertPlainContractDiagnostic(t, source, true)
	}
}

func assertPlainContractDiagnostic(t *testing.T, source string, want bool) {
	t.Helper()
	for _, projection := range []Projection{Project(source, testContext), Executable(source, testContext)} {
		count := 0
		for _, diagnostic := range projection.Diagnostics {
			if diagnostic.Code != 2003 {
				t.Fatalf("unexpected diagnostic: %+v\n%s", diagnostic, source)
			}
			count++
			if !strings.Contains(diagnostic.Message, "plain") {
				t.Fatal(diagnostic)
			}
			if diagnostic.Start < 0 || diagnostic.Length <= 0 || diagnostic.Start+diagnostic.Length > len(source) {
				t.Fatal(diagnostic)
			}
		}
		if (count > 0) != want || count > 1 {
			t.Fatalf("diagnostics=%v, want EK2003=%v\n%s", projection.Diagnostics, want, source)
		}
	}
}
