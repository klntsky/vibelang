package effeck

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
)

// An @effeck-diagnostics comment changes the category of an adjustable EK
// code for part of a file (D241), as an @effect-diagnostics comment does for
// Effect's language service:
//
//	// @effeck-diagnostics EK4801:off EK4804:error    from here to the end of the file
//	// @effeck-diagnostics-next-line EK4801:off        on the next line only
//	// @effeck-diagnostics *:off                       every adjustable code
//
// The values are those of compilerOptions.effeck.diagnostics: "error",
// "warning", and "off". A later comment overrides an earlier one, and within
// one comment a code named on its own overrides *. A comment changes only
// the codes in adjustableCodes; every other EK diagnostic keeps its category.

const (
	directiveMarker   = "@effeck-diagnostics"
	directiveNextLine = "-next-line"
)

// directiveCategory returns the category that file's @effeck-diagnostics
// comments give m at pos, given the category and on that the project's
// options give it.
func directiveCategory(file *ast.SourceFile, pos int, m *Message, category diagnostics.Category, on bool) (diagnostics.Category, bool) {
	return directiveCategoryIn(file.Text(), pos, m, category, on)
}

// directiveCategoryIn is directiveCategory for a file's text.
func directiveCategoryIn(text string, pos int, m *Message, category diagnostics.Category, on bool) (diagnostics.Category, bool) {
	if !strings.Contains(text, directiveMarker) {
		return category, on
	}
	line := strings.Count(text[:pos], "\n")
	var named, wildcard string
	for offset := 0; offset < pos; {
		i := strings.Index(text[offset:pos], directiveMarker)
		if i < 0 {
			break
		}
		start := offset + i
		offset = start + len(directiveMarker)
		if !inComment(text, start) {
			continue
		}
		if strings.HasPrefix(text[offset:], directiveNextLine) {
			offset += len(directiveNextLine)
			if strings.Count(text[:start], "\n")+1 != line {
				continue
			}
		}
		end := strings.IndexAny(text[offset:], "\r\n")
		if end < 0 {
			end = len(text) - offset
		}
		var commentNamed, commentWildcard string
		for field := range strings.FieldsSeq(strings.TrimSuffix(strings.TrimSpace(text[offset:offset+end]), "*/")) {
			code, value, ok := strings.Cut(field, ":")
			switch {
			case !ok:
			case code == m.String():
				commentNamed = value
			case code == "*":
				commentWildcard = value
			}
		}
		if commentWildcard != "" {
			named, wildcard = "", commentWildcard // * sets every code, including those named before
		}
		if commentNamed != "" {
			named = commentNamed
		}
	}
	for _, value := range []string{named, wildcard} {
		if category, on, ok := categoryNamed(value, m); ok {
			return category, on
		}
	}
	return category, on
}

// inComment reports whether the text at start, on its line, follows only a
// comment's opening: //, /*, or the * of a block comment's later line.
func inComment(text string, start int) bool {
	lineStart := strings.LastIndexAny(text[:start], "\r\n") + 1
	before := strings.TrimSpace(text[lineStart:start])
	return before == "//" || before == "/*" || before == "/**" || before == "*"
}
