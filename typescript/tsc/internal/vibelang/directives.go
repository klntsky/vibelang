package vibelang

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
)

// An @vibelang-diagnostics comment changes the category of an adjustable EK
// code for part of a file (D241), as an @effect-diagnostics comment does for
// Effect's language service:
//
//	// @vibelang-diagnostics EK4801:off EK4804:error    from here to the end of the file
//	// @vibelang-diagnostics-next-line EK4801:off        on the next line only
//	// @vibelang-diagnostics *:off                       every adjustable code
//
// The values are those of compilerOptions.vibelang.diagnostics: "error",
// "warning", and "off". A later comment overrides an earlier one, and within
// one comment a code named on its own overrides *. Words after the codes are
// prose, and punctuation ends a category, so a comment can give its reason:
// "EK4801:off: the program is returned". A comment changes only the codes in
// adjustableCodes; every other EK diagnostic keeps its category.

const (
	directiveMarker   = "@vibelang-diagnostics"
	directiveNextLine = "-next-line"
)

// directiveCategory returns the category that file's @vibelang-diagnostics
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
			// Punctuation ends a category, so a reason can follow it as prose:
			// EK4801:off: why, EK4801:off, EK4818:error.
			value = strings.TrimRight(value, ":;,.")
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
