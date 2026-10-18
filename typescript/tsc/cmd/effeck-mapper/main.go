// Command effeck-mapper is Effeck's TypeScript 7 content mapper. TypeScript
// starts it from the effeck-ts-mapper package's manifest and talks to it over
// stdio; see internal/effeckmapper.
//
//	effeck-mapper                    serve the content mapper protocol on stdio
//	effeck-mapper --print f.effeck   print f's virtual TypeScript, its segments and diagnostics
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/microsoft/TypeScript/tsc/internal/effeckmapper"
)

// main serves the protocol, or prints one projection with --print.
func main() {
	if len(os.Args) == 3 && os.Args[1] == "--print" {
		os.Exit(printProjection(os.Args[2]))
	}
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: effeck-mapper [--print file.effeck]")
		os.Exit(2)
	}
	if err := effeckmapper.Serve(context.Background(), stdio{}); err != nil {
		fmt.Fprintln(os.Stderr, "effeck-mapper:", err)
		os.Exit(1)
	}
}

// printProjection writes the projection of the file at path to stdout, for
// debugging the projector without TypeScript.
func printProjection(path string) int {
	text, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "effeck-mapper:", err)
		return 1
	}
	projection := effeckmapper.Project(string(text))
	fmt.Print(projection.Text)
	fmt.Println("\n// --- segments [virtualStart, virtualEnd) <- [originalStart, originalEnd)")
	for _, s := range projection.Segments {
		fmt.Printf("// [%d, %d) <- [%d, %d)\n", s.VirtualStart, s.VirtualEnd, s.OriginalStart, s.OriginalEnd)
	}
	for _, d := range projection.Diagnostics {
		fmt.Printf("// %s%d at [%d, %d): %s\n", effeckmapper.DiagnosticSource, d.Code, d.Start, d.Start+d.Length, d.Message)
	}
	return 0
}

// stdio joins the process's stdin and stdout into the connection the
// protocol runs on.
type stdio struct{}

// Read reads from stdin.
func (stdio) Read(p []byte) (int, error) { return os.Stdin.Read(p) }

// Write writes to stdout.
func (stdio) Write(p []byte) (int, error) { return os.Stdout.Write(p) }

// Close closes stdin, which ends the read loop.
func (stdio) Close() error { return os.Stdin.Close() }
