package vibelang

import (
	"regexp"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
)

// How hovers, messages, and fixes write Effect types.
//
// .vibe code writes an Effect type as TypeScript does, Effect<A, E, R>, and a
// type argument left out means what it means in TypeScript: Effect<A> is
// Effect<A, never, never> and Effect<A, E> is Effect<A, E, never> (D308).
// Hovers and messages about .vibe code write every Effect type in that short
// form, under the prelude's name: where TypeScript prints
// Effect.Effect<string, never, never>, a hover shows Effect<string>.
// Declaration files keep the types TypeScript writes, so TypeScript callers
// are unaffected.

func init() {
	ls.RegisterQuickInfoTextHook(effectQuickInfo)
}

// effectQuickInfo writes the Effect types of a hover on .vibe code in the
// short form.
func effectQuickInfo(c *checker.Checker, node *ast.Node, text string) string {
	if !isVibeLangFile(ast.GetSourceFileOfNode(node)) {
		return text
	}
	return effectTypes(text)
}

// effectText writes an Effect type from the texts of its channels, A, E, and
// R, in TypeScript's short form: each trailing channel that is never, empty,
// or not given is left out, as the type's defaults allow, so ("string",
// "never", "never") is Effect<string>. An empty channel before a written one
// is never. The success channel is always written.
func effectText(channels ...string) string {
	return namedEffectText("Effect", channels...)
}

// namedEffectText is effectText for code that a fix writes where name names
// effect's Effect type: Effect, the prelude's, or Effect.Effect in a file
// whose import of the Effect namespace hides the prelude's type
// (effectTypeName).
func namedEffectText(name string, channels ...string) string {
	slots := make([]string, 3)
	for i := range slots {
		slots[i] = "never"
		if i < len(channels) && channels[i] != "" {
			slots[i] = channels[i]
		}
	}
	n := len(slots)
	for n > 1 && slots[n-1] == "never" {
		n--
	}
	return name + "<" + strings.Join(slots[:n], ", ") + ">"
}

// effectTypeName returns how code at node names effect's Effect type:
// Effect, the prelude's type, unless the file binds the name Effect itself to
// something else; then Effect.Effect, through the prelude's or the imported
// Effect namespace. import { Effect } from "effect" is such a binding: it
// names the namespace alone, and Effect<A> there is TypeScript's TS2709. An
// import of effect's Effect type, as from effect/Effect, keeps Effect. It
// reads the file's own bindings, so it answers the same before the file
// uses the prelude, which loads only for a file that names it.
func effectTypeName(c *checker.Checker, node *ast.Node) string {
	symbol := c.ResolveName("Effect", node, ast.SymbolFlagsValue|ast.SymbolFlagsType|ast.SymbolFlagsNamespace, false /*excludeGlobals*/)
	if symbol == nil || !core.Some(symbol.Declarations(), func(d *ast.Node) bool { return ast.GetSourceFileOfNode(d) == ast.GetSourceFileOfNode(node) }) {
		return "Effect" // the prelude's, or nothing the file declares
	}
	target := symbol
	if target.Flags()&ast.SymbolFlagsAlias != 0 {
		target = c.GetAliasedSymbol(target)
	}
	switch {
	case target.Flags()&(ast.SymbolFlagsTypeAlias|ast.SymbolFlagsInterface) != 0:
		if isEffectType(c, c.GetDeclaredTypeOfSymbol(target)) {
			return "Effect"
		}
	case target.Flags()&(ast.SymbolFlagsType|ast.SymbolFlagsModule) == 0:
		return "Effect" // a value: the type Effect is still the prelude's
	}
	return "Effect.Effect"
}

// hasTopLevel reports whether text contains token outside brackets and string
// literals.
func hasTopLevel(text string, token string) bool {
	depth := 0
	for i := 0; i < len(text); i++ {
		switch ch := text[i]; {
		case strings.HasPrefix(text[i:], "=>"):
			if token == "=>" && depth == 0 {
				return true
			}
			i++
		case ch == '"' || ch == '\'' || ch == '`':
			i = skipString(text, i)
		case ch == '(' || ch == '[' || ch == '{' || ch == '<':
			depth++
		case ch == ')' || ch == ']' || ch == '}' || ch == '>':
			depth--
		case depth == 0 && strings.HasPrefix(text[i:], token):
			return true
		}
	}
	return false
}

// skipString returns the index of the quote that closes the string literal
// that starts at text[start].
func skipString(text string, start int) int {
	quote := text[start]
	for i := start + 1; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
		case quote:
			return i
		}
	}
	return len(text) - 1
}

// effectStart matches the start of an Effect type in type text: Effect<,
// Effect.Effect<, or an import type of effect/Effect's Effect.
var effectStart = regexp.MustCompile(`(?:import\("[^"]*"(?:, \{[^()]*\})?\)\.)?(?:Effect\.)?Effect<`)

// effectTypes rewrites every Effect type in text, a type or a declaration
// that TypeScript printed, into the short form that effectText writes,
// innermost first.
func effectTypes(text string) string {
	return namedEffectTypes("Effect", text)
}

// namedEffectTypes is effectTypes for code that a fix writes, under name
// (effectTypeName).
func namedEffectTypes(name string, text string) string {
	var out strings.Builder
	rest := text
	for {
		loc := effectStart.FindStringIndex(rest)
		if loc == nil {
			out.WriteString(rest)
			return out.String()
		}
		start, open := loc[0], loc[1]
		if start > 0 && isIdentifierChar(rest[start-1]) {
			out.WriteString(rest[:open])
			rest = rest[open:]
			continue
		}
		close, arguments := typeArguments(rest, open)
		if close < 0 || len(arguments) == 0 || len(arguments) > 3 {
			out.WriteString(rest[:open])
			rest = rest[open:]
			continue
		}
		for i, argument := range arguments {
			arguments[i] = namedEffectTypes(name, argument)
		}
		out.WriteString(rest[:start])
		out.WriteString(namedEffectText(name, arguments...))
		rest = rest[close+1:]
	}
}

// typeArguments reads the type arguments that start after the < at
// text[open-1]. It returns the index of the closing > and each argument's
// text, or -1 when the arguments do not close.
func typeArguments(text string, open int) (int, []string) {
	depth := 0
	var arguments []string
	last := open
	for i := open; i < len(text); i++ {
		switch ch := text[i]; {
		case strings.HasPrefix(text[i:], "=>"):
			i++
		case ch == '"' || ch == '\'' || ch == '`':
			i = skipString(text, i)
		case ch == '(' || ch == '[' || ch == '{' || ch == '<':
			depth++
		case ch == ')' || ch == ']' || ch == '}':
			depth--
		case ch == '>':
			if depth == 0 {
				return i, append(arguments, strings.TrimSpace(text[last:i]))
			}
			depth--
		case ch == ',' && depth == 0:
			arguments = append(arguments, strings.TrimSpace(text[last:i]))
			last = i + 1
		}
	}
	return -1, nil
}

func isIdentifierChar(ch byte) bool {
	return ch == '_' || ch == '$' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9'
}
