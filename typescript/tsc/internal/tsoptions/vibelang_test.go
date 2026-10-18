package tsoptions_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/json"
	"github.com/microsoft/TypeScript/tsc/internal/tsoptions"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/vfstest"
)

func vibeConfig(t *testing.T, text string, base string) *tsoptions.ParsedCommandLine {
	t.Helper()
	fs := vfstest.FromMap(map[string]string{
		"/project/tsconfig.json": text,
		"/project/base.json":     base,
		"/project/main.vibe":     "export const value = 1",
	}, tspath.CaseSensitive)
	config, errors := tsoptions.GetParsedCommandLineOfConfigFile("/project/tsconfig.json", &core.CompilerOptions{}, nil, fs, nil)
	if len(errors) != 0 {
		t.Fatalf("config read failed: %v", errors)
	}
	return config
}

// D34 and D241 permit only the listed policies. --showConfig must preserve
// every accepted policy and tracing mode so it can be used as a new config.
func TestVibeLangConfigRoundTrip(t *testing.T) {
	for _, tracing := range []string{"none", "exported", "all"} {
		for _, category := range []string{"error", "warning", "off"} {
			t.Run(tracing+"/"+category, func(t *testing.T) {
				var policies []string
				want := &core.VibeLangOptions{Tracing: tracing, Diagnostics: map[string]string{}}
				for _, code := range core.VibeLangAdjustableCodes {
					policies = append(policies, fmt.Sprintf("%q:%q", code, category))
					want.Diagnostics[code] = category
				}
				config := vibeConfig(t, `{"compilerOptions":{"vibelang":{"tracing":"`+tracing+`","diagnostics":{`+strings.Join(policies, ",")+`}}},"files":["main.vibe"]}`, "{}")
				if len(config.Errors) != 0 || !reflect.DeepEqual(config.CompilerOptions().VibeLang, want) {
					t.Fatalf("accepted settings changed: %+v; errors %v", config.CompilerOptions().VibeLang, config.Errors)
				}
				shown, err := json.Marshal(tsoptions.ConvertToTSConfig(config, "/project/tsconfig.json"))
				if err != nil {
					t.Fatal(err)
				}
				again := vibeConfig(t, string(shown), "{}")
				if len(again.Errors) != 0 || !reflect.DeepEqual(again.CompilerOptions().VibeLang, want) {
					t.Fatalf("showConfig did not round-trip: %s; errors %v", shown, again.Errors)
				}
				// Stable lexical order is independent of the input/map order.
				if strings.Index(string(shown), `"EK2004"`) > strings.Index(string(shown), `"EK9100"`) {
					t.Fatalf("diagnostic policies are not sorted: %s", shown)
				}
			})
		}
	}
	config := vibeConfig(t, `{"compilerOptions":{"vibelang":{}},"files":["main.vibe"]}`, "{}")
	shown, err := json.Marshal(tsoptions.ConvertToTSConfig(config, "/project/tsconfig.json"))
	if err != nil || !strings.Contains(string(shown), `"vibelang":{}`) {
		t.Fatalf("empty settings should not invent explicit defaults: %s, %v", shown, err)
	}
}

func TestVibeLangConfigDiagnosticLocations(t *testing.T) {
	for _, test := range []struct {
		name, value, span, message string
		code                       int32
	}{
		{"unknown setting", `{"trace":"all"}`, `"trace"`, "vibelang.trace", 5023},
		{"soundness diagnostic", `{"diagnostics":{"EK2001":"off"}}`, `"EK2001"`, "vibelang.diagnostics.EK2001", 5023},
		{"TypeScript diagnostic", `{"diagnostics":{"EK4813":"off"}}`, `"EK4813"`, "vibelang.diagnostics.EK4813", 5023},
		{"category spelling", `{"diagnostics":{"EK2004":"Warning"}}`, `"Warning"`, "'error', 'warning', 'off'", 6046},
		{"category type", `{"diagnostics":{"EK4403":false}}`, `false`, "vibelang.diagnostics.EK4403", 6046},
		{"diagnostics type", `{"diagnostics":[]}`, `[]`, "vibelang.diagnostics", 5024},
		{"diagnostics null", `{"diagnostics":null}`, `null`, "vibelang.diagnostics", 5024},
		{"tracing spelling", `{"tracing":"ALL"}`, `"ALL"`, "'none', 'exported', 'all'", 6046},
		{"tracing type", `{"tracing":42}`, `42`, "vibelang.tracing", 6046},
	} {
		t.Run(test.name, func(t *testing.T) {
			// The emoji before the options also exercises byte-based AST spans.
			text := "{\n// 😀 config\n\"compilerOptions\":{\"vibelang\":" + test.value + "},\"files\":[\"main.vibe\"]}"
			config := vibeConfig(t, text, "{}")
			if len(config.Errors) != 1 {
				t.Fatalf("want one error, got %v", config.Errors)
			}
			d := config.Errors[0]
			if d.Code() != test.code || d.File() == nil || text[d.Pos():d.End()] != test.span || !strings.Contains(d.String(), test.message) {
				t.Fatalf("wrong config diagnostic: %s at %d:%d", d.String(), d.Pos(), d.End())
			}
		})
	}
}

func TestVibeLangConfigInheritance(t *testing.T) {
	base := `{"compilerOptions":{"vibelang":{"tracing":"all","diagnostics":{"EK4403":"off"}}},"files":["main.vibe"]}`
	for _, test := range []struct {
		name, override string
		want           *core.VibeLangOptions
	}{
		{"inherit", "", &core.VibeLangOptions{Tracing: "all", Diagnostics: map[string]string{"EK4403": "off"}}},
		{"replace", `,"compilerOptions":{"vibelang":{"tracing":"exported"}}`, &core.VibeLangOptions{Tracing: "exported"}},
		{"empty object replaces", `,"compilerOptions":{"vibelang":{}}`, &core.VibeLangOptions{}},
		{"null resets", `,"compilerOptions":{"vibelang":null}`, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			config := vibeConfig(t, `{"extends":"./base.json"`+test.override+`}`, base)
			if len(config.Errors) != 0 || !reflect.DeepEqual(config.CompilerOptions().VibeLang, test.want) {
				t.Fatalf("inherited settings: %+v; want %+v; errors %v", config.CompilerOptions().VibeLang, test.want, config.Errors)
			}
		})
	}
}
