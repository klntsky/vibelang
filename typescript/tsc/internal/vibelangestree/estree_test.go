package vibelangestree

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf16"
)

// The Node tests in packages/compiler/test/eslint hold the conversion to
// typescript-estree's and lint whole files; these pin the tree's contract
// without Node.

func parse(t *testing.T, fileName, text string) map[string]any {
	t.Helper()
	tree, failure := Parse(fileName, text)
	if failure != nil {
		t.Fatalf("Parse(%q): %v", text, failure)
	}
	var program map[string]any
	if err := json.Unmarshal([]byte(tree), &program); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, tree)
	}
	return program
}

func rangeOf(node map[string]any) (int, int) {
	r := node["range"].([]any)
	return int(r[0].(float64)), int(r[1].(float64))
}

// nodes returns every node of tree in document order.
func nodes(tree any) []map[string]any {
	var out []map[string]any
	var walk func(value any)
	walk = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if _, ok := v["type"].(string); ok {
				out = append(out, v)
			}
			for key, child := range v {
				if key != "tokens" && key != "comments" {
					walk(child)
				}
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(tree)
	return out
}

func utf16Slice(text string, start, end int) string {
	units := utf16.Encode([]rune(text))
	return string(utf16.Decode(units[start:end]))
}

var forms = map[string]string{
	"contracts":    "function read(id: string): string!Gone!Users { return Users.find(id) }\ntype Program = number!!\n",
	"declarations": "error Gone { id: string }\nerror Timeout\nservice Users { find(id: string): string!Gone }\ncontrol Ask { answer(): boolean!! }\nschema Ticket { id: string; mutable count: number }\nlayer Live: Users { find(id) { return id } }\n",
	"blocks":       "function f() { return withSpan(\"f\") { const value = do { 1 }; return value } }\n",
	"provide":      "function f() { provide Live, Clock = clock { return Users.find(\"1\") } }\n",
	"catch":        "function f() { const a = read() catch \"x\"; return read() catch (e) => match (e) { when Gone: 1; default: 2; } }\n",
	"pipes":        "function f(n: number) { return n |> % + 1 |> String(%) }\n",
	"patterns":     "function f(o: unknown) { if (o is Exit.Failure(let cause)) return cause; return 0 }\n",
	"primitives":   "function f() { uninterruptible { restore { return read() } } }\nfunction g() { try { return read() } finally (exit) { log(exit) } }\n",
	"fibers":       "function f(ids: string[]) { const t = fork read(); const all = await.all [read()]; for concurrent(2) (const id of ids) log(id); return join t }\n",
	"handle":       "function f() { try { return ask() } handle Ask.answer() { resume(true) } handle Ask.other(n) { resume(n) } }\n",
	"match":        "function f(x: number) { match (x) { when 1: log(\"one\"); default: log(\"other\"); } }\n",
	"hatches":      "function f(text: string) { return try((text) => JSON.parse(text), text) ?? throw new Gone({ id: text }) }\n",
}

func TestTokensAndCommentsAreTheSourceText(t *testing.T) {
	for name, text := range forms {
		t.Run(name, func(t *testing.T) {
			text := "// leading\n" + text + "/** trailing */\n"
			program := parse(t, "/x.vibe", text)
			var pieces []map[string]any
			for _, key := range []string{"tokens", "comments"} {
				for _, piece := range program[key].([]any) {
					pieces = append(pieces, piece.(map[string]any))
				}
			}
			covered := make([]bool, len(text))
			for _, piece := range pieces {
				start, end := rangeOf(piece)
				value := piece["value"].(string)
				source := text[start:end]
				switch piece["type"] {
				case "Line":
					value = "//" + value
				case "Block":
					value = "/*" + value + "*/"
				case "PrivateIdentifier":
					value = "#" + value
				}
				if source != value {
					t.Errorf("%s %q at %d is %q in the source", piece["type"], value, start, source)
				}
				for i := start; i < end; i++ {
					if covered[i] {
						t.Errorf("offset %d is in two tokens", i)
					}
					covered[i] = true
				}
			}
			for i, ch := range []byte(text) {
				if !covered[i] && !strings.ContainsRune(" \t\r\n", rune(ch)) {
					t.Errorf("%q at %d is in no token", ch, i)
				}
			}
			for _, node := range nodes(program) {
				if start, end := rangeOf(node); start < 0 || start > end || end > len(text) {
					t.Errorf("%s has range [%d, %d]", node["type"], start, end)
				}
			}
		})
	}
}

func TestVibeLangFormsHaveTheirOwnNodeTypes(t *testing.T) {
	want := map[string][]string{
		"contracts":    {"VibeEffectType"},
		"declarations": {"ClassDeclaration", "VariableDeclaration", "TSTypeLiteral", "PropertyDefinition"},
		"provide":      {"VibeProvideStatement", "VibeServiceProvider"},
		"catch":        {"VibeCatchExpression"},
		"pipes":        {"VibePipeExpression", "TopicReference"},
		"primitives":   {"VibeMaskStatement"},
	}
	for name, types := range want {
		seen := map[string]bool{}
		for _, node := range nodes(parse(t, "/x.vibe", forms[name])) {
			seen[node["type"].(string)] = true
		}
		for _, typ := range types {
			if !seen[typ] {
				t.Errorf("%s: no %s", name, typ)
			}
		}
	}
}

func TestSynthesizedNamesAreMarkedOrLeftOut(t *testing.T) {
	for name, text := range forms {
		for _, node := range nodes(parse(t, "/x.vibe", text)) {
			if node["type"] == "Identifier" && strings.HasPrefix(node["name"].(string), "$") && node["synthesized"] != true {
				t.Errorf("%s: %s is synthesized but not marked", name, node["name"])
			}
			if node["type"] == "CatchClause" && node["param"] != nil {
				if param := node["param"].(map[string]any); strings.HasPrefix(param["name"].(string), "$") {
					t.Errorf("%s: a catch clause binds %s", name, param["name"])
				}
			}
			if node["type"] == "LabeledStatement" {
				t.Errorf("%s: a synthesized label", name)
			}
			if node["type"] == "IfStatement" {
				if test := node["test"].(map[string]any); test["synthesized"] == true || test["type"] == "Literal" && test["value"] == true {
					t.Errorf("%s: an if statement tests a synthesized %v", name, test["name"])
				}
			}
		}
	}
}

func TestRangesCountUTF16CodeUnits(t *testing.T) {
	text := "const s = \"é😀\"; const after = 1\n"
	program := parse(t, "/x.vibe", text)
	for _, node := range nodes(program) {
		if node["type"] == "Identifier" && node["name"] == "after" {
			start, end := rangeOf(node)
			if got := utf16Slice(text, start, end); got != "after" {
				t.Errorf("after is at %q", got)
			}
			return
		}
	}
	t.Fatal("no identifier after")
}

func TestTokenTypesFollowTypescriptEstree(t *testing.T) {
	text := "const v = o.default + this.x >> 1 / 2; const r = /a\\/b/g; const s = `a${1}b`; const p = v |> %\n"
	tokens := map[string]string{}
	for _, token := range parse(t, "/x.vibe", text)["tokens"].([]any) {
		token := token.(map[string]any)
		tokens[token["value"].(string)] = token["type"].(string)
	}
	want := map[string]string{
		"default": "Identifier", "this": "Keyword", ">>": "Punctuator", "/a\\/b/g": "RegularExpression",
		"`a${": "Template", "}b`": "Template", "|>": "Punctuator", "%": "Punctuator", "const": "Keyword",
	}
	for value, typ := range want {
		if tokens[value] != typ {
			t.Errorf("token %q is %q, want %q", value, tokens[value], typ)
		}
	}
}

func TestASyntaxErrorHasItsLineAndColumn(t *testing.T) {
	_, failure := Parse("/x.vibe", "const a = 1\nconst é = (\n")
	if failure == nil {
		t.Fatal("no error")
	}
	if failure.Line != 3 || failure.Column != 1 {
		t.Errorf("error at %d:%d, want 3:1: %s", failure.Line, failure.Column, failure.Message)
	}
}

func TestLoneSurrogatesSurviveAsEscapes(t *testing.T) {
	tree, failure := Parse("/x.ts", `const s = "\ud800a"`)
	if failure != nil {
		t.Fatal(failure)
	}
	if !strings.Contains(tree, `"value":"\ud800a"`) {
		t.Errorf("the literal's value lost its surrogate: %s", tree)
	}
}
