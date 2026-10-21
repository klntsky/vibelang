package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/microsoft/TypeScript/tsc/internal/vibelangestree"
)

// runESTree serves `vibelang --estree <file>`, which VibeLang's ESLint parser
// (packages/compiler/eslint.mjs) runs once per file: it reads the file's
// text from stdin, since an editor lints unsaved text, and writes one JSON
// object to stdout, {"ast": Program} or {"error": {message, index,
// lineNumber, column}}. <file> names the file; a name ending in .vibe is
// VibeLang. The exit status is 0 for both answers and 2 for a usage error.
func runESTree(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: vibelang --estree <file>  (the file's text on stdin)")
		return 2
	}
	text, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "vibelang --estree:", err)
		return 2
	}
	tree, failure := vibelangestree.Parse(args[0], string(text))
	if failure != nil {
		encoded, _ := json.Marshal(map[string]any{"error": failure})
		os.Stdout.Write(encoded)
		return 0
	}
	os.Stdout.WriteString(`{"ast":`)
	os.Stdout.WriteString(tree)
	os.Stdout.WriteString("}")
	return 0
}
