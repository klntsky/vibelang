package vibelang

import "testing"

func TestEntryMainExplicitRunner(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, setup string
		automatic   bool
	}{
		{"standalone", `runPromise(main())`, false},
		{"initializer", `const pending = runPromise(main())`, false},
		{"exported initializer", `export const pending = runPromise(main())`, false},
		{"nested expression", `const started = { pending: Effect.runPromise(main()) }`, false},
		{"static field", `class Started { static pending = runPromise(main()) }`, false},
		{"static block", `class Started { static { runPromise(main()) } }`, false},
		{"delayed function", `function start() { runPromise(main()) }`, true},
		{"delayed arrow", `const start = () => runPromise(main())`, true},
		{"instance field", `class Start { pending = runPromise(main()) }`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := computedKeyFiles(false)
			files["/Users/x/src/main.vibe"] = "export function main() { return 1 }\n" + test.setup
			program := computedKeyProgram(t, files)
			file := program.GetSourceFile("/Users/x/src/main.vibe")
			if got := entryMain(file) != nil; got != test.automatic {
				t.Errorf("automatic main = %v, want %v for %s", got, test.automatic, test.setup)
			}
		})
	}
}
