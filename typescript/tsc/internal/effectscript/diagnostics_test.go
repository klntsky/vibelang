package effectscript

import (
	"os"
	"regexp"
	"slices"
	"strconv"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
)

func TestCategory(t *testing.T) {
	t.Parallel()
	with := func(codes map[string]string) *core.CompilerOptions {
		return &core.CompilerOptions{EffectScript: &core.EffectScriptOptions{Diagnostics: codes}}
	}
	tests := []struct {
		name    string
		message *Message
		options *core.CompilerOptions
		want    diagnostics.Category
		on      bool
	}{
		{"ES2004 is a warning by default", ES2004, nil, diagnostics.CategoryWarning, true},
		{"ES4403 is an error by default", ES4403, &core.CompilerOptions{}, diagnostics.CategoryError, true},
		{"ES2004 can be an error", ES2004, with(map[string]string{"ES2004": "error"}), diagnostics.CategoryError, true},
		{"ES4403 can be a warning", ES4403, with(map[string]string{"ES4403": "warning"}), diagnostics.CategoryWarning, true},
		{"ES4403 can be off", ES4403, with(map[string]string{"ES4403": "off"}), 0, false},
		{"other codes stay errors", ES2001, with(map[string]string{"ES2001": "off"}), diagnostics.CategoryError, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, on := category(test.options, test.message)
			if got != test.want || on != test.on {
				t.Errorf("category(%s) = %v, %v; want %v, %v", test.message, got, on, test.want, test.on)
			}
		})
	}
}

// TestRegistryMatchesSpec checks that the registry lists exactly the codes in
// the table of Chapter 28 of the spec.
func TestRegistryMatchesSpec(t *testing.T) {
	t.Parallel()
	spec, err := os.ReadFile("../../../../docs/design/src/428-part4-ch28-tooling.md")
	if err != nil {
		t.Fatal(err)
	}
	var want []int32
	for _, m := range regexp.MustCompile(`(?m)^\| ES(\d{4}) \|`).FindAllStringSubmatch(string(spec), -1) {
		code, _ := strconv.Atoi(m[1])
		want = append(want, int32(code))
	}
	var got []int32
	for code := range registry {
		got = append(got, code)
	}
	slices.Sort(want)
	slices.Sort(got)
	if len(want) == 0 || !slices.Equal(got, want) {
		t.Errorf("registry codes %v differ from Chapter 28's %v", got, want)
	}
}
