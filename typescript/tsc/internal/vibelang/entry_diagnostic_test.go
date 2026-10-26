package vibelang

import "testing"

func TestUnusedMainDiagnostic(t *testing.T) {
	for _, test := range []struct {
		name, source string
		want         int
	}{
		{"unused", "function main() { return 1 }", 1},
		{"exported", "export function main() { return 1 }", 0},
		{"referenced", "function main() { return 1 }; export function start() { return main() }", 0},
		{"export list", "function main() { return 1 }; export { main }", 0},
		{"shadow", "function main() { return 1 }; export function start(main: () => number) { return main() }", 1},
		{"parameters", "function main(value: number) { return value }", 0},
		{"off", "// @vibelang-diagnostics EK1016:off\nfunction main() { return 1 }", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			program, file, _, _ := editorProgram(t, test.source)
			got := 0
			for _, d := range program.GetSemanticDiagnostics(t.Context(), file) {
				if d.Source() == diagnosticSource && d.Code() == EK1016.Code {
					got++
				}
			}
			if got != test.want {
				t.Fatalf("got %d unused-main diagnostics, want %d", got, test.want)
			}
		})
	}
}
