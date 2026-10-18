// The runtime harness's emit entry: the production --emit command has no
// mapper-entry options. run.sh overlays this file on its main.go to build
// a test-only entry that passes D34 tracing to the same mapper Emit API.
// No projector or command source is copied or changed by the adapter.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/microsoft/TypeScript/tsc/internal/vibelangmapper"
)

// main emits with explicit file context, so the runtime column exercises
// the project's tracing option as the mapper server's openProject does.
func main() {
	if len(os.Args) < 3 || os.Args[1] != "--emit" {
		fail("usage: --emit file [--out file.js] [--tracing all|exported]")
	}
	file, out, tracing := os.Args[2], "", ""
	for i := 3; i < len(os.Args); i += 2 {
		if i+1 == len(os.Args) {
			fail("missing option value")
		}
		switch os.Args[i] {
		case "--out":
			out = os.Args[i+1]
		case "--tracing":
			tracing = os.Args[i+1]
		default:
			fail("unknown option: " + os.Args[i])
		}
	}
	if tracing != "" && tracing != "all" && tracing != "exported" {
		fail("invalid tracing: " + tracing)
	}
	text, err := os.ReadFile(file)
	if err != nil {
		fail(err.Error())
	}
	absolute, err := filepath.Abs(file)
	if err != nil {
		fail(err.Error())
	}
	ctx := vibelangmapper.FileContextOf(filepath.ToSlash(absolute))
	ctx.Tracing = tracing
	js, sourceMap, diagnostics := vibelangmapper.Emit(string(text), ctx)
	for _, d := range diagnostics {
		fmt.Fprintf(os.Stderr, "%s: error EK%d: %s\n", file, d.Code, d.Message)
	}
	if out == "" {
		fmt.Print(js)
		return
	}
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		fail(err.Error())
	}
	if err := os.WriteFile(out, []byte(js+"//# sourceMappingURL="+filepath.Base(out)+".map\n"), 0644); err != nil {
		fail(err.Error())
	}
	if err := os.WriteFile(out+".map", []byte(sourceMap), 0644); err != nil {
		fail(err.Error())
	}
}

// fail reports adapter errors with a nonzero exit, rather than hiding a
// missing option or unreadable input as a runtime mismatch.
func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(2) }
